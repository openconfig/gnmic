// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package inputs

import (
	"context"
	"fmt"
	"log"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/types"
)

type Input interface {
	Start(context.Context, string, map[string]interface{}, ...Option) error
	Close() error
	SetLogger(*log.Logger)
	SetOutputs(map[string]outputs.Output)
	SetEventProcessors(map[string]map[string]interface{}, *log.Logger, map[string]*types.TargetConfig) error
	SetName(string)
}

type Initializer func() Input

var InputTypes = []string{
	"nats",
	"stan",
	"kafka",
}

var Inputs = map[string]Initializer{}

func Register(name string, initFn Initializer) {
	Inputs[name] = initFn
}

type Option func(Input) error

func WithLogger(logger *log.Logger) Option {
	return func(i Input) error {
		i.SetLogger(logger)
		return nil
	}
}

func WithOutputs(outs map[string]outputs.Output) Option {
	return func(i Input) error {
		i.SetOutputs(outs)
		return nil
	}
}

func WithName(name string) Option {
	return func(i Input) error {
		i.SetName(name)
		return nil
	}
}

func WithEventProcessors(eps map[string]map[string]interface{}, log *log.Logger, tcs map[string]*types.TargetConfig) Option {
	return func(i Input) error {
		return i.SetEventProcessors(eps, log, tcs)
	}
}

func MakeEventProcessors(
	logger *log.Logger,
	processorNames []string,
	ps map[string]map[string]interface{},
	tcs map[string]*types.TargetConfig,
) ([]formatters.EventProcessor, error) {
	evps := make([]formatters.EventProcessor, len(processorNames))
	for i, epName := range processorNames {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType], formatters.WithLogger(logger), formatters.WithTargets(tcs))
				if err != nil {
					return nil, fmt.Errorf("failed initializing event processor %q of type=%q: %w", epName, epType, err)
				}
				evps[i] = ep
				logger.Printf("added event processor %q of type=%q to kafka input", epName, epType)
			}
		}
	}
	return evps, nil
}
