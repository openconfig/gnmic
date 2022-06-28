package testutils

import (
	"bytes"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
)

func CapabilitiesResponsesEqual(rsp1, rsp2 *gnmi.CapabilityResponse) bool {
	if rsp1 == nil && rsp2 == nil {
		return true
	}
	if rsp1 == nil || rsp2 == nil {
		return false
	}
	if rsp1.GNMIVersion != rsp2.GNMIVersion {
		return false
	}
	if len(rsp1.SupportedEncodings) != len(rsp2.SupportedEncodings) {
		return false
	}
	if len(rsp1.SupportedModels) != len(rsp2.SupportedModels) {
		return false
	}
	for i := range rsp1.SupportedEncodings {
		if rsp1.SupportedEncodings[i] != rsp2.SupportedEncodings[i] {
			return false
		}
	}
	for i := range rsp1.SupportedModels {
		if !cmp.Equal(rsp1.SupportedModels[i], rsp2.SupportedModels[i]) {
			return false
		}
	}
	return true
}

func GetRequestsEqual(req1, req2 *gnmi.GetRequest) bool {
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

func SetRequestsEqual(req1, req2 *gnmi.SetRequest) bool {
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
	if !GnmiPathsEqual(req1.GetPrefix(), req2.GetPrefix()) {
		return false
	}
	for i := range req1.GetDelete() {
		if !GnmiPathsEqual(req1.GetDelete()[i], req2.GetDelete()[i]) {
			return false
		}
	}
	for i := range req1.GetUpdate() {
		if !GnmiPathsEqual(req1.GetUpdate()[i].GetPath(), req2.GetUpdate()[i].GetPath()) {
			return false
		}
		if !cmp.Equal(req1.GetUpdate()[i].GetVal().GetValue(), req2.GetUpdate()[i].GetVal().GetValue()) {
			return false
		}
	}
	for i := range req1.GetReplace() {
		if !GnmiPathsEqual(req1.GetReplace()[i].GetPath(), req2.GetReplace()[i].GetPath()) {
			return false
		}
		if !cmp.Equal(req1.GetReplace()[i].GetVal().GetValue(), req2.GetReplace()[i].GetVal().GetValue()) {
			return false
		}
	}
	return true
}

func SubscribeRequestsEqual(req1, req2 *gnmi.SubscribeRequest) bool {
	if req1 == nil && req2 == nil {
		return true
	}
	if req1 == nil || req2 == nil {
		return false
	}
	if len(req1.GetExtension()) != len(req2.GetExtension()) {
		return false
	}
	// only checks if extensions are of the same type
	for i, ext := range req1.GetExtension() {
		switch ext.Ext.(type) {
		case *gnmi_ext.Extension_RegisteredExt:
			switch req2.GetExtension()[i].Ext.(type) {
			case *gnmi_ext.Extension_RegisteredExt:
			default:
				return false
			}
		case *gnmi_ext.Extension_History:
			switch req2.GetExtension()[i].Ext.(type) {
			case *gnmi_ext.Extension_History:
			default:
				return false
			}
		case *gnmi_ext.Extension_MasterArbitration:
			switch req2.GetExtension()[i].Ext.(type) {
			case *gnmi_ext.Extension_MasterArbitration:
			default:
				return false
			}
		}
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
				if !GnmiSubscriptionEqual(sub, req1.Subscribe.GetSubscription()[i]) {
					return false
				}
			}
		}
	}
	return true
}

func GetResponsesEqual(rsp1, rsp2 *gnmi.GetResponse) bool {
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
		if !GnmiNotificationsEqual(rsp1.GetNotification()[i], rsp2.GetNotification()[i]) {
			return false
		}
	}
	return true
}

func SetResponsesEqual(rsp1, rsp2 *gnmi.SetResponse) bool {
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
		if !GnmiUpdateResultEqual(rsp1.GetResponse()[i], rsp2.GetResponse()[i]) {
			return false
		}
	}
	return true
}

