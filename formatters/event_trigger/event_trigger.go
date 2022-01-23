package event_trigger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/itchyny/gojq"
	"github.com/karimra/gnmic/actions"
	_ "github.com/karimra/gnmic/actions/all"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"gopkg.in/yaml.v2"
)

const (
	processorType    = "event-trigger"
	loggingPrefix    = "[" + processorType + "] "
	defaultCondition = "any([true])"
)

// Trigger triggers an action when certain conditions are met
type Trigger struct {
	Condition      string                 `mapstructure:"condition,omitempty"`
	MinOccurrences int                    `mapstructure:"min-occurrences,omitempty"`
	MaxOccurrences int                    `mapstructure:"max-occurrences,omitempty"`
	Window         time.Duration          `mapstructure:"window,omitempty"`
	Actions        []string               `mapstructure:"actions,omitempty"`
	Vars           map[string]interface{} `mapstructure:"vars,omitempty"`
	VarsFile       string                 `mapstructure:"vars-file,omitempty"`
	Debug          bool                   `mapstructure:"debug,omitempty"`
	Async          bool                   `mapstructure:"async,omitempty"`

	occurrencesTimes []time.Time
	lastTrigger      time.Time
	code             *gojq.Code
	actions          []actions.Action
	vars             map[string]interface{}

	targets map[string]*types.TargetConfig
	acts    map[string]map[string]interface{}
	logger  *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &Trigger{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *Trigger) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
	}

	err = p.setDefaults()
	if err != nil {
		return err
	}

	p.Condition = strings.TrimSpace(p.Condition)
	q, err := gojq.Parse(p.Condition)
	if err != nil {
		return err
	}
	p.code, err = gojq.Compile(q)
	if err != nil {
		return err
	}

	for _, name := range p.Actions {
		if actCfg, ok := p.acts[name]; ok {
			err = p.initializeAction(actCfg)
			if err != nil {
				return err
			}
			continue
		}
		return fmt.Errorf("failed to initialize action %q: config not found", name)
	}
	err = p.readVars()
	if err != nil {
		return err
	}

	p.logger.Printf("%q initalized: %+v", processorType, p)

	return nil
}

func (p *Trigger) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	now := time.Now()
	for _, e := range es {
		if e == nil {
			continue
		}
		res, err := formatters.CheckCondition(p.code, e)
		if err != nil {
			p.logger.Printf("failed evaluating condition %q: %v", p.Condition, err)
			continue
		}
		if p.Debug {
			p.logger.Printf("msg=%+v, condition %q result: (%T)%v", e, p.Condition, res, res)
		}
		if res {
			if p.evalOccurrencesWithinWindow(now) {
				if p.Async {
					go p.triggerActions(e)
				} else {
					p.triggerActions(e)
				}
			}
		}
	}
	return es
}

func (p *Trigger) WithLogger(l *log.Logger) {
	if p.Debug && l != nil {
		p.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (p *Trigger) WithTargets(tcs map[string]*types.TargetConfig) {
	if p.Debug {
		p.logger.Printf("with targets: %+v", tcs)
	}
	p.targets = tcs
}

func (p *Trigger) WithActions(acts map[string]map[string]interface{}) {
	if p.Debug {
		p.logger.Printf("with actions: %+v", acts)
	}
	p.acts = acts
}

func (p *Trigger) initializeAction(cfg map[string]interface{}) error {
	if len(cfg) == 0 {
		return errors.New("missing action definition")
	}
	if actType, ok := cfg["type"]; ok {
		switch actType := actType.(type) {
		case string:
			if in, ok := actions.Actions[actType]; ok {
				act := in()
				err := act.Init(cfg, actions.WithLogger(p.logger), actions.WithTargets(p.targets))
				if err != nil {
					return err
				}
				p.actions = append(p.actions, act)
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

func (p *Trigger) setDefaults() error {
	if p.Condition == "" {
		p.Condition = defaultCondition
	}
	if p.MinOccurrences <= 0 {
		p.MinOccurrences = 1
	}
	if p.MaxOccurrences <= 0 {
		p.MaxOccurrences = 1
	}
	if p.MaxOccurrences < p.MinOccurrences {
		return errors.New("max-occurrences cannot be lower than min-occurrences")
	}
	if p.Window <= 0 {
		p.Window = time.Minute
	}
	return nil
}

func (p *Trigger) readVars() error {
	if p.VarsFile == "" {
		p.vars = p.Vars
		return nil
	}
	b, err := utils.ReadFile(context.TODO(), p.VarsFile)
	if err != nil {
		return err
	}
	v := make(map[string]interface{})
	err = yaml.Unmarshal(b, &v)
	if err != nil {
		return err
	}
	p.vars = utils.MergeMaps(v, p.Vars)
	return nil
}

func (p *Trigger) triggerActions(e *formatters.EventMsg) {
	actx := &actions.Context{Input: e, Env: make(map[string]interface{}), Vars: p.vars}
	for _, act := range p.actions {
		res, err := act.Run(actx)
		if err != nil {
			p.logger.Printf("trigger action %q failed: %+v", act.NName(), err)
			return
		}
		actx.Env[act.NName()] = res
		p.logger.Printf("action %q result: %+v", act.NName(), res)
	}
}

func (p *Trigger) evalOccurrencesWithinWindow(now time.Time) bool {
	if p.occurrencesTimes == nil {
		p.occurrencesTimes = make([]time.Time, 0)
	}
	occurrencesInWindow := make([]time.Time, 0, len(p.occurrencesTimes))
	if p.Debug {
		p.logger.Printf("occurrencesTimes: %v", p.occurrencesTimes)
	}
	for _, t := range p.occurrencesTimes {
		if t.Add(p.Window).After(now) {
			if p.Debug {
				p.logger.Printf("time=%s + %s is after now=%s", t, p.Window, now)
			}
			occurrencesInWindow = append(occurrencesInWindow, t)
		}
	}
	p.occurrencesTimes = append(occurrencesInWindow, now)
	numOccurrences := len(p.occurrencesTimes)
	if numOccurrences > p.MaxOccurrences {
		p.occurrencesTimes = p.occurrencesTimes[numOccurrences-p.MaxOccurrences-1:]
		numOccurrences = len(p.occurrencesTimes)
	}

	if p.Debug {
		p.logger.Printf("numOccurrences: %d", numOccurrences)
	}

	if numOccurrences >= p.MinOccurrences && numOccurrences <= p.MaxOccurrences {
		p.lastTrigger = now
		return true
	}
	// check last trigger
	if numOccurrences > p.MinOccurrences && p.lastTrigger.Add(p.Window).Before(now) {
		p.lastTrigger = now
		return true
	}
	return false
}
