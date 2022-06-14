package formatters

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
)

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
		return o.formatsubscribeRequest(m)
	case *gnmi.SubscribeResponse:
		return o.formatSubscribeResponse(m, meta)
	}
	return nil, nil
}

func (o *MarshalOptions) formatsubscribeRequest(m *gnmi.SubscribeRequest) ([]byte, error) {
	msg := subscribeReq{}
	switch m := m.Request.(type) {
	case *gnmi.SubscribeRequest_Subscribe:
		msg.Subscribe.Prefix = utils.GnmiPathToXPath(m.Subscribe.GetPrefix(), false)
		msg.Subscribe.Target = m.Subscribe.GetPrefix().GetTarget()
		msg.Subscribe.Subscriptions = make([]subscription, 0, len(m.Subscribe.GetSubscription()))
		if m.Subscribe != nil {
			msg.Subscribe.UseAliases = m.Subscribe.UseAliases
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
						Path:              utils.GnmiPathToXPath(sub.Path, false),
						Mode:              sub.GetMode().String(),
						SampleInterval:    sub.SampleInterval,
						HeartbeatInterval: sub.HeartbeatInterval,
						SuppressRedundant: sub.SuppressRedundant,
					})
			}
		}
	case *gnmi.SubscribeRequest_Poll:
		msg.Poll = new(poll)
	case *gnmi.SubscribeRequest_Aliases:
		msg.Aliases = make(map[string]string)
		for _, a := range m.Aliases.GetAlias() {
			msg.Aliases[a.Alias] = utils.GnmiPathToXPath(a.Path, false)
		}
	}
	if o.Multiline {
		return json.MarshalIndent(msg, "", o.Indent)
	}
	return json.Marshal(msg)
}

func (o *MarshalOptions) formatSubscribeResponse(m *gnmi.SubscribeResponse, meta map[string]string) ([]byte, error) {
	switch m := m.Response.(type) {
	case *gnmi.SubscribeResponse_Update:
		msg := NotificationRspMsg{
			Timestamp: m.Update.Timestamp,
		}
		t := time.Unix(0, m.Update.Timestamp)
		msg.Time = &t
		if meta == nil {
			meta = make(map[string]string)
		}
		msg.Prefix = utils.GnmiPathToXPath(m.Update.GetPrefix(), false)
		msg.Target = m.Update.Prefix.GetTarget()
		if s, ok := meta["source"]; ok {
			msg.Source = s
		}
		if s, ok := meta["system-name"]; ok {
			msg.SystemName = s
		}
		if s, ok := meta["subscription-name"]; ok {
			msg.SubscriptionName = s
		}
		for i, upd := range m.Update.Update {
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
					Path:   utils.GnmiPathToXPath(upd.Path, false),
					Values: make(map[string]interface{}),
				})
			msg.Updates[i].Values[strings.Join(pathElems, "/")] = value
		}
		for _, del := range m.Update.Delete {
			msg.Deletes = append(msg.Deletes, utils.GnmiPathToXPath(del, false))
		}
		if o.Multiline {
			return json.MarshalIndent(msg, "", o.Indent)
		}
		return json.Marshal(msg)
	}
	return nil, nil
}

func (o *MarshalOptions) formatCapabilitiesRequest(m *gnmi.CapabilityRequest) ([]byte, error) {
	capReq := capRequest{
		Extensions: make([]string, 0, len(m.Extension)),
	}
	for _, e := range m.Extension {
		capReq.Extensions = append(capReq.Extensions, e.String())
	}
	if o.Multiline {
		return json.MarshalIndent(capReq, "", o.Indent)
	}
	return json.Marshal(capReq)
}

func (o *MarshalOptions) formatCapabilitiesResponse(m *gnmi.CapabilityResponse) ([]byte, error) {
	capRspMsg := capResponse{}
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
		return json.MarshalIndent(capRspMsg, "", o.Indent)
	}
	return json.Marshal(capRspMsg)
}

