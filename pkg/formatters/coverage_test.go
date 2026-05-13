// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package formatters

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
)

func TestEventMsgString(t *testing.T) {
	e := &EventMsg{
		Name:      "sub1",
		Timestamp: 42,
		Tags:      map[string]string{"a": "b"},
		Values:    map[string]interface{}{"v": int64(1)},
	}
	got := e.String()
	var rt EventMsg
	if err := json.Unmarshal([]byte(got), &rt); err != nil {
		t.Fatalf("invalid json: %v: %s", err, got)
	}
	if rt.Name != "sub1" || rt.Timestamp != 42 {
		t.Fatalf("round-trip mismatch: %+v", rt)
	}
}

func TestNum64(t *testing.T) {
	cases := []struct {
		in   interface{}
		want interface{}
	}{
		{int(1), int64(1)},
		{int8(1), int64(1)},
		{int16(1), int64(1)},
		{int32(1), int64(1)},
		{int64(1), int64(1)},
		{uint(1), uint64(1)},
		{uintptr(1), uint64(1)},
		{uint8(1), uint64(1)},
		{uint16(1), uint64(1)},
		{uint32(1), uint64(1)},
		{uint64(1), uint64(1)},
		{float64(1.5), uint64(1)},
		{"x", nil},
	}
	for _, tc := range cases {
		got := num64(tc.in)
		if got != tc.want {
			t.Errorf("num64(%T=%v)=%v want %v", tc.in, tc.in, got, tc.want)
		}
	}
}

func TestAddMetaTags(t *testing.T) {
	e := &EventMsg{Tags: map[string]string{"existing": "v1"}}
	addMetaTags(e, map[string]string{
		"format":   "should-be-skipped",
		"existing": "v2",
		"new":      "n",
	})
	if _, ok := e.Tags["format"]; ok {
		t.Errorf("format tag should be skipped")
	}
	if e.Tags["existing"] != "v1" {
		t.Errorf("existing must remain: %v", e.Tags)
	}
	if e.Tags["meta_existing"] != "v2" {
		t.Errorf("meta_existing not set: %v", e.Tags)
	}
	if e.Tags["new"] != "n" {
		t.Errorf("new tag missing: %v", e.Tags)
	}
}

