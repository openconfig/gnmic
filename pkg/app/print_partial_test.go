// © 2026 Supranett
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/formatters"
)

func TestPrintMsgPreservesPartialEvents(t *testing.T) {
	for _, atomic := range []bool{false, true} {
		a := New()
		t.Cleanup(a.Cfn)
		a.Config.Format = "event"
		a.Config.Log = true
		a.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
		var output bytes.Buffer
		a.out = &output
		err := a.PrintMsg("device-a", "", &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{
				Atomic: atomic,
				Update: []*gnmi.Update{
					{Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "valid"}}}, Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 42}}},
					{Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "invalid"}}}, Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("{")}}},
				},
				Delete: []*gnmi.Path{{Elem: []*gnmi.PathElem{{Name: "removed"}}}},
			}},
		})
		if err == nil {
			t.Fatal("PrintMsg lost the decode diagnostic")
		}
		if atomic {
			if output.Len() != 0 {
				t.Fatalf("rejected atomic notification printed %q", output.String())
			}
			continue
		}
		var events []*formatters.EventMsg
		if err := json.Unmarshal(output.Bytes(), &events); err != nil {
			t.Fatalf("printed invalid JSON %q: %v", output.String(), err)
		}
		if len(events) != 2 || events[0].Values["/valid"] != float64(42) || len(events[1].Deletes) != 1 {
			t.Fatalf("printed events = %+v, want the healthy value and delete", events)
		}
	}
}
