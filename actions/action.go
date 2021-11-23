package actions

import (
	"log"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/mitchellh/mapstructure"
)

type Action interface {
	Init(map[string]interface{}, ...Option) error
	Run(*formatters.EventMsg, map[string]interface{}, map[string]interface{}) (interface{}, error)
	NName() string

	WithTargets(map[string]*types.TargetConfig)
	WithLogger(*log.Logger)
}

type Option func(Action)

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

func WithTargets(tcs map[string]*types.TargetConfig) Option {
	return func(a Action) {
		a.WithTargets(tcs)
	}
}

func WithLogger(l *log.Logger) Option {
	return func(a Action) {
		a.WithLogger(l)
	}
}

type Input struct {
	Event  *formatters.EventMsg
	Env    map[string]interface{}
	Vars   map[string]interface{}
	Target string
}
