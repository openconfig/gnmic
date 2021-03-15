package event_trigger

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"github.com/karimra/gnmic/actions"
	_ "github.com/karimra/gnmic/actions/all"
	"github.com/karimra/gnmic/formatters"
)

const (
	processorType = "event-trigger"
	loggingPrefix = "[" + processorType + "] "
)

// Trigger triggers an action when certain conditions are met
type Trigger struct {
	Expression string                 `mapstructure:"expression,omitempty"`
	Action     map[string]interface{} `mapstructure:"action,omitempty"`
	Debug      bool                   `mapstructure:"debug,omitempty"`

	prg    *vm.Program
	action actions.Action

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &Trigger{
			logger: log.New(ioutil.Discard, "", 0),
		}
	})
}

func (p *Trigger) Init(cfg interface{}, logger *log.Logger) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	if p.Debug && logger != nil {
		p.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, log.LstdFlags|log.Lmicroseconds)
	}

	p.prg, err = expr.Compile(p.Expression)
	if err != nil {
		return err
	}
	err = p.initializeAction(p.Action)
	if err != nil {
		return err
	}
	p.logger.Printf("%q initalized: %+v", processorType, p)
	return nil
}

func (p *Trigger) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		params := make(map[string]interface{})
		b, err := json.Marshal(e)
		if err != nil {
			p.logger.Printf("failed marshaling event message: %v", err)
			continue
		}
		err = json.Unmarshal(b, &params)
		if err != nil {
			p.logger.Printf("failed unmarshaling event message: %v", err)
			continue
		}
		res, err := expr.Run(p.prg, params)
		if err != nil {
			p.logger.Printf("failed evaluating: %v", err)
			continue
		}
		switch res := res.(type) {
		case bool:
			if res {
				go func() {
					res, err := p.action.Run(e)
					if err != nil {
						return
					}
					p.logger.Printf("result: %+v", res)
				}()
			}
		}
	}
	return nil
}

func (p *Trigger) initializeAction(cfg map[string]interface{}) error {
	if len(cfg) == 0 {
		return errors.New("missing action definition")
	}
	if actType, ok := cfg["type"]; ok {
		switch actType := actType.(type) {
		case string:
			if in, ok := actions.Actions[actType]; ok {
				p.action = in()
				err := p.action.Init(cfg, p.logger)
				if err != nil {
					return err
				}
				return nil
			}
			return fmt.Errorf("unknown action type %q", actType)
		default:
			return fmt.Errorf("unexpected action field type %T", actType)
		}
	}
	return errors.New("missing type field under action")
}

func (p *Trigger) String() string {
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(b)
}
