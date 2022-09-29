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

	"github.com/openconfig/gnmic/outputs"
	"github.com/openconfig/gnmic/types"
)

type Input interface {
	Start(context.Context, string, map[string]interface{}, ...Option) error
	Close() error
	SetLogger(*log.Logger)
	SetOutputs(map[string]outputs.Output)
	SetEventProcessors(map[string]map[string]interface{}, *log.Logger, map[string]*types.TargetConfig)
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

type Option func(Input)

func WithLogger(logger *log.Logger) Option {
	return func(i Input) {
		i.SetLogger(logger)
	}
}

func WithOutputs(outs map[string]outputs.Output) Option {
	return func(i Input) {
		i.SetOutputs(outs)
	}
}

func WithName(name string) Option {
	return func(i Input) {
		i.SetName(name)
	}
}

func WithEventProcessors(eps map[string]map[string]interface{}, log *log.Logger, tcs map[string]*types.TargetConfig) Option {
	return func(i Input) {
		i.SetEventProcessors(eps, log, tcs)
	}
}
