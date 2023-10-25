package event_rate_limit

import (
	"testing"

	"github.com/openconfig/gnmic/formatters"
)

type item struct {
	input  []*formatters.EventMsg
	output []*formatters.EventMsg
}

var testset = map[string]struct {
	processor map[string]interface{}
	tests     []item
}{
	"1pps-notags-pass": {
		processor: map[string]interface{}{
			"type":  processorType,
			"debug": true,
			"per-second": 1.0,
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 0,
					},
					{
						Timestamp: 1e9+1,
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 0,
					},
					{
						Timestamp: 1e9+1,
					},
				},
			},
		},
	},
	"1pps-tags-pass": {
		processor: map[string]interface{}{
			"type":      processorType,
			"per-second": 1.0,
			"debug":     true,
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 0,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
					{
						Timestamp: 1,
					},
					{
						Timestamp: 1+1e9,
					},
					{
						Timestamp: 1e9+1,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 0,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
					{
						Timestamp: 1,
					},
					{
						Timestamp: 1+1e9,
					},
					{
						Timestamp: 1e9+1,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
				},
			},
		},
	},
	"1pps-notags-drop": {
		processor: map[string]interface{}{
			"type":  processorType,
			"debug": true,
			"per-second": 1.0,
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 0,
					},
					{
						Timestamp: 1e9-1,
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 0,
					},
				},
			},
		},
	},
	"1pps-tags-drop": {
		processor: map[string]interface{}{
			"type":      processorType,
			"per-second": 1.0,
			"debug":     true,
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 0,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
					{
						Timestamp: 1,
					},
					{
						Timestamp: 1e9-1,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 0,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
					{
						Timestamp: 1,
					},
				},
			},
		},
	},
	"100pps-tags-pass": {
		processor: map[string]interface{}{
			"type":      processorType,
			"per-second": 100.0,
			"debug":     true,
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 0,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
					{
						Timestamp: 1,
					},
					{
						Timestamp: 1e9/100,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 0,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
					{
						Timestamp: 1,
					},
					{
						Timestamp: 1e9/100,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
				},
			},
		},
	},
	"100pps-tags-drop": {
		processor: map[string]interface{}{
			"type":      processorType,
			"per-second": 100.0,
			"debug":     true,
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 0,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
					{
						Timestamp: 1,
					},
					{
						Timestamp: 1e9/100-1,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 0,
						Tags: map[string]string{
							"a": "val-x",
							"b": "val-y",
						},
					},
					{
						Timestamp: 1,
					},
				},
			},
		},
	},
	"same-ts-pass": {
		processor: map[string]interface{}{
			"type":      processorType,
			"per-second": 100.0,
			"debug":     true,
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: []*formatters.EventMsg{},
			},
			{
				input: []*formatters.EventMsg{
					{
						Timestamp: 0,
					},
					{
						Timestamp: 0,
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: 0,
					},
					{
						Timestamp: 0,
					},
				},
			},
		},
	},
}

func TestRateLimit(t *testing.T) {
	for name, ts := range testset {
		t.Log(name)
		if typ, ok := ts.processor["type"]; ok {
			t.Log("found type")
			if pi, ok := formatters.EventProcessors[typ.(string)]; ok {
				t.Log("found processor")
				p := pi()
				err := p.Init(ts.processor, formatters.WithLogger(nil))
				if err != nil {
					t.Errorf("failed to initialize processors: %v", err)
					return
				}
				t.Logf("initialized for test %s: %+v", name, p)
				for i, item := range ts.tests {
					t.Run(name, func(t *testing.T) {
						t.Logf("running test item %d", i)
						outs := p.Apply(item.input...)
						if len(outs) != len(item.output) {
							t.Logf("failed at event rate_limit, item %d", i)
							t.Logf("different number of events between output=%d and wanted=%d", len(outs), len(item.output))
							t.Fail()
						}
					})
				}
			}
		}
	}
}
