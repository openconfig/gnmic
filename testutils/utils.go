package testutils

import (
	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmi/proto/gnmi"
)

func CompareGetRequests(req1, req2 *gnmi.GetRequest) bool {
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
	if !GnmiPathsEqual(req1.Prefix, req2.Prefix) {
		return false
	}
	if len(req1.Path) != len(req2.Path) {
		return false
	}
	for i := range req1.Path {
		if !GnmiPathsEqual(req1.Path[i], req2.Path[i]) {
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

func CompareSetRequests(req1, req2 *gnmi.SetRequest) bool {
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
	if !GnmiPathsEqual(req1.Prefix, req2.Prefix) {
		return false
	}
	for i := range req1.Delete {
		if !GnmiPathsEqual(req1.Delete[i], req2.Delete[i]) {
			return false
		}
	}
	for i := range req1.Update {
		if !GnmiPathsEqual(req1.Update[i].Path, req2.Update[i].Path) {
			return false
		}
	}
	for i := range req1.Replace {
		if !GnmiPathsEqual(req1.Replace[i].Path, req2.Replace[i].Path) {
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

func GnmiPathsEqual(p1, p2 *gnmi.Path) bool {
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
