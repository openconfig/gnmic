// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/openconfig/gnmic/pkg/api/types"
)

// TargetConfigToNotification converts a target configuration to a gNMI
// notification under the `gnmic` origin, in the requested encoding.
// It is used by the gNMI server(s) to reply to Get requests with
// origin=gnmic and path elem `targets`.
func TargetConfigToNotification(tc *types.TargetConfig, e gnmi.Encoding) *gnmi.Notification {
	switch e {
	case gnmi.Encoding_JSON, gnmi.Encoding_JSON_IETF:
		b, _ := json.Marshal(tc)
		n := &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "gnmic",
						Elem: []*gnmi.PathElem{
							{
								Name: "target",
								Key:  map[string]string{"name": tc.Name},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{JsonVal: b},
					},
				},
			},
		}
		return n
	case gnmi.Encoding_BYTES:
		n := &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Prefix: &gnmi.Path{
				Origin: "gnmic",
				Elem: []*gnmi.PathElem{
					{
						Name: "target",
						Key:  map[string]string{"name": tc.Name},
					},
				},
			},
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "address"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(tc.Address)},
					},
				},
			},
		}
		if tc.Username != nil {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "username"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(*tc.Username)},
				},
			})
		}
		if tc.Insecure != nil {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "insecure"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(fmt.Sprint(*tc.Insecure))},
				},
			})
		}
		if tc.SkipVerify != nil {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "skip-verify"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(fmt.Sprint(*tc.SkipVerify))},
				},
			})
		}
		n.Update = append(n.Update, &gnmi.Update{
			Path: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "timeout"},
				},
			},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(tc.Timeout.String())},
			},
		})
		if tc.TLSCA != nil && *tc.TLSCA != "" {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "tls-ca"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte((tc.TLSCAString()))},
				},
			})
		}
		if tc.TLSCert != nil && *tc.TLSCert != "" {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "tls-cert"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(tc.TLSCertString())},
				},
			})
		}
		if tc.TLSKey != nil && *tc.TLSKey != "" {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "tls-key"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(tc.TLSKeyString())},
				},
			})
		}
		if len(tc.Outputs) > 0 {
			typedVals := make([]*gnmi.TypedValue, 0, len(tc.Outputs))
			for _, out := range tc.Outputs {
				typedVals = append(typedVals, &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(out)},
				})
			}
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "outputs"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_LeaflistVal{
						LeaflistVal: &gnmi.ScalarArray{
							Element: typedVals,
						},
					},
				},
			})
		}
		if len(tc.Subscriptions) > 0 {
			typedVals := make([]*gnmi.TypedValue, 0, len(tc.Subscriptions))
			for _, sub := range tc.Subscriptions {
				typedVals = append(typedVals, &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(sub)},
				})
			}
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "subscriptions"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_LeaflistVal{
						LeaflistVal: &gnmi.ScalarArray{
							Element: typedVals,
						},
					},
				},
			})
		}
		return n
	case gnmi.Encoding_ASCII:
		n := &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Prefix: &gnmi.Path{
				Origin: "gnmic",
				Elem: []*gnmi.PathElem{
					{
						Name: "target",
						Key:  map[string]string{"name": tc.Name},
					},
				},
			},
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "address"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_AsciiVal{AsciiVal: tc.Address},
					},
				},
			},
		}
		if tc.Username != nil {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "username"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_AsciiVal{AsciiVal: *tc.Username},
				},
			})
		}
		if tc.Insecure != nil {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "insecure"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_AsciiVal{AsciiVal: fmt.Sprint(*tc.Insecure)},
				},
			})
		}
		if tc.SkipVerify != nil {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "skip-verify"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_AsciiVal{AsciiVal: fmt.Sprint(*tc.SkipVerify)},
				},
			})
		}
		n.Update = append(n.Update, &gnmi.Update{
			Path: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "timeout"},
				},
			},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{AsciiVal: tc.Timeout.String()},
			},
		})
		if tc.TLSCA != nil && *tc.TLSCA != "" {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "tls-ca"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_AsciiVal{AsciiVal: tc.TLSCAString()},
				},
			})
		}
		if tc.TLSCert != nil && *tc.TLSCert != "" {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "tls-cert"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_AsciiVal{AsciiVal: tc.TLSCertString()},
				},
			})
		}
		if tc.TLSKey != nil && *tc.TLSKey != "" {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "tls-key"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_AsciiVal{AsciiVal: tc.TLSKeyString()},
				},
			})
		}
		if len(tc.Outputs) > 0 {
			typedVals := make([]*gnmi.TypedValue, 0, len(tc.Outputs))
			for _, out := range tc.Outputs {
				typedVals = append(typedVals, &gnmi.TypedValue{
					Value: &gnmi.TypedValue_AsciiVal{AsciiVal: out},
				})
			}
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "outputs"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_LeaflistVal{
						LeaflistVal: &gnmi.ScalarArray{
							Element: typedVals,
						},
					},
				},
			})
		}
		if len(tc.Subscriptions) > 0 {
			typedVals := make([]*gnmi.TypedValue, 0, len(tc.Subscriptions))
			for _, sub := range tc.Subscriptions {
				typedVals = append(typedVals, &gnmi.TypedValue{
					Value: &gnmi.TypedValue_AsciiVal{AsciiVal: sub},
				})
			}
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "subscriptions"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_LeaflistVal{
						LeaflistVal: &gnmi.ScalarArray{
							Element: typedVals,
						},
					},
				},
			})
		}
		return n
	}
	return nil
}

// SubscriptionConfigToNotification converts a subscription configuration to a
// gNMI notification under the `gnmic` origin, in the requested encoding.
// Only JSON and JSON_IETF encodings are supported.
func SubscriptionConfigToNotification(sub *types.SubscriptionConfig, e gnmi.Encoding) *gnmi.Notification {
	switch e {
	case gnmi.Encoding_JSON, gnmi.Encoding_JSON_IETF:
		b, _ := json.Marshal(sub)
		n := &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "gnmic",
						Elem: []*gnmi.PathElem{
							{
								Name: "subscriptions",
								Key:  map[string]string{"name": sub.Name},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{JsonVal: b},
					},
				},
			},
		}
		return n
	case gnmi.Encoding_BYTES:
	case gnmi.Encoding_ASCII:
	}
	return nil
}
