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

	"github.com/openconfig/gnmic/pkg/config/store"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"google.golang.org/protobuf/proto"
)

type Input interface {
	Start(context.Context, string, map[string]any, ...Option) error
	Close() error
}

type Initializer func() Input

var InputTypes = []string{
	"nats",
	"kafka",
	"jetstream",
}

var Inputs = map[string]Initializer{}

func Register(name string, initFn Initializer) {
	Inputs[name] = initFn
}

type InputOptions struct {
	Logger   *log.Logger
	Outputs  map[string]outputs.Output
	Name     string
	Store    store.Store[any]
	Pipeline chan *pipeline.Msg
}

type PipeMessage interface {
	Proto() proto.Message
	Meta() outputs.Meta
	Events() []*formatters.EventMsg
	Outputs() map[string]struct{}
}

type Option func(*InputOptions) error

func WithLogger(logger *log.Logger) Option {
	return func(i *InputOptions) error {
		i.Logger = logger
		return nil
	}
}

func WithOutputs(outs map[string]outputs.Output) Option {
	return func(i *InputOptions) error {
		i.Outputs = outs
		return nil
	}
}

func WithName(name string) Option {
	return func(i *InputOptions) error {
		i.Name = name
		return nil
	}
}

func WithConfigStore(st store.Store[any]) Option {
	return func(i *InputOptions) error {
		i.Store = st
		return nil
	}
}

func WithPipeline(pipeline chan *pipeline.Msg) Option {
	return func(i *InputOptions) error {
		i.Pipeline = pipeline
		return nil
	}
}
