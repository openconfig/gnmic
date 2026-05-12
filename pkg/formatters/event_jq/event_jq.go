// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_jq

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/itchyny/gojq"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const (
	processorType     = "event-jq"
	defaultCondition  = "all([true])"
	defaultExpression = "."
)

// jq runs a jq expression on the received event messages
type jq struct {
	formatters.BaseProcessor
	Condition  string `mapstructure:"condition,omitempty"`
	Expression string `mapstructure:"expression,omitempty"`
	Debug      bool   `mapstructure:"debug,omitempty"`

	cond *gojq.Code
	expr *gojq.Code
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &jq{}
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
	if p.Logger == nil {
		p.Logger = logging.DiscardLogger()
	}
	p.Logger = p.Logger.With("processor", processorType)
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
			p.Logger.Warn("failed to evaluate condition", "err", err)
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
		p.Logger.Warn("failed to apply jq expression", "err", err)
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
		p.Logger.Debug("condition jq result", "result_type", fmt.Sprintf("%T", res), "result", res, "input", input)
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
	for {
		r, ok := iter.Next()
		if !ok {
			p.Logger.Debug("iter status", "done", ok, "r", r)
			break
		}
		p.Logger.Debug("iter result", "type", fmt.Sprintf("%T", r), "value", r)
		switch r := r.(type) {
		case error:
			return nil, err
		default:
			p.Logger.Debug("adding result", "value", r)
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
					p.Logger.Warn("unexpected type", "type", fmt.Sprintf("%T", ee), "value", ee)
				}
			}
		case map[string]interface{}:
			ev, err := formatters.EventFromMap(es)
			if err != nil {
				return nil, err
			}
			evs = append(evs, ev)
		default:
			p.Logger.Warn("unexpected type", "type", fmt.Sprintf("%T", e), "value", e)
		}
	}
	return evs, nil
}

func (p *jq) WithLogger(l *slog.Logger) {
	if !p.Debug {
		l = nil
	}
	p.BaseProcessor.WithLogger(l)
}
