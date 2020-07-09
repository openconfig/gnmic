package outputs

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

type Output interface {
	Init(map[string]interface{}, *log.Logger) error
	Write([]byte, Meta)
	Close() error
	Metrics() []prometheus.Collector
	String() string
}
type Initializer func() Output

var Outputs = map[string]Initializer{}

func Register(name string, initFn Initializer) {
	Outputs[name] = initFn
}

type Meta map[string]interface{}
