// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package outputs

import (
	"context"
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
)

func TestSubscriptionInfoContext(t *testing.T) {
	want := SubscriptionInfo{
		Source:   "leaf-1",
		Name:     "counters",
		Instance: "stream-1",
		Mode:     gnmi.SubscriptionList_STREAM,
	}
	ctx := WithSubscriptionInfo(context.Background(), want)
	got, ok := SubscriptionInfoFromContext(ctx)
	if !ok || got != want {
		t.Fatalf("SubscriptionInfoFromContext() = (%v, %v), want (%v, true)", got, ok, want)
	}

	if _, ok := SubscriptionInfoFromContext(WithSubscriptionInfo(ctx, SubscriptionInfo{})); !ok {
		t.Fatal("empty subscription info should preserve the existing context value")
	}
	if _, ok := SubscriptionInfoFromContext(nil); ok {
		t.Fatal("nil context returned subscription info")
	}
}
