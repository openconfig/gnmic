package collector

import (
	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/proto/gnmi"
)

const (
	subscriptionDefaultMode       = "STREAM"
	subscriptionDefaultStreamMode = "TARGET_DEFINED"
	subscriptionDefaultEncoding   = "JSON"
)

type subscriptionRequest struct {
	name string
	req  *gnmi.SubscribeRequest
}

// SubscribeResponse //
type SubscribeResponse struct {
	SubscriptionName   string
	SubscriptionConfig *types.SubscriptionConfig
	Response           *gnmi.SubscribeResponse
}
