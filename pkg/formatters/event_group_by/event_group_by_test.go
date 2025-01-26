// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_group_by

import (
	"fmt"
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
	"group_by_1_tag": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tags": []string{"tag1"},
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
						Values: map[string]interface{}{"value1": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
					{
						Values: map[string]interface{}{"value2": 2},
						Tags:   map[string]string{"tag1": "1"},
					},
					{
						Values: map[string]interface{}{"value3": 3},
						Tags:   map[string]string{"tag2": "2"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{
							"value3": 3,
						},
						Tags: map[string]string{
							"tag2": "2",
						},
					},
					{
						Values: map[string]interface{}{
							"value1": 1,
							"value2": 2,
						},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
			},
		},
	},
	"group_by_2_tags": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tags": []string{"tag1", "tag2"},
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
						Values: map[string]interface{}{"value1": 1},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "2",
						},
					},
					{
						Values: map[string]interface{}{"value2": 2},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "2",
						},
					},
					{
						Values: map[string]interface{}{"value3": 3},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "3",
						},
					},
					{
						Values: map[string]interface{}{"value4": 4},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "3",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{
							"value1": 1,
							"value2": 2,
						},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "2",
						},
					},
					{
						Values: map[string]interface{}{
							"value3": 3,
							"value4": 4,
						},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "3",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value1": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
					{
						Values: map[string]interface{}{"value1": 1},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "2",
						},
					},
					{
						Values: map[string]interface{}{"value2": 2},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "2",
						},
					},
					{
						Values: map[string]interface{}{"value3": 3},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "3",
						},
					},
					{
						Values: map[string]interface{}{"value4": 4},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "3",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value1": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
					{
						Values: map[string]interface{}{
							"value1": 1,
							"value2": 2,
						},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "2",
						},
					},
					{
						Values: map[string]interface{}{
							"value3": 3,
							"value4": 4,
						},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "3",
						},
					},
				},
			},
		},
	},
	"group_by_name": {
		processorType: processorType,
		processor: map[string]interface{}{
			"by-name": true,
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
						Name:   "sub1",
						Values: map[string]interface{}{"value1": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value2": 2},
						Tags: map[string]string{
							"tag2": "2",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value1": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value2": 2},
						Tags: map[string]string{
							"tag2": "2",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name:   "sub2",
						Values: map[string]interface{}{"value3": 3},
						Tags: map[string]string{
							"tag2": "2",
						},
					},
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value1": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value2": 2},
						Tags: map[string]string{
							"tag2": "2",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value1": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value2": 2},
						Tags: map[string]string{
							"tag2": "2",
						},
					},
					{
						Name:   "sub2",
						Values: map[string]interface{}{"value3": 3},
						Tags: map[string]string{
							"tag2": "2",
						},
					},
				},
			},
		},
	},
	"group_by_name_by_tags": {
		processorType: processorType,
		processor: map[string]interface{}{
			"by-name": true,
			"tags":    []string{"tag1"},
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
						Name:   "sub1",
						Values: map[string]interface{}{"value1": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value2": 2},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Values: map[string]interface{}{
							"value1": 1,
							"value2": 2,
						},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value1": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value2": 2},
						Tags: map[string]string{
							"tag1": "2",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Values: map[string]interface{}{
							"value1": 1,
						},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
					{
						Name: "sub1",
						Values: map[string]interface{}{
							"value2": 2,
						},
						Tags: map[string]string{
							"tag1": "2",
						},
					},
				},
			},
		},
	},
}

func TestEventGroupBy(t *testing.T) {
	for name, ts := range testset {
		if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
			t.Log("found processor")
			p := pi()
			err := p.Init(ts.processor)
			if err != nil {
				t.Errorf("failed to initialize processors: %v", err)
				return
			}
			t.Logf("processor: %+v", p)
			for i, item := range ts.tests {
				t.Run(name, func(t *testing.T) {
					t.Logf("running test item %d", i)
					outs := p.Apply(item.input...)
					if len(outs) != len(item.output) {
						t.Errorf("failed at %s, outputs not of same length", name)
						t.Errorf("expected: %v", item.output)
						t.Errorf("     got: %v", outs)
						return
					}
					if !slicesEqual(outs, item.output) {
						t.Errorf("failed at %s, expected: %+v", name, item.output)
						t.Errorf("failed at %s,      got: %+v", name, outs)
					}
				})
			}
		} else {
			t.Errorf("event processor %s not found", ts.processorType)
		}
	}
}

func generateMockEvents(numEvents, numTags int) []*formatters.EventMsg {
	es := make([]*formatters.EventMsg, numEvents)
	for i := 0; i < numEvents; i++ {
		tags := make(map[string]string, numTags)
		values := make(map[string]interface{}, numTags)
		for j := 0; j < numTags; j++ {
			tags[fmt.Sprintf("tag%d", j)] = fmt.Sprintf("value%d", j)
			values[fmt.Sprintf("valueKey%d", j)] = fmt.Sprintf("value%d", j)
		}
		es[i] = &formatters.EventMsg{
			Name:      fmt.Sprintf("event%d", i%5), // Group some events by name
			Timestamp: int64(i),
			Tags:      tags,
			Values:    values,
		}
	}
	return es
}

func BenchmarkByTags(b *testing.B) {
	p := &groupBy{Tags: []string{"tag1", "tag2"}}

	// Generate mock event messages
	es := generateMockEvents(100_000, 5)

	b.Run("OldByTags", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = p.byTagsOld(es)
		}
	})

	b.Run("NewByTags", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = p.byTags(es)
		}
	})
}

func slicesEqual(slice1, slice2 []*formatters.EventMsg) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	// Create a map to track matches in slice2
	used := make([]bool, len(slice2))

	// Check that every item in slice1 has a match in slice2
	for _, e1 := range slice1 {
		found := false
		for i, e2 := range slice2 {
			if !used[i] && eventMsgEqual(e1, e2) {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			return false // No match found for this item
		}
	}

	return true
}

func eventMsgEqual(a, b *formatters.EventMsg) bool {
	if a == nil || b == nil {
		return a == b
	}

	if a.Name != b.Name || a.Timestamp != b.Timestamp {
		return false
	}

	if !reflect.DeepEqual(a.Tags, b.Tags) {
		return false
	}
	if !reflect.DeepEqual(a.Values, b.Values) {
		return false
	}
	if a.Deletes == nil && b.Deletes == nil {
		return true
	}
	if len(a.Deletes) == 0 && len(b.Deletes) == 0 {
		return true
	}
	if !reflect.DeepEqual(a.Deletes, b.Deletes) {
		return false
	}
	return true
}
