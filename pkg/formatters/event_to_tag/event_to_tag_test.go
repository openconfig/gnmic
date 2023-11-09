// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_to_tag

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
	"1_value_match": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{".*name$"},
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
						Values: map[string]interface{}{"name": "dummy"}},
				},
				output: []*formatters.EventMsg{
					{
						Tags:   map[string]string{"name": "dummy"},
						Values: map[string]interface{}{}},
				},
			},
		},
	},
	"1_value_match_with_keep": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{".*name$"},
			"keep":        true,
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
						Values: map[string]interface{}{"name": "dummy"}},
				},
				output: []*formatters.EventMsg{
					{
						Tags:   map[string]string{"name": "dummy"},
						Values: map[string]interface{}{"name": "dummy"}},
				},
			},
		},
	},
	"2_value_match": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{".*name$"},
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
						Values: map[string]interface{}{
							"name":        "dummy",
							"second_name": "dummy2"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"name":        "dummy",
							"second_name": "dummy2"},
						Values: map[string]interface{}{}},
				},
			},
		},
	},
	"2_value_match_with_keep": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{".*name$"},
			"keep":        true,
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
						Values: map[string]interface{}{
							"name":        "dummy",
							"second_name": "dummy2"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{
							"name":        "dummy",
							"second_name": "dummy2"},
						Values: map[string]interface{}{
							"name":        "dummy",
							"second_name": "dummy2"}},
				},
			},
		},
	},
}

func TestEventToTag(t *testing.T) {
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
				t.Run("uint_convert", func(t *testing.T) {
					t.Logf("running test item %d", i)
					outs := p.Apply(item.input...)
					for j := range outs {
						if !reflect.DeepEqual(outs[j], item.output[j]) {
							t.Logf("failed at event to_tag %s, item %d, index %d", name, i, j)
							t.Logf("expected: %#v", item.output[j])
							t.Logf("     got: %#v", outs[j])
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
