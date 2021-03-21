package event_trigger

import (
	"log"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
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
	"init": {
		processorType: processorType,
		processor: map[string]interface{}{
			"debug": true,
			"action": map[string]interface{}{
				"type": "http",
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
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
			},
		},
	},
	"with_condition": {
		processorType: processorType,
		processor: map[string]interface{}{
			"condition": `.values["counter1"] > 90`,
			"debug":     true,
			"action": map[string]interface{}{
				"type": "http",
				"url":  "http://remote-alerting-system:9090/",
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
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter1": 91,
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter1": 91,
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter1": 89,
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter1": 89,
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter2": 91,
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter2": 91,
						},
					},
				},
			},
		},
	},
}

func TestEventTrigger(t *testing.T) {
	for name, ts := range testset {
		if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
			t.Log("found processor")
			p := pi()
			err := p.Init(ts.processor, formatters.WithLogger(log.New(os.Stderr, loggingPrefix, log.LstdFlags|log.Lmicroseconds)))
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
						if !cmp.Equal(outs[j], item.output[j]) {
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
