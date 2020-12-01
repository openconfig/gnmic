package event_date_string

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
	processor map[string]interface{}
	tests     []item
}{
	"seconds_date_string": {
		processor: map[string]interface{}{
			"type":             "event_date_string",
			"values":           []string{"timestamp"},
			"timestamp_format": "s",
			//"date_time_format": "2005-12-12",
			"location": "Asia/Taipei",
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
					Values: map[string]interface{}{"timestamp": 1606824673},
					Tags:   map[string]string{"timestamp": "0"},
				},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{"timestamp": "2020-12-01T20:11:13+08:00"},
					Tags:   map[string]string{"timestamp": "0"},
				},
			},
		},
	},
}

func TestEventDateString(t *testing.T) {
	for name, ts := range testset {
		if typ, ok := ts.processor["type"]; ok {
			t.Log("found type")
			if pi, ok := formatters.EventProcessors[typ.(string)]; ok {
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
						if !reflect.DeepEqual(item.input, item.output) {
							t.Errorf("failed at %s item %d, expected %+v, got: %+v", name, i, item.output, item.input)
						}
						// if !cmp.Equal(item.input, item.output) {
						// 	t.Errorf("failed at %s item %d, expected %+v, got: %+v", name, i, item.output, item.input)
						// }
					})
				}
			}
		}
	}
}
