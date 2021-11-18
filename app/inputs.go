package app

import (
	"context"

	"github.com/karimra/gnmic/inputs"
	"github.com/karimra/gnmic/types"
)

func (a *App) InitInput(ctx context.Context, name string, tcs map[string]*types.TargetConfig) {
	a.configLock.Lock()
	defer a.configLock.Unlock()
	if _, ok := a.Inputs[name]; ok {
		return
	}
	if cfg, ok := a.Config.Outputs[name]; ok {
		if inputType, ok := cfg["type"]; ok {
			a.Logger.Printf("starting input type %s", inputType)
			if initializer, ok := inputs.Inputs[inputType.(string)]; ok {
				in := initializer()
				go func() {
					err := in.Start(ctx, name, cfg,
						inputs.WithLogger(a.Logger),
						inputs.WithEventProcessors(a.Config.Processors, a.Logger, a.Config.Targets),
						inputs.WithName(a.Config.InstanceName),
					)
					if err != nil {
						a.Logger.Printf("failed to init input type %q: %v", inputType, err)
					}
				}()
				a.operLock.Lock()
				a.Inputs[name] = in
				a.operLock.Unlock()
			}
		}
	}
}

func (a *App) InitInputs(ctx context.Context) {
	for name := range a.Config.Inputs {
		a.InitInput(ctx, name, a.Config.Targets)
	}
}
