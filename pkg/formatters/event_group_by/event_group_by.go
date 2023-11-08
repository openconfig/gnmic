// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_group_by

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/types"
	"github.com/openconfig/gnmic/pkg/utils"
)

const (
	processorType = "event-group-by"
	loggingPrefix = "[" + processorType + "] "
)

// groupBy groups values from different event messages in the same event message
// based on tags values
type groupBy struct {
	Tags   []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	ByName bool     `mapstructure:"by-name,omitempty" json:"by-name,omitempty"`
	Debug  bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	logger *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &groupBy{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (p *groupBy) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(p)
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

func (p *groupBy) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	result := make([]*formatters.EventMsg, 0, len(es))
	if p.Debug {
		p.logger.Printf("before: %+v", es)
	}
	if !p.ByName {
		result = p.byTags(es)
		if p.Debug {
			p.logger.Printf("after: %+v", result)
		}
		return result
	}
	groups := make(map[string][]*formatters.EventMsg)
	names := make([]string, 0)
	for _, e := range es {
		if _, ok := groups[e.Name]; !ok {
			groups[e.Name] = make([]*formatters.EventMsg, 0)
			names = append(names, e.Name)
		}
		groups[e.Name] = append(groups[e.Name], e)
	}
	sort.Strings(names)
	for _, n := range names {
		result = append(result, p.byTags(groups[n])...)
	}
	if p.Debug {
		p.logger.Printf("after: %+v", result)
	}
	return result
}

func (p *groupBy) WithLogger(l *log.Logger) {
	if p.Debug && l != nil {
		p.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if p.Debug {
		p.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (p *groupBy) WithTargets(tcs map[string]*types.TargetConfig) {}

func (p *groupBy) WithActions(act map[string]map[string]interface{}) {}

func (p *groupBy) WithProcessors(procs map[string]map[string]any) {}

func (p *groupBy) byTags(es []*formatters.EventMsg) []*formatters.EventMsg {
	if len(p.Tags) == 0 {
		return es
	}
	result := make([]*formatters.EventMsg, 0, len(es))
	groups := make(map[string]*formatters.EventMsg)
	keys := make([]string, 0)
	for _, e := range es {
		if e == nil || e.Tags == nil || e.Values == nil {
			continue
		}
		exist := true
		var key strings.Builder
		for _, t := range p.Tags {
			if v, ok := e.Tags[t]; ok {
				key.WriteString(v)
				continue
			}
			exist = false
			break
		}
		if exist {
			skey := key.String()
			if _, ok := groups[skey]; !ok {
				keys = append(keys, skey)
				groups[skey] = &formatters.EventMsg{
					Name:      e.Name,
					Timestamp: e.Timestamp,
					Tags:      make(map[string]string),
					Values:    make(map[string]interface{}),
				}
			}
			for k, v := range e.Tags {
				groups[skey].Tags[k] = v
			}
			for k, v := range e.Values {
				groups[skey].Values[k] = v
			}
			if e.Deletes != nil {
				groups[skey].Deletes = make([]string, 0)
				groups[skey].Deletes = append(groups[skey].Deletes, e.Deletes...)
			}

			continue
		}
		result = append(result, e)
	}
	sort.Strings(keys)
	for _, k := range keys {
		result = append(result, groups[k])
	}
	return result
}
