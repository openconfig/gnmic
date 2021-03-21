package formatters

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/itchyny/gojq"
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
	"event-trigger",
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

func CheckCondition(code *gojq.Code, e *EventMsg) (bool, error) {
	var res interface{}
	if code != nil {
		input := make(map[string]interface{})
		b, err := json.Marshal(e)
		if err != nil {
			return false, err
		}
		err = json.Unmarshal(b, &input)
		if err != nil {
			return false, err
		}
		iter := code.Run(input)
		if err != nil {
			return false, err
		}
		var ok bool
		res, ok = iter.Next()
		// iterator not done, so the final result won't be a boolean
		if !ok {
			//
			return false, nil
		}
		if err, ok = res.(error); ok {
			return false, err
		}
	}
	switch res := res.(type) {
	case bool:
		return res, nil
	default:
		return false, errors.New("unexpected condition return type")
	}
}
