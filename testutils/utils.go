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
	if !CompareGnmiPaths(req1.Prefix, req2.Prefix) {
		return false
	}
	if len(req1.Path) != len(req2.Path) {
		return false
	}
	for i := range req1.Path {
		if !CompareGnmiPaths(req1.Path[i], req2.Path[i]) {
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
	if len(req1.GetDelete()) != len(req2.GetDelete()) ||
		len(req1.GetReplace()) != len(req2.GetReplace()) ||
		len(req1.GetUpdate()) != len(req2.GetUpdate()) {
		return false
	}
	if !CompareGnmiPaths(req1.GetPrefix(), req2.GetPrefix()) {
		return false
	}
	for i := range req1.GetDelete() {
		if !CompareGnmiPaths(req1.GetDelete()[i], req2.GetDelete()[i]) {
			return false
		}
	}
	for i := range req1.GetUpdate() {
		if !CompareGnmiPaths(req1.GetUpdate()[i].GetPath(), req2.GetUpdate()[i].GetPath()) {
			return false
		}
		if !cmp.Equal(req1.GetUpdate()[i].GetVal().GetValue(), req2.GetUpdate()[i].GetVal().GetValue()) {
			return false
		}
	}
	for i := range req1.GetReplace() {
		if !CompareGnmiPaths(req1.GetReplace()[i].GetPath(), req2.GetReplace()[i].GetPath()) {
			return false
		}
		if !cmp.Equal(req1.GetReplace()[i].GetVal().GetValue(), req2.GetReplace()[i].GetVal().GetValue()) {
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
			if !CompareGnmiPaths(req1.Subscribe.Prefix, req2.Subscribe.Prefix) {
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

func CompareGetResponses(rsp1, rsp2 *gnmi.GetResponse) bool {
	if rsp1 == nil && rsp2 == nil {
		return true
	}
	if rsp1 == nil || rsp2 == nil {
		return false
	}
	if len(rsp1.GetNotification()) != len(rsp2.GetNotification()) {
		return false
	}
	for i := range rsp1.GetNotification() {
		if !CompareNotifications(rsp1.GetNotification()[i], rsp2.GetNotification()[i]) {
			return false
		}
	}
	return true
}

func CompareSetResponses(rsp1, rsp2 *gnmi.SetResponse) bool {
	if rsp1 == nil && rsp2 == nil {
		return true
	}
	if rsp1 == nil || rsp2 == nil {
		return false
	}
	if len(rsp1.GetResponse()) != len(rsp2.GetResponse()) {
		return false
	}
	for i := range rsp1.GetResponse() {
		if !CompareUpdateResult(rsp1.GetResponse()[i], rsp2.GetResponse()[i]) {
			return false
		}
	}
	return true
}

func CompareSubscribeResponses(rsp1, rsp2 *gnmi.SubscribeResponse) bool {
	if rsp1 == nil && rsp2 == nil {
		return true
	}
	if rsp1 == nil || rsp2 == nil {
		return false
	}

	switch rsp1.GetResponse().(type) {
	case *gnmi.SubscribeResponse_Update:
		switch rsp2.GetResponse().(type) {
		case *gnmi.SubscribeResponse_Update:
		default:
			return false
		}
	case *gnmi.SubscribeResponse_SyncResponse:
		switch rsp2.GetResponse().(type) {
		case *gnmi.SubscribeResponse_SyncResponse:
		default:
			return false
		}
	}

	switch rsp1 := rsp1.GetResponse().(type) {
	case *gnmi.SubscribeResponse_Update:
		switch rsp2 := rsp2.GetResponse().(type) {
		case *gnmi.SubscribeResponse_Update:
			return CompareNotifications(rsp1.Update, rsp2.Update)
		}
	case *gnmi.SubscribeResponse_SyncResponse:
		switch rsp2 := rsp2.GetResponse().(type) {
		case *gnmi.SubscribeResponse_SyncResponse:
			if rsp1.SyncResponse != rsp2.SyncResponse {
				return false
			}
		}
	}

	return true
}

func CompareGnmiPaths(p1, p2 *gnmi.Path) bool {
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
	if !CompareGnmiPaths(s1.Path, s2.Path) {
		return false
	}
	return true
}

func CompareGnmiUpdates(u1, u2 *gnmi.Update) bool {
	if u1 == nil && u2 == nil {
		return true
	}
	if u1 == nil || u2 == nil {
		return false
	}
	if u1.GetDuplicates() != u2.GetDuplicates() {
		return false
	}
	if !CompareGnmiPaths(u1.GetPath(), u2.GetPath()) {
		return false
	}
	return cmp.Equal(u1.GetVal().GetValue(), u2.GetVal().GetValue())
}

func CompareNotifications(n1, n2 *gnmi.Notification) bool {
	if n1.GetAtomic() != n2.GetAtomic() {
		return false
	}
	if n1.GetAlias() != n2.GetAlias() {
		return false
	}
	// compare timestamps
	if n1.GetTimestamp() != n2.GetTimestamp() {
		return false
	}
	// compare prefixes
	if !CompareGnmiPaths(n1.GetPrefix(), n2.GetPrefix()) {
		return false
	}
	// compare updates
	for j := range n1.GetUpdate() {
		if !CompareGnmiUpdates(n1.GetUpdate()[j], n2.GetUpdate()[j]) {
			return false
		}
	}
	// compare deletes
	for j := range n1.GetDelete() {
		if !CompareGnmiPaths(n1.GetDelete()[j], n2.GetDelete()[j]) {
			return false
		}
	}
	return true
}

func CompareUpdateResult(u1, u2 *gnmi.UpdateResult) bool {
	if u1 == nil && u2 == nil {
		return true
	}
	if u1 == nil || u2 == nil {
		return false
	}
	if u1.GetOp() != u2.GetOp() {
		return false
	}
	if !CompareGnmiPaths(u1.GetPath(), u2.GetPath()) {
		return false
	}
	return true
}
