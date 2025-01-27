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
	"sync"

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
	Rules  []*rule `mapstructure:"rules,omitempty" json:"rules,omitempty"`
	Debug  bool    `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	logger *log.Logger

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
		return &valueTag{m: new(sync.RWMutex), logger: log.New(io.Discard, "", 0)}
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
	for _, r := range vt.Rules {
		if r.TagName == "" {
			r.TagName = r.ValueName
		}
	}

	vt.applyRules = make([]map[uint64]*applyRule, len(vt.Rules))
	for i := range vt.applyRules {
		vt.applyRules[i] = make(map[uint64]*applyRule, 0)
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
