// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"fmt"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"google.golang.org/protobuf/proto"
)

// Extension creates a GNMIOption that applies the supplied gnmi_ext.Extension to the provided
// proto.Message.
func Extension(ext *gnmi_ext.Extension) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.CapabilityRequest:
			if len(msg.Extension) == 0 {
				msg.Extension = make([]*gnmi_ext.Extension, 0)
			}
			msg.Extension = append(msg.Extension, ext)
		case *gnmi.GetRequest:
			if len(msg.Extension) == 0 {
				msg.Extension = make([]*gnmi_ext.Extension, 0)
			}
			msg.Extension = append(msg.Extension, ext)
		case *gnmi.GetResponse:
			if len(msg.Extension) == 0 {
				msg.Extension = make([]*gnmi_ext.Extension, 0)
			}
			msg.Extension = append(msg.Extension, ext)
		case *gnmi.SetRequest:
			if len(msg.Extension) == 0 {
				msg.Extension = make([]*gnmi_ext.Extension, 0)
			}
			msg.Extension = append(msg.Extension, ext)
		case *gnmi.SetResponse:
			if len(msg.Extension) == 0 {
				msg.Extension = make([]*gnmi_ext.Extension, 0)
			}
			msg.Extension = append(msg.Extension, ext)
		case *gnmi.SubscribeRequest:
			if len(msg.Extension) == 0 {
				msg.Extension = make([]*gnmi_ext.Extension, 0)
			}
			msg.Extension = append(msg.Extension, ext)
		case *gnmi.SubscribeResponse:
			if len(msg.Extension) == 0 {
				msg.Extension = make([]*gnmi_ext.Extension, 0)
			}
			msg.Extension = append(msg.Extension, ext)
		}
		return nil
	}
}

// Extension_HistorySnapshotTime creates a GNMIOption that adds a gNMI extension of
// type History Snapshot with the supplied snapshot time.
// the snapshot value can be nanoseconds since Unix epoch or a date in RFC3339 format
func Extension_HistorySnapshotTime(tm time.Time) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest:
			fn := Extension(
				&gnmi_ext.Extension{
					Ext: &gnmi_ext.Extension_History{
						History: &gnmi_ext.History{
							Request: &gnmi_ext.History_SnapshotTime{
								SnapshotTime: tm.UnixNano(),
							},
						},
					},
				},
			)
			return fn(msg)
		default:
			return fmt.Errorf("option Extension_HistorySnapshotTime: %w: %T", ErrInvalidMsgType, msg)
		}
	}
}

// Extension_HistoryRange creates a GNMIOption that adds a gNMI extension of
// type History TimeRange with the supplied start and end times.
// the start/end values can be nanoseconds since Unix epoch or a date in RFC3339 format
func Extension_HistoryRange(start, end time.Time) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest:
			fn := Extension(
				&gnmi_ext.Extension{
					Ext: &gnmi_ext.Extension_History{
						History: &gnmi_ext.History{
							Request: &gnmi_ext.History_Range{
								Range: &gnmi_ext.TimeRange{
									Start: start.UnixNano(),
									End:   end.UnixNano(),
								},
							},
						},
					},
				},
			)
			return fn(msg)
		default:
			return fmt.Errorf("option Extension_HistoryRange: %w: %T", ErrInvalidMsgType, msg)
		}
	}
}

// Extension_DepthLevel creates a GNMIOption that adds a gNMI extension of
// type Depth with the supplied depth level.
func Extension_DepthLevel(level uint32) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest, *gnmi.GetRequest:
			fn := Extension(
				&gnmi_ext.Extension{
					Ext: &gnmi_ext.Extension_Depth{
						Depth: &gnmi_ext.Depth{
							Level: level,
						},
					},
				},
			)
			return fn(msg)
		default:
			return fmt.Errorf("option Extension_HistorySnapshotTime: %w: %T", ErrInvalidMsgType, msg)
		}
	}
}
