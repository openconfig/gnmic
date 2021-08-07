package inputs

import (
	"context"
	"log"

	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
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
