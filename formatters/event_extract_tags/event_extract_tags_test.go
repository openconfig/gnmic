package event_extract_tags

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
	"match_value_names": {
		processorType: processorType,
		processor: map[string]interface{}{
			"debug": true,
			"value-names": []string{
				`/(?P<e1>\w+)/(?P<e2>\w+)/(?P<e3>\w+)`,
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
						Values: map[string]interface{}{"/elem1/elem2/elem3": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"/elem1/elem2/elem3": 1},
						Tags: map[string]string{
							"tag1": "1",
							"e1":   "elem1",
							"e2":   "elem2",
							"e3":   "elem3",
						},
					},
				},
			},
		},
	},
	"match_value_names_partial": {
		processorType: processorType,
		processor: map[string]interface{}{
			"debug": true,
			"value-names": []string{
				`/(?P<e1>\w+)/(\w+)/(?P<e3>\w+)`,
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
						Values: map[string]interface{}{"/elem1/elem2/elem3": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"/elem1/elem2/elem3": 1},
						Tags: map[string]string{
							"tag1": "1",
							"e1":   "elem1",
							"e3":   "elem3",
						},
					},
				},
			},
		},
	},
	"match_tag_names": {
		processorType: processorType,
		processor: map[string]interface{}{
			"debug": true,
			"tag-names": []string{
				`/(?P<e1>\w+)/(?P<e2>\w+)/(?P<e3>\w+)`,
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
							"tag1":               "1",
							"/elem1/elem2/elem3": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"/elem1/elem2/elem3": "1",
							"tag1":               "1",
							"e1":                 "elem1",
							"e2":                 "elem2",
							"e3":                 "elem3",
						},
					},
				},
			},
		},
	},
	"match_tag_names_partial": {
		processorType: processorType,
		processor: map[string]interface{}{
			"debug": true,
			"tag-names": []string{
				`/(?P<e1>\w+)/(\w+)/(?P<e3>\w+)`,
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
							"tag1":               "1",
							"/elem1/elem2/elem3": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"/elem1/elem2/elem3": "1",
							"tag1":               "1",
							"e1":                 "elem1",
							"e3":                 "elem3",
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
