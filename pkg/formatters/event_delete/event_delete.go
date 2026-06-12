// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_delete

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const (
	processorType = "event-delete"
)

// deletep, deletes ALL the tags or values matching one of the regexes
type deletep struct {
	formatters.BaseProcessor
	Tags       []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	TagNames   []string `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tags   []*regexp.Regexp
	values []*regexp.Regexp

	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &deletep{}
	})
}

func (d *deletep) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, d)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(d)
	}
	if d.Logger == nil {
		d.Logger = logging.DiscardLogger()
	}
	d.Logger = d.Logger.With("processor", processorType)
	// init tags regex
	d.tags = make([]*regexp.Regexp, 0, len(d.Tags))
	for _, reg := range d.Tags {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tags = append(d.tags, re)
	}
	// init tag names regex
	d.tagNames = make([]*regexp.Regexp, 0, len(d.TagNames))
	for _, reg := range d.TagNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tagNames = append(d.tagNames, re)
	}
	// init values regex
	d.values = make([]*regexp.Regexp, 0, len(d.Values))
	for _, reg := range d.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.values = append(d.values, re)
	}
	// init values names regex
	d.valueNames = make([]*regexp.Regexp, 0, len(d.ValueNames))
	for _, reg := range d.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.valueNames = append(d.valueNames, re)
	}
	if d.Logger.Enabled(context.Background(), slog.LevelDebug) {
		if b, err := json.Marshal(d); err == nil {
			d.Logger.Debug("initialized processor", "config", string(b))
		} else {
			d.Logger.Debug("initialized processor", "config", d)
		}
	}
	return nil
}

func (d *deletep) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		for k, v := range e.Values {
			for _, re := range d.valueNames {
				if re.MatchString(k) {
					d.Logger.Debug("key matched regex", "key", k, "regex", re.String())
					delete(e.Values, k)
				}
			}
			for _, re := range d.values {
				if vs, ok := v.(string); ok {
					if re.MatchString(vs) {
						d.Logger.Debug("key matched regex", "key", k, "regex", re.String())
						delete(e.Values, k)
					}
				}
			}
		}
		for k, v := range e.Tags {
			for _, re := range d.tagNames {
				if re.MatchString(k) {
					d.Logger.Debug("key matched regex", "key", k, "regex", re.String())
					delete(e.Tags, k)
				}
			}
			for _, re := range d.tags {
				if re.MatchString(v) {
					d.Logger.Debug("key matched regex", "key", k, "regex", re.String())
					delete(e.Tags, k)
				}
			}
		}
	}
	return es
}

func (d *deletep) WithLogger(l *slog.Logger) {
	if !d.Debug {
		l = nil
	}
	d.BaseProcessor.WithLogger(l)
}
