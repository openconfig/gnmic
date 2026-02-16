package inputs_manager

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/inputs"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/zestor-dev/zestor/store"
)

type ManagedInput struct {
	sync.RWMutex
	Name string
	Impl inputs.Input
	Cfg  map[string]any
}

type InputsManager struct {
	ctx            context.Context
	store          *collstore.Store
	inputFactories map[string]inputs.Initializer
	pipeline       chan *pipeline.Msg
	logger         *slog.Logger

	mu              sync.RWMutex
	inputs          map[string]*ManagedInput
	processorsInUse map[string]map[string]struct{} // processor name -> input names
}

func NewInputsManager(ctx context.Context, store *collstore.Store, pipeline chan *pipeline.Msg) *InputsManager {
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
	mgr.logger = logging.NewLogger(mgr.store.Config, "component", "inputs-manager")
	mgr.logger.Info("starting inputs manager")
	inputsCh, inputsCancel, err := mgr.store.Config.Watch("inputs",
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
		mgr.setInputState(mi.Name, collstore.StateStopped, "")
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
		mgr.setInputState(name, collstore.StateFailed, fmt.Sprintf("unknown input type: %s", typ))
		return
	}
	impl := f()
	if err := impl.Start(mgr.ctx, name, cfg,
		inputs.WithLogger(log.New(os.Stdout, "", log.LstdFlags)),
		inputs.WithConfigStore(mgr.store.Config),
		inputs.WithPipeline(mgr.pipeline),
	); err != nil {
		mgr.setInputState(name, collstore.StateFailed, err.Error())
		return
	}
	procs := extractProcessors(cfg)
	mi := &ManagedInput{Name: name, Impl: impl, Cfg: cfg}
	mgr.mu.Lock()
	mgr.trackProcessorsInUse(name, procs)
	mgr.inputs[name] = mi
	mgr.mu.Unlock()
	mgr.setInputState(name, collstore.StateRunning, "")
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
		mgr.setInputState(name, collstore.StateStopping, "")
		err := mi.Impl.Close()
		if err != nil {
			mgr.logger.Error("failed to close input", "name", name, "error", err)
			return fmt.Errorf("failed to close input: %w", err)
		}
		procs := extractProcessors(mi.Cfg)
		mgr.untrackProcessorsInUse(name, procs)
		mgr.setInputState(name, collstore.StateStopped, "")
		delete(mgr.inputs, name)
		mgr.store.Config.Delete("inputs", name)
	}
	mgr.store.State.Delete(collstore.KindInputs, name)
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

// State store helpers

func (mgr *InputsManager) setInputState(name, state, failedReason string) {
	is := &collstore.InputState{
		ComponentState: collstore.ComponentState{
			// Name:          name,
			IntendedState: collstore.IntendedStateEnabled,
			State:         state,
			FailedReason:  failedReason,
			LastUpdated:   time.Now(),
		},
	}
	mgr.store.State.Set(collstore.KindInputs, name, is)
}

// GetInputState returns the runtime state of an input from the state store.
func (mgr *InputsManager) GetInputState(name string) *collstore.InputState {
	v, ok, err := mgr.store.State.Get(collstore.KindInputs, name)
	if err != nil || !ok {
		return nil
	}
	is, ok := v.(*collstore.InputState)
	if !ok {
		return nil
	}
	return is
}

// ListInputStates returns all input states from the state store.
func (mgr *InputsManager) ListInputStates() []*collstore.InputState {
	states := make([]*collstore.InputState, 0)
	mgr.store.State.List(collstore.KindInputs, func(name string, v any) bool {
		if is, ok := v.(*collstore.InputState); ok {
			states = append(states, is)
		}
		return false
	})
	return states
}
