package outputs

import (
	"context"
	"log"

	"github.com/karimra/gnmic/formatters"
	_ "github.com/karimra/gnmic/formatters/all"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

type Output interface {
	Init(context.Context, string, map[string]interface{}, ...Option) error
	Write(context.Context, proto.Message, Meta)
	WriteEvent(context.Context, *formatters.EventMsg)
	Close() error
	RegisterMetrics(*prometheus.Registry)
	String() string

	SetLogger(*log.Logger)
	SetEventProcessors(map[string]map[string]interface{}, *log.Logger)
	SetName(string)
	SetClusterName(string)
}

type Initializer func() Output

var Outputs = map[string]Initializer{}

var OutputTypes = []string{
	"file",
	"influxdb",
	"kafka",
	"nats",
	"prometheus",
	"stan",
	"tcp",
	"udp",
}

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

