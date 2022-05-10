package event_value_tag

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
						Values:    map[string]interface{}{"foo": "value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value", "foo": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"foo": "value"},
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
						Values:    map[string]interface{}{"foo": "value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value", "bar": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value"},
						Values:    map[string]interface{}{"foo": "value"},
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
						Values:    map[string]interface{}{"foo": "value"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 1,
						Tags:      map[string]string{"tag": "value", "foo": "value"},
					},
					{
						Timestamp: 2,
						Tags:      map[string]string{"tag": "value", "foo": "value"},
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
