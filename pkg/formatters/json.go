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
	"strings"
	"sync"
	"time"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/path"
	"github.com/openconfig/gnmic/pkg/utils"
)

var bytesBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// jsonMarshal encodes v to JSON without HTML-escaping '<', '>', or '&'.
func jsonMarshal(v any) ([]byte, error) {
	buf := bytesBufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bytesBufferPool.Put(buf)
	}()
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	result := bytes.TrimRight(buf.Bytes(), "\n")
	out := make([]byte, len(result))
	copy(out, result)
	return out, nil
}

// jsonMarshalIndent is like jsonMarshal but applies indented formatting.
func jsonMarshalIndent(v any, prefix, indent string) ([]byte, error) {
	buf := bytesBufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bytesBufferPool.Put(buf)
	}()
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent(prefix, indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	result := bytes.TrimRight(buf.Bytes(), "\n")
	out := make([]byte, len(result))
	copy(out, result)
	return out, nil
}

func formatRegisteredExtensions(
	extensions []*gnmi_ext.Extension,
	protoDir,
	protoFiles []string,
	extensionDecodeMap utils.RegisteredExtensions,
) (map[int32]decodedExtension, error) {
	decodedExtensions := map[int32]decodedExtension{}

	if len(extensions) == 0 {
		return decodedExtensions, nil
	}

	if len(protoFiles) > 0 {
		descSource, err := grpcurl.DescriptorSourceFromProtoFiles(protoDir, protoFiles...)
		if err != nil {
			return nil, err
		}

		for _, ext := range extensions {
			rext := ext.GetRegisteredExt()

			if rext == nil {
				continue
			}

			id := int32(rext.Id)
			msg, exists := extensionDecodeMap[id]

			if !exists {
				continue
			}

			desc, err := descSource.FindSymbol(msg)

			if err != nil {
				return nil, err
			}

			pm := dynamic.NewMessage(desc.GetFile().FindMessage(msg))

			if err = pm.Unmarshal(rext.Msg); err != nil {
				return nil, err
			}

			jsondata, err := pm.MarshalJSON()

			if err != nil {
				return nil, err
			}

			msgJson := map[string]any{}

			if err = json.Unmarshal(jsondata, &msgJson); err != nil {
				return nil, err
			}

			decodedExtensions[id] = msgJson
		}
	}

	return decodedExtensions, nil
}

// FormatJSON formats a proto.Message and returns a []byte and an error
func (o *MarshalOptions) FormatJSON(m proto.Message, meta map[string]string) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	switch m := m.ProtoReflect().Interface().(type) {
	case *gnmi.CapabilityRequest:
		return o.formatCapabilitiesRequest(m)
	case *gnmi.CapabilityResponse:
		return o.formatCapabilitiesResponse(m)
	case *gnmi.GetRequest:
		return o.formatGetRequest(m)
	case *gnmi.GetResponse:
		return o.formatGetResponse(m, meta)
	case *gnmi.SetRequest:
		return o.formatSetRequest(m)
	case *gnmi.SetResponse:
		return o.formatSetResponse(m, meta)
	case *gnmi.SubscribeRequest:
		return o.formatSubscribeRequest(m)
	case *gnmi.SubscribeResponse:
		return o.formatSubscribeResponse(m, meta)
	}
	return nil, nil
}

