package formatters

import (
	"log"

	"github.com/mitchellh/mapstructure"
)

var EventProcessors = map[string]Initializer{}

var EventProcessorTypes = []string{
	"event-add-tag",
	"event-convert",
	"event-date-string",
	"event-delete",
	"event-drop",
	"event-override-ts",
	"event-strings",
	"event-to-tag",
	"event-write",
	"event-merge",
}

type Initializer func() EventProcessor

func Register(name string, initFn Initializer) {
	EventProcessors[name] = initFn
}

type Option func(EventProcessor)
type EventProcessor interface {
	Init(interface{}, ...Option) error
	Apply(...*EventMsg) []*EventMsg

	WithTargets(map[string]interface{})
	WithLogger(l *log.Logger)
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
func WithLogger(l *log.Logger) Option {
	return func(p EventProcessor) {
		p.WithLogger(l)
	}
}
func WithTargets(tcs map[string]interface{}) Option {
	return func(p EventProcessor) {
		p.WithTargets(tcs)
	}
}
