package outputs_manager

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config/store"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/prometheus/client_golang/prometheus"
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

	mu      sync.RWMutex
	outputs map[string]*ManagedOutput
	logger  *slog.Logger
}

type outputStats struct {
	msgCount    prometheus.CounterVec
	msgCountErr prometheus.CounterVec
}

func NewOutputsManager(ctx context.Context, st store.Store[any], pipe <-chan *pipeline.Msg) *OutputsManager {
	logger := slog.With("component", "outputs-manager")
	return &OutputsManager{
		ctx:            ctx,
		store:          st,
		OutputsFactory: outputs.Outputs,
		in:             pipe,
		outputs:        map[string]*ManagedOutput{},
		logger:         logger,
	}
}

func (om *OutputsManager) Start(wg *sync.WaitGroup) error {
	om.logger.Info("starting outputs manager")
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
				cfg := ev.Object.(map[string]any)
				switch ev.EventType {
				case store.EventTypeCreate:
					om.createOutput(ev.Name, cfg)
				case store.EventTypeUpdate:
					om.updateOutput(ev.Name, cfg)
				case store.EventTypeDelete:
					om.deleteOutput(ev.Name)
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

	wg.Add(1)
	// Forward incoming events to all running outputs
	// that are in the list of outputs to write to.
	go om.writeLoop(wg)

	return nil
}

func (om *OutputsManager) writeLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-om.ctx.Done():
			return
		case e := <-om.in:
			go om.write(e)
		}
	}
}

func (om *OutputsManager) write(e *pipeline.Msg) {
	outs := om.getOutputsForTarget(e.Outputs)
	// om.logger.Info("writing to outputs", "outputs", outs)
	for _, mo := range outs {
		if len(e.Events) > 0 { // from inputs
			for _, ev := range e.Events {
				mo.Impl.WriteEvent(om.ctx, ev)
			}
		} else {
			// from targets
			mo.Impl.Write(om.ctx, e.Msg, e.Meta)
		}
	}
}

func (om *OutputsManager) addProcessor(name string, cfg map[string]any) {
	om.mu.Lock()
	defer om.mu.Unlock()
	for _, mo := range om.outputs {
		_ = mo
		// err := mo.Impl.AddEventProcessor(name, cfg)
		// if err != nil {
		// 	om.logger.Error("failed to add event processor", "name", name, "error", err)
		// }
	}
}

func (om *OutputsManager) updateProcessor(name string, cfg map[string]any) {
	om.mu.Lock()
	defer om.mu.Unlock()
	for _, mo := range om.outputs {
		_ = mo
		// err := mo.Impl.UpdateEventProcessor(name, cfg)
		// if err != nil {
		// 	om.logger.Error("failed to update event processor for output", "processorName", name, "outputName", mo.Name, "error", err)
		// }
	}
}

func (om *OutputsManager) deleteProcessor(name string) {
	om.mu.Lock()
	defer om.mu.Unlock()
	for _, mo := range om.outputs {
		_ = mo
		// 	err := mo.Impl.RemoveEventProcessor(name)
		// if err != nil {
		// 	om.logger.Error("failed to remove event processor for output", "processorName", name, "outputName", mo.Name, "error", err)
		// }
	}
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
	mo := &ManagedOutput{Name: name, Impl: impl, Cfg: cfg}
	mo.State.Store("running")
	om.mu.Lock()
	om.outputs[name] = mo
	om.mu.Unlock()
}

// func getReferencedProcessors(cfg map[string]any) []string {
// 	switch v := cfg["event-processors"].(type) {
// 	case []string:
// 		return v
// 	case string:
// 		return []string{v}
// 	}
// 	return nil

// }

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
	om.logger.Info("updated output", "name", name, "cfg", cfg)
	mo.Cfg = cfg
	om.outputs[name] = mo
}

func (om *OutputsManager) deleteOutput(name string) {
	om.mu.Lock()
	defer om.mu.Unlock()
	if mo, ok := om.outputs[name]; ok {
		mo.State.Store("stopping")
		err := mo.Impl.Close()
		if err != nil {
			om.logger.Error("failed to stop output", "name", name, "error", err)
			return
		}
		mo.State.Store("stopped")
		delete(om.outputs, name)
	}
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

func (om *OutputsManager) getProcessors(ls ...string) (map[string]map[string]any, error) {
	requested := map[string]struct{}{}
	for _, l := range ls {
		requested[l] = struct{}{}
	}
	filterFunc := func(key string, val any) bool {
		if len(requested) == 0 { // return all if none requested
			return true
		}
		if _, ok := requested[key]; ok {
			return true
		}
		return false
	}
	procsMap, err := om.store.List("processors", filterFunc)
	if err != nil {
		return nil, err
	}
	procs := make(map[string]map[string]any, len(procsMap))
	for pn, proc := range procsMap {
		switch proc := proc.(type) {
		case map[string]any:
			procs[pn] = proc
		}
	}
	return procs, nil
}

func (om *OutputsManager) getTargets() (map[string]*types.TargetConfig, error) {
	targetsMap, err := om.store.List("targets")
	if err != nil {
		return nil, err
	}
	targets := make(map[string]*types.TargetConfig, len(targetsMap))
	for tn, target := range targetsMap {
		switch target := target.(type) {
		case *types.TargetConfig:
			targets[tn] = target
		}
	}
	return targets, nil
}

func (om *OutputsManager) getActions() (map[string]map[string]any, error) {
	actionsMap, err := om.store.List("actions")
	if err != nil {
		return nil, err
	}
	actions := make(map[string]map[string]interface{}, len(actionsMap))
	for an, action := range actionsMap {
		switch action := action.(type) {
		case map[string]interface{}:
			actions[an] = action
		}
	}
	return actions, nil
}
