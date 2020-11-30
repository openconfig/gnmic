package processors

import (
	"github.com/karimra/gnmic/formatters"
	"github.com/mitchellh/mapstructure"
)

var EventProcessors = map[string]EventProcessor{}

type EventProcessor interface {
	Init(interface{}) error
	Apply(*formatters.EventMsg) *formatters.EventMsg
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
