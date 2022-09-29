// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package formatters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type MarshalOptions struct {
	Multiline  bool // could get rid of this and deduct it from len(Indent)
	Indent     string
	Format     string
	OverrideTS bool
	ValuesOnly bool
}

// Marshal //
func (o *MarshalOptions) Marshal(msg proto.Message, meta map[string]string, eps ...EventProcessor) ([]byte, error) {
	msg = o.OverrideTimestamp(msg)
	switch o.Format {
	default: // json
		return o.FormatJSON(msg, meta)
	case "proto":
		return proto.Marshal(msg)
	case "protojson":
		return protojson.MarshalOptions{Multiline: o.Multiline, Indent: o.Indent}.Marshal(msg)
	case "prototext":
		return prototext.MarshalOptions{Multiline: o.Multiline, Indent: o.Indent}.Marshal(msg)
	case "event":
		b := make([]byte, 0)
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeResponse:
			var subscriptionName string
			var ok bool
			if subscriptionName, ok = meta["subscription-name"]; !ok {
				subscriptionName = "default"
			}
			switch msg.GetResponse().(type) {
			case *gnmi.SubscribeResponse_Update:
				events, err := ResponseToEventMsgs(subscriptionName, msg, meta, eps...)
				if err != nil {
					return nil, fmt.Errorf("failed converting response to events: %v", err)
				}
				if o.Multiline {
					b, err = json.MarshalIndent(events, "", o.Indent)
				} else {
					b, err = json.Marshal(events)
				}
				if err != nil {
					return nil, fmt.Errorf("failed marshaling format 'event': %v", err)
				}
			}
			return b, nil
		case *gnmi.GetResponse:
			events, err := GetResponseToEventMsgs(msg, meta, eps...)
			if err != nil {
				return nil, fmt.Errorf("failed converting response to events: %v", err)
			}

			if o.Multiline {
				b, err = json.MarshalIndent(events, "", o.Indent)
			} else {
				b, err = json.Marshal(events)
			}
			if err != nil {
				return nil, fmt.Errorf("failed marshaling format 'event': %v", err)
			}
			return b, nil
		default:
			return nil, fmt.Errorf("format 'event' not supported for msg type %T", msg.ProtoReflect().Interface())
		}
	case "flat":
		flatMsg, err := responseFlat(msg)
		if err != nil {
			return nil, err
		}
		msgLen := len(flatMsg)
		if msgLen == 0 {
			return nil, nil
		}

		sortedPaths := make([]string, 0, msgLen)
		for k := range flatMsg {
			sortedPaths = append(sortedPaths, k)
		}
		sort.Strings(sortedPaths)

		buf := new(bytes.Buffer)
		for _, p := range sortedPaths {
			buf.WriteString(fmt.Sprintf("%s: %v\n", p, flatMsg[p]))
		}
		return buf.Bytes(), nil
	}
}

func (o *MarshalOptions) OverrideTimestamp(msg proto.Message) proto.Message {
	if o.OverrideTS {
		ts := time.Now().UnixNano()
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeResponse:
			switch msg.GetResponse().(type) {
			case *gnmi.SubscribeResponse_Update:
				upd := msg.GetUpdate()
				if upd != nil {
					upd.Timestamp = ts
				}
				return msg
			}
		}
	}
	return msg
}
