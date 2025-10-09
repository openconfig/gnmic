package inputs_manager

import (
	"context"
	"log"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"

	"github.com/openconfig/gnmic/pkg/config/store"
	"github.com/openconfig/gnmic/pkg/inputs"
	"github.com/openconfig/gnmic/pkg/pipeline"
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

	mu     sync.RWMutex
	inputs map[string]*ManagedInput
}

func NewInputsManager(ctx context.Context, store store.Store[any], pipeline chan *pipeline.Msg) *InputsManager {
	return &InputsManager{
		ctx:            ctx,
		store:          store,
		pipeline:       pipeline,
		inputFactories: inputs.Inputs,
		inputs:         map[string]*ManagedInput{},
		logger:         slog.With("component", "inputs-manager"),
	}
}

func (im *InputsManager) Start(wg *sync.WaitGroup) error {
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
				cfg := ev.Object.(map[string]any)
				switch ev.EventType {
				case store.EventTypeCreate:
					im.create(ev.Name, cfg)
				case store.EventTypeUpdate:
					im.update(ev.Name, cfg)
				case store.EventTypeDelete:
					im.delete(ev.Name)
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
func (im *InputsManager) create(name string, cfg map[string]any) {
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
	mi := &ManagedInput{Name: name, Impl: impl, Cfg: cfg}
	mi.State.Store("running")
	im.mu.Lock()
	im.inputs[name] = mi
	im.mu.Unlock()
}

func (im *InputsManager) update(name string, cfg map[string]any) {
	im.mu.RLock()
	mi, ok := im.inputs[name]
	im.mu.RUnlock()
	if !ok {
		return
	}
	mi.Cfg = cfg
}

func (im *InputsManager) delete(name string) {
	im.mu.Lock()
	defer im.mu.Unlock()
	delete(im.inputs, name)
}

// func (im *InputsManager) addProcessor(name string, cfg map[string]any) {
// 	im.mu.Lock()
// 	defer im.mu.Unlock()
// 	for _, mi := range im.inputs {
// 		err := mi.Impl.AddEventProcessor(name, cfg)
// 		if err != nil {
// 			im.logger.Error("failed to add event processor to input", "name", name, "inputName", mi.Name, "error", err)
// 		}
// 	}
// }

// func (im *InputsManager) updateProcessor(name string, cfg map[string]any) {
// 	im.mu.Lock()
// 	defer im.mu.Unlock()
// 	for _, mi := range im.inputs {
// 		err := mi.Impl.UpdateEventProcessor(name, cfg)
// 		if err != nil {
// 			im.logger.Error("failed to update event processor for input", "name", name, "inputName", mi.Name, "error", err)
// 		}
// 	}
// }

// func (im *InputsManager) deleteProcessor(name string) {
// 	im.mu.Lock()
// 	defer im.mu.Unlock()
// 	for _, mi := range im.inputs {
// 		err := mi.Impl.RemoveEventProcessor(name)
// 		if err != nil {
// 			im.logger.Error("failed to remove event processor for input", "name", name, "inputName", mi.Name, "error", err)
// 		}
// 	}
// }