func (o *MarshalOptions) formatSubscribeRequest(m *gnmi.SubscribeRequest) ([]byte, error) {
	msg := subscribeReq{}
	switch m := m.Request.(type) {
	case *gnmi.SubscribeRequest_Subscribe:
		msg.Subscribe.Prefix = path.GnmiPathToXPath(m.Subscribe.GetPrefix(), false)
		msg.Subscribe.Target = m.Subscribe.GetPrefix().GetTarget()
		msg.Subscribe.Subscriptions = make([]subscription, 0, len(m.Subscribe.GetSubscription()))
		if m.Subscribe != nil {
			msg.Subscribe.AllowAggregation = m.Subscribe.AllowAggregation
			msg.Subscribe.UpdatesOnly = m.Subscribe.UpdatesOnly
			msg.Subscribe.Encoding = m.Subscribe.Encoding.String()
			msg.Subscribe.Mode = m.Subscribe.Mode.String()
			if m.Subscribe.Qos != nil {
				msg.Subscribe.Qos = m.Subscribe.GetQos().GetMarking()
			}
			for _, sub := range m.Subscribe.Subscription {
				msg.Subscribe.Subscriptions = append(msg.Subscribe.Subscriptions,
					subscription{
						Path:              path.GnmiPathToXPath(sub.Path, false),
						Mode:              sub.GetMode().String(),
						SampleInterval:    sub.SampleInterval,
						HeartbeatInterval: sub.HeartbeatInterval,
						SuppressRedundant: sub.SuppressRedundant,
					})
			}
		}
	case *gnmi.SubscribeRequest_Poll:
		msg.Poll = new(poll)
	}
	if len(m.GetExtension()) > 0 {
		msg.Extensions = m.GetExtension()
	}
	if o.Multiline {
		return jsonMarshalIndent(msg, "", o.Indent)
	}
	return jsonMarshal(msg)
}

func (o *MarshalOptions) formatSubscribeResponse(m *gnmi.SubscribeResponse, meta map[string]string) ([]byte, error) {
	dext, err := formatRegisteredExtensions(m.GetExtension(), o.ProtoDir, o.ProtoFiles, o.RegisteredExtensions)

	if err != nil {
		return nil, err
	}

	switch mr := m.GetResponse().(type) {
	default:
		if len(m.GetExtension()) > 0 {

			msg := notificationRspMsg{
				Extensions:        m.GetExtension(),
				DecodedExtensions: dext,
			}
			if o.Multiline {
				return jsonMarshalIndent(msg, "", o.Indent)
			}
			return jsonMarshal(msg)
		}
	case *gnmi.SubscribeResponse_SyncResponse:
		msg := &syncResponseMsg{
			SyncResponse:      mr.SyncResponse,
			Extensions:        m.GetExtension(),
			DecodedExtensions: dext,
		}
		if o.Multiline {
			return jsonMarshalIndent(msg, "", o.Indent)
		}
		return jsonMarshal(msg)
	case *gnmi.SubscribeResponse_Update:
		msg := notificationRspMsg{
			Timestamp: mr.Update.Timestamp,
		}
		t := time.Unix(0, mr.Update.Timestamp)
		msg.Time = &t
		if o.CalculateLatency {
			msg.RecvTimestamp = time.Now().UnixNano()
			rt := time.Unix(0, msg.RecvTimestamp)
			msg.RecvTime = &rt
			msg.LatencyNano = msg.RecvTimestamp - msg.Timestamp
			msg.LatencyMilli = msg.LatencyNano / 1000 / 1000
		}
		if meta == nil {
			meta = make(map[string]string)
		}
		msg.Prefix = path.GnmiPathToXPath(mr.Update.GetPrefix(), false)
		msg.Target = mr.Update.Prefix.GetTarget()
		if s, ok := meta["source"]; ok {
			msg.Source = s
		}
		if s, ok := meta["system-name"]; ok {
			msg.SystemName = s
		}
		if s, ok := meta["subscription-name"]; ok {
			msg.SubscriptionName = s
		}
		for i, upd := range mr.Update.Update {
			if upd.Path == nil {
				upd.Path = new(gnmi.Path)
			}
			pathElems := make([]string, 0, len(upd.Path.Elem))
			for _, pElem := range upd.Path.Elem {
				pathElems = append(pathElems, pElem.GetName())
			}
			value, err := getValue(upd.Val)
			if err != nil {
				return nil, err
			}
			msg.Updates = append(msg.Updates,
				update{
					Path:   path.GnmiPathToXPath(upd.Path, false),
					Values: make(map[string]interface{}),
				})
			msg.Updates[i].Values[strings.Join(pathElems, "/")] = value
		}
		for _, del := range mr.Update.Delete {
			msg.Deletes = append(msg.Deletes, path.GnmiPathToXPath(del, false))
		}
		if len(m.GetExtension()) > 0 {
			msg.Extensions = m.GetExtension()
			msg.DecodedExtensions = dext
		}
		if o.Multiline {
			return jsonMarshalIndent(msg, "", o.Indent)
		}
		return jsonMarshal(msg)
	}
	return nil, nil
}

