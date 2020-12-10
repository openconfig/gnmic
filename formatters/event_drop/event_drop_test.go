package event_drop

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
	"drop_values": {
		processorType: "event_drop",
		processor: map[string]interface{}{
			"value_names": []string{"^number$"},
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
					Values: map[string]interface{}{"number": 1}},
				output: &formatters.EventMsg{},
			},
		},
	},
	"drop_tags": {
		processorType: "event_drop",
		processor: map[string]interface{}{
			"tag_names": []string{"^name*"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{}},
				output: &formatters.EventMsg{
					Tags: map[string]string{}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{"name": "dummy"}},
				output: &formatters.EventMsg{},
			},
		},
	},
}

func TestEventDrop(t *testing.T) {
	for name, ts := range testset {
		if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
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
						t.Logf("failed at %s item %d", name, i)
						t.Logf("expected: %#v", item.output)
						t.Logf("     got: %#v", item.input)
						t.Fail()
					}
				})
			}
		}
	}
}
