package event_jq

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
	"default_values": {
		processorType: processorType,
		processor: map[string]interface{}{
			"debug": true,
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
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
			},
		},
	},
	"simple_select_expression": {
		processorType: processorType,
		processor: map[string]interface{}{
			"expression": `select(.name=="sub1")`,
			"debug":      true,
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
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name:   "sub2",
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
					{
						Name:   "sub2",
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
			},
		},
	},
	"double_condition_and_select_expression": {
		processorType: processorType,
		processor: map[string]interface{}{
			"expression": `select(.name=="sub1" and .values.counter1 > 90)`,
			"debug":      true,
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
						Name: "sub1",
						Values: map[string]interface{}{
							"counter1": 91,
						},
						Tags: map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Values: map[string]interface{}{
							"counter1": 91,
						},
						Tags: map[string]string{"tag1": "1"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{},
			},
		},
	},
	"complex_select_expression": {
		processorType: processorType,
		processor: map[string]interface{}{
			"expression": `select((.name=="sub1" and .values.counter1 > 90) or (.name=="sub2" and .values.counter2 > 80))`,
			"debug":      true,
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
						Name: "sub1",
						Values: map[string]interface{}{
							"counter1": 91,
						},
						Tags: map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Values: map[string]interface{}{
							"counter1": 91,
						},
						Tags: map[string]string{"tag1": "1"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name:   "sub1",
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name: "sub2",
						Values: map[string]interface{}{
							"counter2": 91,
						},
						Tags: map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub2",
						Values: map[string]interface{}{
							"counter2": 91,
						},
						Tags: map[string]string{"tag1": "1"},
					},
				},
			},
		},
	},
}

func TestEventJQ(t *testing.T) {
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
					outs := p.Apply(item.input...)
					for j := range item.input {
						t.Logf("%q item %d, index %d, inputs=%+v", name, i, j, item.input[j])
					}
					// compare lengths first
					if len(outs) != len(item.output) {
						t.Logf("expected and gotten outputs are not of the same length")
						t.Logf("expected: %+v", item.output)
						t.Logf("     got: %+v", outs)
						t.Fail()
					}
					//
					for j := range outs {
						t.Logf("%q item %d, index %d, output=%+v", name, i, j, outs[j])
						if !reflect.DeepEqual(outs[j], item.output[j]) {
							t.Logf("failed at %s item %d, index %d", name, i, j)
							t.Logf("expected: %+v", item.output[j])
							t.Logf("     got: %+v", outs[j])
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
