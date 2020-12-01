package event_delete

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/processors"
)

type item struct {
	input  *formatters.EventMsg
	output *formatters.EventMsg
}

var testset = map[string]struct {
	processor map[string]interface{}
	tests     []item
}{
	"tags_delete": {
		processor: map[string]interface{}{
			"type": "event_delete",
			"tags": []string{"^name*"},
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
	"2_tags_delete": {
		processor: map[string]interface{}{
			"type": "event_delete",
			"tags": []string{"^name*", "to_delete"},
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
	"values_delete": {
		processor: map[string]interface{}{
			"type":   "event_delete",
			"values": []string{"deleteme*"},
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
	"2_values_delete": {
		processor: map[string]interface{}{
			"type":   "event_delete",
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
	"tags_and_values_delete": {
		processor: map[string]interface{}{
			"type":   "event_delete",
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
}

func TestEventDelete(t *testing.T) {
	for name, ts := range testset {
		if typ, ok := ts.processor["type"]; ok {
			t.Log("found type")
			if pi, ok := processors.EventProcessors[typ.(string)]; ok {
				t.Log("found processor")
				p := pi()
				err := p.Init(ts.processor)
				if err != nil {
					t.Errorf("failed to initialized processors: %v", err)
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
			}
		}
	}
}
