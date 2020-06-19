package outputs

import "log"

type Output interface {
	Init(map[string]interface{}, *log.Logger) error
	Write([]byte)
	Close() error
}
type Initializer func() Output

var Outputs = map[string]Initializer{}

func Register(name string, initFn Initializer) {
	Outputs[name] = initFn
}
