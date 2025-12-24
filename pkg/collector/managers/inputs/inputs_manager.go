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

func (im *InputsManager) Start(wg *sync.WaitGroup) error {
	im.logger = logging.NewLogger(im.store, "component", "inputs-manager")
	im.logger.Info("starting inputs manager")
	inputsCh, inputsCancel, err := im.store.Watch("inputs", store.WithInitialReplay[any]())
	if err != nil {
		return err
	}

	// watch processors config changes
	procsCh, processorsCancel, err := im.store.Watch("processors", store.WithInitialReplay[any]())
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
			case <-im.ctx.Done():
				return
			case ev, ok := <-inputsCh:
				if !ok {
					return
				}
				im.logger.Info("got input event", "event", ev)
				cfg, ok := ev.Object.(map[string]any)
				if !ok {
					im.logger.Error("invalid input config", "event", ev)
					continue
				}

				switch ev.EventType {
				case store.EventTypeCreate:
					im.createInput(ev.Name, cfg)
				case store.EventTypeUpdate:
					im.updateInput(ev.Name, cfg)
				case store.EventTypeDelete:
					im.DeleteInput(ev.Name)
				}
			case ev, ok := <-procsCh:
				if !ok {
					return
				}
				cfg := ev.Object.(map[string]any)
				_ = cfg
				// switch ev.EventType {
				// case store.EventTypeCreate:
				// 	im.addProcessor(ev.Name, cfg)
				// case store.EventTypeUpdate:
				// 	im.updateProcessor(ev.Name, cfg)
				// case store.EventTypeDelete:
				// 	im.deleteProcessor(ev.Name)
				// }
			}
		}
	}()
	return nil
}

func (im *InputsManager) Stop() {
	im.mu.Lock()
	defer im.mu.Unlock()
	for _, mi := range im.inputs {
		mi.State.Store("stopped")
		err := mi.Impl.Close()
		if err != nil {
			im.logger.Error("failed to stop input", "name", mi.Name, "error", err)
		}
	}
}

func (im *InputsManager) createInput(name string, cfg map[string]any) {
	typ, _ := cfg["type"].(string)
	f := im.inputFactories[typ]
	if f == nil {
		return
	}
	impl := f()
	if err := impl.Start(im.ctx, name, cfg,
		inputs.WithLogger(log.New(os.Stdout, "", log.LstdFlags)),
		inputs.WithConfigStore(im.store),
		inputs.WithPipeline(im.pipeline),
	); err != nil {
		return
	}
	procs := extractProcessors(cfg)
	mi := &ManagedInput{Name: name, Impl: impl, Cfg: cfg}
	mi.State.Store("running")
	im.mu.Lock()
	im.trackProcessorsInUse(name, procs)
	im.inputs[name] = mi
	im.mu.Unlock()
}

func (im *InputsManager) updateInput(name string, cfg map[string]any) {
	im.mu.Lock()
	defer im.mu.Unlock()
	mi, ok := im.inputs[name]
	if !ok {
		im.createInput(name, cfg)
		return
	}
	im.logger.Info("updating input", "name", name, "cfg", cfg)
	mi.Lock()
	defer mi.Unlock()
	// err := mi.Impl.Update(im.ctx, cfg) // TODO: implement input update method
	// if err != nil {
	// 	im.logger.Error("failed to update input", "name", name, "error", err)
	// 	return
	// }
	oldProcs := extractProcessors(mi.Cfg)
	newProcs := extractProcessors(cfg)
	im.logger.Info("tracking input processors in use", "name", name, "oldProcs", oldProcs, "newProcs", newProcs)
	im.untrackProcessorsInUse(name, oldProcs)
	im.trackProcessorsInUse(name, newProcs)
	im.logger.Info("updated input", "name", name, "cfg", cfg)
	mi.Cfg = cfg
	im.inputs[name] = mi
}

func (im *InputsManager) DeleteInput(name string) error {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.logger.Info("finding input", "name", name)
	if mi, ok := im.inputs[name]; ok {
		im.logger.Info("stopping input", "name", name)
		mi.State.Store("stopping")
		err := mi.Impl.Close()
		if err != nil {
			im.logger.Error("failed to close input", "name", name, "error", err)
			return fmt.Errorf("failed to close input: %w", err)
		}
		procs := extractProcessors(mi.Cfg)
		im.untrackProcessorsInUse(name, procs)
		mi.State.Store("stopped")
		delete(im.inputs, name)
		im.store.Delete("inputs", name)
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

func (im *InputsManager) trackProcessorsInUse(in string, procs []string) {
	for _, p := range procs {
		if im.processorsInUse[p] == nil {
			im.processorsInUse[p] = make(map[string]struct{})
		}
		im.processorsInUse[p][in] = struct{}{}
	}
}

func (im *InputsManager) untrackProcessorsInUse(in string, procs []string) {
	for _, p := range procs {
		if users, ok := im.processorsInUse[p]; ok {
			delete(users, in)
			if len(users) == 0 {
				delete(im.processorsInUse, p)
			}
		}
	}
}

func (im *InputsManager) ProcessorInUse(name string) bool {
	im.mu.RLock()
	defer im.mu.RUnlock()
	users, ok := im.processorsInUse[name]
	if !ok {
		return false
	}
	return len(users) > 0
}
