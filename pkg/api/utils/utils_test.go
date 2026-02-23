// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var convertTestSet = []struct {
	name string
	in   interface{}
	out  interface{}
}{
	{
		name: "string",
		in:   "test1",
		out:  "test1",
	},
	{
		name: "map[interface{}]interface{}",
		in: map[interface{}]interface{}{
			"a": "b",
		},
		out: map[string]interface{}{
			"a": "b",
		},
	},
	{
		name: "map[string]interface{}",
		in: map[string]interface{}{
			"a": map[interface{}]interface{}{
				"b": "c",
			},
		},
		out: map[string]interface{}{
			"a": map[string]interface{}{
				"b": "c",
			},
		},
	},
	{
		name: "[]interface{}",
		in: []interface{}{
			"a",
		},
		out: []interface{}{
			"a",
		},
	},
}

func TestConvert(t *testing.T) {
	for _, item := range convertTestSet {
		t.Run(item.name, func(t *testing.T) {
			o := Convert(item.in)
			if !cmp.Equal(o, item.out) {
				t.Logf("%q failed", item.name)
				t.Fail()
			}
		})
	}
}

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		dst  map[string]interface{}
		src  map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "empty",
			dst:  nil,
			src:  nil,
			want: map[string]interface{}{},
		},
		{
			name: "empty_dst",
			dst:  nil,
			src:  map[string]interface{}{"a": "b"},
			want: map[string]interface{}{"a": "b"},
		},
		{
			name: "empty_src",
			dst:  map[string]interface{}{"a": "b"},
			src:  nil,
			want: map[string]interface{}{"a": "b"},
		},
		{
			name: "merge",
			dst:  map[string]interface{}{"a": "b"},
			src:  map[string]interface{}{"a": "c"},
			want: map[string]interface{}{"a": "c"},
		},
		{
			name: "merge_with_map",
			dst:  map[string]interface{}{"a": "b"},
			src:  map[string]interface{}{"a": map[string]interface{}{"c": "d"}},
			want: map[string]interface{}{"a": map[string]interface{}{"c": "d"}},
		},
		{
			name: "merge_with_map_and_slice",
			dst:  map[string]interface{}{"a": "b"},
			src:  map[string]interface{}{"a": map[string]interface{}{"c": "d"}},
			want: map[string]interface{}{"a": map[string]interface{}{"c": "d"}},
		},
		{
			name: "merge_with_slice",
			dst:  map[string]interface{}{"a": "b"},
			src:  map[string]interface{}{"a": []interface{}{"c", "d"}},
			want: map[string]interface{}{"a": []interface{}{"c", "d"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeMaps(tt.dst, tt.src)
			if !reflect.DeepEqual(got, tt.want) {
				t.Logf("%q failed", tt.name)
				t.Logf("got: %v", got)
				t.Logf("want: %v", tt.want)
				t.Fail()
			}
		})
	}
}
