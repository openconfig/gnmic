package outputs

type Output interface {
	Initialize(map[string]interface{}) error
	Write([]byte)
	Start()
	Close() error
}
type Initializer func() Output

var Outputs = map[string]Initializer{}

func Register(name string, initFn Initializer) {
	Outputs[name] = initFn
}
