package outputs

import (
	"context"
	"fmt"
	"log"

	_ "github.com/karimra/gnmic/formatters/all"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

type Output interface {
	Init(context.Context, map[string]interface{}, ...Option) error
	Write(context.Context, proto.Message, Meta)
	Close() error
	Metrics() []prometheus.Collector
	String() string
	SetLogger(*log.Logger)
	SetEventProcessors(map[string]map[string]interface{}, *log.Logger)
}

type Initializer func() Output

var Outputs = map[string]Initializer{}

func Register(name string, initFn Initializer) {
	Outputs[name] = initFn
}

type Meta map[string]string

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

type Option func(Output)

func WithLogger(logger *log.Logger) Option {
	return func(o Output) {
		o.SetLogger(logger)
	}
}

func WithEventProcessors(eps map[string]map[string]interface{}, log *log.Logger) Option {
	return func(o Output) {
		fmt.Println("adding event processors to output:", eps)
		o.SetEventProcessors(eps, log)
	}
}
