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

func CompareSubscribeRequests(req1, req2 *gnmi.SubscribeRequest) bool {
	if req1 == nil && req2 == nil {
		return true
	}
	if req1 == nil || req2 == nil {
		return false
	}
	switch req1.Request.(type) {
	case *gnmi.SubscribeRequest_Subscribe:
		switch req2.Request.(type) {
		case *gnmi.SubscribeRequest_Subscribe:
		default:
			return false
		}
	case *gnmi.SubscribeRequest_Poll:
		switch req2.Request.(type) {
		case *gnmi.SubscribeRequest_Poll:
		default:
			return false
		}
	case *gnmi.SubscribeRequest_Aliases:
		switch req2.Request.(type) {
		case *gnmi.SubscribeRequest_Aliases:
		default:
			return false
		}
	}
	// compare subscribe request subscribe
	switch req1 := req1.Request.(type) {
	case *gnmi.SubscribeRequest_Subscribe:
		switch req2 := req2.Request.(type) {
		case *gnmi.SubscribeRequest_Subscribe:
			if req1.Subscribe.GetEncoding() != req2.Subscribe.GetEncoding() {
				return false
			}
			if req1.Subscribe.GetMode() != req2.Subscribe.GetMode() {
				return false
			}
			if req1.Subscribe.GetQos().GetMarking() != req2.Subscribe.GetQos().GetMarking() {
				return false
			}
			if len(req1.Subscribe.GetSubscription()) != len(req2.Subscribe.GetSubscription()) {
				return false
			}
			if req1.Subscribe.GetUpdatesOnly() != req2.Subscribe.GetUpdatesOnly() {
				return false
			}
			if req1.Subscribe.GetAllowAggregation() != req2.Subscribe.GetAllowAggregation() {
				return false
			}
			if req1.Subscribe.GetUseAliases() != req2.Subscribe.GetUseAliases() {
				return false
			}
			if !GnmiPathsEqual(req1.Subscribe.Prefix, req2.Subscribe.Prefix) {
				return false
			}
			if len(req1.Subscribe.GetUseModels()) != len(req2.Subscribe.GetUseModels()) {
				return false
			}
			for i := range req1.Subscribe.GetUseModels() {
				if req1.Subscribe.GetUseModels()[i].Name != req2.Subscribe.GetUseModels()[i].Name {
					return false
				}
			}
			for i, sub := range req1.Subscribe.GetSubscription() {
				if !CompareGnmiSubscription(sub, req1.Subscribe.GetSubscription()[i]) {
					return false
				}
			}
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

func CompareGnmiSubscription(s1, s2 *gnmi.Subscription) bool {
	if s1 == nil && s2 != nil {
		return false
	}
	if s1 != nil && s2 == nil {
		return false
	}
	if s1.Mode != s2.Mode {
		return false
	}
	if s1.SampleInterval != s2.SampleInterval {
		return false
	}
	if s1.SuppressRedundant != s2.SuppressRedundant {
		return false
	}
	if !GnmiPathsEqual(s1.Path, s2.Path) {
		return false
	}
	return true
}
