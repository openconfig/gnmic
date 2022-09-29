// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_delete

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"regexp"

	"github.com/openconfig/gnmic/formatters"
	"github.com/openconfig/gnmic/types"
	"github.com/openconfig/gnmic/utils"
)

const (
	processorType = "event-delete"
	loggingPrefix = "[" + processorType + "] "
)

// Delete, deletes ALL the tags or values matching one of the regexes
type Delete struct {
	Tags       []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	TagNames   []string `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tags   []*regexp.Regexp
	values []*regexp.Regexp

	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &Delete{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (d *Delete) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, d)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(d)
	}
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
	if d.logger.Writer() != io.Discard {
		b, err := json.Marshal(d)
		if err != nil {
			d.logger.Printf("initialized processor '%s': %+v", processorType, d)
			return nil
		}
		d.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (d *Delete) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		for k, v := range e.Values {
			for _, re := range d.valueNames {
				if re.MatchString(k) {
					d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
					delete(e.Values, k)
				}
			}
			for _, re := range d.values {
				if vs, ok := v.(string); ok {
					if re.MatchString(vs) {
						d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
						delete(e.Values, k)
					}
				}
			}
		}
		for k, v := range e.Tags {
			for _, re := range d.tagNames {
				if re.MatchString(k) {
					d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
					delete(e.Tags, k)
				}
			}
			for _, re := range d.tags {
				if re.MatchString(v) {
					d.logger.Printf("key '%s' matched regex '%s'", k, re.String())
					delete(e.Tags, k)
				}
			}
		}
	}
	return es
}

func (d *Delete) WithLogger(l *log.Logger) {
	if d.Debug && l != nil {
		d.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if d.Debug {
		d.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (d *Delete) WithTargets(tcs map[string]*types.TargetConfig) {}

func (d *Delete) WithActions(act map[string]map[string]interface{}) {}
