package event_allow

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
	"allow_condition": {
		processorType: processorType,
		processor: map[string]interface{}{
			"condition": ".values.value == 1",
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{}},
				},
				output: []*formatters.EventMsg{
					{},
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
					},
				},
			},
		},
	},
	"allow_value_names": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"^number$"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{}},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{}},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": 1},
					},
					{
						Values: map[string]interface{}{"not-number": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": 1},
					},
					//{},
				},
			},
		},
	},
	"allow_tag_names": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tag-names": []string{"^name*"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{
					{
						Tags: map[string]string{}},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{}},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Tags: map[string]string{"name": "dummy"},
					},
					{
						Tags: map[string]string{"not-name": "dummy"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{"name": "dummy"},
					},
					//{},
				},
			},
		},
	},
	"allow_tag_values": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tags": []string{"router1"},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{
					{
						Tags: map[string]string{},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Tags: map[string]string{"name": "router1"},
					},
					{
						Tags: map[string]string{"not-name": "dummy"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{"name": "router1"},
					},
					//{},
				},
			},
		},
	},
	"allow_multiple_value_names": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{
				"^number$",
				"^name$",
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{}},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{}},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": 1},
					},
					{
						Values: map[string]interface{}{"not-number": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": 1},
					},
					//{},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": "123"},
					},
					{
						Values: map[string]interface{}{"not-name": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": "123"},
					},
					//{},
				},
			},
		},
	},
	"allow_multiple_tag_names": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tag-names": []string{
				"^id$",
				"^name$",
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{}},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{}},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Tags: map[string]string{"name": "dummy"},
					},
					{
						Tags: map[string]string{"not-name": "dummy"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{"name": "dummy"},
					},
					//{},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Tags: map[string]string{"name": "dummy"},
					},
					{
						Tags: map[string]string{"id": "dummy"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{"name": "dummy"},
					},
					{
						Tags: map[string]string{"id": "dummy"},
					},
					//{},
				},
			},
		},
	},
}

func TestEventAllow(t *testing.T) {
	for name, ts := range testset {
		if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
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
							t.Logf("failed at event allow, item %d, index %d", i, j)
							t.Logf("expected: %#v", item.output[j])
							t.Logf("     got: %#v", outs[j])
							t.Fail()
						}
					}
				})
			}
		}
	}
}
