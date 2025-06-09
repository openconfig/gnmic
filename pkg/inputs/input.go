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
	"log"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/outputs"
)

type Input interface {
	Start(context.Context, string, map[string]interface{}, ...Option) error
	Close() error
	SetLogger(*log.Logger)
	SetOutputs(map[string]outputs.Output)
	SetEventProcessors(map[string]map[string]interface{}, *log.Logger, map[string]*types.TargetConfig, map[string]map[string]interface{}) error
	SetName(string)
}

type Initializer func() Input

var InputTypes = []string{
	"nats",
	"stan",
	"kafka",
	"jetstream",
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

func WithEventProcessors(eps map[string]map[string]interface{}, log *log.Logger, tcs map[string]*types.TargetConfig, acts map[string]map[string]interface{}) Option {
	return func(i Input) error {
		return i.SetEventProcessors(eps, log, tcs, acts)
	}
}
