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

	"github.com/openconfig/gnmic/formatters"
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

func Test_shift(t *testing.T) {
	type args struct {
		es          []string
		dropIndexes []int
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			// drop even indexes
			name: "1",
			args: args{
				es:          []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
				dropIndexes: []int{0, 2, 4, 6, 8},
			},
			want: []string{"B", "D", "F", "H"},
		},
		{
			// drop first and last
			name: "2",
			args: args{
				es:          []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
				dropIndexes: []int{0, 8},
			},
			want: []string{"B", "C", "D", "E", "F", "G", "H"},
		},
		{
			// no dropping
			name: "3",
			args: args{
				es:          []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
				dropIndexes: nil,
			},
			want: []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
		},
		{
			// drop index out of bound
			name: "4",
			args: args{
				es:          []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
				dropIndexes: []int{42},
			},
			want: []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
		},
		{
			// drop all
			name: "5",
			args: args{
				es:          []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
				dropIndexes: []int{0, 1, 2, 3, 4, 5, 6, 7, 8},
			},
			want: []string{},
		},
		{
			// drop first
			name: "6",
			args: args{
				es:          []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
				dropIndexes: []int{0},
			},
			want: []string{"B", "C", "D", "E", "F", "G", "H", "I"},
		},
		{
			// drop last
			name: "7",
			args: args{
				es:          []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
				dropIndexes: []int{8},
			},
			want: []string{"A", "B", "C", "D", "E", "F", "G", "H"},
		},
		{
			// drop all but last
			name: "8",
			args: args{
				es:          []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
				dropIndexes: []int{0, 1, 2, 3, 4, 5, 6, 7},
			},
			want: []string{"I"},
		},
		{
			// drop all but first
			name: "9",
			args: args{
				es:          []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"},
				dropIndexes: []int{1, 2, 3, 4, 5, 6, 7, 8},
			},
			want: []string{"A"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shift(tt.args.es, tt.args.dropIndexes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("shift() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkShift(b *testing.B) {
	var sl = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}
	drop := []int{0, 2, 3, 5, 8}
	for n := 0; n < b.N; n++ {
		shift(sl, drop)
	}
}
