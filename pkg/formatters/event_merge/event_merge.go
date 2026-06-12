// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_merge

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const processorType = "event-merge"

// merge merges a list of event messages into one or multiple messages based on some criteria
type merge struct {
	formatters.BaseProcessor
	Always bool `mapstructure:"always,omitempty" json:"always,omitempty"`
	Debug  bool `mapstructure:"debug,omitempty" json:"debug,omitempty"`
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &merge{}
	})
}

func (p *merge) Init(cfg interface{}, opts ...formatters.Option) error {
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

	if p.Logger.Enabled(context.Background(), slog.LevelDebug) {
		if b, err := json.Marshal(p); err == nil {
			p.Logger.Debug("initialized processor", "config", string(b))
		} else {
			p.Logger.Debug("initialized processor", "config", p)
		}
	}
	return nil
}

func (p *merge) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	if len(es) == 0 {
		return nil
	}
	if p.Always {
		for i, e := range es {
			if e == nil {
				continue
			}
			if i > 0 {
				mergeEvents(es[0], e)
			}
		}
		return []*formatters.EventMsg{es[0]}
	}
	result := make([]*formatters.EventMsg, 0, len(es))
	timestamps := make(map[int64]int)
	for _, e := range es {
		if e == nil {
			continue
		}
		if idx, ok := timestamps[e.Timestamp]; ok {
			mergeEvents(result[idx], e)
			continue
		}
		result = append(result, e)
		timestamps[e.Timestamp] = len(result) - 1
	}
	return result
}

func (p *merge) WithLogger(l *slog.Logger) {
	if !p.Debug {
		l = nil
	}
	p.BaseProcessor.WithLogger(l)
}

func mergeEvents(e1, e2 *formatters.EventMsg) {
	if e1.Tags == nil {
		e1.Tags = make(map[string]string)
	}
	if e1.Values == nil {
		e1.Values = make(map[string]interface{})
	}
	for n, t := range e2.Tags {
		e1.Tags[n] = t
	}
	for n, v := range e2.Values {
		e1.Values[n] = v
	}
	e1.Deletes = append(e1.Deletes, e2.Deletes...)
	if e2.Timestamp > e1.Timestamp {
		e1.Timestamp = e2.Timestamp
	}
}
