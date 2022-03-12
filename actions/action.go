package actions

import (
	"log"

	"github.com/karimra/gnmic/types"
	"github.com/mitchellh/mapstructure"
)

type Action interface {
	// Init initializes an Action given its configuration and a list of options
	Init(cfg map[string]interface{}, opts ...Option) error
	// Run, well runs the action.
	// it takes an action Context which is made of:
	//  - `Input`  : an interface{} event message, target name added/deleted,...
	//  - `Env`    : a map[string]interface{} containing the output of previous actions
	//  - `Vars`   : a map[string]interface{} containing variables passed to the action
	//  - `Targets`: a map[string]*types.TargetConfig containing (if the action is ran by a loader)
	//               the currently known targets configurations
	Run(aCtx *Context) (interface{}, error)
	// NName returns the configured action name
	NName() string
	// WithTargets passes the known configured targets to the action when initialized
	WithTargets(map[string]*types.TargetConfig)
	// WithLogger passes the configured logger to the action
	WithLogger(*log.Logger)
}

// Context defines an action execution context
type Context struct {
	// Input event message, target name added/deleted,...
	Input interface{} `json:"Input,omitempty"`
	// Env used to store the output of a sequence of actions
	Env map[string]interface{} `json:"Env,omitempty"`
	// Vars contains the variables passed to the action
	Vars map[string]interface{} `json:"Vars,omitempty"`
	// a map of known targets configurations
	Targets map[string]*types.TargetConfig `json:"Targets,omitempty"`
}

var ActionTypes = []string{
	"gnmi",
	"http",
	"script",
	"template",
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
