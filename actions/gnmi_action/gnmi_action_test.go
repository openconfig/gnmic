package gnmi_action

import (
	"testing"

	"github.com/karimra/gnmic/actions"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/testutils"
	"github.com/openconfig/gnmi/proto/gnmi"
)

type getRequestTestItem struct {
	input  *formatters.EventMsg
	output *gnmi.GetRequest
}

type setRequestTestItem struct {
	input  *formatters.EventMsg
	output *gnmi.SetRequest
}

var getRequestTestSet = map[string]struct {
	actionType string
	action     map[string]interface{}
	tests      []getRequestTestItem
}{
	"get_no_templates": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "gnmi",
			"name":  "act1",
			"paths": []string{"/path"},
			"debug": true,
			"vars":  nil,
		},
		tests: []getRequestTestItem{
			{
				input: nil,
				output: &gnmi.GetRequest{
					Path: []*gnmi.Path{
						{
							Elem: []*gnmi.PathElem{
								{
									Name: "path",
								},
							},
						},
					},
					Encoding: gnmi.Encoding_JSON,
				},
			},
		},
	},
	"get_with_templates_in_path": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "gnmi",
			"name":  "act1",
			"paths": []string{`/{{.Input.Name}}`},
			"debug": true,
		},
		tests: []getRequestTestItem{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
				},
				output: &gnmi.GetRequest{
					Path: []*gnmi.Path{
						{
							Elem: []*gnmi.PathElem{
								{
									Name: "sub1",
								},
							},
						},
					},
					Encoding: gnmi.Encoding_JSON,
				},
			},
		},
	},
	"get_with_templates_in_prefix": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":   "gnmi",
			"name":   "act1",
			"prefix": `/{{.Input.Name}}`,
			"paths":  []string{`/{{.Input.Name}}`},
			"debug":  true,
		},
		tests: []getRequestTestItem{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
				},
				output: &gnmi.GetRequest{
					Prefix: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{
								Name: "sub1",
							},
						},
					},
					Path: []*gnmi.Path{
						{
							Elem: []*gnmi.PathElem{
								{
									Name: "sub1",
								},
							},
						},
					},
					Encoding: gnmi.Encoding_JSON,
				},
			},
		},
	},
}

