package event_strings

import (
	"reflect"
	"testing"

	"github.com/karimra/gnmic/formatters"
)

type item struct {
	input  *formatters.EventMsg
	output *formatters.EventMsg
}

var testset = map[string]struct {
	processorType string
	processor     map[string]interface{}
	tests         []item
}{
	"replace": {
		processorType: "event_strings",
		processor: map[string]interface{}{
			"value_names": []string{"^name$"},
			"tag_names":   []string{"tag"},
			"transforms": []map[string]*transform{
				{
					"replace": &transform{
						On:  "name",
						Old: "name",
						New: "new_name",
					},
				},
				{
					"replace": &transform{
						On:  "name",
						Old: "tag",
						New: "new_tag",
					},
				},
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{
						"name": "foo",
					}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{
						"new_name": "foo",
					}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"tag": "foo",
					}},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"new_tag": "foo",
					}},
			},
		},
	},
	"trim_prefix": {
		processorType: "event_strings",
		processor: map[string]interface{}{
			"value_names": []string{"^prefix_"},
			"transforms": []map[string]*transform{
				{
					"trim_prefix": &transform{
						On:     "name",
						Prefix: "prefix_",
					},
				},
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"prefix_name": "foo",
					},
					Values: map[string]interface{}{
						"prefix_name": "foo",
					}},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"prefix_name": "foo",
					},
					Values: map[string]interface{}{
						"name": "foo",
					}},
			},
		},
	},
	"trim_suffix": {
		processorType: "event_strings",
		processor: map[string]interface{}{
			"value_names": []string{"_suffix$"},
			"transforms": []map[string]*transform{
				{
					"trim_suffix": &transform{
						On:     "name",
						Suffix: "_suffix",
					},
				},
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"name_suffix": "foo",
					},
					Values: map[string]interface{}{
						"name_suffix": "foo",
					}},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"name_suffix": "foo",
					},
					Values: map[string]interface{}{
						"name": "foo",
					}},
			},
		},
	},
	"title": {
		processorType: "event_strings",
		processor: map[string]interface{}{
			"value_names": []string{"title"},
			"transforms": []map[string]*transform{
				{
					"title": &transform{
						On: "name",
					},
				},
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"title": "foo",
					},
					Values: map[string]interface{}{
						"title": "foo",
					}},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"title": "foo",
					},
					Values: map[string]interface{}{
						"Title": "foo",
					}},
			},
		},
	},
	"to_upper": {
		processorType: "event_strings",
		processor: map[string]interface{}{
			"value_names": []string{"to_be_capitalized"},
			"transforms": []map[string]*transform{
				{
					"to_upper": &transform{
						On: "name",
					},
				},
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"to_be_capitalized": "foo",
					},
					Values: map[string]interface{}{
						"to_be_capitalized": "foo",
					}},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"to_be_capitalized": "foo",
					},
					Values: map[string]interface{}{
						"TO_BE_CAPITALIZED": "foo",
					}},
			},
		},
	},
	"to_lower": {
		processorType: "event_strings",
		processor: map[string]interface{}{
			"value_names": []string{"TO_BE_LOWERED"},
			"transforms": []map[string]*transform{
				{
					"to_lower": &transform{
						On: "name",
					},
				},
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"TO_BE_LOWERED": "foo",
					},
					Values: map[string]interface{}{
						"TO_BE_LOWERED": "foo",
					}},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"TO_BE_LOWERED": "foo",
					},
					Values: map[string]interface{}{
						"to_be_lowered": "foo",
					}},
			},
		},
	},
	"split": {
		processorType: "event_strings",
		processor: map[string]interface{}{
			"value_names": []string{"path/to/a/resource"},
			"transforms": []map[string]*transform{
				{
					"split": &transform{
						On:          "name",
						SplitOn:     "/",
						JoinWith:    "_",
						IgnoreFirst: 2,
					},
				},
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"path/to/a/resource": "foo",
					},
					Values: map[string]interface{}{
						"path/to/a/resource": "foo",
					}},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"path/to/a/resource": "foo",
					},
					Values: map[string]interface{}{
						"a_resource": "foo",
					}},
			},
		},
	},
	"path_base": {
		processorType: "event_strings",
		processor: map[string]interface{}{
			"value_names": []string{"path/to/a/resource"},
			"transforms": []map[string]*transform{
				{
					"path_base": &transform{
						On: "name",
					},
				},
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input: &formatters.EventMsg{
					Values: map[string]interface{}{}},
				output: &formatters.EventMsg{
					Values: map[string]interface{}{}},
			},
			{
				input: &formatters.EventMsg{
					Tags: map[string]string{
						"path/to/a/resource": "foo",
					},
					Values: map[string]interface{}{
						"path/to/a/resource": "foo",
					}},
				output: &formatters.EventMsg{
					Tags: map[string]string{
						"path/to/a/resource": "foo",
					},
					Values: map[string]interface{}{
						"resource": "foo",
					}},
			},
		},
	},
}

func TestEventStrings(t *testing.T) {
	for name, ts := range testset {
		if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
			t.Log("found processor")
			p := pi()
			err := p.Init(ts.processor, nil)
			if err != nil {
				t.Errorf("failed to initialize processors: %v", err)
				return
			}
			t.Logf("initialized for test %s: %+v", name, p)
			for i, item := range ts.tests {
				t.Run(name, func(t *testing.T) {
					t.Logf("running test item %d", i)
					var inputMsg *formatters.EventMsg
					if item.input != nil {
						inputMsg = &formatters.EventMsg{
							Name:      item.input.Name,
							Timestamp: item.input.Timestamp,
							Tags:      make(map[string]string),
							Values:    make(map[string]interface{}),
							Deletes:   item.input.Deletes,
						}
						for k, v := range item.input.Tags {
							inputMsg.Tags[k] = v
						}
						for k, v := range item.input.Values {
							inputMsg.Values[k] = v
						}
					}
					p.Apply(item.input)
					t.Logf("input: %+v, changed: %+v", inputMsg, item.input)
					if !reflect.DeepEqual(item.input, item.output) {
						t.Errorf("failed at %s item %d, expected %+v, got: %+v", name, i, item.output, item.input)
					}
				})
			}
		}
	}
}
