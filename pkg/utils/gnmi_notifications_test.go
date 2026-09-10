// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"encoding/json"
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/openconfig/gnmic/pkg/api/types"
)

func TestTargetConfigToNotification_JSONRedactsPassword(t *testing.T) {
	pass := "s3cret"
	token := "tok"
	tc := &types.TargetConfig{
		Name:     "router1",
		Address:  "10.0.0.1:57400",
		Password: &pass,
		Token:    &token,
	}

	n := TargetConfigToNotification(tc, gnmi.Encoding_JSON)
	if n == nil {
		t.Fatal("expected a notification")
	}
	raw := n.GetUpdate()[0].GetVal().GetJsonVal()
	if len(raw) == 0 {
		t.Fatal("JSON value is empty; RedactedDeepCopy was likely not called")
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal notification JSON: %v", err)
	}
	if got["password"] != "****" {
		t.Fatalf("password: got %#v, want redacted", got["password"])
	}
	if got["token"] != "****" {
		t.Fatalf("token: got %#v, want redacted", got["token"])
	}
	if *tc.Password != "s3cret" || *tc.Token != "tok" {
		t.Fatal("original target config was mutated")
	}
}
