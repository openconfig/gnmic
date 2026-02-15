package outputs_manager

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/cache"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zestor-dev/zestor/store"
	"google.golang.org/protobuf/proto"
)

type ManagedOutput struct {
	sync.RWMutex
	Name string
	Impl outputs.Output
	Cfg  map[string]any
}

// OutputsManager runs outputs.
type OutputsManager struct {
	ctx   context.Context
	store *collstore.Store

	OutputsFactory map[string]outputs.Initializer
	in             <-chan *pipeline.Msg // pipe from targets and/or inputs

	mu              sync.RWMutex
	outputs         map[string]*ManagedOutput
	processorsInUse map[string]map[string]struct{} // processor name -> output names

	cache  cache.Cache
	logger *slog.Logger
	reg    *prometheus.Registry
	stats  *outputStats
}

type outputStats struct {
	msgCount    *prometheus.CounterVec
	msgCountErr *prometheus.CounterVec
}

func NewOutputsManager(ctx context.Context, store *collstore.Store, pipe <-chan *pipeline.Msg, reg *prometheus.Registry) *OutputsManager {
	return &OutputsManager{
		ctx:             ctx,
		store:           store,
		OutputsFactory:  outputs.Outputs,
		in:              pipe,
		outputs:         map[string]*ManagedOutput{},
		processorsInUse: make(map[string]map[string]struct{}),
		stats:           newOutputStats(),
		reg:             reg,
	}
}

func (mgr *OutputsManager) Start(cache cache.Cache, wg *sync.WaitGroup) error {
	mgr.logger = logging.NewLogger(mgr.store.Config, "component", "outputs-manager")
	mgr.logger.Info("starting outputs manager")
	mgr.cache = cache
	// register metrics
	mgr.registerMetrics()
	// watch outputs config changes
	outputCh, outputsCancel, err := mgr.store.Config.Watch("outputs",
		store.WithInitialReplay[any]())
	if err != nil {
		return err
	}
	// watch processors config changes (update only)
	procsCh, processorsCancel, err := mgr.store.Config.Watch("processors",
		store.WithEventTypes[any](store.EventTypeUpdate))
	if err != nil {
		return err
	}

	wg.Add(1)
	// forward incoming events to all running outputs
	// that are in the list of outputs to write to.
	go mgr.writeLoop(wg)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer outputsCancel()
		defer processorsCancel()
		for {
			select {
			case <-mgr.ctx.Done():
				return
			case ev, ok := <-outputCh:
				if !ok {
					return
				}
				mgr.logger.Info("got output event", "event", ev)
				cfg, ok := ev.Object.(map[string]any)
				if !ok {
					mgr.logger.Error("invalid output config", "event", ev)
					continue
				}
				switch ev.EventType {
				case store.EventTypeCreate:
					mgr.createOutput(ev.Name, cfg)
				case store.EventTypeUpdate:
					mgr.updateOutput(ev.Name, cfg)
				case store.EventTypeDelete:
					mgr.DeleteOutput(ev.Name)
				}
			case ev, ok := <-procsCh:
				if !ok {
					return
				}
				mgr.logger.Info("got processor event", "event", ev)
				cfg, ok := ev.Object.(map[string]any)
				if !ok {
					mgr.logger.Error("invalid processor config", "event", ev)
					continue
				}
				switch ev.EventType {
				case store.EventTypeUpdate:
					mgr.updateProcessor(ev.Name, cfg)
				}
			}
		}
	}()

	return nil
}

func (mgr *OutputsManager) writeLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-mgr.ctx.Done():
			return
		case e, ok := <-mgr.in:
			if !ok {
				mgr.logger.Debug("pipeline channel closed")
				return
			}
			mgr.logger.Debug("got pipeline message", "message", e) // Debug
			go mgr.write(e)
			if mgr.cache != nil {
				go mgr.cache.Write(mgr.ctx, e.Meta["subscription-name"], e.Msg)
			}
		}
	}
}

func (mgr *OutputsManager) write(e *pipeline.Msg) {
	outs := mgr.getOutputsForTarget(e.Outputs)
	outsNames := make([]string, 0, len(outs))
	if mgr.logger.Enabled(mgr.ctx, slog.LevelDebug) {
		for _, o := range outs {
			outsNames = append(outsNames, o.Name)
		}
		mgr.logger.Debug("writing msg to outputs", "outputs", outsNames)
	}
	for _, mo := range outs {
		mgr.stats.msgCount.WithLabelValues(mo.Name).Inc()
		if len(e.Events) > 0 { // from inputs
			for _, ev := range e.Events {
				mo.Impl.WriteEvent(mgr.ctx, ev)
			}
		} else {
			// from targets or inputs
			mo.Impl.Write(mgr.ctx, e.Msg, e.Meta)
		}
	}
}

