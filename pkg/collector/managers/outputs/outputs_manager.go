package outputs_manager

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/openconfig/gnmic/pkg/store"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

type ManagedOutput struct {
	sync.RWMutex
	Name  string
	Impl  outputs.Output
	Cfg   map[string]any
	State atomic.Value // "running|stopped|failed|paused"

}

// OutputsManager runs outputs.
type OutputsManager struct {
	ctx   context.Context
	store store.Store[any]

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

func NewOutputsManager(ctx context.Context, store store.Store[any], pipe <-chan *pipeline.Msg, reg *prometheus.Registry) *OutputsManager {
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

func (om *OutputsManager) Start(cache cache.Cache, wg *sync.WaitGroup) error {
	om.logger = logging.NewLogger(om.store, "component", "outputs-manager")
	om.logger.Info("starting outputs manager")
	om.cache = cache
	// register metrics
	om.registerMetrics()
	// watch outputs config changes
	outputCh, outputsCancel, err := om.store.Watch("outputs", store.WithInitialReplay[any]())
	if err != nil {
		return err
	}
	// watch processors config changes
	procsCh, processorsCancel, err := om.store.Watch("processors", store.WithInitialReplay[any]())
	if err != nil {
		return err
	}

	wg.Add(1)
	// forward incoming events to all running outputs
	// that are in the list of outputs to write to.
	go om.writeLoop(wg)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer outputsCancel()
		defer processorsCancel()
		for {
			select {
			case <-om.ctx.Done():
				return
			case ev, ok := <-outputCh:
				if !ok {
					return
				}
				om.logger.Info("got output event", "event", ev)
				cfg, ok := ev.Object.(map[string]any)
				if !ok {
					om.logger.Error("invalid output config", "event", ev)
					continue
				}
				switch ev.EventType {
				case store.EventTypeCreate:
					om.createOutput(ev.Name, cfg)
				case store.EventTypeUpdate:
					om.updateOutput(ev.Name, cfg)
				case store.EventTypeDelete:
					om.DeleteOutput(ev.Name)
				}
			case ev, ok := <-procsCh:
				if !ok {
					return
				}
				om.logger.Info("got processor event", "event", ev)
				cfg := ev.Object.(map[string]any)
				switch ev.EventType {
				case store.EventTypeCreate:
					om.addProcessor(ev.Name, cfg)
				case store.EventTypeUpdate:
					om.updateProcessor(ev.Name, cfg)
				case store.EventTypeDelete:
					om.deleteProcessor(ev.Name)
				}
			}
		}
	}()

	return nil
}

func (om *OutputsManager) writeLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-om.ctx.Done():
			return
		case e, ok := <-om.in:
			if !ok {
				om.logger.Debug("pipeline channel closed")
				return
			}
			om.logger.Info("got pipeline message", "message", e) // Debug
			go om.write(e)
			if om.cache != nil {
				go om.cache.Write(om.ctx, e.Meta["subscription-name"], e.Msg)
			}
		}
	}
}

func (om *OutputsManager) write(e *pipeline.Msg) {
	outs := om.getOutputsForTarget(e.Outputs)
	om.logger.Info("writing msg to outputs", "outputs", outs) // Debug
	for _, mo := range outs {
		om.stats.msgCount.WithLabelValues(mo.Name).Inc()
		if len(e.Events) > 0 { // from inputs
			for _, ev := range e.Events {
				mo.Impl.WriteEvent(om.ctx, ev)
			}
		} else {
			// from targets or inputs
			mo.Impl.Write(om.ctx, e.Msg, e.Meta)
		}
	}
}

func (om *OutputsManager) addProcessor(name string, cfg map[string]any) {
	// noop
}

func (om *OutputsManager) updateProcessor(name string, cfg map[string]any) {
	om.mu.Lock()
	defer om.mu.Unlock()
	for _, mo := range om.outputs {
		err := mo.updateProcessor(name, cfg)
		if err != nil {
			om.logger.Error("failed to update event processor for output", "processorName", name, "outputName", mo.Name, "error", err)
		}
	}
}

func (mo *ManagedOutput) updateProcessor(name string, cfg map[string]any) error {
	mo.Lock()
	defer mo.Unlock()
	return mo.Impl.UpdateProcessor(name, cfg)
}

func (om *OutputsManager) deleteProcessor(name string) {
	// noop
}

func (om *OutputsManager) getOutputsForTarget(outputs map[string]struct{}) []*ManagedOutput {
	om.mu.RLock()
	defer om.mu.RUnlock()
	// all outputs
	if len(outputs) == 0 {
		outs := make([]*ManagedOutput, 0, len(om.outputs))
		for _, mo := range om.outputs {
			if mo.State.Load() == "running" {
				outs = append(outs, mo)
			}
		}
		return outs
	}
	// specific outputs per target
	outs := make([]*ManagedOutput, 0, len(outputs))
	for name, mo := range om.outputs {
		if _, ok := outputs[name]; !ok || mo.State.Load() != "running" {
			continue
		}
		outs = append(outs, mo)
	}
	return outs
}

