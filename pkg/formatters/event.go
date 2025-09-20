// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package formatters

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	flattener "github.com/karimra/go-map-flattener"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
)

// EventMsg represents a gNMI update message,
// The name is derived from the subscription in case the update was received in a subscribeResponse
// the tags are derived from the keys in gNMI path as well as some metadata from the subscription.
type EventMsg struct {
	Name      string                 `json:"name,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
	Tags      map[string]string      `json:"tags,omitempty"`
	Values    map[string]interface{} `json:"values,omitempty"`
	Deletes   []string               `json:"deletes,omitempty"`
}

func (e *EventMsg) String() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// ResponseToEventMsgs //
func ResponseToEventMsgs(name string, rsp *gnmi.SubscribeResponse, meta map[string]string, eps ...EventProcessor) ([]*EventMsg, error) {
	if rsp == nil {
		return nil, nil
	}
	evs := make([]*EventMsg, 0, len(rsp.GetUpdate().GetUpdate())+len(rsp.GetUpdate().GetDelete()))
	response := rsp
	switch rsp := rsp.Response.(type) {
	case *gnmi.SubscribeResponse_Update:
		namePrefix, prefixTags := tagsFromGNMIPath(rsp.Update.GetPrefix())
		// Extension message to tags
		if prefixTags == nil {
			prefixTags = make(map[string]string)
		}
		for _, ext := range response.Extension {
			extensionValues, err := extensionToMap(ext)
			if err != nil {
				return nil, err
			}
			for k, v := range extensionValues {
				switch v := v.(type) {
				case string:
					prefixTags[k] = v
				case float64:
					prefixTags[k] = strconv.FormatFloat(v, 'G', -1, 64)
				}
			}
		}
		// notification updates
		uevs, err := updatesToEvent(name, namePrefix, rsp.Update.GetTimestamp(), rsp.Update.GetUpdate(), prefixTags, meta)
		if err != nil {
			return nil, err
		}
		evs = append(evs, uevs...)
		// notification deletes
		for _, del := range rsp.Update.GetDelete() {
			e := deleteToEvent(name, namePrefix, rsp.Update.GetTimestamp(), del, prefixTags)
			addMetaTags(e, meta)
			if (e != nil && e != &EventMsg{}) {
				evs = append(evs, e)
			}
		}

		for _, ep := range eps {
			evs = ep.Apply(evs...)
		}
	}
	return evs, nil
}

func GetResponseToEventMsgs(rsp *gnmi.GetResponse, meta map[string]string, eps ...EventProcessor) ([]*EventMsg, error) {
	if rsp == nil {
		return nil, nil
	}
	evs := make([]*EventMsg, 0, len(rsp.GetNotification()))
	for _, notif := range rsp.GetNotification() {
		namePrefix, prefixTags := tagsFromGNMIPath(notif.GetPrefix())
		uevs, err := updatesToEvent("get-request", namePrefix, notif.GetTimestamp(), notif.GetUpdate(), prefixTags, meta)
		if err != nil {
			return nil, err
		}
		evs = append(evs, uevs...)
	}
	for _, ep := range eps {
		evs = ep.Apply(evs...)
	}
	return evs, nil
}

func updatesToEvent(name, prefix string, ts int64, upds []*gnmi.Update, tags, meta map[string]string) ([]*EventMsg, error) {
	evs := make([]*EventMsg, 0, len(upds))
	for _, upd := range upds {
		e, err := updateToEvent(name, prefix, ts, upd, tags)
		if err != nil {
			return nil, err
		}
		addMetaTags(e, meta)
		if (e != nil && e != &EventMsg{}) {
			evs = append(evs, e)
		}
	}
	return evs, nil
}

func updateToEvent(name, prefix string, ts int64, upd *gnmi.Update, tags map[string]string) (*EventMsg, error) {
	e := &EventMsg{
		Name:      name,
		Timestamp: ts,
		Tags:      make(map[string]string),
		Values:    make(map[string]interface{}),
	}
	for k, v := range tags {
		e.Tags[k] = v
	}
	pathName, pTags := tagsFromGNMIPath(upd.GetPath())
	psb := strings.Builder{}
	psb.WriteString(strings.TrimRight(prefix, "/"))
	psb.WriteString("/")
	psb.WriteString(strings.TrimLeft(pathName, "/"))
	pathName = psb.String()
	for k, v := range pTags {
		if vv, ok := e.Tags[k]; ok {
			if v != vv {
				e.Tags[fmt.Sprintf("%s_%s", pathName, k)] = v
			}
			continue
		}
		e.Tags[k] = v
	}
	var err error
	e.Values, err = getValueFlat(pathName, upd.GetVal())
	if err != nil {
		return nil, err
	}
	return e, nil
}

func deleteToEvent(name, prefix string, ts int64, del *gnmi.Path, tags map[string]string) *EventMsg {
	e := &EventMsg{
		Name:      name,
		Timestamp: ts,
		Tags:      make(map[string]string),
		Deletes:   make([]string, 0, 1),
	}
	for k, v := range tags {
		e.Tags[k] = v
	}
	pathName, pTags := tagsFromGNMIPath(del)
	psb := strings.Builder{}
	psb.WriteString(strings.TrimRight(prefix, "/"))
	psb.WriteString("/")
	psb.WriteString(strings.TrimLeft(pathName, "/"))
	pathName = psb.String()
	for k, v := range pTags {
		if vv, ok := e.Tags[k]; ok {
			if v != vv {
				e.Tags[fmt.Sprintf("%s_%s", pathName, k)] = v
			}
			continue
		}
		e.Tags[k] = v
	}
	e.Deletes = append(e.Deletes, pathName)
	return e
}

// tagsFromGNMIPath returns a string representation of the gNMI path without keys,
// as well as a map of the keys in the path.
// the key map will also contain a target value if present in the gNMI path.
func tagsFromGNMIPath(p *gnmi.Path) (string, map[string]string) {
	if p == nil {
		return "", nil
	}
	tags := make(map[string]string)
	sb := strings.Builder{}
	if p.Origin != "" {
		sb.WriteString(p.Origin)
		sb.WriteString(":")
	}
	for _, e := range p.GetElem() {
		if e.Name != "" {
			sb.WriteString("/")
			sb.WriteString(e.Name)
		}
		if e.Key != nil {
			for k, v := range e.Key {
				if e.Name == "" {
					tags[k] = v
					continue
				}
				elems := strings.Split(e.Name, ":")
				ksb := strings.Builder{}
				ksb.WriteString(elems[len(elems)-1])
				ksb.WriteString("_")
				ksb.WriteString(k)
				tags[ksb.String()] = v
			}
		}
	}
	if p.GetTarget() != "" {
		tags["target"] = p.GetTarget()
	}
	return sb.String(), tags
}

func extensionToMap(ext *gnmi_ext.Extension) (map[string]interface{}, error) {
	jsondata := ext.GetRegisteredExt().GetMsg()

	var anyJson map[string]interface{}
	if len(jsondata) != 0 {
		err := json.Unmarshal(jsondata, &anyJson)
		if err != nil {
			return nil, err
		}
		return anyJson, nil
	}
	return nil, fmt.Errorf("0 length JSON decoded")
}

func getValueFlat(prefix string, updValue *gnmi.TypedValue) (map[string]interface{}, error) {
	if updValue == nil {
		return nil, nil
	}
	var jsondata []byte
	values := make(map[string]interface{})
	switch updValue.Value.(type) {
	case *gnmi.TypedValue_AsciiVal:
		values[prefix] = updValue.GetAsciiVal()
	case *gnmi.TypedValue_BoolVal:
		values[prefix] = updValue.GetBoolVal()
	case *gnmi.TypedValue_BytesVal:
		values[prefix] = updValue.GetBytesVal()
	case *gnmi.TypedValue_DecimalVal:
		//lint:ignore SA1019 still need DecimalVal for backward compatibility
		v := updValue.GetDecimalVal()
		values[prefix] = float64(v.Digits) / math.Pow10(int(v.Precision))
	case *gnmi.TypedValue_FloatVal:
		//lint:ignore SA1019 still need GetFloatVal for backward compatibility
		values[prefix] = updValue.GetFloatVal()
	case *gnmi.TypedValue_DoubleVal:
		values[prefix] = updValue.GetDoubleVal()
	case *gnmi.TypedValue_IntVal:
		values[prefix] = updValue.GetIntVal()
	case *gnmi.TypedValue_StringVal:
		values[prefix] = updValue.GetStringVal()
	case *gnmi.TypedValue_UintVal:
		values[prefix] = updValue.GetUintVal()
	case *gnmi.TypedValue_LeaflistVal:
		leafListVals := make([]interface{}, 0)
		for _, tv := range updValue.GetLeaflistVal().GetElement() {
			v, err := getValue(tv)
			if err != nil {
				return nil, err
			}
			leafListVals = append(leafListVals, v)
		}
		values[prefix] = leafListVals
	case *gnmi.TypedValue_ProtoBytes:
		values[prefix] = updValue.GetProtoBytes()
	case *gnmi.TypedValue_AnyVal:
		values[prefix] = updValue.GetAnyVal()
	case *gnmi.TypedValue_JsonIetfVal:
		jsondata = updValue.GetJsonIetfVal()
	case *gnmi.TypedValue_JsonVal:
		jsondata = updValue.GetJsonVal()
	}
	if len(jsondata) != 0 {
		var value interface{}
		err := json.Unmarshal(jsondata, &value)
		if err != nil {
			return nil, err
		}
		switch value := value.(type) {
		case map[string]interface{}:
			f := flattener.NewFlattener()
			f.SetPrefix(prefix)
			values, err = f.Flatten(value)
		default:
			values[prefix] = value
		}
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func (e *EventMsg) ToMap() map[string]interface{} {
	if e == nil {
		return nil
	}
	m := make(map[string]interface{})
	if e.Name != "" {
		m["name"] = e.Name
	}
	if e.Timestamp != 0 {
		m["timestamp"] = e.Timestamp
	}
	if len(e.Tags) > 0 {
		in := make(map[string]interface{})
		for k, v := range e.Tags {
			in[k] = v
		}
		m["tags"] = in
	}
	if len(e.Values) > 0 {
		m["values"] = e.Values
	}
	if len(e.Deletes) > 0 {
		m["deletes"] = e.Deletes
	}
	return m
}

func EventFromMap(m map[string]interface{}) (*EventMsg, error) {
	if m == nil {
		return nil, nil
	}
	e := new(EventMsg)

	if v, ok := m["name"]; ok {
		switch v := v.(type) {
		case string:
			e.Name = v
		default:
			return nil, fmt.Errorf("could not convert map to event message, name it not a string")
		}
	}
	if v, ok := m["timestamp"]; ok {
		i := num64(v)
		if i == nil {
			return nil, fmt.Errorf("could not convert map to event message, timestamp is not an int64: %T", v)
		}
		switch i := i.(type) {
		case int64:
			e.Timestamp = i
		case uint64:
			e.Timestamp = int64(i)
		}

	}
	if v, ok := m["tags"]; ok {
		switch v := v.(type) {
		case map[string]string:
			e.Tags = v
		case map[string]interface{}:
			e.Tags = make(map[string]string)
			for k, v := range v {
				e.Tags[k], _ = v.(string)
			}
		default:
			return nil, fmt.Errorf("could not convert map to event message, tags are not a map[string]string")
		}
	}
	if v, ok := m["values"]; ok {
		switch v := v.(type) {
		case map[string]interface{}:
			e.Values = v
		case map[string]string:
			e.Values = make(map[string]interface{})
			for k, v := range v {
				e.Values[k] = v
			}
		default:
			return nil, fmt.Errorf("could not convert map to event message, values are not a map[string]interface{}")
		}
	}
	if v, ok := m["deletes"]; ok {
		switch v := v.(type) {
		case []string:
			e.Deletes = v
		case []interface{}:
			for _, d := range v {
				if ds, ok := d.(string); ok {
					e.Deletes = append(e.Deletes, ds)
				}
			}
		default:
			return nil, fmt.Errorf("could not convert map to event message, name it not a string")
		}
	}
	return e, nil
}

func num64(n interface{}) interface{} {
	switch n := n.(type) {
	case int:
		return int64(n)
	case int8:
		return int64(n)
	case int16:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return int64(n)
	case uint:
		return uint64(n)
	case uintptr:
		return uint64(n)
	case uint8:
		return uint64(n)
	case uint16:
		return uint64(n)
	case uint32:
		return uint64(n)
	case uint64:
		return uint64(n)
	case float64:
		return uint64(n)
	}
	return nil
}

func addMetaTags(e *EventMsg, meta map[string]string) {
	for k, v := range meta {
		if k == "format" {
			continue
		}
		if _, ok := e.Tags[k]; ok {
			e.Tags[fmt.Sprintf("meta_%s", k)] = v
			continue
		}
		e.Tags[k] = v
	}
}