func (mgr *OutputsManager) updateProcessor(name string, cfg map[string]any) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for _, mo := range mgr.outputs {
		err := mo.updateProcessor(name, cfg)
		if err != nil {
			mgr.logger.Error("failed to update event processor for output", "processorName", name, "outputName", mo.Name, "error", err)
		}
	}
}

func (mo *ManagedOutput) updateProcessor(name string, cfg map[string]any) error {
	mo.Lock()
	defer mo.Unlock()
	return mo.Impl.UpdateProcessor(name, cfg)
}

func (mgr *OutputsManager) getOutputsForTarget(outputs map[string]struct{}) []*ManagedOutput {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	// all outputs
	if len(outputs) == 0 {
		outs := make([]*ManagedOutput, 0, len(mgr.outputs))
		for _, mo := range mgr.outputs {
			if mgr.getOutputStateStr(mo.Name) == collstore.StateRunning {
				outs = append(outs, mo)
			}
		}
		return outs
	}
	// specific outputs per target
	outs := make([]*ManagedOutput, 0, len(outputs))
	for name, mo := range mgr.outputs {
		if _, ok := outputs[name]; !ok || mgr.getOutputStateStr(name) != collstore.StateRunning {
			continue
		}
		outs = append(outs, mo)
	}
	return outs
}

func (mgr *OutputsManager) createOutput(name string, cfg map[string]any) {
	typ, _ := cfg["type"].(string)
	f := mgr.OutputsFactory[typ]
	if f == nil {
		mgr.logger.Error("unknown output type", "name", name, "type", typ)
		mgr.setOutputState(name, collstore.StateFailed, fmt.Sprintf("unknown output type: %s", typ))
		return
	}
	impl := f()

	opts := make([]outputs.Option, 0, 2)
	opts = append(opts,
		outputs.WithName(name),
		outputs.WithConfigStore(mgr.store.Config),
		outputs.WithLogger(log.New(os.Stdout, "", log.LstdFlags)), // temporary logger
	)

	clustering, ok, err := mgr.store.Config.Get("global", "clustering")
	if err != nil {
		mgr.logger.Error("failed to get clustering for output", "name", name, "error", err)
		return
	}
	if ok {
		clus, ok := clustering.(map[string]any)
		if cname, cOk := clus["cluster-name"].(string); cOk && ok {
			opts = append(opts, outputs.WithClusterName(cname))
		}
	}

	err = impl.Init(mgr.ctx, name, cfg, opts...)
	if err != nil {
		mgr.logger.Error("failed to init output", "name", name, "error", err)
		mgr.setOutputState(name, collstore.StateFailed, err.Error())
		return
	}
	procs := extractProcessors(cfg)
	mo := &ManagedOutput{Name: name, Impl: impl, Cfg: cfg}
	mgr.mu.Lock()
	mgr.trackProcessorsInUse(name, procs)
	mgr.outputs[name] = mo
	mgr.mu.Unlock()
	mgr.setOutputState(name, collstore.StateRunning, "")
}

func (mgr *OutputsManager) updateOutput(name string, cfg map[string]any) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mo, ok := mgr.outputs[name]
	if !ok {
		mgr.createOutput(name, cfg)
		return
	}

	mgr.logger.Info("updating output", "name", name, "cfg", cfg)
	mo.Lock()
	defer mo.Unlock()
	err := mo.Impl.Update(mgr.ctx, cfg)
	if err != nil {
		mgr.logger.Error("failed to update output", "name", name, "error", err)
		return
	}
	oldProcs := extractProcessors(mo.Cfg)
	newProcs := extractProcessors(cfg)
	mgr.logger.Info("tracking output processors in use", "name", name, "oldProcs", oldProcs, "newProcs", newProcs)
	mgr.untrackProcessorsInUse(name, oldProcs)
	mgr.trackProcessorsInUse(name, newProcs)
	mgr.logger.Info("updated output", "name", name, "cfg", cfg)
	mo.Cfg = cfg
	mgr.outputs[name] = mo
}

func (mgr *OutputsManager) StopOutput(name string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.logger.Info("finding output", "name", name)
	if mo, ok := mgr.outputs[name]; ok {
		mgr.logger.Info("stopping output", "name", name)
		mgr.setOutputState(name, collstore.StateStopping, "")
		err := mo.Impl.Close()
		if err != nil {
			mgr.logger.Error("failed to close output", "name", name, "error", err)
			return fmt.Errorf("failed to close output: %w", err)
		}
		procs := extractProcessors(mo.Cfg)
		mgr.untrackProcessorsInUse(name, procs)
		mgr.setOutputState(name, collstore.StateStopped, "")
		delete(mgr.outputs, name)
	}
	return nil
}