func SubscribeResponsesEqual(rsp1, rsp2 *gnmi.SubscribeResponse) bool {
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
			return GnmiNotificationsEqual(rsp1.Update, rsp2.Update)
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

func GnmiSubscriptionEqual(s1, s2 *gnmi.Subscription) bool {
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

func GnmiUpdatesEqual(u1, u2 *gnmi.Update) bool {
	if u1 == nil && u2 == nil {
		return true
	}
	if u1 == nil || u2 == nil {
		return false
	}
	if u1.GetDuplicates() != u2.GetDuplicates() {
		return false
	}
	if !GnmiPathsEqual(u1.GetPath(), u2.GetPath()) {
		return false
	}
	return cmp.Equal(u1.GetVal().GetValue(), u2.GetVal().GetValue())
}

func GnmiNotificationsEqual(n1, n2 *gnmi.Notification) bool {
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
	if !GnmiPathsEqual(n1.GetPrefix(), n2.GetPrefix()) {
		return false
	}
	// compare updates
	for j := range n1.GetUpdate() {
		if !GnmiUpdatesEqual(n1.GetUpdate()[j], n2.GetUpdate()[j]) {
			return false
		}
	}
	// compare deletes
	for j := range n1.GetDelete() {
		if !GnmiPathsEqual(n1.GetDelete()[j], n2.GetDelete()[j]) {
			return false
		}
	}
	return true
}

func GnmiUpdateResultEqual(u1, u2 *gnmi.UpdateResult) bool {
	if u1 == nil && u2 == nil {
		return true
	}
	if u1 == nil || u2 == nil {
		return false
	}
	if u1.GetOp() != u2.GetOp() {
		return false
	}
	if !GnmiPathsEqual(u1.GetPath(), u2.GetPath()) {
		return false
	}
	return true
}

func GnmiValuesEqual(v1, v2 *gnmi.TypedValue) bool {
	if v1 == nil && v2 == nil {
		return true
	}
	if v1 == nil || v2 == nil {
		return false
	}
	switch v1 := v1.GetValue().(type) {
	case *gnmi.TypedValue_AnyVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_AnyVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			if v1.AnyVal == nil && v2.AnyVal == nil {
				return true
			}
			if v1.AnyVal == nil || v2.AnyVal == nil {
				return false
			}
			if v1.AnyVal.GetTypeUrl() != v2.AnyVal.GetTypeUrl() {
				return false
			}
			return bytes.Equal(v1.AnyVal.GetValue(), v2.AnyVal.GetValue())
		default:
			return false
		}
	case *gnmi.TypedValue_AsciiVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_AsciiVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			return v1.AsciiVal == v2.AsciiVal
		default:
			return false
		}
	case *gnmi.TypedValue_BoolVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_BoolVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			return v1.BoolVal == v2.BoolVal
		default:
			return false
		}
	case *gnmi.TypedValue_BytesVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_BytesVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			return bytes.Equal(v1.BytesVal, v2.BytesVal)
		default:
			return false
		}
	case *gnmi.TypedValue_DecimalVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_DecimalVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			//lint:ignore SA1019 still need DecimalVal for backward compatibility
			if v1.DecimalVal.GetDigits() != v2.DecimalVal.GetDigits() {
				return false
			}
			//lint:ignore SA1019 still need DecimalVal for backward compatibility
			return v1.DecimalVal.GetPrecision() == v2.DecimalVal.GetPrecision()
		default:
			return false
		}
	case *gnmi.TypedValue_FloatVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_FloatVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			//lint:ignore SA1019 still need FloatVal for backward compatibility
			return v1.FloatVal == v2.FloatVal
		default:
			return false
		}
	case *gnmi.TypedValue_IntVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_IntVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			return v1.IntVal == v2.IntVal
		default:
			return false
		}
	case *gnmi.TypedValue_JsonIetfVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_JsonIetfVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			return bytes.Equal(v1.JsonIetfVal, v2.JsonIetfVal)
		default:
			return false
		}
	case *gnmi.TypedValue_JsonVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_JsonVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			return bytes.Equal(v1.JsonVal, v2.JsonVal)
		default:
			return false
		}
	case *gnmi.TypedValue_LeaflistVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_LeaflistVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			if len(v1.LeaflistVal.GetElement()) != len(v2.LeaflistVal.GetElement()) {
				return false
			}
			for i := range v1.LeaflistVal.GetElement() {
				if !GnmiValuesEqual(v1.LeaflistVal.Element[i], v2.LeaflistVal.Element[i]) {
					return false
				}
			}
		default:
			return false
		}
	case *gnmi.TypedValue_ProtoBytes:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_ProtoBytes:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			return bytes.Equal(v1.ProtoBytes, v2.ProtoBytes)
		default:
			return false
		}
	case *gnmi.TypedValue_StringVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_StringVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			return v1.StringVal == v2.StringVal
		default:
			return false
		}
	case *gnmi.TypedValue_UintVal:
		switch v2 := v2.GetValue().(type) {
		case *gnmi.TypedValue_UintVal:
			if v1 == nil && v2 == nil {
				return true
			}
			if v1 == nil || v2 == nil {
				return false
			}
			return v1.UintVal == v2.UintVal
		default:
			return false
		}
	}
	return true
}

