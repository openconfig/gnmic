package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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
		err: errors.New("invalid configuration"),
	},
	"unknown_encoding_type": {
		in: &Config{
			Globals: &GlobalFlags{
				Encoding: "dummy",
			},
			LocalFlags: &LocalFlags{},
		},
		out: nil,
		err: errors.New("invalid encoding type"),
	},
	"invalid_prefix": {
		in: &Config{
			Globals: &GlobalFlags{
				Encoding: "json",
			},
			LocalFlags: &LocalFlags{
				GetPrefix: "/invalid/]prefix",
			},
		},
		out: nil,
		err: errors.New("prefix parse error"),
	},
	"invalid_path": {
		in: &Config{
			Globals: &GlobalFlags{
				Encoding: "json",
			},
			LocalFlags: &LocalFlags{
				GetPrefix: "/invalid/]path",
			},
		},
		out: nil,
		err: errors.New("prefix parse error"),
	},
	"unknown_data_type": {
		in: &Config{
			Globals: &GlobalFlags{
				Encoding: "json",
			},
			LocalFlags: &LocalFlags{
				GetPrefix: "/valid/path",
				GetType:   "dummy",
			},
		},
		out: nil,
		err: errors.New("unknown data type"),
	},
	"basic_get_request": {
		in: &Config{
			Globals: &GlobalFlags{
				Encoding: "json",
			},
			LocalFlags: &LocalFlags{
				GetPath: []string{"/valid/path"},
			},
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
			Globals: &GlobalFlags{
				Encoding: "json",
			},
			LocalFlags: &LocalFlags{
				GetPath: []string{"/valid/path"},
				GetType: "state",
			},
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
			Globals: &GlobalFlags{
				Encoding: "proto",
			},
			LocalFlags: &LocalFlags{
				GetPath: []string{"/valid/path"},
			},
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
			Globals: &GlobalFlags{
				Encoding: "proto",
			},
			LocalFlags: &LocalFlags{
				GetPrefix: "/valid/prefix",
				GetPath:   []string{"/valid/path"},
			},
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
			Globals: &GlobalFlags{
				Encoding: "json",
			},
			LocalFlags: &LocalFlags{
				GetPath: []string{
					"/valid/path1",
					"/valid/path2",
				},
			},
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
			Globals: &GlobalFlags{},
			LocalFlags: &LocalFlags{
				SetDelimiter: ":::",
				SetUpdate:    []string{"/valid/path:::json:::value"},
			},
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
			Globals: &GlobalFlags{},
			LocalFlags: &LocalFlags{
				SetDelimiter: ":::",
				SetReplace:   []string{"/valid/path:::json:::value"},
			},
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
			Globals: &GlobalFlags{},
			LocalFlags: &LocalFlags{
				SetDelete: []string{"/valid/path"},
			},
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
			Globals: &GlobalFlags{},
			LocalFlags: &LocalFlags{
				SetDelimiter: ":::",
				SetUpdate: []string{
					"/valid/path1:::json:::value1",
					"/valid/path2:::json_ietf:::value2",
				},
			},
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
			Globals: &GlobalFlags{},
			LocalFlags: &LocalFlags{
				SetDelimiter: ":::",
				SetReplace: []string{
					"/valid/path1:::json:::value1",
					"/valid/path2:::json_ietf:::value2",
				},
			},
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
			Globals: &GlobalFlags{},
			LocalFlags: &LocalFlags{
				SetDelete: []string{
					"/valid/path1",
					"/valid/path2",
				},
			},
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
			Globals: &GlobalFlags{},
			LocalFlags: &LocalFlags{
				SetDelimiter: ":::",
				SetUpdate:    []string{"/valid/path1:::json:::value1"},
				SetReplace:   []string{"/valid/path2:::json:::value2"},
				SetDelete:    []string{"/valid/path"},
			},
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
			Globals: &GlobalFlags{
				Encoding: "json",
			},
			LocalFlags: &LocalFlags{
				SetUpdatePath:  []string{"/valid/path"},
				SetUpdateValue: []string{"value"},
			},
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
			Globals: &GlobalFlags{
				Encoding: "json",
			},
			LocalFlags: &LocalFlags{
				SetReplacePath:  []string{"/valid/path"},
				SetReplaceValue: []string{"value"},
			},
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

var adminStr = "admin"
var emptyStr = ""
var falseBool = false
var trueBool = true

func TestCreateGetRequest(t *testing.T) {
	for name, data := range createGetRequestTestSet {
		t.Run(name, func(t *testing.T) {
			getReq, err := data.in.CreateGetRequest()
			t.Logf("exp value: %+v", data.out)
			t.Logf("got value: %+v", getReq)
			t.Logf("exp error: %+v", data.err)
			t.Logf("got error: %+v", err)
			if err != nil {
				if !strings.HasPrefix(err.Error(), data.err.Error()) {
					t.Fail()
				}
			}
			if !compareGetRequests(getReq, data.out) {
				t.Fail()
			}
		})
	}
}

func compareGetRequests(req1, req2 *gnmi.GetRequest) bool {
	if req1 == nil && req2 == nil {
		return true
	}
	if req1 == nil || req2 == nil {
		return false
	}
	if req1.Encoding != req2.Encoding ||
		req1.Type != req2.Type {
		return false
	}
	if !gnmiPathsEqual(req1.Prefix, req2.Prefix) {
		return false
	}
	if len(req1.Path) != len(req2.Path) {
		return false
	}
	for i := range req1.Path {
		if !gnmiPathsEqual(req1.Path[i], req2.Path[i]) {
			return false
		}
	}
	if len(req1.Extension) != len(req2.Extension) {
		return false
	}
	if len(req1.UseModels) != len(req2.UseModels) {
		return false
	}
	for i := range req1.UseModels {
		if req1.UseModels[i].Name != req2.UseModels[i].Name {
			return false
		}
	}
	return true
}

func gnmiPathsEqual(p1, p2 *gnmi.Path) bool {
	if p1 == nil && p2 == nil {
		return true
	}
	if p1 == nil || p2 == nil {
		return false
	}
	if p1.Origin != p2.Origin {
		return false
	}
	if p1.Target != p2.Target {
		return false
	}
	if len(p1.Elem) != len(p2.Elem) {
		return false
	}
	for i, e := range p1.Elem {
		if e.Name != p2.Elem[i].Name {
			return false
		}
		if !cmp.Equal(e.Key, p2.Elem[i].Key) {
			return false
		}
	}
	return true
}

func TestCreateSetRequest(t *testing.T) {
	for name, data := range createSetRequestTestSet {
		t.Run(name, func(t *testing.T) {
			setReq, err := data.in.CreateSetRequest()
			t.Logf("exp value: %+v", data.out)
			t.Logf("exp error: %+v", data.err)
			t.Logf("got value: %+v", setReq)
			t.Logf("got error: %+v", err)
			if err != nil {
				if !strings.HasPrefix(err.Error(), data.err.Error()) {
					t.Fail()
				}
			}
			if !compareSetRequests(setReq, data.out) {
				t.Fail()
			}
		})
	}
}

func compareSetRequests(req1, req2 *gnmi.SetRequest) bool {
	if req1 == nil && req2 == nil {
		return true
	}
	if req1 == nil || req2 == nil {
		return false
	}
	if len(req1.Delete) != len(req2.Delete) ||
		len(req1.Replace) != len(req2.Replace) ||
		len(req1.Update) != len(req2.Update) {
		return false
	}
	if !gnmiPathsEqual(req1.Prefix, req2.Prefix) {
		return false
	}
	for i := range req1.Delete {
		if !gnmiPathsEqual(req1.Delete[i], req2.Delete[i]) {
			return false
		}
	}
	for i := range req1.Update {
		if !gnmiPathsEqual(req1.Update[i].Path, req2.Update[i].Path) {
			return false
		}
	}
	for i := range req1.Replace {
		if !gnmiPathsEqual(req1.Replace[i].Path, req2.Replace[i].Path) {
			return false
		}
	}
	for i := range req1.Update {
		if !cmp.Equal(req1.Update[i].Val.Value, req2.Update[i].Val.Value) {
			return false
		}
	}
	for i := range req1.Replace {
		if !cmp.Equal(req1.Replace[i].Val.Value, req2.Replace[i].Val.Value) {
			return false
		}
	}
	return true
}
