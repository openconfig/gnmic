// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_value_tag

import (
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/openconfig/gnmic/pkg/formatters"
)

type item struct {
	input  []*formatters.EventMsg
	output []*formatters.EventMsg
}

var testset = map[string]struct {
	processorType string
	processor     map[string]interface{}
	tests         []item
}{
	"no-options": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-name": "foo",
			"debug":      true,
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input:  make([]*formatters.EventMsg, 0),
				output: make([]*formatters.EventMsg, 0),
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"foo": "new_value"},
					},
					{
						Timestamp: 3,
						Tags:      map[string]string{"other_tag": "value"},
						Values:    map[string]interface{}{"other_val": "val"},
					},
					{
						Timestamp: 4,
						Tags:      map[string]string{"foo": "other_value"},
						Values:    map[string]interface{}{"other_val": "val"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value", "foo": "new_value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value", "foo": "new_value"},
						Values:    map[string]interface{}{"foo": "new_value"},
					},
					{
						Timestamp: 3,
						Tags:      map[string]string{"other_tag": "value"},
						Values:    map[string]interface{}{"other_val": "val"},
					},
					{
						Timestamp: 4,
						Tags:      map[string]string{"foo": "other_value"},
						Values:    map[string]interface{}{"other_val": "val"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"bar": "value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"bar": "value"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"counter1": "1"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"foo": "value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value", "foo": "value"},
						Values:    map[string]interface{}{"counter1": "1"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value", "foo": "value"},
						Values:    map[string]interface{}{"foo": "value"},
					},
				},
			},
		},
	},
	"rename-tag": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-name": "foo",
			"tag-name":   "bar",
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input:  make([]*formatters.EventMsg, 0),
				output: make([]*formatters.EventMsg, 0),
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"foo": "new_value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value", "bar": "new_value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value", "bar": "new_value"},
						Values:    map[string]interface{}{"foo": "new_value"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"bar": "value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"bar": "value"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"counter1": "1"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"foo": "value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value", "bar": "value"},
						Values:    map[string]interface{}{"counter1": "1"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value", "bar": "value"},
						Values:    map[string]interface{}{"foo": "value"},
					},
				},
			},
		},
	},
	"consume-value": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-name": "foo",
			"consume":    true,
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input:  make([]*formatters.EventMsg, 0),
				output: make([]*formatters.EventMsg, 0),
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"foo": "new_value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value", "foo": "new_value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value", "foo": "new_value"},
						Values:    make(map[string]interface{}, 0),
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"bar": "value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"bar": "value"},
					},
				},
			},
		},
	},
	"integer_val": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-name": "foo",
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input:  make([]*formatters.EventMsg, 0),
				output: make([]*formatters.EventMsg, 0),
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"foo": 42},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value", "foo": "42"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value", "foo": "42"},
						Values:    map[string]interface{}{"foo": 42},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"bar": "value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"bar": "value"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value1"},
						Values:    map[string]interface{}{"foo": "value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value1", "foo": "value"},
						Values:    map[string]interface{}{"foo": "value"},
					},
				},
			},
		},
	},
}

func TestEventValueTag(t *testing.T) {
	for name, ts := range testset {
		if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
			t.Log("found processor")
			p := pi()
			err := p.Init(ts.processor, formatters.WithLogger(log.Default()))
			if err != nil {
				t.Errorf("failed to initialize processors: %v", err)
				return
			}
			t.Logf("processor: %+v", p)
			for i, item := range ts.tests {
				t.Run(name, func(t *testing.T) {
					t.Logf("running test item %d", i)
					outs := p.Apply(item.input...)
					for j := range outs {
						if !reflect.DeepEqual(outs[j], item.output[j]) {
							t.Errorf("failed at %s item %d, index %d, expected %+v", name, i, j, item.output[j])
							t.Errorf("failed at %s item %d, index %d, got:     %+v", name, i, j, outs[j])
						}
					}
				})
			}
		} else {
			t.Errorf("event processor %s not found", ts.processorType)
		}
	}
}

func generateEventMsgs(numEvents, numValues int, targetKey, targetValue string) []*formatters.EventMsg {
	evs := make([]*formatters.EventMsg, numEvents)
	for i := 0; i < numEvents; i++ {
		values := make(map[string]any)
		for j := 0; j < numValues; j++ {
			values[fmt.Sprintf("key%d", j)] = fmt.Sprintf("value%d", j)
		}
		values[targetKey] = targetValue
		evs[i] = &formatters.EventMsg{
			Tags:   map[string]string{"tag": "test"},
			Values: values,
		}
	}
	return evs
}

func BenchmarkBuildApplyRules(b *testing.B) {
	evs := generateEventMsgs(100_000, 10, "targetKey", "targetValue")
	vt := &valueTag{ValueName: "targetKey", Consume: true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vt.buildApplyRules(evs)
	}
}

func BenchmarkBuildApplyRules2(b *testing.B) {
	evs := generateEventMsgs(100_000, 10, "targetKey", "targetValue")
	vt := &valueTag{ValueName: "targetKey", Consume: true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vt.buildApplyRules2(evs)
	}
}

// as ref
func (vt *valueTag) buildApplyRules2(evs []*formatters.EventMsg) []*tagVal {
	toApply := make([]*tagVal, 0)

	for _, ev := range evs {
		for k, v := range ev.Values {
			if vt.ValueName == k {
				toApply = append(toApply, &tagVal{
					// copyTags(ev.Tags),
					ev.Tags,
					v,
				})
				if vt.Consume {
					delete(ev.Values, vt.ValueName)
				}
			}
		}
	}
	return toApply
}
