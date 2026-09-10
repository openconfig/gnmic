// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package outputs

import (
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
)

func updateRsp(target string) *gnmi.SubscribeResponse {
	return &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Timestamp: 42,
				Prefix:    &gnmi.Path{Target: target},
				Update: []*gnmi.Update{{
					Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}},
					Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}},
				}},
			},
		},
	}
}

func syncRsp() *gnmi.SubscribeResponse {
	return &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true},
	}
}

func TestAddSubscriptionTarget(t *testing.T) {
	meta := Meta{"source": "router1:57400", "subscription-name": "sub1"}
	type testCase struct {
		name       string
		msg        proto.Message
		addTarget  string
		wantTarget string // expected Prefix.Target when the result is an update
		wantSame   bool   // the exact input message must be returned
	}
	cases := []testCase{}
	for _, addTarget := range []string{"", AddTargetOverwrite, AddTargetIfNotPresent, "overwite"} {
		cases = append(cases,
			testCase{
				name:      addTarget + "/sync-response",
				msg:       syncRsp(),
				addTarget: addTarget,
				wantSame:  true,
			},
			testCase{
				name:      addTarget + "/non-subscribe-response",
				msg:       &gnmi.GetResponse{},
				addTarget: addTarget,
				wantSame:  true,
			},
		)
	}
	cases = append(cases,
		testCase{name: "empty/update-without-target", msg: updateRsp(""), addTarget: "", wantTarget: "", wantSame: true},
		testCase{name: "empty/update-with-target", msg: updateRsp("t1"), addTarget: "", wantTarget: "t1", wantSame: true},
		testCase{name: "overwrite/update-without-target", msg: updateRsp(""), addTarget: AddTargetOverwrite, wantTarget: "router1"},
		testCase{name: "overwrite/update-with-target", msg: updateRsp("t1"), addTarget: AddTargetOverwrite, wantTarget: "router1"},
		testCase{name: "if-not-present/update-without-target", msg: updateRsp(""), addTarget: AddTargetIfNotPresent, wantTarget: "router1"},
		testCase{name: "if-not-present/update-with-target", msg: updateRsp("t1"), addTarget: AddTargetIfNotPresent, wantTarget: "t1", wantSame: true},
		testCase{name: "garbage/update-without-target", msg: updateRsp(""), addTarget: "overwite", wantTarget: "", wantSame: true},
		testCase{name: "garbage/update-with-target", msg: updateRsp("t1"), addTarget: "overwite", wantTarget: "t1", wantSame: true},
	)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := proto.Clone(tc.msg)
			got, err := AddSubscriptionTarget(tc.msg, meta, tc.addTarget, DefaultTargetTemplate)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatalf("got a nil message")
			}
			// never a typed nil either
			if rsp, ok := got.(*gnmi.SubscribeResponse); ok && rsp == nil {
				t.Fatalf("got a typed nil *gnmi.SubscribeResponse")
			}
			if tc.wantSame && got != tc.msg {
				t.Fatalf("expected the input message to be returned unchanged, got %v", got)
			}
			if !tc.wantSame && got == tc.msg {
				t.Fatalf("expected a copy, got the input message")
			}
			// the input is never modified
			if !proto.Equal(in, tc.msg) {
				t.Fatalf("input message was modified: %v -> %v", in, tc.msg)
			}
			if rsp, ok := got.(*gnmi.SubscribeResponse); ok && rsp.GetUpdate() != nil {
				if gotTarget := rsp.GetUpdate().GetPrefix().GetTarget(); gotTarget != tc.wantTarget {
					t.Fatalf("target: got %q, want %q", gotTarget, tc.wantTarget)
				}
			}
		})
	}
}

func TestAddSubscriptionTargetNil(t *testing.T) {
	got, err := AddSubscriptionTarget(nil, nil, AddTargetOverwrite, DefaultTargetTemplate)
	if err != nil || got != nil {
		t.Fatalf("nil input: got (%v, %v), want (nil, nil)", got, err)
	}
	var typedNil *gnmi.SubscribeResponse
	got, err = AddSubscriptionTarget(typedNil, nil, AddTargetOverwrite, DefaultTargetTemplate)
	if err != nil {
		t.Fatalf("typed nil input: unexpected error %v", err)
	}
	if rsp, ok := got.(*gnmi.SubscribeResponse); !ok || rsp != nil {
		t.Fatalf("typed nil input: got %v, want the same typed nil back", got)
	}
}

func TestAddSubscribeResponseTargetErrors(t *testing.T) {
	in := updateRsp("")
	got, err := AddSubscribeResponseTarget(in, Meta{}, AddTargetOverwrite, nil)
	if err == nil {
		t.Fatalf("expected an error with a nil template")
	}
	if got != in {
		t.Fatalf("expected the input response back on error")
	}
}

func TestValidateAddTarget(t *testing.T) {
	for _, v := range []any{nil, "", AddTargetOverwrite, AddTargetIfNotPresent} {
		if err := ValidateAddTarget(v); err != nil {
			t.Errorf("%#v: unexpected error %v", v, err)
		}
	}
	for _, v := range []any{"overwite", "true", "yes", true, 1} {
		if err := ValidateAddTarget(v); err == nil {
			t.Errorf("%#v: expected an error", v)
		}
	}
}

func TestDecodeConfigValidatesAddTarget(t *testing.T) {
	type cfg struct {
		Name      string `mapstructure:"name,omitempty"`
		AddTarget string `mapstructure:"add-target,omitempty"`
	}
	c := new(cfg)
	err := DecodeConfig(map[string]any{"name": "o1", "add-target": "if-not-present"}, c)
	if err != nil {
		t.Fatalf("valid add-target rejected: %v", err)
	}
	if c.AddTarget != AddTargetIfNotPresent {
		t.Fatalf("add-target not decoded: %q", c.AddTarget)
	}
	err = DecodeConfig(map[string]any{"name": "o1"}, new(cfg))
	if err != nil {
		t.Fatalf("missing add-target rejected: %v", err)
	}
	err = DecodeConfig(map[string]any{"name": "o1", "add-target": "overwite"}, new(cfg))
	if err == nil {
		t.Fatalf("expected an error for a mistyped add-target")
	}
}
