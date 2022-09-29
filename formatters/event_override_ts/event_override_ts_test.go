// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_override_ts

import (
	"testing"
	"time"

	"github.com/karimra/gnmic/formatters"
)

type item struct {
	input  []*formatters.EventMsg
	output []*formatters.EventMsg
}

var now = time.Now()

var testset = map[string]struct {
	processor map[string]interface{}
	tests     []item
}{
	"ms": {
		processor: map[string]interface{}{
			"type":  processorType,
			"debug": true,
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
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: now.UnixNano() / 1000000,
					},
				},
			},
		},
	},
	"ns": {
		processor: map[string]interface{}{
			"type":      processorType,
			"precision": "ns",
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
						Timestamp: -1,
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: now.UnixNano(),
					},
				},
			},
		},
	},
	"us": {
		processor: map[string]interface{}{
			"type":      processorType,
			"precision": "us",
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
						Timestamp: -1,
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: now.UnixNano() / 1000,
					},
				},
			},
		},
	},
	"s": {
		processor: map[string]interface{}{
			"type":      processorType,
			"precision": "s",
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
						Timestamp: -1,
					},
				},
				output: []*formatters.EventMsg{
					{
						Timestamp: now.Unix(),
					},
				},
			},
		},
	},
}

func TestEventDateString(t *testing.T) {
	for name, ts := range testset {
		t.Log(name)
		if typ, ok := ts.processor["type"]; ok {
			t.Log("found type")
			if pi, ok := formatters.EventProcessors[typ.(string)]; ok {
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
							if outs[j].Timestamp < item.output[j].Timestamp {
								t.Logf("failed at event override_ts, item %d, index %d", i, j)
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
}
