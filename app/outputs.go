package app

import (
	"context"
	"fmt"

	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
)

func (a *App) InitOutput(ctx context.Context, name string, tcs map[string]*types.TargetConfig) {
	a.configLock.Lock()
	defer a.configLock.Unlock()
	if _, ok := a.Outputs[name]; ok {
		return
	}
	if cfg, ok := a.Config.Outputs[name]; ok {
		if outType, ok := cfg["type"]; ok {
			a.Logger.Printf("starting output type %s", outType)
			if initializer, ok := outputs.Outputs[outType.(string)]; ok {
				out := initializer()
				go func() {
					err := out.Init(ctx, name, cfg,
						outputs.WithLogger(a.Logger),
						outputs.WithEventProcessors(
							a.Config.Processors,
							a.Logger, 
							a.Config.Targets,
							a.Config.Actions,
						),
						outputs.WithRegister(a.reg),
						outputs.WithName(a.Config.InstanceName),
						outputs.WithClusterName(a.Config.ClusterName),
						outputs.WithTargetsConfig(tcs),
					)
					if err != nil {
						a.Logger.Printf("failed to init output type %q: %v", outType, err)
					}
				}()
				a.operLock.Lock()
				a.Outputs[name] = out
				a.operLock.Unlock()
			}
		}
	}
}

func (a *App) InitOutputs(ctx context.Context) {
	for name := range a.Config.Outputs {
		a.InitOutput(ctx, name, a.Config.Targets)
	}
}

// AddOutputConfig adds an output called name, with config cfg if it does not already exist
func (a *App) AddOutputConfig(name string, cfg map[string]interface{}) error {
	// if a.Outputs == nil {
	// 	a.Outputs = make(map[string]outputs.Output)
	// }
	if a.Config.Outputs == nil {
		a.Config.Outputs = make(map[string]map[string]interface{})
	}
	if _, ok := a.Outputs[name]; ok {
		return fmt.Errorf("output %q already exists", name)
	}
	a.configLock.Lock()
	defer a.configLock.Unlock()
	a.Config.Outputs[name] = cfg
	return nil
}

func (a *App) DeleteOutput(name string) error {
	if a.Outputs == nil {
		return nil
	}
	a.operLock.Lock()
	defer a.operLock.Unlock()
	if _, ok := a.Outputs[name]; !ok {
		return fmt.Errorf("output %q does not exist", name)
	}
	o := a.Outputs[name]
	err := o.Close()
	if err != nil {
		a.Logger.Printf("failed to close output %q: %v", name, err)
	}
	delete(a.Outputs, name)
	return nil
}