func (mgr *OutputsManager) DeleteOutput(name string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.logger.Info("deleting output", "name", name)
	if mo, ok := mgr.outputs[name]; ok {
		mgr.logger.Info("stopping output", "name", name)
		mgr.setOutputState(name, collstore.StateStopping, "")
		err := mo.Impl.Close()
		if err != nil {
			mgr.logger.Error("failed to close output", "name", name, "error", err)
			return fmt.Errorf("failed to close output: %w", err)
		}
		procs := extractProcessors(mo.Cfg)
		mgr.untrackProcessorsInUse(name, procs)
		mgr.setOutputState(name, collstore.StateStopped, "")
		delete(mgr.outputs, name)
		mgr.store.Config.Delete("outputs", name)
	}
	mgr.store.State.Delete(collstore.KindOutputs, name)
	return nil
}

func (mgr *OutputsManager) Stop() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for _, mo := range mgr.outputs {
		mgr.setOutputState(mo.Name, collstore.StateStopped, "")
		err := mo.Impl.Close()
		if err != nil {
			mgr.logger.Error("failed to stop output", "name", mo.Name, "error", err)
		}
	}
}

func newOutputStats() *outputStats {
	return &outputStats{
		msgCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "gnmic",
			Subsystem: "outputs",
			Name:      "msg_sent_to_output_count",
			Help:      "Number of messages sent to the output",
		}, []string{"name"}),
		msgCountErr: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "gnmic",
			Subsystem: "outputs",
			Name:      "msg_failed_to_sent_to_output_count_error",
			Help:      "Number of messages sent to the output with error",
		}, []string{"name"}),
	}
}

func (mgr *OutputsManager) registerMetrics() {
	if mgr.reg == nil {
		return
	}
	mgr.reg.MustRegister(mgr.stats.msgCount)
	mgr.reg.MustRegister(mgr.stats.msgCountErr)
}

func (mgr *OutputsManager) WriteToCache(ctx context.Context, msg *pipeline.Msg) {
	if mgr.cache == nil {
		return
	}
	if msg.Msg == nil {
		return
	}
	switch msg.Msg.(type) {
	case *gnmi.SubscribeResponse:
		subName, ok := msg.Meta["subscription-name"]
		if !ok || subName == "" {
			subName = "default"
		}
		targetName := utils.GetHost(msg.Meta["source"])
		mgr.cache.Write(ctx, subName, addTargetToMsg(msg.Msg, targetName))
	}
}

func addTargetToMsg(msg proto.Message, targetName string) proto.Message {
	switch msg := msg.(type) {
	case *gnmi.SubscribeResponse:
		switch rsp := msg.Response.(type) {
		case *gnmi.SubscribeResponse_Update:
			if rsp.Update.GetPrefix() == nil {
				rsp.Update.Prefix = new(gnmi.Path)
			}
			rsp.Update.Prefix.Target = targetName
		}
	}
	return msg
}

func extractProcessors(cfg map[string]any) []string {
	v, ok := cfg["event-processors"]
	if !ok {
		return nil
	}
	switch v := v.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, it := range v {
			if s, ok := it.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	}

	return nil
}

func (mgr *OutputsManager) trackProcessorsInUse(out string, procs []string) {
	for _, p := range procs {
		if mgr.processorsInUse[p] == nil {
			mgr.processorsInUse[p] = make(map[string]struct{})
		}
		mgr.processorsInUse[p][out] = struct{}{}
	}
}

func (mgr *OutputsManager) untrackProcessorsInUse(out string, procs []string) {
	for _, p := range procs {
		if users, ok := mgr.processorsInUse[p]; ok {
			delete(users, out)
			if len(users) == 0 {
				delete(mgr.processorsInUse, p)
			}
		}
	}
}

func (mgr *OutputsManager) ProcessorInUse(name string) bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	users, ok := mgr.processorsInUse[name]
	if !ok {
		return false
	}
	return len(users) > 0
}

// State store helpers

func (mgr *OutputsManager) setOutputState(name, state, failedReason string) {
	os := &collstore.OutputState{
		ComponentState: collstore.ComponentState{
			// Name:          name,
			IntendedState: collstore.IntendedStateEnabled,
			State:         state,
			FailedReason:  failedReason,
			LastUpdated:   time.Now(),
		},
	}
	mgr.store.State.Set(collstore.KindOutputs, name, os)
}

func (mgr *OutputsManager) getOutputStateStr(name string) string {
	os := mgr.GetOutputState(name)
	if os == nil {
		return ""
	}
	return os.State
}

// GetOutputState returns the runtime state of an output from the state store.
func (mgr *OutputsManager) GetOutputState(name string) *collstore.OutputState {
	v, ok, err := mgr.store.State.Get(collstore.KindOutputs, name)
	if err != nil || !ok {
		return nil
	}
	os, ok := v.(*collstore.OutputState)
	if !ok {
		return nil
	}
	return os
}

// ListOutputStates returns all output states from the state store.
func (mgr *OutputsManager) ListOutputStates() []*collstore.OutputState {
	states := make([]*collstore.OutputState, 0)
	mgr.store.State.List(collstore.KindOutputs, func(name string, v any) bool {
		if os, ok := v.(*collstore.OutputState); ok {
			states = append(states, os)
		}
		return false
	})
	return states
}
