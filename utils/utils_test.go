package utils

import (
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
