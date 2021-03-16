package event_date_string

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
	"seconds_date_string": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"timestamp"},
			"precision":   "s",
			"location":    "Asia/Taipei",
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"timestamp": 1606824673},
						Tags:   map[string]string{"timestamp": "0"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"timestamp": "2020-12-01T20:11:13+08:00"},
						Tags:   map[string]string{"timestamp": "0"},
					},
				},
			},
		},
	},
}

func TestEventDateString(t *testing.T) {
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
					outs := p.Apply(item.input...)
					for j := range outs {
						if !reflect.DeepEqual(outs[j], item.output[j]) {
							t.Logf("failed at event date string, item %d, index %d", i, j)
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
