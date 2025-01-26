// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_value_tag

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
)

const (
	processorType = "event-value-tag"
	loggingPrefix = "[" + processorType + "] "
)

type valueTag struct {
	TagName   string `mapstructure:"tag-name,omitempty" json:"tag-name,omitempty"`
	ValueName string `mapstructure:"value-name,omitempty" json:"value-name,omitempty"`
	Consume   bool   `mapstructure:"consume,omitempty" json:"consume,omitempty"`
	Debug     bool   `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	logger    *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &valueTag{logger: log.New(io.Discard, "", 0)}
	})
}

func (vt *valueTag) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, vt)
	if err != nil {
		return err
	}
	if vt.TagName == "" {
		vt.TagName = vt.ValueName
	}
	for _, opt := range opts {
		opt(vt)
	}

	if vt.logger.Writer() != io.Discard {
		b, err := json.Marshal(vt)
		if err != nil {
			vt.logger.Printf("initialized processor '%s': %+v", processorType, vt)
			return nil
		}
		vt.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}
	return nil
}

type tagVal struct {
	tags  map[string]string
	value interface{}
}

func (vt *valueTag) Apply(evs ...*formatters.EventMsg) []*formatters.EventMsg {
	vts := vt.buildApplyRules(evs)
	for _, tv := range vts {
		for _, ev := range evs {
			match := compareTags(tv.tags, ev.Tags)
			if match {
				switch v := tv.value.(type) {
				case string:
					ev.Tags[vt.TagName] = v
				default:
					ev.Tags[vt.TagName] = fmt.Sprint(tv.value)
				}
			}
		}
	}
	return evs
}

func (vt *valueTag) WithLogger(l *log.Logger) {
	if vt.Debug && l != nil {
		vt.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if vt.Debug {
		vt.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (vt *valueTag) WithTargets(tcs map[string]*types.TargetConfig) {}

func (vt *valueTag) WithActions(act map[string]map[string]interface{}) {}

// returns true if all keys match, false otherwise.
func compareTags(a map[string]string, b map[string]string) bool {
	if len(a) > len(b) {
		return false
	}
	for k, v := range a {
		if vv, ok := b[k]; !ok || v != vv {
			return false
		}
	}
	return true
}

func (vt *valueTag) WithProcessors(procs map[string]map[string]any) {}

func (vt *valueTag) buildApplyRules(evs []*formatters.EventMsg) []*tagVal {
	toApply := make([]*tagVal, 0)
	for _, ev := range evs {
		if v, ok := ev.Values[vt.ValueName]; ok {
			toApply = append(toApply,
				&tagVal{
					tags:  copyTags(ev.Tags),
					value: v,
				})
			if vt.Consume {
				delete(ev.Values, vt.ValueName)
			}
		}
	}
	return toApply
}

func copyTags(src map[string]string) map[string]string {
	dest := make(map[string]string, len(src))
	for k, v := range src {
		dest[k] = v
	}
	return dest
}
