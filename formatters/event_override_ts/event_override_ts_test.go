package event_override_ts

import (
	"testing"
	"time"

	"github.com/karimra/gnmic/formatters"
)

type item struct {
	input  *formatters.EventMsg
	output *formatters.EventMsg
}

var now = time.Now()

var testset = map[string]struct {
	processor map[string]interface{}
	tests     []item
}{
	"seconds_date_string": {
		processor: map[string]interface{}{},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{},
			},
			{
				input: &formatters.EventMsg{
					Timestamp: now.Unix(),
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
						if inputMsg == nil && item.input != nil {
							t.Errorf("failed at %s item %d", name, i)
							t.Fail()
							return
						} else if inputMsg != nil && item.input == nil {
							t.Errorf("failed at %s item %d", name, i)
							t.Fail()
							return
						}
						if item.input.Timestamp >= inputMsg.Timestamp {
							t.Errorf("failed at %s item %d", name, i)
							t.Fail()
						}
					})
				}
			}
		}
	}
}