var setRequestTestSet = map[string]struct {
	actionType string
	action     map[string]interface{}
	tests      []setRequestTestItem
}{
	"set_no_templates": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":   "gnmi",
			"name":   "act1",
			"rpc":    "set",
			"paths":  []string{"/path"},
			"values": []string{"value1"},
			"debug":  true,
		},
		tests: []setRequestTestItem{
			{
				input: nil,
				output: &gnmi.SetRequest{
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{
										Name: "path",
									},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonVal{
									JsonVal: []byte("\"value1\""),
								},
							},
						},
					},
				},
			},
		},
	},
	"set_with_templates_in_path": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":   "gnmi",
			"name":   "act1",
			"rpc":    "set",
			"paths":  []string{"/{{.Input.Name}}"},
			"values": []string{"value1"},
			"debug":  true,
		},
		tests: []setRequestTestItem{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
				},
				output: &gnmi.SetRequest{
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{
										Name: "sub1",
									},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonVal{
									JsonVal: []byte("\"value1\""),
								},
							},
						},
					},
				},
			},
		},
	},
	// changing a value via set update
	"set_with_template_in_values": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":   "gnmi",
			"name":   "act1",
			"rpc":    "set",
			"paths":  []string{`{{ range $k, $v := .Input.Values }}{{if eq $k "path1" }}{{$k}}{{end}}{{end}}`},
			"values": []string{`{{ range $k, $v := .Input.Values }}{{if eq $k "path1" }}value2{{end}}{{end}}`},
			"debug":  true,
		},
		tests: []setRequestTestItem{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Values: map[string]interface{}{
						"path1": "value1",
					},
				},
				output: &gnmi.SetRequest{
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{
										Name: "path1",
									},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonVal{
									JsonVal: []byte("\"value2\""),
								},
							},
						},
					},
				},
			},
		},
	},
	// changing multiple values via set update
	"set_with_multiple_templates_in_values": {
		actionType: actionType,
		action: map[string]interface{}{
			"type": "gnmi",
			"name": "act1",
			"rpc":  "set",
			"paths": []string{
				`{{ range $k, $v := .Input.Values }}{{if eq $k "path1" }}{{$k}}{{end}}{{end}}`,
				`{{ range $k, $v := .Input.Values }}{{if eq $k "path2" }}{{$k}}{{end}}{{end}}`,
			},
			"values": []string{
				`{{ range $k, $v := .Input.Values }}{{if eq $k "path1" }}value11{{end}}{{end}}`,
				`{{ range $k, $v := .Input.Values }}{{if eq $k "path2" }}value22{{end}}{{end}}`,
			},
			"debug": true,
		},
		tests: []setRequestTestItem{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Values: map[string]interface{}{
						"path1": "value1",
						"path2": "value2",
					},
				},
				output: &gnmi.SetRequest{
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{
										Name: "path1",
									},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonVal{
									JsonVal: []byte("\"value11\""),
								},
							},
						},
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{
										Name: "path2",
									},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonVal{
									JsonVal: []byte("\"value22\""),
								},
							},
						},
					},
				},
			},
		},
	},
	// changing a value via set replace
	"set_replace_with_template_in_values": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":   "gnmi",
			"name":   "act1",
			"rpc":    "set-replace",
			"paths":  []string{`{{ range $k, $v := .Input.Values }}{{if and (eq $k "path1") (eq $v "value1")}}{{$k}}{{end}}{{end}}`},
			"values": []string{`{{ range $k, $v := .Input.Values }}{{if and (eq $k "path1") (eq $v "value1")}}value2{{end}}{{end}}`},
			"debug":  true,
		},
		tests: []setRequestTestItem{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Values: map[string]interface{}{
						"path1": "value1",
					},
				},
				output: &gnmi.SetRequest{
					Replace: []*gnmi.Update{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{
										Name: "path1",
									},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonVal{
									JsonVal: []byte("\"value2\""),
								},
							},
						},
					},
				},
			},
		},
	},
	// changing multiple values via set update replace
	"set_replace_with_multiple_templates_in_values": {
		actionType: actionType,
		action: map[string]interface{}{
			"type": "gnmi",
			"name": "act1",
			"rpc":  "set-replace",
			"paths": []string{
				`{{ range $k, $v := .Input.Values }}{{if and (eq $k "path1") (eq $v "value1")}}{{$k}}{{end}}{{end}}`,
				`{{ range $k, $v := .Input.Values }}{{if and (eq $k "path2") (eq $v "value2")}}{{$k}}{{end}}{{end}}`,
			},
			"values": []string{
				`{{ range $k, $v := .Input.Values }}{{if and (eq $k "path1") (eq $v "value1")}}value11{{end}}{{end}}`,
				`{{ range $k, $v := .Input.Values }}{{if and (eq $k "path2") (eq $v "value2")}}value22{{end}}{{end}}`,
			},
			"debug": true,
		},
		tests: []setRequestTestItem{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Values: map[string]interface{}{
						"path1": "value1",
						"path2": "value2",
					},
				},
				output: &gnmi.SetRequest{
					Replace: []*gnmi.Update{
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{
										Name: "path1",
									},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonVal{
									JsonVal: []byte("\"value11\""),
								},
							},
						},
						{
							Path: &gnmi.Path{
								Elem: []*gnmi.PathElem{
									{
										Name: "path2",
									},
								},
							},
							Val: &gnmi.TypedValue{
								Value: &gnmi.TypedValue_JsonVal{
									JsonVal: []byte("\"value22\""),
								},
							},
						},
					},
				},
			},
		},
	},
}

func TestGnmiGetRequest(t *testing.T) {
	for name, ts := range getRequestTestSet {
		if ai, ok := actions.Actions[ts.actionType]; ok {
			t.Log("found action")
			a := ai()
			err := a.Init(ts.action)
			if err != nil {
				t.Errorf("failed to initialize action: %v", err)
				return
			}
			t.Logf("action: %+v", a)
			ga := a.(*gnmiAction)
			for i, item := range ts.tests {
				t.Run(name, func(t *testing.T) {
					t.Logf("running test item %d", i)
					gReq, err := ga.createGetRequest(&actions.Context{Input: item.input})
					if err != nil {
						t.Logf("failed: %v", err)
						t.Fail()
					}
					if !testutils.GetRequestsEqual(gReq, item.output) {
						t.Errorf("failed at %s item %d, expected %+v, got: %+v", name, i, item.output, gReq)
					}
				})
			}
		} else {
			t.Errorf("action %q not found", ts.actionType)
		}
	}
}

func TestGnmiSetRequest(t *testing.T) {
	for name, ts := range setRequestTestSet {
		if ai, ok := actions.Actions[ts.actionType]; ok {
			t.Log("found action")
			a := ai()
			err := a.Init(ts.action)
			if err != nil {
				t.Errorf("failed to initialize action: %v", err)
				return
			}
			t.Logf("action: %+v", a)
			ga := a.(*gnmiAction)
			for i, item := range ts.tests {
				t.Run(name, func(t *testing.T) {
					t.Logf("running test item %d", i)
					gReq, err := ga.createSetRequest(&actions.Context{Input: item.input})
					if err != nil {
						t.Logf("failed: %v", err)
						t.Fail()
					}
					if !testutils.SetRequestsEqual(gReq, item.output) {
						t.Errorf("failed at %s item %d, expected %+v, got: %+v", name, i, item.output, gReq)
					}
				})
			}
		} else {
			t.Errorf("action %q not found", ts.actionType)
		}
	}
}
