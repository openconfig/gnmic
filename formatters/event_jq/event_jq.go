package event_jq

import (
	"errors"
	"io"
	"log"
	"os"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

const (
	processorType     = "event-jq"
	loggingPrefix     = "[" + processorType + "] "
	defaultCondition  = "all([true])"
	defaultExpression = "."
)

// jq runs a jq expression on the received event messages
type jq struct {
	Condition  string `mapstructure:"condition,omitempty"`
	Expression string `mapstructure:"expression,omitempty"`
	Debug      bool   `mapstructure:"debug,omitempty"`

	cond   *gojq.Code
	expr   *gojq.Code
	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &jq{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *jq) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
	}
	p.setDefaults()
	p.Condition = strings.TrimSpace(p.Condition)
	q, err := gojq.Parse(p.Condition)
	if err != nil {
		return err
	}
	p.cond, err = gojq.Compile(q)
	if err != nil {
		return err
	}

	p.Expression = strings.TrimSpace(p.Expression)
	q, err = gojq.Parse(p.Expression)
	if err != nil {
		return err
	}
	p.expr, err = gojq.Compile(q)
	if err != nil {
		return err
	}
	return nil
}

func (p *jq) setDefaults() {
	if p.Condition == "" {
		p.Condition = defaultCondition
	}
	if p.Expression == "" {
		p.Expression = defaultExpression
	}
}

func (p *jq) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	nuMsgs := len(es)
	inputs := make([]interface{}, 0, nuMsgs)
	res := make([]*formatters.EventMsg, 0, nuMsgs)
	for _, e := range es {
		if e == nil {
			continue
		}
		input := e.ToMap()
		ok, err := p.evaluateCondition(input)
		if err != nil {
			p.logger.Printf("failed to evaluate condition: %v", err)
			continue
		}
		if ok {
			inputs = append(inputs, input)
			continue
		}
		res = append(res, e)
	}
	evs, err := p.applyExpression(inputs)
	if err != nil {
		p.logger.Printf("failed to apply jq expression: %v", err)
		return nil
	}
	return append(res, evs...)
}

func (p *jq) evaluateCondition(input map[string]interface{}) (bool, error) {
	var res interface{}
	var err error
	if p.cond != nil {
		iter := p.cond.Run(input)
		var ok bool
		res, ok = iter.Next()
		if !ok {
			// iterator not done, so the final result won't be a boolean
			return false, nil
		}
		if err, ok = res.(error); ok {
			return false, err
		}
		p.logger.Printf("condition jq result: (%T)%v for input %+v", res, res, input)
	}
	switch res := res.(type) {
	case bool:
		return res, nil
	default:
		return false, errors.New("unexpected condition return type")
	}
}

func (p *jq) applyExpression(input []interface{}) ([]*formatters.EventMsg, error) {
	var res []interface{}
	var err error
	var evs = make([]*formatters.EventMsg, 0)
	iter := p.expr.Run(input)
	if err != nil {
		return nil, err
	}
	for {
		r, ok := iter.Next()
		if !ok {
			p.logger.Printf("iter done? %v | r=%v", ok, r)
			break
		}
		p.logger.Printf("iter result: (%T)%+v\n", r, r)
		switch r := r.(type) {
		case error:
			return nil, err
		default:
			p.logger.Printf("adding %+v\n", r)
			res = append(res, r)
		}
	}
	for _, e := range res {
		switch es := e.(type) {
		case []interface{}:
			for _, ee := range es {
				switch ee := ee.(type) {
				case map[string]interface{}:
					ev, err := formatters.EventFromMap(ee)
					if err != nil {
						return nil, err
					}
					evs = append(evs, ev)
				default:
					p.logger.Printf("unexpected type (%T)%+v", ee, ee)
				}
			}
		case map[string]interface{}:
			ev, err := formatters.EventFromMap(es)
			if err != nil {
				return nil, err
			}
			evs = append(evs, ev)
		default:
			p.logger.Printf("unexpected type (%T)%+v", e, e)
		}
	}
	return evs, nil
}

func (p *jq) WithLogger(l *log.Logger) {
	if p.Debug && l != nil {
		p.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (p *jq) WithTargets(tcs map[string]*types.TargetConfig) {}

func (p *jq) WithActions(act map[string]map[string]interface{}) {}
