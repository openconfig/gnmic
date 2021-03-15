package actions

import (
	"log"

	"github.com/karimra/gnmic/formatters"
	"github.com/mitchellh/mapstructure"
)

type Action interface {
	Init(map[string]interface{}, *log.Logger) error
	Run(*formatters.EventMsg) (interface{}, error)
}

var Actions = map[string]Initializer{}

type Initializer func() Action

func Register(name string, initFn Initializer) {
	Actions[name] = initFn
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
