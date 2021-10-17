package event_add_tag

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
	"match_condition": {
		processorType: processorType,
		processor: map[string]interface{}{
			"condition": `.values.value == 1`,
			"add":       map[string]string{"tag1": "new_tag"},
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
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
						Tags: map[string]string{
							"tag1": "new_tag",
						},
					},
				},
			},
		},
	},
	"match_condition_overwrite": {
		processorType: processorType,
		processor: map[string]interface{}{
			"condition": `.values.value == 1`,
			"add":       map[string]string{"tag1": "new_tag"},
			"overwrite": true,
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
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
						Tags: map[string]string{
							"tag1": "new_tag",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
						Tags: map[string]string{
							"tag1": "new_tag",
						},
					},
				},
			},
		},
	},
	// match value name
	"match_value_name_add": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"value"},
			"add":         map[string]string{"tag1": "new_tag"},
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
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
			},
		},
	},
	"match_value_name_overwrite": {
		processorType: processorType,
		processor: map[string]interface{}{
			"debug":       true,
			"value-names": []string{"value"},
			"overwrite":   true,
			"add": map[string]string{
				"tag1": "new_tag",
			},
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
						Tags: map[string]string{"tag1": "1"},
					},
					{
						Values: map[string]interface{}{"value": 2},
						Tags:   map[string]string{"tag1": "2"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{"tag1": "1"},
					},
					{
						Values: map[string]interface{}{"value": 2},
						Tags:   map[string]string{"tag1": "new_tag"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
					{
						Values: map[string]interface{}{"value": 2},
						Tags:   map[string]string{"tag1": "2"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "new_tag"},
					},
					{
						Values: map[string]interface{}{"value": 2},
						Tags:   map[string]string{"tag1": "new_tag"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
			},
		},
	},
	"match_value_name_add_many": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"value"},
			"add": map[string]string{
				"tag1": "new_tag",
				"tag2": "new_tag2",
			},
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
						Values: map[string]interface{}{"value": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "new_tag2",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
			},
		},
	},
	"match_value_name_add_many_overwrite": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"value"},
			"overwrite":   true,
			"add": map[string]string{
				"tag1": "new_tag",
				"tag2": "new_tag2",
			},
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
						Values: map[string]interface{}{"value": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
						Tags: map[string]string{
							"tag1": "new_tag",
							"tag2": "new_tag2",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
			},
		},
	},
	// match value
	"match_value_add": {
		processorType: processorType,
		processor: map[string]interface{}{
			"values": []string{"value"},
			"add":    map[string]string{"tag1": "new_tag"},
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
						Values: map[string]interface{}{"v": "value"},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"v": "value"},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
			},
		},
	},
	"match_value_overwrite": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"value"},
			"overwrite":   true,
			"add": map[string]string{
				"tag1": "new_tag",
			},
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
						Values: map[string]interface{}{"value": "value"},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": "value"},
						Tags:   map[string]string{"tag1": "new_tag"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
			},
		},
	},
	"match_value_add_many": {
		processorType: processorType,
		processor: map[string]interface{}{
			"values": []string{"value"},
			"add": map[string]string{
				"tag1": "new_tag",
				"tag2": "new_tag2",
			},
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
						Values: map[string]interface{}{"value": "value"},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": "value"},
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "new_tag2",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
			},
		},
	},
	"match_value_add_many_overwrite": {
		processorType: processorType,
		processor: map[string]interface{}{
			"values":    []string{"value"},
			"overwrite": true,
			"add": map[string]string{
				"tag1": "new_tag",
				"tag2": "new_tag2",
			},
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
						Values: map[string]interface{}{"value": "value"},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": "value"},
						Tags: map[string]string{
							"tag1": "new_tag",
							"tag2": "new_tag2",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
			},
		},
	},
	// match tag name
	"match_tag_name_add": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tag-names": []string{"."},
			"add":       map[string]string{"tag1": "new_tag"},
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
						Tags: map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{"tag1": "1"},
					},
				},
			},
		},
	},
	"match_tag_name_overwrite": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tag-names": []string{"."},
			"overwrite": true,
			"add": map[string]string{
				"tag1": "new_tag",
			},
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
						Tags: map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{"tag1": "new_tag"},
					},
				},
			},
		},
	},
	"match_tag_name_add_many": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tag-names": []string{"."},
			"add": map[string]string{
				"tag1": "new_tag",
				"tag2": "new_tag2",
			},
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
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "new_tag2",
						},
					},
				},
			},
		},
	},
	"match_tag_name_add_many_overwrite": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tag-names": []string{"."},
			"overwrite": true,
			"add": map[string]string{
				"tag1": "new_tag",
				"tag2": "new_tag2",
			},
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
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"tag1": "new_tag",
							"tag2": "new_tag2",
						},
					},
				},
			},
		},
	},
	// match tag
	"match_tag_add": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tags": []string{"tag_value"},
			"add":  map[string]string{"tag1": "new_tag"},
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
						Tags: map[string]string{"old_tag": "tag_value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"old_tag": "tag_value",
							"tag1":    "new_tag",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"old_tag": "tag_value",
							"tag1":    "old_value",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"old_tag": "tag_value",
							"tag1":    "old_value",
						},
					},
				},
			},
		},
	},
	"match_tag_overwrite": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tags":      []string{"tag_value"},
			"overwrite": true,
			"add": map[string]string{
				"tag1": "new_tag",
			},
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
						Tags: map[string]string{"tag1": "tag_value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{"tag1": "new_tag"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"old_tag": "tag_value",
							"tag1":    "old_value",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"old_tag": "tag_value",
							"tag1":    "new_tag",
						},
					},
				},
			},
		},
	},
	"match_tag_add_many": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tags": []string{"1"},
			"add": map[string]string{
				"tag1": "new_tag",
				"tag2": "new_tag2",
			},
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
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"tag1": "1",
							"tag2": "new_tag2",
						},
					},
				},
			},
		},
	},
	"match_tag_add_many_overwrite": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tag-names": []string{"1"},
			"overwrite": true,
			"add": map[string]string{
				"tag1": "new_tag",
				"tag2": "new_tag2",
			},
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
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"tag1": "new_tag",
							"tag2": "new_tag2",
						},
					},
				},
			},
		},
	},
}

func TestEventAddTag(t *testing.T) {
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
							t.Logf("failed at %s item %d, index %d, expected: %+v", name, i, j, item.output[j])
							t.Logf("failed at %s item %d, index %d,      got: %+v", name, i, j, outs[j])
							t.Fail()
						}
					}
				})
			}
		} else {
			t.Errorf("event processor %s not found", ts.processorType)
		}
	}
}
