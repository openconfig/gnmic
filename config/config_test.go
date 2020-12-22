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

func TestCreateGetRequest(t *testing.T) {
	for name, data := range createGetRequestTestSet {
		t.Run(name, func(t *testing.T) {
			getReq, err := data.in.CreateGetRequest()
			if err != nil {
				if !strings.HasPrefix(err.Error(), data.err.Error()) {
					t.Logf("expected: %+v", data.err)
					t.Logf("got     : %+v", err)
					t.Fail()
				}
			}
			if !compareGetRequests(getReq, data.out) {
				t.Logf("expected: %+v", data.out)
				t.Logf("got     : %+v", getReq)
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