func TestEventFromMapErrors(t *testing.T) {
	cases := []map[string]interface{}{
		{"name": 1},
		{"timestamp": "abc"},
		{"tags": 1},
		{"values": 1},
		{"deletes": 1},
	}
	for i, c := range cases {
		if _, err := EventFromMap(c); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
	// nil
	if got, err := EventFromMap(nil); err != nil || got != nil {
		t.Errorf("nil input: %v %v", got, err)
	}
	// values as map[string]string and deletes as []interface{}
	got, err := EventFromMap(map[string]interface{}{
		"values":  map[string]string{"k": "v"},
		"deletes": []interface{}{"a", "b"},
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Values["k"] != "v" || len(got.Deletes) != 2 {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestGetResponseToEventMsgs(t *testing.T) {
	rsp := &gnmi.GetResponse{
		Notification: []*gnmi.Notification{
			{
				Timestamp: 100,
				Prefix: &gnmi.Path{
					Target: "tgt",
					Elem:   []*gnmi.PathElem{{Name: "a"}},
				},
				Update: []*gnmi.Update{
					{
						Path: &gnmi.Path{
							Elem: []*gnmi.PathElem{{Name: "b", Key: map[string]string{"name": "x"}}, {Name: "c"}},
						},
						Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}},
					},
				},
			},
		},
	}
	evs, err := GetResponseToEventMsgs(rsp, map[string]string{"source": "h"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events", len(evs))
	}
	if evs[0].Name != "get-request" || evs[0].Timestamp != 100 {
		t.Errorf("unexpected event: %+v", evs[0])
	}
	if evs[0].Tags["target"] != "tgt" {
		t.Errorf("missing target tag: %+v", evs[0].Tags)
	}
	if evs[0].Tags["source"] != "h" {
		t.Errorf("missing source meta: %+v", evs[0].Tags)
	}
	// nil
	if got, err := GetResponseToEventMsgs(nil, nil); err != nil || got != nil {
		t.Errorf("nil: %v %v", got, err)
	}
}

func TestGetValue(t *testing.T) {
	cases := []struct {
		name string
		in   *gnmi.TypedValue
		want interface{}
	}{
		{"nil", nil, nil},
		{"ascii", &gnmi.TypedValue{Value: &gnmi.TypedValue_AsciiVal{AsciiVal: "a"}}, "a"},
		{"bool", &gnmi.TypedValue{Value: &gnmi.TypedValue_BoolVal{BoolVal: true}}, true},
		{"int", &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 7}}, int64(7)},
		{"uint", &gnmi.TypedValue{Value: &gnmi.TypedValue_UintVal{UintVal: 7}}, uint64(7)},
		{"string", &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "s"}}, "s"},
		{"double", &gnmi.TypedValue{Value: &gnmi.TypedValue_DoubleVal{DoubleVal: 1.5}}, 1.5},
		{"json_str", &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{JsonVal: []byte(`"hello"`)}}, "hello"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getValue(tc.in)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMarshalFormats(t *testing.T) {
	rsp := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Timestamp: 1,
				Update: []*gnmi.Update{
					{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}},
						Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}},
					},
				},
			},
		},
	}
	for _, format := range []string{"", "json", "proto", "protojson", "prototext", "event", "flat"} {
		t.Run("format="+format, func(t *testing.T) {
			o := &MarshalOptions{Format: format}
			b, err := o.Marshal(rsp, map[string]string{"subscription-name": "sub1"})
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if format != "proto" && len(b) == 0 {
				t.Errorf("empty output for format=%q", format)
			}
		})
	}

	// SyncResponse
	sync := &gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true}}
	o := &MarshalOptions{Format: "json"}
	if _, err := o.Marshal(sync, nil); err != nil {
		t.Errorf("sync json: %v", err)
	}
	o = &MarshalOptions{Format: "event"}
	if got, err := o.Marshal(sync, nil); err != nil || len(got) != 0 {
		t.Errorf("sync event: %v %s", err, got)
	}

	// GetResponse / event
	gr := &gnmi.GetResponse{
		Notification: []*gnmi.Notification{{
			Timestamp: 1,
			Update: []*gnmi.Update{{
				Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}},
				Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}},
			}},
		}},
	}
	o = &MarshalOptions{Format: "event", Multiline: true, Indent: "  "}
	if _, err := o.Marshal(gr, nil); err != nil {
		t.Errorf("get event: %v", err)
	}

	// Unsupported event for SetResponse
	sr := &gnmi.SetResponse{}
	o = &MarshalOptions{Format: "event"}
	if _, err := o.Marshal(sr, nil); err == nil {
		t.Errorf("expected error on unsupported event")
	}
}

func TestOverrideTimestamp(t *testing.T) {
	rsp := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{Timestamp: 0}},
	}
	o := &MarshalOptions{OverrideTS: true}
	out := o.OverrideTimestamp(rsp)
	if out.(*gnmi.SubscribeResponse).GetUpdate().GetTimestamp() == 0 {
		t.Errorf("timestamp not overridden")
	}
	// no override
	o2 := &MarshalOptions{OverrideTS: false}
	out2 := o2.OverrideTimestamp(rsp)
	if out2 == nil {
		t.Errorf("nil output")
	}
}

