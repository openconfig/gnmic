package inputs_manager

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"

	"github.com/openconfig/gnmic/pkg/inputs"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/openconfig/gnmic/pkg/store"
)

type ManagedInput struct {
	sync.RWMutex
	Name          string
	Impl          inputs.Input
	Cfg           map[string]any
	IntendedState atomic.Value // "enabled|disabled"
	State         atomic.Value // "running|stopped|failed|paused"
}

type InputsManager struct {
	ctx            context.Context
	store          store.Store[any]
	inputFactories map[string]inputs.Initializer
	pipeline       chan *pipeline.Msg
	logger         *slog.Logger

	mu              sync.RWMutex
	inputs          map[string]*ManagedInput
	processorsInUse map[string]map[string]struct{} // processor name -> input names
}

func NewInputsManager(ctx context.Context, store store.Store[any], pipeline chan *pipeline.Msg) *InputsManager {
	return &InputsManager{
		ctx:             ctx,
		store:           store,
		pipeline:        pipeline,
		inputFactories:  inputs.Inputs,
		inputs:          map[string]*ManagedInput{},
		processorsInUse: make(map[string]map[string]struct{}),
	}
}

func (mgr *InputsManager) Start(wg *sync.WaitGroup) error {
	mgr.logger = logging.NewLogger(mgr.store, "component", "inputs-manager")
	mgr.logger.Info("starting inputs manager")
	inputsCh, inputsCancel, err := mgr.store.Watch("inputs",
		store.WithInitialReplay[any]())
	if err != nil {
		return err
	}

	// watch processors config changes (update only)
	procsCh, processorsCancel, err := mgr.store.Watch("processors",
		store.WithEventTypes[any](store.EventTypeUpdate))
	if err != nil {
		return err
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer inputsCancel()
		defer processorsCancel()
		for {
			select {
			case <-mgr.ctx.Done():
				return
			case ev, ok := <-inputsCh:
				if !ok {
					return
				}
				mgr.logger.Info("got input event", "event", ev)
				cfg, ok := ev.Object.(map[string]any)
				if !ok {
					mgr.logger.Error("invalid input config", "event", ev)
					continue
				}

				switch ev.EventType {
				case store.EventTypeCreate:
					mgr.createInput(ev.Name, cfg)
				case store.EventTypeUpdate:
					mgr.updateInput(ev.Name, cfg)
				case store.EventTypeDelete:
					mgr.DeleteInput(ev.Name)
				}
			case ev, ok := <-procsCh:
				if !ok {
					return
				}
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

func (mgr *InputsManager) Stop() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for _, mi := range mgr.inputs {
		mi.State.Store("stopped")
		err := mi.Impl.Close()
		if err != nil {
			mgr.logger.Error("failed to stop input", "name", mi.Name, "error", err)
		}
	}
}

func (mgr *InputsManager) createInput(name string, cfg map[string]any) {
	typ, _ := cfg["type"].(string)
	f := mgr.inputFactories[typ]
	if f == nil {
		return
	}
	impl := f()
	if err := impl.Start(mgr.ctx, name, cfg,
		inputs.WithLogger(log.New(os.Stdout, "", log.LstdFlags)),
		inputs.WithConfigStore(mgr.store),
		inputs.WithPipeline(mgr.pipeline),
	); err != nil {
		return
	}
	procs := extractProcessors(cfg)
	mi := &ManagedInput{Name: name, Impl: impl, Cfg: cfg}
	mi.State.Store("running")
	mgr.mu.Lock()
	mgr.trackProcessorsInUse(name, procs)
	mgr.inputs[name] = mi
	mgr.mu.Unlock()
}

func (mgr *InputsManager) updateInput(name string, cfg map[string]any) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mi, ok := mgr.inputs[name]
	if !ok {
		mgr.createInput(name, cfg)
		return
	}
	mgr.logger.Info("updating input", "name", name, "cfg", cfg)
	mi.Lock()
	defer mi.Unlock()
	err := mi.Impl.Update(cfg)
	if err != nil {
		mgr.logger.Error("failed to update input", "name", name, "error", err)
		return
	}
	oldProcs := extractProcessors(mi.Cfg)
	newProcs := extractProcessors(cfg)
	mgr.logger.Info("tracking input processors in use", "name", name, "oldProcs", oldProcs, "newProcs", newProcs)
	mgr.untrackProcessorsInUse(name, oldProcs)
	mgr.trackProcessorsInUse(name, newProcs)
	mgr.logger.Info("updated input", "name", name, "cfg", cfg)
	mi.Cfg = cfg
	mgr.inputs[name] = mi
}

func (mgr *InputsManager) DeleteInput(name string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.logger.Info("finding input", "name", name)
	if mi, ok := mgr.inputs[name]; ok {
		mgr.logger.Info("stopping input", "name", name)
		mi.State.Store("stopping")
		err := mi.Impl.Close()
		if err != nil {
			mgr.logger.Error("failed to close input", "name", name, "error", err)
			return fmt.Errorf("failed to close input: %w", err)
		}
		procs := extractProcessors(mi.Cfg)
		mgr.untrackProcessorsInUse(name, procs)
		mi.State.Store("stopped")
		delete(mgr.inputs, name)
		mgr.store.Delete("inputs", name)
	}
	return nil
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

func (mgr *InputsManager) trackProcessorsInUse(in string, procs []string) {
	for _, p := range procs {
		if mgr.processorsInUse[p] == nil {
			mgr.processorsInUse[p] = make(map[string]struct{})
		}
		mgr.processorsInUse[p][in] = struct{}{}
	}
}

func (mgr *InputsManager) untrackProcessorsInUse(in string, procs []string) {
	for _, p := range procs {
		if users, ok := mgr.processorsInUse[p]; ok {
			delete(users, in)
			if len(users) == 0 {
				delete(mgr.processorsInUse, p)
			}
		}
	}
}

func (mgr *InputsManager) ProcessorInUse(name string) bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	users, ok := mgr.processorsInUse[name]
	if !ok {
		return false
	}
	return len(users) > 0
}

func (mgr *InputsManager) updateProcessor(name string, cfg map[string]any) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for _, mi := range mgr.inputs {
		err := mi.updateProcessor(name, cfg)
		if err != nil {
			mgr.logger.Error("failed to update event processor for input", "processorName", name, "inputName", mi.Name, "error", err)
		}
	}
}

func (mi *ManagedInput) updateProcessor(name string, cfg map[string]any) error {
	mi.Lock()
	defer mi.Unlock()
	return mi.Impl.UpdateProcessor(name, cfg)
}
