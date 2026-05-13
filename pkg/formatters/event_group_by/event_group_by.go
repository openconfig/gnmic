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
	"hash/fnv"
	"log/slog"
	"slices"
	"strings"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
)

const (
	processorType = "event-group-by"
)

// groupBy groups values from different event messages in the same event message
// based on tags values
type groupBy struct {
	formatters.BaseProcessor
	Tags   []string `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	ByName bool     `mapstructure:"by-name,omitempty" json:"by-name,omitempty"`
	Debug  bool     `mapstructure:"debug,omitempty" json:"debug,omitempty"`
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &groupBy{}
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
	if p.Logger == nil {
		p.Logger = logging.DiscardLogger()
	}
	p.Logger = p.Logger.With("processor", processorType)

	if p.Debug {
		if b, err := json.Marshal(p); err == nil {
			p.Logger.Debug("initialized processor", "config", string(b))
		} else {
			p.Logger.Debug("initialized processor", "config", p)
		}
	}
	return nil
}

func (p *groupBy) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	result := make([]*formatters.EventMsg, 0, len(es))
	p.Logger.Debug("group_by input", "events", es)
	if !p.ByName {
		result = p.byTags(es)
		p.Logger.Debug("group_by result", "events", result)
		return result
	}
	groups := make(map[string][]*formatters.EventMsg)
	names := make([]string, 0)
	for _, e := range es {
		_, ok := groups[e.Name]
		if !ok {
			groups[e.Name] = make([]*formatters.EventMsg, 0)
			names = append(names, e.Name)
		}
		groups[e.Name] = append(groups[e.Name], e)
	}
	slices.Sort(names)
	for _, n := range names {
		result = append(result, p.byTags(groups[n])...)
	}
	p.Logger.Debug("group_by result", "events", result)
	return result
}

func (p *groupBy) WithLogger(l *slog.Logger) {
	if !p.Debug {
		l = nil
	}
	p.BaseProcessor.WithLogger(l)
}

func (p *groupBy) byTagsOld(es []*formatters.EventMsg) []*formatters.EventMsg {
	if len(p.Tags) == 0 {
		return es
	}
	result := make([]*formatters.EventMsg, 0, len(es))
	groups := make(map[string]*formatters.EventMsg)
	keys := make([]string, 0)
	for _, e := range es {
		if e == nil || e.Tags == nil || (e.Values == nil && e.Deletes == nil) {
			continue
		}
		exist := true
		var key strings.Builder
		for _, t := range p.Tags {
			if v, ok := e.Tags[t]; ok {
				key.WriteString(t)
				key.Write(eqByte)
				key.WriteString(v)
				key.Write(pipeByte)
				continue
			}
			exist = false
			break
		}
		if !exist {
			result = append(result, e)
			continue
		}

		skey := key.String()
		group, ok := groups[skey]
		if !ok {
			keys = append(keys, skey)
			group = &formatters.EventMsg{
				Name:      e.Name,
				Timestamp: e.Timestamp,
				Tags:      make(map[string]string),
				Values:    make(map[string]interface{}),
			}
			groups[skey] = group
		}
		for k, v := range e.Tags {
			group.Tags[k] = v
		}
		for k, v := range e.Values {
			group.Values[k] = v
		}
		if e.Deletes != nil {
			group.Deletes = append(group.Deletes, e.Deletes...)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		result = append(result, groups[k])
	}
	return result
}

func (p *groupBy) byTags(es []*formatters.EventMsg) []*formatters.EventMsg {
	if len(p.Tags) == 0 {
		return es
	}

	result := make([]*formatters.EventMsg, 0, len(es))
	groups := make(map[uint64]*formatters.EventMsg)

	for _, e := range es {
		if e == nil || e.Tags == nil || (e.Values == nil && e.Deletes == nil) {
			continue
		}

		//grouping key based on tags value
		skey, match := generateKeyAndCheck(e.Tags, p.Tags)
		if !match {
			result = append(result, e)
			continue
		}

		group, exists := groups[skey]
		if !exists {
			group = &formatters.EventMsg{
				Name:      e.Name,
				Timestamp: e.Timestamp,
				Tags:      make(map[string]string, len(e.Tags)),
				Values:    make(map[string]interface{}, len(e.Values)),
				Deletes:   make([]string, 0, len(e.Deletes)),
			}
			groups[skey] = group
		}

		// merge tags, values and deletes into the group
		for k, v := range e.Tags {
			group.Tags[k] = v
		}
		for k, v := range e.Values {
			group.Values[k] = v
		}
		if e.Deletes != nil {
			group.Deletes = append(group.Deletes, e.Deletes...)
		}
	}

	for _, ev := range groups {
		result = append(result, ev)
	}

	return result
}

func generateKeyAndCheck(tags map[string]string, keys []string) (uint64, bool) {
	h := fnv.New64a()

	for _, k := range keys {
		v, ok := tags[k]
		if !ok {
			return 0, false
		}
		h.Write([]byte(k))
		h.Write([]byte(eqByte))
		h.Write([]byte(v))
		h.Write([]byte(pipeByte))
	}

	return h.Sum64(), true
}

var (
	eqByte   = []byte("=")
	pipeByte = []byte("|")
)
