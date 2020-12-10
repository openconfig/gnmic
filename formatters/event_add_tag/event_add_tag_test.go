package event_add_tag

import (
	"reflect"
	"testing"

	"github.com/karimra/gnmic/formatters"
)

type item struct {
	input  *formatters.EventMsg
	output *formatters.EventMsg
}

var testset = map[string]struct {
	processorType string
	processor     map[string]interface{}
	tests         []item
}{
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"value": 1},
					Tags:   map[string]string{"tag1": "1"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"value": 1},
					Tags:   map[string]string{"tag1": "1"},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
				},
			},
		},
	},
	"match_value_name_overwrite": {
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"value": 1},
					Tags:   map[string]string{"tag1": "1"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"value": 1},
					Tags:   map[string]string{"tag1": "new_tag"},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"value": 1},
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"value": 1},
					Tags: map[string]string{
						"tag1": "1",
						"tag2": "new_tag2",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"value": 1},
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"value": 1},
					Tags: map[string]string{
						"tag1": "new_tag",
						"tag2": "new_tag2",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"v": "value"},
					Tags:   map[string]string{"tag1": "1"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"v": "value"},
					Tags:   map[string]string{"tag1": "1"},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"value": "value"},
					Tags:   map[string]string{"tag1": "1"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"value": "value"},
					Tags:   map[string]string{"tag1": "new_tag"},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"value": "value"},
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"value": "value"},
					Tags: map[string]string{
						"tag1": "1",
						"tag2": "new_tag2",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"value": "value"},
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"value": "value"},
					Tags: map[string]string{
						"tag1": "new_tag",
						"tag2": "new_tag2",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{"tag1": "1"},
				},
				output: &formatters.EventMsg{
					Tags: map[string]string{"tag1": "1"},
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{"tag1": "1"},
				},
				output: &formatters.EventMsg{
					Tags: map[string]string{"tag1": "new_tag"},
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{

					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"tag1": "1",
						"tag2": "new_tag2",
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"tag1": "new_tag",
						"tag2": "new_tag2",
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{"old_tag": "tag_value"},
				},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"old_tag": "tag_value",
						"tag1":    "new_tag",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"old_tag": "tag_value",
						"tag1":    "old_value",
					},
				},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"old_tag": "tag_value",
						"tag1":    "old_value",
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{"tag1": "tag_value"},
				},
				output: &formatters.EventMsg{
					Tags: map[string]string{"tag1": "new_tag"},
				},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"old_tag": "tag_value",
						"tag1":    "old_value",
					},
				},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"old_tag": "tag_value",
						"tag1":    "new_tag",
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{

					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"tag1": "1",
						"tag2": "new_tag2",
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
				input:  &formatters.EventMsg{},
				output: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"tag1": "new_tag",
						"tag2": "new_tag2",
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
			err := p.Init(ts.processor, nil)
			if err != nil {
				t.Errorf("failed to initialize processors: %v", err)
				return
			}
			t.Logf("processor: %+v", p)
			for i, item := range ts.tests {
				t.Run(name, func(t *testing.T) {
					t.Logf("running test item %d", i)
					var inputMsg *formatters.EventMsg
					if item.input != nil {
						inputMsg = &formatters.EventMsg{
							Name:      item.input.Name,
							Timestamp: item.input.Timestamp,
							Tags:      make(map[string]string),
							Values:    make(map[string]interface{}),
							Deletes:   item.input.Deletes,
						}
						for k, v := range item.input.Tags {
							inputMsg.Tags[k] = v
						}
						for k, v := range item.input.Values {
							inputMsg.Values[k] = v
						}
					}
					p.Apply(item.input)
					t.Logf("input: %+v, changed: %+v", inputMsg, item.input)
					if !reflect.DeepEqual(item.input, item.output) {
						t.Errorf("failed at %s item %d, expected %+v, got: %+v", name, i, item.output, item.input)
					}
				})
			}
		} else {
			t.Errorf("event processor %s not found", ts.processorType)
		}
	}
}
