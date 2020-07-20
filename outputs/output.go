package outputs

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

type Output interface {
	Init(map[string]interface{}, *log.Logger) error
	Write(proto.Message, Meta)
	Close() error
	Metrics() []prometheus.Collector
	String() string
}
type Initializer func() Output

var Outputs = map[string]Initializer{}

func Register(name string, initFn Initializer) {
	Outputs[name] = initFn
}

type Meta map[string]string
