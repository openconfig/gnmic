package event_group_by

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
					for j := range outs {
						if !reflect.DeepEqual(outs[j], item.output[j]) {
							t.Errorf("failed at %s item %d, index %d, expected: %+v", name, i, j, item.output[j])
							t.Errorf("failed at %s item %d, index %d,      got: %+v", name, i, j, outs[j])
						}
					}
				})
			}
		} else {
			t.Errorf("event processor %s not found", ts.processorType)
		}
	}
}
