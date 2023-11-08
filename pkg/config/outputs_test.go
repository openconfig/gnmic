// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

var getOutputsTestSet = map[string]struct {
	envs []string
	in   []byte
	out  map[string]map[string]interface{}
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
	"basic_outputs_env": {
		envs: []string{
			"NATS_ADDRESS=1.1.1.1",
		},
		in: []byte(`
outputs:
  output1:
    type: file
    file-type: stdout
  output2:
    type: nats
    address: ${NATS_ADDRESS}:1123
`),
		out: map[string]map[string]interface{}{
			"output1": {
				"type":      "file",
				"file-type": "stdout",
				"format":    "",
			},
			"output2": {
				"type":    "nats",
				"format":  "",
				"address": "1.1.1.1:1123",
			},
		},
	},
}

func TestGetOutputs(t *testing.T) {
	for name, data := range getOutputsTestSet {
		t.Run(name, func(t *testing.T) {
			for _, e := range data.envs {
				p := strings.SplitN(e, "=", 2)
				os.Setenv(p[0], p[1])
			}
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
