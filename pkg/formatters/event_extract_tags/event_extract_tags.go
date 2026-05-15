// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_extract_tags

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const (
	processorType = "event-extract-tags"
)

// extractTags extracts tags from a value, a value name, a tag name or a tag value using regex named groups
type extractTags struct {
	formatters.BaseProcessor
	Tags       []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	TagNames   []string `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Overwrite  bool     `mapstructure:"overwrite,omitempty" json:"overwrite,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tags       []*regexp.Regexp
	values     []*regexp.Regexp
	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &extractTags{}
	})
}

func (p *extractTags) Init(cfg interface{}, opts ...formatters.Option) error {
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
	// init tags regex
	p.tags = make([]*regexp.Regexp, 0, len(p.Tags))
	for _, reg := range p.Tags {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.tags = append(p.tags, re)
	}
	// init tag names regex
	p.tagNames = make([]*regexp.Regexp, 0, len(p.TagNames))
	for _, reg := range p.TagNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.tagNames = append(p.tagNames, re)
	}
	// init values regex
	p.values = make([]*regexp.Regexp, 0, len(p.Values))
	for _, reg := range p.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.values = append(p.values, re)
	}
	// init value names regex
	p.valueNames = make([]*regexp.Regexp, 0, len(p.ValueNames))
	for _, reg := range p.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		p.valueNames = append(p.valueNames, re)
	}

	if p.Logger.Enabled(context.Background(), slog.LevelDebug) {
		if b, err := json.Marshal(p); err == nil {
			p.Logger.Debug("initialized processor", "config", string(b))
		} else {
			p.Logger.Debug("initialized processor", "config", p)
		}
	}
	return nil
}

func (p *extractTags) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		for k, v := range e.Values {
			for _, re := range p.valueNames {
				p.addTags(e, re, k)
			}
			for _, re := range p.values {
				if vs, ok := v.(string); ok {
					p.addTags(e, re, vs)
				}
			}
		}
		for k, v := range e.Tags {
			for _, re := range p.tagNames {
				p.addTags(e, re, k)
			}
			for _, re := range p.tags {
				p.addTags(e, re, v)
			}
		}
	}
	return es
}

func (p *extractTags) WithLogger(l *slog.Logger) {
	if !p.Debug {
		l = nil
	}
	p.BaseProcessor.WithLogger(l)
}

func (p *extractTags) addTags(e *formatters.EventMsg, re *regexp.Regexp, s string) {
	if e.Tags == nil {
		e.Tags = make(map[string]string)
	}

	matches := re.FindStringSubmatch(s)
	p.Logger.Debug("regex matches", "matches", matches)
	if len(matches) != len(re.SubexpNames()) {
		return
	}
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			p.Logger.Debug("adding extracted tag", "name", name, "value", matches[i])
			if p.Overwrite {
				e.Tags[name] = matches[i]
				continue
			}
			if _, ok := e.Tags[matches[i]]; !ok {
				e.Tags[name] = matches[i]
			}
		}
	}
}
