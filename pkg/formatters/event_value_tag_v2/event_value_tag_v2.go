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
	"log/slog"
	"slices"
	"sync"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const (
	processorType = "event-value-tag-v2"
)

var (
	eqByte   = []byte("=")
	semiC    = []byte(";")
	pipeByte = []byte("|")
)

type valueTag struct {
	formatters.BaseProcessor
	Rules []*rule `mapstructure:"rules,omitempty" json:"rules,omitempty"`
	Debug bool    `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	m          *sync.RWMutex
	applyRules []map[uint64]*applyRule
}

type rule struct {
	TagName   string `mapstructure:"tag-name,omitempty" json:"tag-name,omitempty"`
	ValueName string `mapstructure:"value-name,omitempty" json:"value-name,omitempty"`
	Consume   bool   `mapstructure:"consume,omitempty" json:"consume,omitempty"`
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &valueTag{m: new(sync.RWMutex)}
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
	if vt.Logger == nil {
		vt.Logger = logging.DiscardLogger()
	}
	vt.Logger = vt.Logger.With("processor", processorType)
	for _, r := range vt.Rules {
		if r.TagName == "" {
			r.TagName = r.ValueName
		}
	}

	vt.applyRules = make([]map[uint64]*applyRule, len(vt.Rules))
	for i := range vt.applyRules {
		vt.applyRules[i] = make(map[uint64]*applyRule, 0)
	}

	if vt.Debug {
		if b, err := json.Marshal(vt); err == nil {
			vt.Logger.Debug("initialized processor", "config", string(b))
		} else {
			vt.Logger.Debug("initialized processor", "config", vt)
		}
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
	vt.m.Lock()
	defer vt.m.Unlock()

	for _, ev := range evs {
		for i, r := range vt.Rules {
			if v, ok := ev.Values[r.ValueName]; ok {
				// calculate apply rule Key
				k := vt.applyRuleKey(ev.Tags, r)
				vt.applyRules[i][k] = &applyRule{
					tags:  copyTags(ev.Tags), // copy map
					value: v,
				}
				if r.Consume {
					delete(ev.Values, r.ValueName)
				}
			}
			for _, ar := range vt.applyRules[i] {
				if includedIn(ar.tags, ev.Tags) {
					switch v := ar.value.(type) {
					case string:
						ev.Tags[r.TagName] = v
					default:
						ev.Tags[r.TagName] = fmt.Sprint(ar.value)
					}
				}
			}
		}
	}
	return evs
}

func (vt *valueTag) WithLogger(l *slog.Logger) {
	if !vt.Debug {
		l = nil
	}
	vt.BaseProcessor.WithLogger(l)
}

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

// the apply rule key is a hash of the valueName and the event msg tags
func (vt *valueTag) applyRuleKey(m map[string]string, r *rule) uint64 {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	h := fnv.New64a()
	h.Write([]byte(r.ValueName))
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
