// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

// Package event_keep implements an event processor that retains matching
// values and tags while removing the rest.
package event_keep

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const processorType = "event-keep"

type keep struct {
	formatters.BaseProcessor

	Tags       []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	TagNames   []string `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tags       []*regexp.Regexp
	values     []*regexp.Regexp
	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &keep{}
	})
}

func (p *keep) Init(cfg any, opts ...formatters.Option) error {
	if err := formatters.DecodeConfig(cfg, p); err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.Logger == nil {
		p.Logger = logging.DiscardLogger()
	}
	p.Logger = p.Logger.With("processor", processorType)

	var err error
	if p.tags, err = compilePatterns("tags", p.Tags); err != nil {
		return err
	}
	if p.values, err = compilePatterns("values", p.Values); err != nil {
		return err
	}
	if p.tagNames, err = compilePatterns("tag-names", p.TagNames); err != nil {
		return err
	}
	if p.valueNames, err = compilePatterns("value-names", p.ValueNames); err != nil {
		return err
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

func compilePatterns(field string, patterns []string) ([]*regexp.Regexp, error) {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid %s pattern %q: %w", field, pattern, err)
		}
		result = append(result, re)
	}
	return result, nil
}

func (p *keep) Apply(events ...*formatters.EventMsg) []*formatters.EventMsg {
	filterValues := len(p.valueNames) > 0 || len(p.values) > 0
	filterTags := len(p.tagNames) > 0 || len(p.tags) > 0
	if !filterValues && !filterTags {
		return events
	}
	kept := events[:0]
	for _, event := range events {
		if event == nil {
			kept = append(kept, event)
			continue
		}
		removedValues, removedTags := 0, 0
		if filterValues {
			for name, value := range event.Values {
				if matchesAny(name, p.valueNames) || matchesString(value, p.values) {
					continue
				}
				delete(event.Values, name)
				removedValues++
			}
		}
		if filterTags {
			for name, value := range event.Tags {
				if matchesAny(name, p.tagNames) || matchesAny(value, p.tags) {
					continue
				}
				delete(event.Tags, name)
				removedTags++
			}
		}
		if removedValues > 0 || removedTags > 0 {
			p.Logger.Debug("removed unmatched event fields", "values", removedValues, "tags", removedTags)
		}
		if len(event.Values) == 0 && len(event.Tags) == 0 && len(event.Deletes) == 0 {
			p.Logger.Debug("removed empty event")
			continue
		}
		kept = append(kept, event)
	}
	clear(events[len(kept):])
	return kept
}

func matchesString(value any, patterns []*regexp.Regexp) bool {
	text, ok := value.(string)
	return ok && matchesAny(text, patterns)
}

func matchesAny(value string, patterns []*regexp.Regexp) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func (p *keep) WithLogger(logger *slog.Logger) {
	if !p.Debug {
		logger = nil
	}
	p.BaseProcessor.WithLogger(logger)
}
