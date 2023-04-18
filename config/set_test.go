// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"
	"testing"
	"text/template"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/testutils"
)

var createSetRequestFromFileTestSet = map[string]struct {
	in         *Config
	targetName string
	out        *gnmi.SetRequest
	err        error
}{

	"set_update_request_from_file": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]*template.Template{
				template.Must(template.New("set-request").Parse(`{
				"updates": [
					{
						"path": "valid/path",
						"value": "value"
					}
				]
			}`))},
			nil,
		},
		out: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value\""),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_replace_request_from_file": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]*template.Template{
				template.Must(template.New("set-request").Parse(`{
				"replaces": [
					{
						"path": "valid/path",
						"value": "value"
					}
				]
			}`))},
			nil,
		},
		out: &gnmi.SetRequest{
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value\""),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_delete_request_from_file": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]*template.Template{
				template.Must(template.New("set-request").Parse(`{
				"deletes": [
					"valid/path"
				]
			}`))},
			nil,
		},
		out: &gnmi.SetRequest{
			Delete: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
		},
		err: nil,
	},
	"set_multiple_update_request": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]*template.Template{
				template.Must(template.New("set-request").Parse(`{
				"updates": [
					{
						"path": "valid/path1",
						"value": "value1"
					},
					{
						"path": "valid/path2",
						"value": "value2",
						"encoding": "json_ietf"
					}
				]
			}`))},
			nil,
		},
		out: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path1"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value1\""),
						},
					},
				},
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path2"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{
							JsonIetfVal: []byte("\"value2\""),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_multiple_replace_request_from_file": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]*template.Template{
				template.Must(template.New("set-request").Parse(`{
				"replaces": [
					{
						"path": "valid/path1",
						"value": "value1"
					},
					{
						"path": "valid/path2",
						"value": "value2",
						"encoding": "json_ietf"
					}
				]
			}`))},
			nil,
		},
		out: &gnmi.SetRequest{
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path1"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value1\""),
						},
					},
				},
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path2"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{
							JsonIetfVal: []byte("\"value2\""),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_multiple_delete_request_from_file": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]*template.Template{
				template.Must(template.New("set-request").Parse(`{
				"deletes": [
					"valid/path1",
					"valid/path2"
				]
			}`))},
			nil,
		},
		out: &gnmi.SetRequest{
			Delete: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path1"},
					},
				},
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path2"},
					},
				},
			},
		},
		err: nil,
	},
	"set_combined_request": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]*template.Template{template.Must(template.New("set-request").Parse(`{
				"updates": [
					{
						"path": "/valid/path1",
						"value": "value1"
					}
				],
				"replaces": [
					{
						"path": "/valid/path2",
						"value": "value2"
					}
				],
				"deletes": [
					"valid/path"
				]
			}`))},
			nil,
		},
		out: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path1"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value1\""),
						},
					},
				},
			},
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path2"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value2\""),
						},
					},
				},
			},
			Delete: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
		},
		err: nil,
	},
	"template_based_set_request": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]*template.Template{
				template.Must(template.New("set-request").Parse(`replaces:
{{- range $interface := index .Vars .TargetName "interfaces" }}
  - path: "/interface[name={{ index $interface "name" }}]"
    encoding: "json_ietf"
    value: 
      admin-state: {{ index $interface "admin-state" }}
{{- range $index, $subinterface := index $interface "subinterfaces" }}
      subinterface:
        - index: {{ $index }}
          admin-state: {{ index $subinterface "admin-state"}}
          ipv4:
            address:
              - ip-prefix: {{ index $subinterface "ipv4-address"}}
{{- end }}
{{- end }}`))},
			map[string]interface{}{
				"target1": map[string]interface{}{
					"interfaces": []interface{}{
						map[string]interface{}{
							"name":        "ethernet-1/1",
							"admin-state": "enable",
							"subinterfaces": []interface{}{
								map[string]interface{}{
									"admin-state":  "enable",
									"ipv4-address": "192.168.88.1/30",
								},
							},
						},
					},
				},
			},
		},
		targetName: "target1",
		out: &gnmi.SetRequest{
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{
								Name: "interface",
								Key: map[string]string{
									"name": "ethernet-1/1",
								},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{
							JsonIetfVal: []byte(`{"admin-state":"enable","subinterface":[{"admin-state":"enable","index":0,"ipv4":{"address":[{"ip-prefix":"192.168.88.1/30"}]}}]}`),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_replace_origin_cli": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]*template.Template{
				template.Must(template.New("set-request").Parse(`{
				"replaces": [
					{
						"path": "cli:/",
						"value": "set interface ethernet-1/1 admin-state enable\nset interface ethernet-1/2 admin-state enable",
						"encoding": "ascii",
					}
				]
			}`))},
			nil,
		},
		out: &gnmi.SetRequest{
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "cli",
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_AsciiVal{
							AsciiVal: "set interface ethernet-1/1 admin-state enable\nset interface ethernet-1/2 admin-state enable",
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_update_origin_cli": {
		in: &Config{
			GlobalFlags{
				Encoding: "ascii",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]*template.Template{
				template.Must(template.New("set-request").Parse(`{
				"updates": [
					{
						"path": "cli:/",
						"value": "set interface ethernet-1/1 admin-state enable"
					}
				]
			}`))},
			nil,
		},
		out: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "cli",
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_AsciiVal{
							AsciiVal: "set interface ethernet-1/1 admin-state enable",
						},
					},
				},
			},
		},
		err: nil,
	},
}

func TestCreateSetRequestFromFile(t *testing.T) {
	for name, data := range createSetRequestFromFileTestSet {
		t.Run(name, func(t *testing.T) {
			setReq, err := data.in.CreateSetRequestFromFile(data.targetName)
			t.Logf("exp value: %+v", data.out)
			t.Logf("exp error: %+v", data.err)
			t.Logf("got value: %+v", setReq)
			t.Logf("got error: %+v", err)
			if err != nil {
				if !strings.HasPrefix(err.Error(), data.err.Error()) {
					t.Fail()
				}
			}
			if !testutils.SetRequestsEqual(setReq[0], data.out) {
				t.Fail()
			}
		})
	}
}
