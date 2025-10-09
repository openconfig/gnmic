// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/outputs"
)

func (a *App) InitOutput(ctx context.Context, name string, tcs map[string]*types.TargetConfig) {
	a.configLock.Lock()
	defer a.configLock.Unlock()
	if _, ok := a.Outputs[name]; ok {
		return
	}
	wg := new(sync.WaitGroup)
	if cfg, ok := a.Config.Outputs[name]; ok {
		if outType, ok := cfg["type"]; ok {
			a.Logger.Printf("starting output type %s", outType)
			if initializer, ok := outputs.Outputs[outType.(string)]; ok {
				out := initializer()
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := out.Init(ctx, name, cfg,
						outputs.WithLogger(a.Logger),
						outputs.WithRegistry(a.reg),
						outputs.WithName(a.Config.InstanceName),
						outputs.WithClusterName(a.Config.ClusterName),
						outputs.WithConfigStore(a.Store),
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
	wg.Wait()
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
