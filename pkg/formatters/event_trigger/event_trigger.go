// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_trigger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/itchyny/gojq"

	"github.com/openconfig/gnmic/pkg/actions"
	_ "github.com/openconfig/gnmic/pkg/actions/all"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	gfile "github.com/openconfig/gnmic/pkg/file"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const (
	processorType    = "event-trigger"
	defaultCondition = "any([true])"
)

// trigger triggers an action when certain conditions are met
type trigger struct {
	formatters.BaseProcessor

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
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &trigger{}
	})
}

func (p *trigger) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.Logger == nil {
		p.Logger = logging.DiscardLogger()
	}
	p.Logger = p.Logger.With("processor", processorType)

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

	p.Logger.Debug("initialized processor", "config", p)

	return nil
}

func (p *trigger) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	now := time.Now()
	for _, e := range es {
		if e == nil {
			continue
		}
		res, err := formatters.CheckCondition(p.code, e)
		if err != nil {
			p.Logger.Warn("failed evaluating condition", "condition", p.Condition, "err", err)
			continue
		}
		p.Logger.Debug("condition result", "msg", e, "condition", p.Condition, "result_type", fmt.Sprintf("%T", res), "result", res)
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

func (p *trigger) WithLogger(l *slog.Logger) {
	if !p.Debug {
		l = nil
	}
	p.BaseProcessor.WithLogger(l)
}

func (p *trigger) WithTargets(tcs map[string]*types.TargetConfig) {
	p.targets = tcs
}

func (p *trigger) WithActions(acts map[string]map[string]interface{}) {
	p.Logger.Debug("with actions", "actions", acts)
	p.acts = acts
}

func (p *trigger) initializeAction(cfg map[string]interface{}) error {
	if len(cfg) == 0 {
		return errors.New("missing action definition")
	}
	if actType, ok := cfg["type"]; ok {
		switch actType := actType.(type) {
		case string:
			if in, ok := actions.Actions[actType]; ok {
				act := in()
				err := act.Init(cfg, actions.WithLogger(p.Logger), actions.WithTargets(p.targets))
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

func (p *trigger) String() string {
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func (p *trigger) setDefaults() error {
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

func (p *trigger) readVars() error {
	if p.VarsFile == "" {
		p.vars = p.Vars
		return nil
	}
	b, err := gfile.ReadFile(context.TODO(), p.VarsFile)
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

func (p *trigger) triggerActions(e *formatters.EventMsg) {
	actx := &actions.Context{Input: e, Env: make(map[string]interface{}), Vars: p.vars}
	for _, act := range p.actions {
		res, err := act.Run(context.TODO(), actx)
		if err != nil {
			p.Logger.Error("trigger action failed", "action", act.NName(), "err", err)
			return
		}
		actx.Env[act.NName()] = res
		p.Logger.Debug("action result", "action", act.NName(), "result", res)
	}
}

func (p *trigger) evalOccurrencesWithinWindow(now time.Time) bool {
	if p.occurrencesTimes == nil {
		p.occurrencesTimes = make([]time.Time, 0)
	}
	occurrencesInWindow := make([]time.Time, 0, len(p.occurrencesTimes))
	p.Logger.Debug("occurrence times", "times", p.occurrencesTimes)
	for _, t := range p.occurrencesTimes {
		if t.Add(p.Window).After(now) {
			p.Logger.Debug("time + window is after now", "time", t, "window", p.Window, "now", now)
			occurrencesInWindow = append(occurrencesInWindow, t)
		}
	}
	p.occurrencesTimes = append(occurrencesInWindow, now)
	numOccurrences := len(p.occurrencesTimes)
	if numOccurrences > p.MaxOccurrences {
		p.occurrencesTimes = p.occurrencesTimes[numOccurrences-p.MaxOccurrences-1:]
		numOccurrences = len(p.occurrencesTimes)
	}

	p.Logger.Debug("num occurrences", "count", numOccurrences)

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

func (p *trigger) WithProcessors(procs map[string]map[string]any) {}