func (om *OutputsManager) createOutput(name string, cfg map[string]any) {
	typ, _ := cfg["type"].(string)
	f := om.OutputsFactory[typ]
	if f == nil {
		om.logger.Error("unknown output type", "name", name, "type", typ)
		return
	}
	impl := f()

	opts := make([]outputs.Option, 0, 2)
	opts = append(opts,
		outputs.WithName(name),
		outputs.WithConfigStore(om.store),
		outputs.WithLogger(log.New(os.Stdout, "", log.LstdFlags)), // temporary logger
	)

	clustering, ok, err := om.store.Get("global", "clustering")
	if err != nil {
		om.logger.Error("failed to get clustering for output", "name", name, "error", err)
		return
	}
	if ok {
		clus, ok := clustering.(map[string]any)
		if cname, cOk := clus["cluster-name"].(string); cOk && ok {
			opts = append(opts, outputs.WithClusterName(cname))
		}
	}

	err = impl.Init(om.ctx, name, cfg, opts...)
	if err != nil {
		om.logger.Error("failed to init output", "name", name, "error", err)
		return
	}
	procs := extractProcessors(cfg)
	mo := &ManagedOutput{Name: name, Impl: impl, Cfg: cfg}
	mo.State.Store("running")
	om.mu.Lock()
	om.trackProcessorsInUse(name, procs)
	om.outputs[name] = mo
	om.mu.Unlock()
}

func (om *OutputsManager) updateOutput(name string, cfg map[string]any) {
	om.mu.Lock()
	defer om.mu.Unlock()
	mo, ok := om.outputs[name]
	if !ok {
		om.createOutput(name, cfg)
		return
	}

	om.logger.Info("updating output", "name", name, "cfg", cfg)
	mo.Lock()
	defer mo.Unlock()
	err := mo.Impl.Update(om.ctx, cfg)
	if err != nil {
		om.logger.Error("failed to update output", "name", name, "error", err)
		return
	}
	oldProcs := extractProcessors(mo.Cfg)
	newProcs := extractProcessors(cfg)
	om.logger.Info("tracking output processors in use", "name", name, "oldProcs", oldProcs, "newProcs", newProcs)
	om.untrackProcessorsInUse(name, oldProcs)
	om.trackProcessorsInUse(name, newProcs)
	om.logger.Info("updated output", "name", name, "cfg", cfg)
	mo.Cfg = cfg
	om.outputs[name] = mo
}

func (om *OutputsManager) StopOutput(name string) error {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.logger.Info("finding output", "name", name)
	if mo, ok := om.outputs[name]; ok {
		om.logger.Info("stopping output", "name", name)
		mo.State.Store("stopping")
		err := mo.Impl.Close()
		if err != nil {
			om.logger.Error("failed to close output", "name", name, "error", err)
			return fmt.Errorf("failed to close output: %w", err)
		}
		procs := extractProcessors(mo.Cfg)
		om.untrackProcessorsInUse(name, procs)
		mo.State.Store("stopped")
		delete(om.outputs, name)
	}
	return nil
}

func (om *OutputsManager) DeleteOutput(name string) error {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.logger.Info("deleting output", "name", name)
	if mo, ok := om.outputs[name]; ok {
		om.logger.Info("stopping output", "name", name)
		mo.State.Store("stopping")
		err := mo.Impl.Close()
		if err != nil {
			om.logger.Error("failed to close output", "name", name, "error", err)
			return fmt.Errorf("failed to close output: %w", err)
		}
		procs := extractProcessors(mo.Cfg)
		om.untrackProcessorsInUse(name, procs)
		mo.State.Store("stopped")
		delete(om.outputs, name)
		om.store.Delete("outputs", name)
	}
	return nil
}

func (om *OutputsManager) Stop() {
	om.mu.Lock()
	defer om.mu.Unlock()
	for _, mo := range om.outputs {
		mo.State.Store("stopped")
		err := mo.Impl.Close()
		if err != nil {
			om.logger.Error("failed to stop output", "name", mo.Name, "error", err)
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

func (om *OutputsManager) registerMetrics() {
	if om.reg == nil {
		return
	}
	om.reg.MustRegister(om.stats.msgCount)
	om.reg.MustRegister(om.stats.msgCountErr)
}

func (om *OutputsManager) WriteToCache(ctx context.Context, msg *pipeline.Msg) {
	if om.cache == nil {
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
		om.cache.Write(ctx, subName, addTargetToMsg(msg.Msg, targetName))
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

func (om *OutputsManager) trackProcessorsInUse(out string, procs []string) {
	for _, p := range procs {
		if om.processorsInUse[p] == nil {
			om.processorsInUse[p] = make(map[string]struct{})
		}
		om.processorsInUse[p][out] = struct{}{}
	}
}

func (om *OutputsManager) untrackProcessorsInUse(out string, procs []string) {
	for _, p := range procs {
		if users, ok := om.processorsInUse[p]; ok {
			delete(users, out)
			if len(users) == 0 {
				delete(om.processorsInUse, p)
			}
		}
	}
}

// func (om *OutputsManager) DeleteProcessor(name string) error {
// 	om.mu.Lock()
// 	defer om.mu.Unlock()

// 	// check whether the processor is used by any output
// 	users := om.processorsInUse[name]
// 	if len(users) > 0 {
// 		outputsNames := make([]string, 0, len(users))
// 		for out := range users {
// 			outputsNames = append(outputsNames, out)
// 		}
// 		return fmt.Errorf("processor is in use by outputs: %v", outputsNames)
// 	}

// 	delete(om.processorsInUse, name)
// 	ok, _, err := om.store.Delete("processors", name)
// 	if err != nil {
// 		return err
// 	}
// 	if !ok {
// 		return fmt.Errorf("processor not found")
// 	}
// 	om.logger.Info("deleted processor", "name", name)
// 	return nil
// }

func (om *OutputsManager) ProcessorInUse(name string) bool {
	om.mu.RLock()
	defer om.mu.RUnlock()
	users, ok := om.processorsInUse[name]
	if !ok {
		return false
	}
	return len(users) > 0
}
