// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_value_tag_v2

import (
	"io"
	"log"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

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
}

func TestEventValueTag(t *testing.T) {
	for name, ts := range testset {
		if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
			t.Log("found processor")
			for i, item := range ts.tests {
				// a processor per test item
				p := pi()
				err := p.Init(ts.processor)
				if err != nil {
					t.Errorf("failed to initialize processors: %v", err)
					return
				}
				t.Logf("processor: %+v", p)
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

func TestValueTagApplySubsequentRuns(t *testing.T) {
	processor := &valueTag{
		TagName:    "moved-tag",
		ValueName:  "important-value",
		Consume:    true,
		Debug:      true,
		logger:     log.New(io.Discard, "", 0),
		applyRules: make(map[uint64]*applyRule),
	}

	// first set
	events1 := []*formatters.EventMsg{
		{
			Tags: map[string]string{"tag1": "value1"},
			Values: map[string]interface{}{
				"important-value": "value-to-move",
			},
		},
		{
			Tags: map[string]string{"tag2": "value2"},
			Values: map[string]interface{}{
				"other-value": "irrelevant",
			},
		},
	}

	// first apply
	processed1 := processor.Apply(events1...)

	// assert
	assert.Equal(t, "value-to-move", processed1[0].Tags["moved-tag"])
	assert.NotContains(t, processed1[0].Values, "important-value")
	assert.NotContains(t, processed1[1].Tags, "moved-tag")

	// second set
	events2 := []*formatters.EventMsg{
		{
			Tags: map[string]string{
				"tag1": "value1",
			},
			Values: map[string]interface{}{
				"new-value": "some-new-data",
			},
		},
		{
			Tags: map[string]string{
				"tag1": "value1",
			},
			Values: map[string]interface{}{
				"counter1": 42,
			},
		},
	}

	// second apply
	processed2 := processor.Apply(events2...)
	// assert
	assert.Equal(t, "value-to-move", processed2[0].Tags["moved-tag"])
	assert.Contains(t, processed2[0].Tags, "tag1")
	assert.Contains(t, processed2[0].Values, "new-value")
	assert.Contains(t, processed2[1].Tags, "tag1")
	assert.Contains(t, processed2[1].Values, "counter1")
}
