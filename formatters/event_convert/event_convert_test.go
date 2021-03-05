package event_convert

import (
	"reflect"
	"testing"

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
	"int_convert": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"^number*"},
			"type":        "int",
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
						Values: map[string]interface{}{"name": 1},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"name": 1},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": "100"},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": int(100)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": "100"},
						Tags:   map[string]string{"number": "name_tag"},
					},
					{
						Values: map[string]interface{}{"number": "200"},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": int(100)},
						Tags:   map[string]string{"number": "name_tag"},
					},
					{
						Values: map[string]interface{}{"number": int(200)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Tags:   map[string]string{"number": "name_tag"},
					},
					{
						Values: map[string]interface{}{"number": "200"},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags:   map[string]string{"number": "name_tag"},
					},
					{
						Values: map[string]interface{}{"number": int(200)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": int(100)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": int(100)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": uint(100)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": int(100)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": float64(100)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": int(100)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": true},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": true},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
		},
	},
	"uint_convert": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"^name.*"},
			"type":        "uint",
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input:  []*formatters.EventMsg{{Values: map[string]interface{}{}}},
				output: []*formatters.EventMsg{{Values: map[string]interface{}{}}},
			},
			{
				input: []*formatters.EventMsg{{
					Values: map[string]interface{}{"name_value_bytes": "42"}}},
				output: []*formatters.EventMsg{{
					Values: map[string]interface{}{"name_value_bytes": uint(42)}}},
			},
			{
				input: []*formatters.EventMsg{{
					Values: map[string]interface{}{"name_value_bytes": uint(42)}}},
				output: []*formatters.EventMsg{{
					Values: map[string]interface{}{"name_value_bytes": uint(42)}}},
			},
			{
				input: []*formatters.EventMsg{{
					Values: map[string]interface{}{"name_value_bytes": -42}}},
				output: []*formatters.EventMsg{{
					Values: map[string]interface{}{"name_value_bytes": uint(0)}}},
			},
			{
				input: []*formatters.EventMsg{{
					Values: map[string]interface{}{"name_value_bytes": true}}},
				output: []*formatters.EventMsg{{
					Values: map[string]interface{}{"name_value_bytes": true}}},
			},
		},
	},
	"float_convert": {
		processorType: processorType,
		processor:     map[string]interface{}{"value-names": []string{"^number*"}, "type": "float"},
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
						Values: map[string]interface{}{"number": "1.1"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": float64(1.1)},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": uint(42)},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": float64(42)},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": int(42)},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": float64(42)},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": true},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": true},
					},
				},
			},
		},
	},
}

func TestEventConvertToUint(t *testing.T) {
	ts := testset["uint_convert"]
	if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
		t.Log("found processor")
		p := pi()
		err := p.Init(ts.processor, nil)
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
						t.Logf("failed at uint_convert item %d, index %d", i, j)
						t.Logf("expected: %#v", item.output[j])
						t.Logf("     got: %#v", outs[j])
						t.Fail()
					}
				}
			})
		}
	}
}

func TestEventConvertToInt(t *testing.T) {
	ts := testset["int_convert"]
	if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
		t.Log("found processor")
		p := pi()
		err := p.Init(ts.processor, nil)
		if err != nil {
			t.Errorf("failed to initialize processors: %v", err)
			return
		}
		for i, item := range ts.tests {
			t.Run("int_convert", func(t *testing.T) {
				t.Logf("running test item %d", i)
				outs := p.Apply(item.input...)
				for j := range outs {
					if !reflect.DeepEqual(outs[j], item.output[j]) {
						t.Logf("failed at int_convert item %d, index %d", i, j)
						t.Logf("expected: %#v", item.output[j])
						t.Logf("     got: %#v", outs[j])
						t.Fail()
					}
				}
			})
		}
	}
}

func TestEventConvertToString(t *testing.T) {
	ts := testset["string_convert"]
	if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
		t.Log("found processor")
		p := pi()
		err := p.Init(ts.processor, nil)
		if err != nil {
			t.Errorf("failed to initialize processors: %v", err)
			return
		}
		for i, item := range ts.tests {
			t.Run("string_convert", func(t *testing.T) {
				t.Logf("running test item %d", i)
				outs := p.Apply(item.input...)
				for j := range outs {
					if !reflect.DeepEqual(outs[j], item.output[j]) {
						t.Logf("failed at string_convert item %d, index %d", i, j)
						t.Logf("expected: %#v", item.output[j])
						t.Logf("     got: %#v", outs[j])
						t.Fail()
					}
				}
			})
		}
	}
}

func TestEventConvertToFloat(t *testing.T) {
	ts := testset["float_convert"]
	if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
		t.Log("found processor")
		p := pi()
		err := p.Init(ts.processor, nil)
		if err != nil {
			t.Errorf("failed to initialize processors: %v", err)
			return
		}
		for i, item := range ts.tests {
			t.Run("float_convert", func(t *testing.T) {
				t.Logf("running test item %d", i)
				outs := p.Apply(item.input...)
				for j := range outs {
					if !reflect.DeepEqual(outs[j], item.output[j]) {
						t.Logf("failed at float_convert item %d, index %d", i, j)
						t.Logf("expected: %#v", item.output[j])
						t.Logf("     got: %#v", outs[j])
						t.Fail()
					}
				}
			})
		}
	}
}
