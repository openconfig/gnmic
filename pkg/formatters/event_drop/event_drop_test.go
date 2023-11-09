// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_drop

import (
	"reflect"
	"testing"

	"github.com/openconfig/gnmic/pkg/formatters"
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
	"drop_condition": {
		processorType: processorType,
		processor: map[string]interface{}{
			"condition": ".values.value == 1",
			"debug":     true,
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
						Values: map[string]interface{}{"value": 1},
					},
				},
				output: nil,
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 0},
					},
					{
						Values: map[string]interface{}{"value": 1},
					},
					{
						Values: map[string]interface{}{"value": 2},
					},
					{
						Values: map[string]interface{}{"value": 3},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 0},
					},
					{
						Values: map[string]interface{}{"value": 2},
					},
					{
						Values: map[string]interface{}{"value": 3},
					},
				},
			},
		},
	},
	"drop_values": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"^number$"},
			"debug":       true,
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
						Values: map[string]interface{}{"number": 1},
					},
				},
				output: nil,
			},
		},
	},
	"drop_tags": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tag-names": []string{"^name*"},
			"debug":     true,
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
						Tags: map[string]string{"name": "dummy"},
					},
				},
				output: nil,
			},
		},
	},
}

func TestEventDrop(t *testing.T) {
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
					if len(outs) != len(item.output) {
						t.Logf("output length mismatch")
						t.Fail()
					}
					for j := range outs {
						if !reflect.DeepEqual(outs[j], item.output[j]) {
							t.Logf("failed at event drop, item %d, index %d", i, j)
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

var input = []*formatters.EventMsg{
	{
		Values: map[string]interface{}{"value": 0},
	},
	{
		Values: map[string]interface{}{"value": 1},
	},
	{
		Values: map[string]interface{}{"value": 2},
	},
	{
		Values: map[string]interface{}{"value": 3},
	},
}

func BenchmarkApply(b *testing.B) {
	pi := formatters.EventProcessors["event-drop"]
	p := pi()
	err := p.Init(map[string]interface{}{
		"condition": ".values.value >= 1",
	})
	if err != nil {
		panic(err)
	}

	for i := 0; i < b.N; i++ {
		p.Apply(input...)
	}
}
