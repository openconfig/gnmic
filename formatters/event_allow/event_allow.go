// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_allow

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/openconfig/gnmic/formatters"
	"github.com/openconfig/gnmic/types"
	"github.com/openconfig/gnmic/utils"
)

const (
	processorType = "event-allow"
	loggingPrefix = "[" + processorType + "] "
)

// Allow Allows the msg if ANY of the Tags or Values regexes are matched
type Allow struct {
	Condition  string   `mapstructure:"condition,omitempty"`
	TagNames   []string `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Tags       []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp
	tags       []*regexp.Regexp
	values     []*regexp.Regexp
	code       *gojq.Code
	logger     *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &Allow{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (d *Allow) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, d)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(d)
	}
	d.Condition = strings.TrimSpace(d.Condition)
	q, err := gojq.Parse(d.Condition)
	if err != nil {
		return err
	}
	d.code, err = gojq.Compile(q)
	if err != nil {
		return err
	}
	// init tag keys regex
	d.tagNames = make([]*regexp.Regexp, 0, len(d.TagNames))
	for _, reg := range d.TagNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tagNames = append(d.tagNames, re)
	}
	d.tags = make([]*regexp.Regexp, 0, len(d.Tags))
	for _, reg := range d.Tags {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.tags = append(d.tags, re)
	}
	//
	d.valueNames = make([]*regexp.Regexp, 0, len(d.ValueNames))
	for _, reg := range d.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.valueNames = append(d.valueNames, re)
	}

	d.values = make([]*regexp.Regexp, 0, len(d.values))
	for _, reg := range d.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		d.values = append(d.values, re)
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

func (d *Allow) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	i := 0
	for _, e := range es {
		if d.allow(e) {
			es[i] = e
			i++
		}
	}
	for j := i; j < len(es); j++ {
		es[j] = nil
	}
	es = es[:i]
	return es
}

func (d *Allow) WithLogger(l *log.Logger) {
	if d.Debug && l != nil {
		d.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if d.Debug {
		d.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (d *Allow) WithTargets(tcs map[string]*types.TargetConfig) {}

func (d *Allow) WithActions(act map[string]map[string]interface{}) {}

func (d *Allow) allow(e *formatters.EventMsg) bool {
	if d.Condition != "" {
		ok, err := formatters.CheckCondition(d.code, e)
		if err != nil {
			d.logger.Printf("condition check failed: %v", err)
			return false
		}
		return ok
	}
	for k, v := range e.Values {
		for _, re := range d.valueNames {
			if re.MatchString(k) {
				d.logger.Printf("value name '%s' matched regex '%s'", k, re.String())
				return true
			}
		}
		for _, re := range d.values {
			if vs, ok := v.(string); ok {
				if re.MatchString(vs) {
					d.logger.Printf("value '%s' matched regex '%s'", v, re.String())
					return true
				}
			}
		}
	}
	for k, v := range e.Tags {
		for _, re := range d.tagNames {
			if re.MatchString(k) {
				d.logger.Printf("tag name '%s' matched regex '%s'", k, re.String())
				return true
			}
		}
		for _, re := range d.tags {
			if re.MatchString(v) {
				d.logger.Printf("tag '%s' matched regex '%s'", v, re.String())
				return true
			}
		}
	}
	return false
}
