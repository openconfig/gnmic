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
	"github.com/openconfig/gnmic/pkg/pipeline"
	pkgutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
	"google.golang.org/protobuf/proto"
)

type Input interface {
	// Start initializes the input and starts it.
	Start(context.Context, string, map[string]any, ...Option) error
	// Validate validates the input configuration.
	Validate(map[string]any) error
	// Update updates the input configuration in place for
	// a running input.
	Update(map[string]any) error
	// UpdateProcessor updates the named processor configuration
	// for a running input.
	// if the processor is not used by the Input, it will be ignored.
	UpdateProcessor(string, map[string]any) error
	// Close stops the input.
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

type BaseInput struct {
}

func (b *BaseInput) Start(context.Context, string, map[string]any, ...Option) error {
	return nil
}

func (b *BaseInput) Validate(map[string]any) error {
	return nil
}

func (b *BaseInput) Update(map[string]any) error {
	return nil
}

func (b *BaseInput) UpdateProcessor(string, map[string]any) error {
	return nil
}

func (b *BaseInput) Close() error {
	return nil
}

func UpdateProcessorInSlice(
	logger *log.Logger,
	storeObj store.Store[any],
	eventProcessors []string,
	currentEvps []formatters.EventProcessor,
	processorName string,
	pcfg map[string]any,
) ([]formatters.EventProcessor, bool, error) {
	tcs, ps, acts, err := pkgutils.GetConfigMaps(storeObj)
	if err != nil {
		return nil, false, err
	}

	for i, epName := range eventProcessors {
		if epName == processorName {
			ep, err := formatters.MakeProcessor(logger, processorName, pcfg, ps, tcs, acts)
			if err != nil {
				return nil, false, err
			}

			if i >= len(currentEvps) {
				return nil, false, fmt.Errorf("output processors are not properly initialized")
			}

			// create new slice with updated processor
			newEvps := make([]formatters.EventProcessor, len(currentEvps))
			copy(newEvps, currentEvps)
			newEvps[i] = ep

			logger.Printf("updated event processor %s", processorName)
			return newEvps, true, nil
		}
	}

	// processor not found - return currentEvps
	return currentEvps, false, nil
}
