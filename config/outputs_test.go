package config

import (
	"bytes"
	"reflect"
	"testing"
)

var getOutputsTestSet = map[string]struct {
	in  []byte
	out map[string]map[string]interface{}
}{
	"basic_outputs": {
		in: []byte(`
outputs:
  output1:
    type: file
    file-type: stdout
  output2:
    type: nats
`),
		out: map[string]map[string]interface{}{
			"output1": {
				"type":      "file",
				"file-type": "stdout",
				"format":    "",
			},
			"output2": {
				"type":   "nats",
				"format": "",
			},
		},
	},
}

func TestGetOutputs(t *testing.T) {
	for name, data := range getOutputsTestSet {
		t.Run(name, func(t *testing.T) {
			cfg := New()
			cfg.Debug = true
			cfg.SetLogger()
			cfg.FileConfig.SetConfigType("yaml")
			err := cfg.FileConfig.ReadConfig(bytes.NewBuffer(data.in))
			if err != nil {
				t.Logf("failed reading config: %v", err)
				t.Fail()
			}
			v := cfg.FileConfig.Get("outputs")
			t.Logf("raw interface outputs: %+v", v)
			outs, err := cfg.GetOutputs()
			t.Logf("exp value: %+v", data.out)
			t.Logf("got value: %+v", outs)
			if err != nil {
				t.Logf("failed getting outputs: %v", err)
				t.Fail()
			}
			if !reflect.DeepEqual(outs, data.out) {
				t.Log("maps not equal")
				t.Fail()
			}
		})
	}
}
