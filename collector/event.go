package collector

import (
	"encoding/json"
	"strings"

	"github.com/jeremywohl/flatten"
	"github.com/openconfig/gnmi/proto/gnmi"
)

// EventMsg //
type EventMsg struct {
	Name      string                 `json:"name,omitempty"` // measurement name
	Timestamp int64                  `json:"timestamp,omitempty"`
	Tags      map[string]string      `json:"tags,omitempty"`
	Values    map[string]interface{} `json:"values,omitempty"`
	Deletes   []string               `json:"deletes,omitempty"`
}

// ResponseToEventMsgs //
func ResponseToEventMsgs(name string, rsp *gnmi.SubscribeResponse, meta map[string]string) ([]*EventMsg, error) {
	if rsp == nil {
		return nil, nil
	}
	evs := make([]*EventMsg, 0)
	switch rsp := rsp.Response.(type) {
	case *gnmi.SubscribeResponse_Update:
		tags := make(map[string]string)
		namePrefix, prefixTags := TagsFromGNMIPath(rsp.Update.Prefix)
		for k, v := range prefixTags {
			if _, ok := tags[k]; ok {
				tags[strings.Join([]string{namePrefix, k}, "/")] = v
				continue
			}
			tags[k] = v
		}
		e := &EventMsg{
			Tags:   make(map[string]string),
			Values: make(map[string]interface{}),
		}
		e.Timestamp = rsp.Update.Timestamp
		e.Name = name
		e.Tags = tags
		e.Values = make(map[string]interface{})
		for _, upd := range rsp.Update.Update {
			pathName, pTags := TagsFromGNMIPath(upd.Path)
			pathName = strings.TrimRight(namePrefix, "/") + "/" + strings.TrimLeft(pathName, "/")
			for k, v := range pTags {
				if _, ok := e.Tags[k]; ok {
					e.Tags[strings.Join([]string{pathName, k}, "/")] = v
					continue
				}
				e.Tags[k] = v
			}
			vs, err := parseUpdate(pathName, upd)
			if err != nil {
				return nil, err
			}
			for k, v := range vs {
				e.Values[k] = v
			}
		}
		for k, v := range meta {
			if k == "format" {
				continue
			}
			if _, ok := e.Tags[k]; ok {
				e.Tags["meta:"+k] = v
				continue
			}
			e.Tags[k] = v
		}
		evs = append(evs, e)
		if len(rsp.Update.Delete) > 0 {
			e := &EventMsg{
				Deletes: make([]string, 0, len(rsp.Update.Delete)),
			}
			e.Timestamp = rsp.Update.Timestamp
			e.Name = name
			e.Tags = tags
			for k, v := range meta {
				if k == "format" {
					continue
				}
				if _, ok := e.Tags[k]; ok {
					e.Tags["meta:"+k] = v
					continue
				}
				e.Tags[k] = v
			}
			for _, del := range rsp.Update.Delete {
				e.Deletes = append(e.Deletes, gnmiPathToXPath(del))
			}
			evs = append(evs, e)
		}
	}
	return evs, nil
}

// TagsFromGNMIPath //
func TagsFromGNMIPath(p *gnmi.Path) (string, map[string]string) {
	tags := make(map[string]string)
	sb := strings.Builder{}
	if p.Origin != "" {
		sb.WriteString(p.Origin)
		sb.Write([]byte(":"))
	}
	for _, e := range p.Elem {
		if e.Name != "" {
			sb.Write([]byte("/"))
			sb.WriteString(e.Name)
		}
		if e.Key != nil {
			for k, v := range e.Key {
				tags[k] = v
			}
		}
	}
	return sb.String(), tags
}

// parseUpdate //
func parseUpdate(prefix string, upd *gnmi.Update) (map[string]interface{}, error) {
	val, err := getValueFlat(prefix, upd.GetVal())
	if err != nil {
		return nil, err
	}
	return val, nil
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
		values[prefix] = updValue.GetDecimalVal()
	case *gnmi.TypedValue_FloatVal:
		values[prefix] = updValue.GetFloatVal()
	case *gnmi.TypedValue_IntVal:
		values[prefix] = updValue.GetIntVal()
	case *gnmi.TypedValue_StringVal:
		values[prefix] = updValue.GetStringVal()
	case *gnmi.TypedValue_UintVal:
		values[prefix] = updValue.GetUintVal()
	case *gnmi.TypedValue_LeaflistVal:
		values[prefix] = updValue.GetLeaflistVal()
	case *gnmi.TypedValue_ProtoBytes:
		values[prefix] = updValue.GetProtoBytes()
	case *gnmi.TypedValue_AnyVal:
		values[prefix] = updValue.GetAnyVal()
	case *gnmi.TypedValue_JsonIetfVal:
		jsondata = updValue.GetJsonIetfVal()
	case *gnmi.TypedValue_JsonVal:
		jsondata = updValue.GetJsonVal()
	}
	//	fmt.Println(string(jsondata))
	if len(jsondata) != 0 {
		var value interface{}
		err := json.Unmarshal(jsondata, &value)
		if err != nil {
			return nil, err
		}
		switch value := value.(type) {
		case map[string]interface{}:
			values, err = flatten.Flatten(value, prefix, flatten.PathStyle)
		default:
			values[prefix] = value
		}
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}
