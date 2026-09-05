// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package outputs

import (
	"context"

	"github.com/openconfig/gnmi/proto/gnmi"
)

// SubscriptionInfo identifies one managed subscription attempt and its
// initial synchronization state.
type SubscriptionInfo struct {
	Source              string
	Name                string
	Instance            string
	Mode                gnmi.SubscriptionList_Mode
	InitialSyncComplete bool
}

type subscriptionInfoContextKey struct{}

// WithSubscriptionInfo attaches subscription delivery state to an output call.
func WithSubscriptionInfo(ctx context.Context, info SubscriptionInfo) context.Context {
	if info.Instance == "" {
		return ctx
	}
	return context.WithValue(ctx, subscriptionInfoContextKey{}, info)
}

// SubscriptionInfoFromContext returns subscription delivery state attached by
// a managed target subscription.
func SubscriptionInfoFromContext(ctx context.Context) (SubscriptionInfo, bool) {
	if ctx == nil {
		return SubscriptionInfo{}, false
	}
	info, ok := ctx.Value(subscriptionInfoContextKey{}).(SubscriptionInfo)
	return info, ok && info.Instance != ""
}
