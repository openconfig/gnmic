// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_value_tag_v2

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"slices"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
)

const (
	processorType = "event-value-tag-v2"
	loggingPrefix = "[" + processorType + "] "
)

var (
	eqByte   = []byte("=")
	semiC    = []byte(";")
	pipeByte = []byte("|")
)

type valueTag struct {
	TagName   string `mapstructure:"tag-name,omitempty" json:"tag-name,omitempty"`
	ValueName string `mapstructure:"value-name,omitempty" json:"value-name,omitempty"`
	Consume   bool   `mapstructure:"consume,omitempty" json:"consume,omitempty"`
	Debug     bool   `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	logger    *log.Logger

	applyRules map[uint64]*applyRule
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
	for _, opt := range opts {
		opt(vt)
	}

	if vt.TagName == "" {
		vt.TagName = vt.ValueName
	}

	vt.applyRules = make(map[uint64]*applyRule)

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

type applyRule struct {
	// Set of tags that must be present in a message
	// in order to add the value as tag.
	tags map[string]string
	// The value to be added as tag.
	// The tag name is taken from the main proc struct.
	value any
}

func (vt *valueTag) Apply(evs ...*formatters.EventMsg) []*formatters.EventMsg {
	vt.updateApplyRules(evs)
	for _, ar := range vt.applyRules {
		for _, ev := range evs {
			if includedIn(ar.tags, ev.Tags) {
				switch v := ar.value.(type) {
				case string:
					ev.Tags[vt.TagName] = v
				default:
					ev.Tags[vt.TagName] = fmt.Sprint(ar.value)
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

// comparison logic for maps
// i.e: a ⊆ b
func includedIn(a, b map[string]string) bool {
	if len(a) > len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || v != bv {
			return false
		}
	}
	return true
}

func (vt *valueTag) WithProcessors(procs map[string]map[string]any) {}

func (vt *valueTag) updateApplyRules(evs []*formatters.EventMsg) {
	for _, ev := range evs {
		if v, ok := ev.Values[vt.ValueName]; ok {
			// calculate apply rule Key
			k := vt.applyRuleKey(ev.Tags)
			vt.applyRules[k] = &applyRule{
				tags:  copyTags(ev.Tags), // copy map
				value: v,
			}
			if vt.Consume {
				delete(ev.Values, vt.ValueName)
			}
		}
	}
}

// the apply rule key is a hash of the valueName and the event msg tags
func (vt *valueTag) applyRuleKey(m map[string]string) uint64 {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	h := fnv.New64a()
	h.Write([]byte(vt.ValueName))
	h.Write(pipeByte)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write(eqByte)
		h.Write([]byte(m[k]))
		h.Write(semiC)
	}
	return h.Sum64()
}

func copyTags(src map[string]string) map[string]string {
	dest := make(map[string]string, len(src))
	for k, v := range src {
		dest[k] = v
	}
	return dest
}
