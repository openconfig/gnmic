// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_add_tag

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/itchyny/gojq"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
)

const (
	processorType = "event-add-tag"
	loggingPrefix = "[" + processorType + "] "
)

// addTag adds a set of tags to the event message if certain criteria's are met.
type addTag struct {
	Condition  string            `mapstructure:"condition,omitempty"`
	Tags       []string          `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Values     []string          `mapstructure:"values,omitempty" json:"values,omitempty"`
	TagNames   []string          `mapstructure:"tag-names,omitempty" json:"tag-names,omitempty"`
	ValueNames []string          `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Deletes    []string          `mapstructure:"deletes,omitempty" json:"deletes,omitempty"`
	Overwrite  bool              `mapstructure:"overwrite,omitempty" json:"overwrite,omitempty"`
	Add        map[string]string `mapstructure:"add,omitempty" json:"add,omitempty"`
	Debug      bool              `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	tags       []*regexp.Regexp
	values     []*regexp.Regexp
	tagNames   []*regexp.Regexp
	valueNames []*regexp.Regexp
	deletes    []*regexp.Regexp
	code       *gojq.Code
	logger     *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &addTag{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *addTag) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.Condition != "" {
		p.Condition = strings.TrimSpace(p.Condition)
		q, err := gojq.Parse(p.Condition)
		if err != nil {
			return err
		}
		p.code, err = gojq.Compile(q)
		if err != nil {
			return err
		}
	}
	// init tags regex
	p.tags, err = compileRegex(p.Tags)
	if err != nil {
		return err
	}
	// init tag names regex
	p.tagNames, err = compileRegex(p.TagNames)
	if err != nil {
		return err
	}
	// init values regex
	p.values, err = compileRegex(p.Values)
	if err != nil {
		return err
	}
	// init value names regex
	p.valueNames, err = compileRegex(p.ValueNames)
	if err != nil {
		return err
	}
	// init deletes regex
	p.deletes, err = compileRegex(p.Deletes)
	if err != nil {
		return err
	}
	if p.logger.Writer() != io.Discard {
		b, err := json.Marshal(p)
		if err != nil {
			p.logger.Printf("initialized processor '%s': %+v", processorType, p)
			return nil
		}
		p.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (p *addTag) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		// condition is set
		if p.code != nil && p.Condition != "" {
			ok, err := formatters.CheckCondition(p.code, e)
			if err != nil {
				p.logger.Printf("condition check failed: %v", err)
			}
			if ok {
				p.addTags(e)
			}
			continue
		}
		// no condition, check regexes
		for k, v := range e.Values {
			for _, re := range p.valueNames {
				if re.MatchString(k) {
					p.addTags(e)
					break
				}
			}
			for _, re := range p.values {
				if vs, ok := v.(string); ok {
					if re.MatchString(vs) {
						p.addTags(e)
					}
					break
				}
			}
		}
		for k, v := range e.Tags {
			for _, re := range p.tagNames {
				if re.MatchString(k) {
					p.addTags(e)
					break
				}
			}
			for _, re := range p.tags {
				if re.MatchString(v) {
					p.addTags(e)
					break
				}
			}
		}
		for _, k := range e.Deletes {
			for _, re := range p.deletes {
				if re.MatchString(k) {
					p.addTags(e)
					break
				}
			}
		}
	}
	return es
}

func (p *addTag) WithLogger(l *log.Logger) {
	if p.Debug && l != nil {
		p.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (p *addTag) WithTargets(tcs map[string]*types.TargetConfig) {}

func (p *addTag) WithActions(act map[string]map[string]interface{}) {}

func (p *addTag) WithProcessors(procs map[string]map[string]any) {}

func (p *addTag) addTags(e *formatters.EventMsg) {
	if e.Tags == nil {
		e.Tags = make(map[string]string)
	}
	for nk, nv := range p.Add {
		if p.Overwrite {
			e.Tags[nk] = nv
			continue
		}
		if _, ok := e.Tags[nk]; !ok {
			e.Tags[nk] = nv
		}
	}
}

func compileRegex(expr []string) ([]*regexp.Regexp, error) {
	res := make([]*regexp.Regexp, 0, len(expr))
	for _, reg := range expr {
		re, err := regexp.Compile(reg)
		if err != nil {
			return nil, err
		}
		res = append(res, re)
	}
	return res, nil
}
