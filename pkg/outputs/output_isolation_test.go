// © 2026 Supranett
//
// SPDX-License-Identifier: Apache-2.0

package outputs

import (
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/formatters"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestMarshalRetainsValidUpdatesAndDeletes(t *testing.T) {
	message := &gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
		Update: []*gnmi.Update{
			{Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "valid"}}}, Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 42}}},
			{Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "invalid"}}}, Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("{")}}},
		},
		Delete: []*gnmi.Path{{Elem: []*gnmi.PathElem{{Name: "removed"}}}},
	}}}
	for _, split := range []bool{false, true} {
		result, err := Marshal(message, nil, &formatters.MarshalOptions{Format: "event"}, split)
		want := 1
		if split {
			want = 2
		}
		if err == nil || len(result) != want {
			t.Fatalf("split=%t payloads=%q error=%v, want %d payloads and a diagnostic", split, result, err, want)
		}
	}
}

func TestMarshalRejectsIncompleteNonEventMessages(t *testing.T) {
	message := &descriptorpb.UninterpretedOption_NamePart{NamePart: proto.String("example")}
	for _, format := range []string{"proto", "protojson", "prototext"} {
		t.Run(format, func(t *testing.T) {
			result, err := Marshal(message, nil, &formatters.MarshalOptions{Format: format}, false)
			if err == nil || len(result) != 0 {
				t.Fatalf("payloads=%q error=%v, want an initialization error and no payload", result, err)
			}
		})
	}
}