func RegisterOpEqual(r1, r2 *tpb.RegisterOp) bool {
	if r1 == nil && r2 == nil {
		return true
	}
	if r1 == nil || r2 == nil {
		return false
	}
	switch r1 := r1.GetRegistration().(type) {
	case *tpb.RegisterOp_Target:
		switch r2 := r2.GetRegistration().(type) {
		case *tpb.RegisterOp_Target:
			if r1.Target.GetAccept() != r2.Target.GetAccept() {
				return false
			}
			if r1.Target.GetOp() != r2.Target.GetOp() {
				return false
			}
			if r1.Target.GetTarget() != r2.Target.GetTarget() {
				return false
			}
			if r1.Target.GetError() != r2.Target.GetError() {
				return false
			}
			if r1.Target.GetTargetType() != r2.Target.GetTargetType() {
				return false
			}
		default:
			return false
		}
	case *tpb.RegisterOp_Session:
		switch r2 := r2.GetRegistration().(type) {
		case *tpb.RegisterOp_Session:
			if r1.Session.GetAccept() != r2.Session.GetAccept() {
				return false
			}
			if r1.Session.GetTarget() != r2.Session.GetTarget() {
				return false
			}
			if r1.Session.GetError() != r2.Session.GetError() {
				return false
			}
			if r1.Session.GetTargetType() != r2.Session.GetTargetType() {
				return false
			}
			if r1.Session.GetTag() != r2.Session.GetTag() {
				return false
			}
		default:
			return false
		}
	case *tpb.RegisterOp_Subscription:
		switch r2 := r2.GetRegistration().(type) {
		case *tpb.RegisterOp_Subscription:
			if r1.Subscription.GetAccept() != r2.Subscription.GetAccept() {
				return false
			}
			if r1.Subscription.GetOp() != r2.Subscription.GetOp() {
				return false
			}
			if r1.Subscription.GetError() != r2.Subscription.GetError() {
				return false
			}
			if r1.Subscription.GetTargetType() != r2.Subscription.GetTargetType() {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func TunnelDataEqual(r1, r2 *tpb.Data) bool {
	if r1 == nil && r2 == nil {
		return true
	}
	if r1 == nil || r2 == nil {
		return false
	}
	if r1.GetClose() != r2.GetClose() {
		return false
	}
	if !bytes.Equal(r1.GetData(), r2.GetData()) {
		return false
	}
	if r1.GetTag() != r2.GetTag() {
		return false
	}
	return true
}
