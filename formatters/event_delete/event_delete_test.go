package event_delete

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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
	"tag_keys_delete": {
		processorType: "event_delete",
		processor: map[string]interface{}{
			"tag_keys": []string{"^name*"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag", "name-2": "name-2_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag", "-name": "name-2_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"-name": "name-2_tag"},
				},
			},
		},
	},
	"2_tag_keys_delete": {
		processorType: "event_delete",
		processor: map[string]interface{}{
			"tag_keys": []string{"^name*", "to_delete"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1, "todelete": "value"}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1, "todelete": "value"}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag", "to_delete": "value"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag", "name-2": "name-2_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{},
				},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag", "-name": "name-2_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"-name": "name-2_tag"},
				},
			},
		},
	},
	"value_keys_delete": {
		processorType: "event_delete",
		processor: map[string]interface{}{
			"value_keys": []string{"deleteme*"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"deleteme": 1},
					Tags:   map[string]string{"-name": "name-2_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{},
					Tags:   map[string]string{"-name": "name-2_tag"}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"deleteme": 1, "dont-deleteme": 1}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
		},
	},
	"2_value_keys_delete": {
		processorType: "event_delete",
		processor: map[string]interface{}{
			"value_keys": []string{"deleteme", "deleteme-too"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"deleteme": 1}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"deleteme": 1, "deleteme-too": 1}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
		},
	},
	"tag_keys_and_value_keys_delete": {
		processorType: "event_delete",
		processor: map[string]interface{}{
			"value_keys": []string{"deleteme-value*"},
			"tag_keys":   []string{"deleteme-tag*"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"deleteme-value": 1}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{"deleteme-tag": "tag"}},
				output: &formatters.EventMsg{
					Tags: map[string]string{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"deleteme-value": 1, "dont-deleteme": 1},
					Tags:   map[string]string{"deleteme-tag": "tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"dont-deleteme": 1},
					Tags:   map[string]string{},
				},
			},
		},
	},
	"tags_delete": {
		processorType: "event_delete",
		processor: map[string]interface{}{
			"tags": []string{"^name*"},
		},
		tests: []item{
			// 0
			{
				input:  nil,
				output: nil,
			},
			// 1
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			// 2
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1}},
			},
			// 3
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{},
				},
			},
			// 4
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag", "name-2": "name-2_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{},
				},
			},
			// 5
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag", "-name": "name-2_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{},
				},
			},
		},
	},
	"2_tags_delete": {
		processorType: "event_delete",
		processor: map[string]interface{}{
			"tags": []string{"^name*", "to_delete"},
		},
		tests: []item{
			// 0
			{
				input:  nil,
				output: nil,
			},
			// 1
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			// 2
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1, "todelete": "to_delete"}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1, "todelete": "to_delete"}},
			},
			// 3
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag", "tag_name": "to_delete"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{},
				},
			},
			// 4
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag", "name-2": "name-2_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{},
				},
			},
			// 5
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name": "name_tag", "name-2": "-name-2_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"name": 1},
					Tags:   map[string]string{"name-2": "-name-2_tag"},
				},
			},
		},
	},
	"values_delete": {
		processorType: "event_delete",
		processor: map[string]interface{}{
			"values": []string{"deleteme*"},
		},
		tests: []item{
			// 0
			{
				input:  nil,
				output: nil,
			},
			// 1
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			// 2
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"deleteme": "deleteme"},
					Tags:   map[string]string{"-name": "name-2_tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{},
					Tags:   map[string]string{"-name": "name-2_tag"}},
			},
			// 3
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"foo": "deleteme", "dont-deleteme": 1}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"dont-deleteme": 1}},
			},
		},
	},
	"2_values_delete": {
		processorType: "event_delete",
		processor: map[string]interface{}{
			"values": []string{"deleteme", "deleteme-too"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"foo": "deleteme"}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"foo": "deleteme", "bar": "deleteme-too"}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
		},
	},
	"tags_and_values_delete": {
		processorType: "event_delete",
		processor: map[string]interface{}{
			"values": []string{"deleteme-value*"},
			"tags":   []string{"deleteme-tag*"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"foo-value": "deleteme-value"}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{"foo-tag": "deleteme-tag"}},
				output: &formatters.EventMsg{
					Tags: map[string]string{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{"foo-value": "deleteme-value", "dont-deleteme": 1},
					Tags:   map[string]string{"foo-tag": "deleteme-tag"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"dont-deleteme": 1},
					Tags:   map[string]string{},
				},
			},
		},
	},
}

func TestEventDelete(t *testing.T) {
	for name, ts := range testset {
		if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
			t.Log("found processor")
			p := pi()
			err := p.Init(ts.processor)
			if err != nil {
				t.Errorf("failed to initialize processors: %v", err)
				return
			}
			t.Logf("initialized for test %s: %+v", name, p)
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
					if !cmp.Equal(item.input, item.output) {
						t.Errorf("failed at %s item %d, expected %+v, got: %+v", name, i, item.output, item.input)
					}
				})
			}
		} else {
			t.Errorf("processors type %s not found", ts.processorType)
			t.Fail()
		}
	}
}
