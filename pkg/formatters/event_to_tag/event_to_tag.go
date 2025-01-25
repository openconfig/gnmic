// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_to_tag

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
)

const (
	processorType = "event-to-tag"
	loggingPrefix = "[" + processorType + "] "
)

// toTag moves ALL values matching any of the regex in .Values to the EventMsg.Tags map.
// if .Keep is true, the matching values are not deleted from EventMsg.Tags
type toTag struct {
	Values     []string `mapstructure:"values,omitempty" json:"values,omitempty"`
	ValueNames []string `mapstructure:"value-names,omitempty" json:"value-names,omitempty"`
	Keep       bool     `mapstructure:"keep,omitempty" json:"keep,omitempty"`
	Debug      bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	valueNames []*regexp.Regexp
	values     []*regexp.Regexp

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &toTag{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (t *toTag) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, t)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(t)
	}
	t.valueNames = make([]*regexp.Regexp, 0, len(t.ValueNames))
	for _, reg := range t.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		t.valueNames = append(t.valueNames, re)
	}
	t.values = make([]*regexp.Regexp, 0, len(t.Values))
	for _, reg := range t.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		t.values = append(t.values, re)
	}
	if t.logger.Writer() != io.Discard {
		b, err := json.Marshal(t)
		if err != nil {
			t.logger.Printf("initialized processor '%s': %+v", processorType, t)
			return nil
		}
		t.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

func (t *toTag) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		if e.Tags == nil {
			e.Tags = make(map[string]string)
		}
		for k, v := range e.Values {
			for _, re := range t.valueNames {
				if re.MatchString(k) {
					switch v := v.(type) {
					case string:
						e.Tags[k] = v
					default:
						e.Tags[k] = fmt.Sprint(v)
					}
					if !t.Keep {
						delete(e.Values, k)
					}
				}
			}
			for _, re := range t.values {
				if vs, ok := v.(string); ok {
					if re.MatchString(vs) {
						e.Tags[k] = vs
						if !t.Keep {
							delete(e.Values, k)
						}
					}
				}
			}
		}
	}
	return es
}

func (t *toTag) Apply2(es ...*formatters.EventMsg) []*formatters.EventMsg {
	for _, e := range es {
		if e == nil {
			continue
		}
		if e.Tags == nil {
			e.Tags = make(map[string]string)
		}
		for k, v := range e.Values {
			for _, re := range t.valueNames {
				if re.MatchString(k) {
					e.Tags[k] = fmt.Sprint(v)  // always cast v results on extra allocations: Apply > Apply2
					if !t.Keep {
						delete(e.Values, k)
					}
				}
			}
			for _, re := range t.values {
				if vs, ok := v.(string); ok {
					if re.MatchString(vs) {
						e.Tags[k] = vs
						if !t.Keep {
							delete(e.Values, k)
						}
					}
				}
			}
		}
	}
	return es
}

func (t *toTag) WithLogger(l *log.Logger) {
	if t.Debug && l != nil {
		t.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if t.Debug {
		t.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (t *toTag) WithTargets(tcs map[string]*types.TargetConfig) {}

func (t *toTag) WithActions(act map[string]map[string]interface{}) {}

func (t *toTag) WithProcessors(procs map[string]map[string]any) {}