func (o *MarshalOptions) formatCapabilitiesRequest(m *gnmi.CapabilityRequest) ([]byte, error) {
	capReq := capRequest{
		Extensions: m.Extension,
	}
	if o.Multiline {
		return jsonMarshalIndent(capReq, "", o.Indent)
	}
	return jsonMarshal(capReq)
}

func (o *MarshalOptions) formatCapabilitiesResponse(m *gnmi.CapabilityResponse) ([]byte, error) {
	capRspMsg := capResponse{
		Extensions: m.Extension,
	}
	capRspMsg.Version = m.GetGNMIVersion()
	for _, sm := range m.SupportedModels {
		capRspMsg.SupportedModels = append(capRspMsg.SupportedModels,
			model{
				Name:         sm.GetName(),
				Organization: sm.GetOrganization(),
				Version:      sm.GetVersion(),
			})
	}
	for _, se := range m.SupportedEncodings {
		capRspMsg.Encodings = append(capRspMsg.Encodings, se.String())
	}
	if o.Multiline {
		return jsonMarshalIndent(capRspMsg, "", o.Indent)
	}
	return jsonMarshal(capRspMsg)
}

func (o *MarshalOptions) formatGetRequest(m *gnmi.GetRequest) ([]byte, error) {
	msg := getRqMsg{
		Prefix:     path.GnmiPathToXPath(m.GetPrefix(), false),
		Target:     m.GetPrefix().GetTarget(),
		Paths:      make([]string, 0, len(m.Path)),
		Encoding:   m.GetEncoding().String(),
		DataType:   m.GetType().String(),
		Extensions: m.Extension,
	}
	for _, p := range m.Path {
		msg.Paths = append(msg.Paths, path.GnmiPathToXPath(p, false))
	}
	for _, um := range m.UseModels {
		msg.Models = append(msg.Models,
			model{
				Name:         um.GetName(),
				Organization: um.GetOrganization(),
				Version:      um.GetVersion(),
			})
	}
	if o.Multiline {
		return jsonMarshalIndent(msg, "", o.Indent)
	}
	return jsonMarshal(msg)
}

func (o *MarshalOptions) formatGetResponse(m *gnmi.GetResponse, meta map[string]string) ([]byte, error) {
	dext, err := formatRegisteredExtensions(m.GetExtension(), o.ProtoDir, o.ProtoFiles, o.RegisteredExtensions)

	if err != nil {
		return nil, err
	}

	getRsp := getRspMsg{
		Notifications:     make([]notificationRspMsg, 0, len(m.GetNotification())),
		Extensions:        m.GetExtension(),
		DecodedExtensions: dext,
	}

	for _, notif := range m.GetNotification() {
		msg := notificationRspMsg{
			Prefix:  path.GnmiPathToXPath(notif.GetPrefix(), false),
			Updates: make([]update, 0, len(notif.GetUpdate())),
			Deletes: make([]string, 0, len(notif.GetDelete())),
		}
		msg.Timestamp = notif.Timestamp
		t := time.Unix(0, notif.Timestamp)
		msg.Time = &t
		if o.CalculateLatency && !o.ValuesOnly {
			msg.RecvTimestamp = time.Now().UnixNano()
			rt := time.Unix(0, msg.RecvTimestamp)
			msg.RecvTime = &rt
			msg.LatencyNano = msg.RecvTimestamp - msg.Timestamp
			msg.LatencyMilli = msg.LatencyNano / 1000 / 1000
		}
		if meta == nil {
			meta = make(map[string]string)
		}
		msg.Prefix = path.GnmiPathToXPath(notif.GetPrefix(), false)
		msg.Target = notif.GetPrefix().GetTarget()
		if s, ok := meta["source"]; ok {
			msg.Source = s
		}
		for i, upd := range notif.GetUpdate() {
			pathElems := make([]string, 0, len(upd.GetPath().GetElem()))
			for _, pElem := range upd.GetPath().GetElem() {
				pathElems = append(pathElems, pElem.GetName())
			}
			value, err := getValue(upd.GetVal())
			if err != nil {
				return nil, err
			}
			msg.Updates = append(msg.Updates,
				update{
					Path:   path.GnmiPathToXPath(upd.GetPath(), false),
					Values: make(map[string]interface{}),
				})
			msg.Updates[i].Values[strings.Join(pathElems, "/")] = value
		}
		for _, del := range notif.GetDelete() {
			msg.Deletes = append(msg.Deletes, path.GnmiPathToXPath(del, false))
		}
		getRsp.Notifications = append(getRsp.Notifications, msg)
	}

	if o.ValuesOnly {
		result := make([]interface{}, 0, len(getRsp.Notifications))
		for _, n := range getRsp.Notifications {
			for _, u := range n.Updates {
				for _, v := range u.Values {
					result = append(result, v)
				}
			}
		}
		return jsonMarshalIndent(result, "", "  ")
	}
	var data any
	if len(getRsp.Extensions) > 0 {
		data = getRsp
	} else {
		data = getRsp.Notifications
	}
	if o.Multiline {
		return jsonMarshalIndent(data, "", o.Indent)
	}
	return jsonMarshal(data)
}