func TestFormatJSON_AllRequestResponseTypes(t *testing.T) {
	o := &MarshalOptions{Format: "json", Multiline: true, Indent: "  "}
	cases := []struct {
		name string
		msg  interface{ proto() }
	}{}
	_ = cases
	// CapabilityRequest
	if _, err := o.FormatJSON(&gnmi.CapabilityRequest{}, nil); err != nil {
		t.Errorf("CapabilityRequest: %v", err)
	}
	// CapabilityResponse
	if _, err := o.FormatJSON(&gnmi.CapabilityResponse{
		GNMIVersion: "0.7.0",
		SupportedModels: []*gnmi.ModelData{
			{Name: "m", Organization: "o", Version: "1"},
		},
		SupportedEncodings: []gnmi.Encoding{gnmi.Encoding_JSON},
	}, nil); err != nil {
		t.Errorf("CapabilityResponse: %v", err)
	}
	// GetRequest
	if _, err := o.FormatJSON(&gnmi.GetRequest{
		Prefix: &gnmi.Path{Target: "t"},
		Path:   []*gnmi.Path{{Elem: []*gnmi.PathElem{{Name: "a"}}}},
		UseModels: []*gnmi.ModelData{
			{Name: "m", Organization: "o", Version: "1"},
		},
	}, nil); err != nil {
		t.Errorf("GetRequest: %v", err)
	}
	// GetResponse
	gr := &gnmi.GetResponse{
		Notification: []*gnmi.Notification{{
			Timestamp: 1,
			Prefix:    &gnmi.Path{Target: "t"},
			Update: []*gnmi.Update{{
				Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}},
				Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}},
			}},
			Delete: []*gnmi.Path{{Elem: []*gnmi.PathElem{{Name: "d"}}}},
		}},
	}
	if _, err := o.FormatJSON(gr, map[string]string{"source": "h"}); err != nil {
		t.Errorf("GetResponse: %v", err)
	}
	// values-only
	o2 := &MarshalOptions{Format: "json", ValuesOnly: true, CalculateLatency: true}
	if _, err := o2.FormatJSON(gr, nil); err != nil {
		t.Errorf("GetResponse values-only: %v", err)
	}
	// SetRequest
	sreq := &gnmi.SetRequest{
		Prefix:  &gnmi.Path{Target: "t"},
		Delete:  []*gnmi.Path{{Elem: []*gnmi.PathElem{{Name: "d"}}}},
		Update:  []*gnmi.Update{{Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "u"}}}, Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}}}},
		Replace: []*gnmi.Update{{Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "r"}}}, Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}}}},
	}
	if _, err := o.FormatJSON(sreq, nil); err != nil {
		t.Errorf("SetRequest: %v", err)
	}
	// SetResponse
	srs := &gnmi.SetResponse{
		Prefix:    &gnmi.Path{Target: "t"},
		Timestamp: 1,
		Response: []*gnmi.UpdateResult{
			{Op: gnmi.UpdateResult_UPDATE, Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}}},
		},
	}
	if _, err := o.FormatJSON(srs, map[string]string{"source": "h"}); err != nil {
		t.Errorf("SetResponse: %v", err)
	}
	// SubscribeRequest variants
	sub := &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: &gnmi.SubscriptionList{
				Prefix:       &gnmi.Path{Target: "t"},
				Subscription: []*gnmi.Subscription{{Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}}}},
				Qos:          &gnmi.QOSMarking{Marking: 1},
				Encoding:     gnmi.Encoding_JSON,
				Mode:         gnmi.SubscriptionList_STREAM,
			},
		},
	}
	if _, err := o.FormatJSON(sub, nil); err != nil {
		t.Errorf("SubscribeRequest: %v", err)
	}
	poll := &gnmi.SubscribeRequest{Request: &gnmi.SubscribeRequest_Poll{}}
	if _, err := o.FormatJSON(poll, nil); err != nil {
		t.Errorf("SubscribeRequest poll: %v", err)
	}
	// SubscribeResponse Update with latency
	subResp := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Timestamp: 1,
				Prefix:    &gnmi.Path{Target: "t"},
				Update: []*gnmi.Update{{
					Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}},
					Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}},
				}},
				Delete: []*gnmi.Path{{Elem: []*gnmi.PathElem{{Name: "d"}}}},
			},
		},
	}
	o3 := &MarshalOptions{Format: "json", CalculateLatency: true, Multiline: true, Indent: "  "}
	if _, err := o3.FormatJSON(subResp, map[string]string{
		"source":            "h",
		"system-name":       "sn",
		"subscription-name": "sub",
	}); err != nil {
		t.Errorf("SubscribeResponse: %v", err)
	}
	// Sync
	if _, err := o.FormatJSON(&gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true}}, nil); err != nil {
		t.Errorf("SyncResponse: %v", err)
	}
	// nil
	if b, err := o.FormatJSON(nil, nil); err != nil || b != nil {
		t.Errorf("nil msg: %v %v", b, err)
	}
}

