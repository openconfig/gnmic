package event_write

import (
	"bytes"
	"io"
	"log"
	"testing"

	"github.com/karimra/gnmic/formatters"
)

type item struct {
	input  *formatters.EventMsg
	output string
}

var testset = map[string]struct {
	processorType string
	processor     map[string]interface{}
	tests         []item
}{
	"write_condition": {
		processorType: processorType,
		processor: map[string]interface{}{
			"condition": `.values.number == "42"`,
			"separator": "sep",
		},
		tests: []item{
			{
				input:  nil,
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{}},
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{}},
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{"number": "42"}},
				output: `{"values":{"number":"42"}}sep`,
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{"name": "foo"},
					Values: map[string]interface{}{"number": "42"}},
				output: `{"tags":{"name":"foo"},"values":{"number":"42"}}sep`,
			},
		},
	},
	"write_values_all": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"."},
			"separator":   "sep",
		},
		tests: []item{
			{
				input:  nil,
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{}},
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{}},
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{"number": "42"}},
				output: `{"values":{"number":"42"}}sep`,
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{"name": "foo"},
					Values: map[string]interface{}{"number": "42"}},
				output: `{"tags":{"name":"foo"},"values":{"number":"42"}}sep`,
			},
		},
	},
	"write_values_some": {
		processorType: processorType,
		processor: map[string]interface{}{
			"value-names": []string{"^number"},
			"separator":   "sep",
		},
		tests: []item{
			{
				input:  nil,
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{}},
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{}},
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{"number": "42"}},
				output: `{"values":{"number":"42"}}sep`,
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{"name": "foo"},
					Values: map[string]interface{}{"not_number": "42"}},
				output: ``,
			},
		},
	},
	"write_tags_all": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tag-names": []string{"."},
			"separator": "sep",
		},
		tests: []item{
			{
				input:  nil,
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{}},
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{}},
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{"number": "42"}},
				output: ``,
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{"name": "foo"},
					Values: map[string]interface{}{"number": "42"}},
				output: `{"tags":{"name":"foo"},"values":{"number":"42"}}sep`,
			},
		},
	},
	"write_tags_some": {
		processorType: processorType,
		processor: map[string]interface{}{
			"tag-names": []string{"^name"},
			"separator": "sep",
		},
		tests: []item{
			{
				input:  nil,
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{}},
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{}},
				output: "",
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{},
					Values: map[string]interface{}{"number": "42"}},
				output: ``,
			},
			{
				input: &formatters.EventMsg{
					Tags:   map[string]string{"name": "foo"},
					Values: map[string]interface{}{"number": "42"}},
				output: `{"tags":{"name":"foo"},"values":{"number":"42"}}sep`,
			},
		},
	},
}

func TestEventWrite(t *testing.T) {
	for name, ts := range testset {
		p := &Write{logger: log.New(io.Discard, "", 0)}
		err := p.Init(ts.processor)
		if err != nil {
			t.Errorf("failed to initialize processors: %v", err)
			return
		}
		t.Logf("initialized for test %s: %+v", name, p)
		for i, item := range ts.tests {
			t.Run(name, func(t *testing.T) {
				buff := new(bytes.Buffer)
				p.dst = buff
				t.Logf("running '%s' test item %d", name, i)
				p.Apply(item.input)
				if buff.String() != item.output {
					t.Errorf("failed at %s item %d, expected %+v, got: %+v", name, i, item.output, buff.String())
				}
			})
		}
	}
}