func (o *MarshalOptions) formatSetRequest(m *gnmi.SetRequest) ([]byte, error) {
	req := setReqMsg{
		Prefix:     path.GnmiPathToXPath(m.GetPrefix(), false),
		Target:     m.GetPrefix().GetTarget(),
		Delete:     make([]string, 0, len(m.GetDelete())),
		Replace:    make([]updateMsg, 0, len(m.GetReplace())),
		Update:     make([]updateMsg, 0, len(m.GetUpdate())),
		Extensions: m.GetExtension(),
	}

	for _, del := range m.GetDelete() {
		p := path.GnmiPathToXPath(del, false)
		req.Delete = append(req.Delete, p)
	}

	for _, upd := range m.GetReplace() {
		req.Replace = append(req.Replace, updateMsg{
			Path: path.GnmiPathToXPath(upd.GetPath(), false),
			Val:  upd.Val.String(),
		})
	}

	for _, upd := range m.GetUpdate() {
		req.Update = append(req.Update, updateMsg{
			Path: path.GnmiPathToXPath(upd.GetPath(), false),
			Val:  upd.Val.String(),
		})
	}
	if o.Multiline {
		return jsonMarshalIndent(req, "", o.Indent)
	}
	return jsonMarshal(req)
}

func (o *MarshalOptions) formatSetResponse(m *gnmi.SetResponse, meta map[string]string) ([]byte, error) {
	dext, err := formatRegisteredExtensions(m.GetExtension(), o.ProtoDir, o.ProtoFiles, o.RegisteredExtensions)

	if err != nil {
		return nil, err
	}

	msg := setRspMsg{
		Prefix:            path.GnmiPathToXPath(m.GetPrefix(), false),
		Target:            m.GetPrefix().GetTarget(),
		Timestamp:         m.GetTimestamp(),
		Time:              time.Unix(0, m.Timestamp),
		Extensions:        m.GetExtension(),
		DecodedExtensions: dext,
	}
	if meta == nil {
		meta = make(map[string]string)
	}
	msg.Results = make([]updateResultMsg, 0, len(m.GetResponse()))
	if s, ok := meta["source"]; ok {
		msg.Source = s
	}
	for _, u := range m.GetResponse() {
		msg.Results = append(msg.Results, updateResultMsg{
			Operation: u.Op.String(),
			Path:      path.GnmiPathToXPath(u.GetPath(), false),
			Target:    u.GetPath().GetTarget(),
		})
	}
	if o.Multiline {
		return jsonMarshalIndent(msg, "", o.Indent)
	}
	return jsonMarshal(msg)
}
