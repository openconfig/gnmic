package event_convert

import (
	"reflect"
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
	"string_convert": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{
				"^convert-me$",
				"^number*",
			},
			"debug": true,
			"type":  "string",
		},
		tests: []item{
			// nil msg
			{
				input:  nil,
				output: nil,
			},
			// empty msg
			{
				input:  make([]*formatters.EventMsg, 0),
				output: make([]*formatters.EventMsg, 0),
			},
			// non matching values
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
			// matching values and tags
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"convert-me": 100},
						Tags:   map[string]string{"convert-me": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"convert-me": "100"},
						Tags:   map[string]string{"convert-me": "name_tag"},
					},
				},
			},
			// 2 msgs, with matching values
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"convert-me": 100},
						Tags:   map[string]string{"convert-me": "name_tag"},
					},
					{
						Values: map[string]interface{}{"convert-me": 200},
						Tags:   map[string]string{"convert-me": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"convert-me": "100"},
						Tags:   map[string]string{"convert-me": "name_tag"},
					},
					{
						Values: map[string]interface{}{"convert-me": "200"},
						Tags:   map[string]string{"convert-me": "name_tag"},
					},
				},
			},
			// 2 msgs, second with matching values
			{
				input: []*formatters.EventMsg{
					{
						Tags: map[string]string{"convert-me": "name_tag"},
					},
					{
						Values: map[string]interface{}{"convert-me": 200},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{"convert-me": "name_tag"},
					},
					{
						Values: map[string]interface{}{"convert-me": "200"},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
			// matching value, already a string
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"convert-me": "1"},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"convert-me": "1"},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
			// matching value, uint
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{
							"number1": uint8(100),
							"number2": uint16(100),
							"number3": uint32(100),
							"number4": uint64(100),
						},
						Tags: map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{
							"number1": "100",
							"number2": "100",
							"number3": "100",
							"number4": "100",
						},
						Tags: map[string]string{"number": "name_tag"},
					},
				},
			},
			// matching value, float64
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": float64(100.1)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": "100.1"},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
			// matching value, bool
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": true},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": "true"},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
		},
	},
	"int_convert": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"^number*"},
			"type":        "int",
		},
		tests: []item{
			// nil msg
			{
				input:  nil,
				output: nil,
			},
			// empty msg
			{
				input:  make([]*formatters.EventMsg, 0),
				output: make([]*formatters.EventMsg, 0),
			},
			// non matching values
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
			// matching values and tags
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
			// 2 msgs, with matching values
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
			// 2 msgs, second with matching values
			{
				input: []*formatters.EventMsg{
					{
						Tags: map[string]string{"number": "name_tag"},
					},
					{
						Values: map[string]interface{}{"number": "200"},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Tags: map[string]string{"number": "name_tag"},
					},
					{
						Values: map[string]interface{}{"number": int(200)},
						Tags:   map[string]string{"number": "name_tag"},
					},
				},
			},
			// matching value, already an int
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
			// matching value, uint
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{
							"number1": uint8(100),
							"number2": uint16(100),
							"number3": uint32(100),
							"number4": uint64(100),
						},
						Tags: map[string]string{"number": "name_tag"},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{
							"number1": int(100),
							"number2": int(100),
							"number3": int(100),
							"number4": int(100),
						},
						Tags: map[string]string{"number": "name_tag"},
					},
				},
			},
			// matching value, float64
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
			// matching value, bool
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
			{
				input: []*formatters.EventMsg{{
					Values: map[string]interface{}{
						"name_value_bytes1": int8(74),
						"name_value_bytes2": int16(75),
						"name_value_bytes3": int32(76),
						"name_value_bytes4": int64(77),
					}}},

				output: []*formatters.EventMsg{{
					Values: map[string]interface{}{
						"name_value_bytes1": uint(74),
						"name_value_bytes2": uint(75),
						"name_value_bytes3": uint(76),
						"name_value_bytes4": uint(77),
					}}},
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
						Values: map[string]interface{}{"number": []uint8{62, 192, 0, 0}},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": float64(0.375)},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": []uint8{64, 9, 33, 251, 84, 68, 45, 24}},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": float64(3.141592653589793)},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": []uint8{64, 9, 33, 251, 84, 68, 45, 24, 32}},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": float64(0)},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": []uint8{62, 192, 0, 0}},
					},
				},
				output: []*formatters.EventMsg{
					{
						Values: map[string]interface{}{"number": float64(0.375)},
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
		err := p.Init(ts.processor, formatters.WithLogger(nil))
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
		err := p.Init(ts.processor, formatters.WithLogger(nil))
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
		err := p.Init(ts.processor, formatters.WithLogger(nil))
		if err != nil {
			t.Errorf("failed to initialize processors: %v", err)
			return
		}
		for i, item := range ts.tests {
			t.Run("string_convert", func(t *testing.T) {
				t.Logf("running test item %d", i)
				outs := p.Apply(item.input...)
				for j := range outs {
					if !cmp.Equal(outs[j], item.output[j]) {
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
		err := p.Init(ts.processor, formatters.WithLogger(nil))
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
