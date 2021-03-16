package event_merge

import (
	"reflect"
	"testing"

	"github.com/karimra/gnmic/formatters"
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
	"merge_by_timestamps": {
		processorType: processorType,
		processor: map[string]interface{}{
			"always": false,
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
						Values:    map[string]interface{}{"value1": 1},
						Tags:      map[string]string{"tag1": "1"},
					},
					{
						Timestamp: 1,
						Values:    map[string]interface{}{"value2": 2},
						Tags:      map[string]string{"tag2": "2"},
					},
					{
						Timestamp: 1,
						Values:    map[string]interface{}{"value3": 3},
						Tags:      map[string]string{"tag3": "3"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Values: map[string]interface{}{
							"value1": 1,
							"value2": 2,
							"value3": 3,
						},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "2",
							"tag3": "3",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Values:    map[string]interface{}{"name": 1},
					},
					{
						Timestamp: 2,
						Values:    map[string]interface{}{"name": "foo"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Values:    map[string]interface{}{"name": 1},
					},
					{
						Timestamp: 2,
						Values:    map[string]interface{}{"name": "foo"},
					},
				},
			},
		},
	},
	"merge_always": {
		processorType: processorType,
		processor: map[string]interface{}{
			"always": true,
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
						Values:    map[string]interface{}{"value1": 1},
						Tags:      map[string]string{"tag1": "1"},
					},
					{
						Timestamp: 1,
						Values:    map[string]interface{}{"value2": 2},
						Tags:      map[string]string{"tag2": "2"},
					},
					{
						Timestamp: 1,
						Values:    map[string]interface{}{"value3": 3},
						Tags:      map[string]string{"tag3": "3"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Values: map[string]interface{}{
							"value1": 1,
							"value2": 2,
							"value3": 3,
						},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "2",
							"tag3": "3",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Values: map[string]interface{}{
							"name": 1,
						},
					},
					{
						Timestamp: 2,
						Values: map[string]interface{}{
							"name2": "foo",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 2,
						Tags:      make(map[string]string),
						Values: map[string]interface{}{
							"name":  1,
							"name2": "foo",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Values: map[string]interface{}{
							"name": 1,
						},
					},
					{
						Timestamp: 2,
						Values: map[string]interface{}{
							"name": "foo",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 2,
						Tags:      make(map[string]string),
						Values: map[string]interface{}{
							"name": "foo",
						},
					},
				},
			},
		},
	},
}

func TestEventMerge(t *testing.T) {
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
					for j := range outs {
						if !reflect.DeepEqual(outs[j], item.output[j]) {
							t.Errorf("failed at %s item %d, index %d, expected %+v, got: %+v", name, i, j, item.output[j], outs[j])
						}
					}
				})
			}
		} else {
			t.Errorf("event processor %s not found", ts.processorType)
		}
	}
}