func TestResponsesFlat(t *testing.T) {
	gr := &gnmi.GetResponse{
		Notification: []*gnmi.Notification{{
			Prefix: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "p"}}},
			Update: []*gnmi.Update{{
				Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}},
				Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}},
			}},
		}},
	}
	got, err := ResponsesFlat(gr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) == 0 {
		t.Errorf("expected entries: %v", got)
	}

	sr := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Prefix: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "p"}}},
				Update: []*gnmi.Update{{
					Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}},
					Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}},
				}},
			},
		},
	}
	got2, err := ResponsesFlat(sr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got2) == 0 {
		t.Errorf("expected entries: %v", got2)
	}

	// unsupported
	if _, err := ResponsesFlat(&gnmi.SetResponse{}); err == nil {
		t.Errorf("expected error")
	}
}

// fakeProcessor satisfies EventProcessor for MakeProcessor tests
type fakeProcessor struct {
	BaseProcessor
}

func (f *fakeProcessor) Init(_ interface{}, opts ...Option) error {
	for _, o := range opts {
		o(f)
	}
	return nil
}

func (f *fakeProcessor) Apply(evs ...*EventMsg) []*EventMsg {
	return evs
}

func TestMakeProcessorAndEventProcessors(t *testing.T) {
	// Register two: one ok, one failing
	prev := EventProcessors
	EventProcessors = map[string]Initializer{}
	defer func() { EventProcessors = prev }()

	Register("fake-ok", func() EventProcessor { return &fakeProcessor{} })

	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))

	procs := map[string]map[string]any{
		"p1": {"fake-ok": map[string]any{"x": 1}},
	}
	tcs := map[string]*types.TargetConfig{}
	acts := map[string]map[string]any{}

	// MakeProcessor success
	ep, err := MakeProcessor(logger, "p1", procs["p1"], procs, tcs, acts)
	if err != nil || ep == nil {
		t.Fatalf("MakeProcessor: %v %v", ep, err)
	}
	// nil logger path
	if _, err := MakeProcessor(nil, "p1", procs["p1"], procs, tcs, acts); err != nil {
		t.Fatalf("nil logger: %v", err)
	}

	// MakeEventProcessors success
	got, err := MakeEventProcessors(logger, []string{"p1"}, procs, tcs, acts)
	if err != nil || len(got) != 1 {
		t.Fatalf("MakeEventProcessors: %v %v", got, err)
	}
	// missing processor in map
	if _, err := MakeEventProcessors(logger, []string{"missing"}, procs, tcs, acts); err == nil {
		t.Errorf("expected not found error")
	}
	// nil logger path
	if _, err := MakeEventProcessors(nil, []string{"p1"}, procs, tcs, acts); err != nil {
		t.Errorf("nil logger: %v", err)
	}
	// unknown type
	bad := map[string]map[string]any{"bad": {"unknown-type": map[string]any{}}}
	if _, err := MakeEventProcessors(logger, []string{"bad"}, bad, tcs, acts); err == nil {
		t.Errorf("expected unknown type error")
	}
}

func TestDecodeConfig(t *testing.T) {
	type cfg struct {
		Name string `mapstructure:"name"`
	}
	dst := &cfg{}
	if err := DecodeConfig(map[string]any{"name": "x"}, dst); err != nil {
		t.Fatalf("err: %v", err)
	}
	if dst.Name != "x" {
		t.Errorf("got %v", dst)
	}
	// nil result fails
	if err := DecodeConfig(map[string]any{"name": "x"}, nil); err == nil {
		t.Errorf("expected error for nil dst")
	}
}

func TestProcessorOptionsAndBaseProcessor(t *testing.T) {
	bp := &BaseProcessor{}
	WithLogger(nil)(bp)
	if bp.Logger == nil {
		t.Errorf("logger should fall back to discard")
	}
	WithLogger(slog.Default())(bp)
	if bp.Logger == nil {
		t.Errorf("logger should be set")
	}
	WithTargets(nil)(bp)
	WithActions(nil)(bp)
	WithProcessors(nil)(bp)
	if err := bp.Init(nil); err != nil {
		t.Errorf("Init: %v", err)
	}
	if got := bp.Apply(); got != nil {
		t.Errorf("Apply: %v", got)
	}
}

func TestRegister(t *testing.T) {
	prev := EventProcessors
	EventProcessors = map[string]Initializer{}
	defer func() { EventProcessors = prev }()

	Register("x", func() EventProcessor { return &BaseProcessor{} })
	if _, ok := EventProcessors["x"]; !ok {
		t.Errorf("not registered")
	}
}