func (o *MarshalOptions) formatGetRequest(m *gnmi.GetRequest) ([]byte, error) {
	msg := getRqMsg{
		Prefix:   utils.GnmiPathToXPath(m.GetPrefix(), false),
		Target:   m.GetPrefix().GetTarget(),
		Paths:    make([]string, 0, len(m.Path)),
		Encoding: m.GetEncoding().String(),
		DataType: m.GetType().String(),
	}
	for _, p := range m.Path {
		msg.Paths = append(msg.Paths, utils.GnmiPathToXPath(p, false))
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
		return json.MarshalIndent(msg, "", o.Indent)
	}
	return json.Marshal(msg)
}

func (o *MarshalOptions) formatGetResponse(m *gnmi.GetResponse, meta map[string]string) ([]byte, error) {
	notifications := make([]NotificationRspMsg, 0, len(m.GetNotification()))
	for _, notif := range m.GetNotification() {
		msg := NotificationRspMsg{
			Prefix:  utils.GnmiPathToXPath(notif.GetPrefix(), false),
			Updates: make([]update, 0, len(notif.GetUpdate())),
			Deletes: make([]string, 0, len(notif.GetDelete())),
		}
		msg.Timestamp = notif.Timestamp
		t := time.Unix(0, notif.Timestamp)
		msg.Time = &t
		if meta == nil {
			meta = make(map[string]string)
		}
		msg.Prefix = utils.GnmiPathToXPath(notif.GetPrefix(), false)
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
					Path:   utils.GnmiPathToXPath(upd.GetPath(), false),
					Values: make(map[string]interface{}),
				})
			msg.Updates[i].Values[strings.Join(pathElems, "/")] = value
		}
		for _, del := range notif.GetDelete() {
			msg.Deletes = append(msg.Deletes, utils.GnmiPathToXPath(del, false))
		}
		notifications = append(notifications, msg)
	}
	if o.ValuesOnly {
		result := make([]interface{}, 0, len(notifications))
		for _, n := range notifications {
			for _, u := range n.Updates {
				for _, v := range u.Values {
					result = append(result, v)
				}
			}
		}
		return json.MarshalIndent(result, "", "  ")
	}
	if o.Multiline {
		return json.MarshalIndent(notifications, "", o.Indent)
	}
	return json.Marshal(notifications)
}

func (o *MarshalOptions) formatSetRequest(m *gnmi.SetRequest) ([]byte, error) {
	req := setReqMsg{
		Prefix:  utils.GnmiPathToXPath(m.GetPrefix(), false),
		Target:  m.GetPrefix().GetTarget(),
		Delete:  make([]string, 0, len(m.GetDelete())),
		Replace: make([]updateMsg, 0, len(m.GetReplace())),
		Update:  make([]updateMsg, 0, len(m.GetUpdate())),
	}

	for _, del := range m.GetDelete() {
		p := utils.GnmiPathToXPath(del, false)
		req.Delete = append(req.Delete, p)
	}

	for _, upd := range m.GetReplace() {
		req.Replace = append(req.Replace, updateMsg{
			Path: utils.GnmiPathToXPath(upd.GetPath(), false),
			Val:  upd.Val.String(),
		})
	}

	for _, upd := range m.GetUpdate() {
		req.Update = append(req.Update, updateMsg{
			Path: utils.GnmiPathToXPath(upd.GetPath(), false),
			Val:  upd.Val.String(),
		})
	}
	if o.Multiline {
		return json.MarshalIndent(req, "", o.Indent)
	}
	return json.Marshal(req)
}

func (o *MarshalOptions) formatSetResponse(m *gnmi.SetResponse, meta map[string]string) ([]byte, error) {
	msg := setRspMsg{}
	msg.Prefix = utils.GnmiPathToXPath(m.GetPrefix(), false)
	msg.Target = m.GetPrefix().GetTarget()
	msg.Timestamp = m.Timestamp
	msg.Time = time.Unix(0, m.Timestamp)
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
			Path:      utils.GnmiPathToXPath(u.GetPath(), false),
			Target:    u.GetPath().GetTarget(),
		})
	}
	if o.Multiline {
		return json.MarshalIndent(msg, "", o.Indent)
	}
	return json.Marshal(msg)
}
