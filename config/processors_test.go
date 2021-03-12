package config

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

var getProcessorsTestSet = map[string]struct {
	envs []string
	in   []byte
	out  map[string]map[string]interface{}
}{
	"basic_processors": {
		in: []byte(`
processors:
  proc-convert-integer:
    event-convert:
      value-names:
        - ".*"
      type: int

  proc-delete-tag-name:
    event-delete:
      tag-names:
        - "^subscription-name"

  proc-delete-value-name:
    event-delete:
      value-names:
        - ".*out-unicast-packets"
`),
		out: map[string]map[string]interface{}{
			"proc-convert-integer": {
				"event-convert": map[string]interface{}{
					"value-names": []interface{}{".*"},
					"type":        "int",
				},
			},
			"proc-delete-tag-name": {
				"event-delete": map[string]interface{}{
					"tag-names": []interface{}{"^subscription-name"},
				},
			},
			"proc-delete-value-name": {
				"event-delete": map[string]interface{}{
					"value-names": []interface{}{".*out-unicast-packets"},
				},
			},
		},
	},
	"basic_processors_with_env": {
		envs: []string{
			"PROC_CONVERT_TYPE=int",
		},
		in: []byte(`
processors:
  proc-convert-integer:
    event-convert:
      value-names:
        - ".*"
      type: ${PROC_CONVERT_TYPE}

  proc-delete-tag-name:
    event-delete:
      tag-names:
        - "^subscription-name"

  proc-delete-value-name:
    event-delete:
      value-names:
        - ".*out-unicast-packets"
`),
		out: map[string]map[string]interface{}{
			"proc-convert-integer": {
				"event-convert": map[string]interface{}{
					"value-names": []interface{}{".*"},
					"type":        "int",
				},
			},
			"proc-delete-tag-name": {
				"event-delete": map[string]interface{}{
					"tag-names": []interface{}{"^subscription-name"},
				},
			},
			"proc-delete-value-name": {
				"event-delete": map[string]interface{}{
					"value-names": []interface{}{".*out-unicast-packets"},
				},
			},
		},
	},
}

func TestGetProcessors(t *testing.T) {
	for name, data := range getProcessorsTestSet {
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
			v := cfg.FileConfig.Get("processors")
			t.Logf("raw interface processors: %+v", v)
			outs, err := cfg.GetEventProcessors()
			t.Logf("exp value: %+v", data.out)
			t.Logf("got value: %+v", outs)
			if err != nil {
				t.Logf("failed getting processors: %v", err)
				t.Fail()
			}
			//assert.EqualValues(t, data.out, outs)
			if !reflect.DeepEqual(outs, data.out) {
				t.Log("maps not equal")
				t.Fail()
			}
		})
	}
}
