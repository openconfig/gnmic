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

	"github.com/openconfig/gnmic/pkg/inputs"
	"github.com/openconfig/gnmic/pkg/types"
)

func (a *App) InitInput(ctx context.Context, name string, tcs map[string]*types.TargetConfig) {
	a.configLock.Lock()
	defer a.configLock.Unlock()
	if _, ok := a.Inputs[name]; ok {
		return
	}
	if cfg, ok := a.Config.Inputs[name]; ok {
		if inputType, ok := cfg["type"]; ok {
			a.Logger.Printf("starting input type %s", inputType)
			if initializer, ok := inputs.Inputs[inputType.(string)]; ok {
				in := initializer()
				go func() {
					err := in.Start(ctx, name, cfg,
						inputs.WithLogger(a.Logger),
						inputs.WithEventProcessors(
							a.Config.Processors,
							a.Logger,
							a.Config.Targets,
							a.Config.Actions,
						),
						inputs.WithName(a.Config.InstanceName),
						inputs.WithOutputs(a.Outputs),
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
