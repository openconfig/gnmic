package loaders

import (
	"context"

	"github.com/karimra/gnmic/collector"
	"github.com/mitchellh/mapstructure"
)

type TargetLoader interface {
	Init(context.Context, map[string]interface{}) error
	Start(context.Context) chan *TargetOperation
}

type Initializer func() TargetLoader

var Loaders = map[string]Initializer{}

var LoadersTypes = []string{
	"file",
	"consul",
}

func Register(name string, initFn Initializer) {
	Loaders[name] = initFn
}

type TargetOperation struct {
	Add []*collector.TargetConfig
	Del []string
}

func DecodeConfig(src, dst interface{}) error {
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     dst,
		},
	)
	if err != nil {
		return err
	}
	return decoder.Decode(src)
}
