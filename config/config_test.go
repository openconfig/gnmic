package config

import (
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/karimra/gnmic/api"
	"github.com/karimra/gnmic/testutils"
	"github.com/openconfig/gnmi/proto/gnmi"
)

var createGetRequestTestSet = map[string]struct {
	in  *Config
	out *gnmi.GetRequest
	err error
}{
	"nil_input": {
		in:  nil,
		out: nil,
		err: ErrInvalidConfig,
	},
	"unknown_encoding_type": {
		in: &Config{
			GlobalFlags{
				Encoding: "dummy",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: nil,
		err: api.ErrInvalidValue,
	},
	"invalid_prefix": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPrefix: "/invalid/]prefix",
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: nil,
		err: api.ErrInvalidValue,
	},
	"invalid_path": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPrefix: "/invalid/]path",
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: nil,
		err: api.ErrInvalidValue,
	},
	"unknown_data_type": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPrefix: "/valid/path",
				GetType:   "dummy",
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: nil,
		err: api.ErrInvalidValue,
	},
	"basic_get_request": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPath: []string{"/valid/path"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.GetRequest{
			Path: []*gnmi.Path{
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
	"get_request_with_type": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPath: []string{"/valid/path"},
				GetType: "state",
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.GetRequest{
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
			Type: gnmi.GetRequest_STATE,
		},
		err: nil,
	},
	"get_request_with_encoding": {
		in: &Config{
			GlobalFlags{
				Encoding: "proto",
			},
			LocalFlags{
				GetPath: []string{"/valid/path"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.GetRequest{
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
			Encoding: gnmi.Encoding_PROTO,
		},
		err: nil,
	},
	"get_request_with_prefix": {
		in: &Config{
			GlobalFlags{
				Encoding: "proto",
			},
			LocalFlags{
				GetPrefix: "/valid/prefix",
				GetPath:   []string{"/valid/path"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.GetRequest{
			Prefix: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "valid"},
					{Name: "prefix"},
				},
			},
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
			Encoding: gnmi.Encoding_PROTO,
		},
		err: nil,
	},
	"get_request_with_2_paths": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPath: []string{
					"/valid/path1",
					"/valid/path2",
				},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.GetRequest{
			Path: []*gnmi.Path{
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
}

var createSetRequestTestSet = map[string]struct {
	in  *Config
	out *gnmi.SetRequest
	err error
}{

	"set_update_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelimiter: ":::",
				SetUpdate:    []string{"/valid/path:::json:::value"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
	"set_replace_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelimiter: ":::",
				SetReplace:   []string{"/valid/path:::json:::value"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
	"set_delete_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelete: []string{"/valid/path"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
			GlobalFlags{},
			LocalFlags{
				SetDelimiter: ":::",
				SetUpdate: []string{
					"/valid/path1:::json:::value1",
					"/valid/path2:::json_ietf:::value2",
				},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
	"set_multiple_replace_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelimiter: ":::",
				SetReplace: []string{
					"/valid/path1:::json:::value1",
					"/valid/path2:::json_ietf:::value2",
				},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
	"set_multiple_delete_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelete: []string{
					"/valid/path1",
					"/valid/path2",
				},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
			GlobalFlags{},
			LocalFlags{
				SetDelimiter: ":::",
				SetUpdate:    []string{"/valid/path1:::json:::value1"},
				SetReplace:   []string{"/valid/path2:::json:::value2"},
				SetDelete:    []string{"/valid/path"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
	"set_update_path_request": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				SetUpdatePath:  []string{"/valid/path"},
				SetUpdateValue: []string{"value"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
	"set_replace_path_request": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				SetReplacePath:  []string{"/valid/path"},
				SetReplaceValue: []string{"value"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
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
}

var execPathTemplateTestSet = map[string]struct {
	tpl   string
	input interface{}
	out   string
}{
	"nil": {
		tpl:   "",
		input: nil,
		out:   "",
	},
	"simple": {
		tpl:   `"/path/"`,
		input: nil,
		out:   "/path/",
	},
	"with_an_expression": {
		tpl: `"/interfaces/" + .name`,
		input: map[string]interface{}{
			"name": "interface",
		},
		out: "/interfaces/interface",
	},
}

func TestCreateGetRequest(t *testing.T) {
	for name, data := range createGetRequestTestSet {
		t.Run(name, func(t *testing.T) {
			getReq, err := data.in.CreateGetRequest()
			t.Logf("exp value: %+v", data.out)
			t.Logf("got value: %+v", getReq)
			t.Logf("exp error: %+v", data.err)
			t.Logf("got error: %+v", err)
			if err != nil {
				uerr := errors.Unwrap(err)
				if !errors.Is(uerr, data.err) {
					t.Fail()
				}
			}
			if !testutils.GetRequestsEqual(getReq, data.out) {
				t.Fail()
			}
		})
	}
}

func TestCreateSetRequest(t *testing.T) {
	for name, data := range createSetRequestTestSet {
		t.Run(name, func(t *testing.T) {
			setReq, err := data.in.CreateSetRequest("")
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

func TestExecPathTemplate(t *testing.T) {
	c := New()
	c.Debug = true
	c.logger = log.New(os.Stderr, "", log.LstdFlags)
	for name, data := range execPathTemplateTestSet {
		t.Run(name, func(t *testing.T) {
			o, err := c.execPathTemplate(data.tpl, data.input)
			if err != nil {
				t.Logf("failed: %v", err)
				t.Fail()
			}
			t.Logf("exp value: %+v", data.out)
			t.Logf("got value: %+v", o)
			if data.out != o {
				t.Fail()
			}
		})
	}
}
