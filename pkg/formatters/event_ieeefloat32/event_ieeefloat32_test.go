// © 2024 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_ieeefloat32

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
	"simple": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{
				"^components/component/power-supply/state/output-current$",
				"^components/component/power-supply/state/input-current$",
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
						Values: map[string]interface{}{"value": 1},
						Tags:   map[string]string{"tag1": "1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"value": 1},
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{
							"components/component/power-supply/state/output-current": "QEYAAA=="},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{
							"components/component/power-supply/state/output-current": float32(3.09375)},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{
							"components/component/power-supply/state/output-current": "QEYAAA==",
							"components/component/power-supply/state/input-current":  "QEYAAA==",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{
							"components/component/power-supply/state/output-current": float32(3.09375),
							"components/component/power-supply/state/input-current":  float32(3.09375),
						},
					},
				},
			},
		},
	},
}

func TestEventIEEEFloat32(t *testing.T) {
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
