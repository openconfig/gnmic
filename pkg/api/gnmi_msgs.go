// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	gvalue "github.com/openconfig/gnmi/value"
	"github.com/openconfig/gnmic/pkg/path"
	"google.golang.org/protobuf/proto"
)

const (
	DefaultGNMIVersion = "0.7.0"
	encodingJSON       = "json"
	encodingJSON_IETF  = "json_ietf"
)

// GNMIOption is a function that acts on the supplied proto.Message.
// The message is expected to be one of the protobuf defined gNMI messages
// exchanged by the RPCs or any of the nested messages.
type GNMIOption func(proto.Message) error

// ErrInvalidMsgType is returned by a GNMIOption in case the Option is supplied
// an unexpected proto.Message
var ErrInvalidMsgType = errors.New("invalid message type")

// ErrInvalidValue is returned by a GNMIOption in case the Option is supplied
// an unexpected value.
var ErrInvalidValue = errors.New("invalid value")

// apply is a helper function that simply applies the options to the proto.Message.
// It returns an error if any of the options fails.
func apply(m proto.Message, opts ...GNMIOption) error {
	for _, o := range opts {
		if err := o(m); err != nil {
			return err
		}
	}
	return nil
}

// NewCapabilitiesRequest creates a new *gnmi.CapabilityeRequest using the provided GNMIOption list.
// returns an error in case one of the options is invalid
func NewCapabilitiesRequest(opts ...GNMIOption) (*gnmi.CapabilityRequest, error) {
	m := new(gnmi.CapabilityRequest)
	err := apply(m, opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// NewCapabilitiesResponse creates a new *gnmi.CapabilityResponse using the provided GNMIOption list.
// returns an error in case one of the options is invalid
func NewCapabilitiesResponse(opts ...GNMIOption) (*gnmi.CapabilityResponse, error) {
	m := new(gnmi.CapabilityResponse)
	err := apply(m, opts...)
	if err != nil {
		return nil, err
	}
	if m.GNMIVersion == "" {
		m.GNMIVersion = DefaultGNMIVersion
	}
	return m, nil
}

// NewGetRequest creates a new *gnmi.GetRequest using the provided GNMIOption list.
// returns an error in case one of the options is invalid
func NewGetRequest(opts ...GNMIOption) (*gnmi.GetRequest, error) {
	m := new(gnmi.GetRequest)
	err := apply(m, opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// NewGetResponse creates a new *gnmi.GetResponse using the provided GNMIOption list.
// returns an error in case one of the options is invalid
func NewGetResponse(opts ...GNMIOption) (*gnmi.GetResponse, error) {
	m := new(gnmi.GetResponse)
	err := apply(m, opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// NewSetRequest creates a new *gnmi.SetRequest using the provided GNMIOption list.
// returns an error in case one of the options is invalid
func NewSetRequest(opts ...GNMIOption) (*gnmi.SetRequest, error) {
	m := new(gnmi.SetRequest)
	err := apply(m, opts...)
	return m, err
}

// NewSetResponse creates a new *gnmi.SetResponse using the provided GNMIOption list.
// returns an error in case one of the options is invalid
func NewSetResponse(opts ...GNMIOption) (*gnmi.SetResponse, error) {
	m := new(gnmi.SetResponse)
	err := apply(m, opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// NewSubscribeRequest creates a new *gnmi.SubscribeRequest using the provided GNMIOption list.
// returns an error in case one of the options is invalid
func NewSubscribeRequest(opts ...GNMIOption) (*gnmi.SubscribeRequest, error) {
	m := &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: new(gnmi.SubscriptionList),
		},
	}
	err := apply(m, opts...)
	return m, err
}

// NewSubscribePollRequest creates a new *gnmi.SubscribeRequest with request type Poll
// using the provided GNMIOption list.
// returns an error in case one of the options is invalid
func NewSubscribePollRequest(opts ...GNMIOption) (*gnmi.SubscribeRequest, error) {
	m := &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Poll{
			Poll: new(gnmi.Poll),
		},
	}
	err := apply(m, opts...)
	return m, err
}

// NewSubscribeResponse creates a *gnmi.SubscribeResponse with a gnmi.SubscribeResponse_Update as
// response type.
func NewSubscribeResponse(opts ...GNMIOption) (*gnmi.SubscribeResponse, error) {
	m := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: new(gnmi.Notification),
		},
	}
	err := apply(m, opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// NewSubscribeResponse creates a *gnmi.SubscribeResponse with a gnmi.SubscribeResponse_SyncResponse as
// response type.
func NewSubscribeSyncResponse(opts ...GNMIOption) (*gnmi.SubscribeResponse, error) {
	m := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_SyncResponse{
			SyncResponse: true,
		},
	}
	err := apply(m, opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Messages options

// Version sets the provided gNMI version string in a gnmi.CapabilityResponse message.
func Version(v string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.CapabilityResponse:
			msg.GNMIVersion = v
		default:
			return fmt.Errorf("option Version: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SupportedEncoding creates an GNMIOption that sets the provided encodings as supported encodings in a gnmi.CapabilitiesResponse
func SupportedEncoding(encodings ...string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.CapabilityResponse:
			if len(msg.SupportedEncodings) == 0 {
				msg.SupportedEncodings = make([]gnmi.Encoding, 0)
			}
			for _, encoding := range encodings {
				enc, ok := gnmi.Encoding_value[strings.ToUpper(encoding)]
				if !ok {
					return fmt.Errorf("option SupportedEncoding: %w: %s", ErrInvalidValue, encoding)
				}
				msg.SupportedEncodings = append(msg.SupportedEncodings, gnmi.Encoding(enc))
			}
		default:
			return fmt.Errorf("option SupportedEncoding: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SupportedModel creates an GNMIOption that sets the provided name, org and version as a supported model in a gnmi.CapabilitiesResponse.
func SupportedModel(name, org, version string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.CapabilityResponse:
			if len(msg.SupportedModels) == 0 {
				msg.SupportedModels = make([]*gnmi.ModelData, 0)
			}
			msg.SupportedModels = append(msg.SupportedModels,
				&gnmi.ModelData{
					Name:         name,
					Organization: org,
					Version:      version,
				})
		default:
			return fmt.Errorf("option SupportedModel: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Prefix creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied
// proto.Message (as a Path Prefix).
// The proto.Message can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Prefix(prefix string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		var err error
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			msg.Prefix, err = path.CreatePrefix(prefix, "")
		case *gnmi.SetRequest:
			msg.Prefix, err = path.CreatePrefix(prefix, "")
		case *gnmi.SetResponse:
			msg.Prefix, err = path.CreatePrefix(prefix, "")
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				if msg.Subscribe == nil {
					msg.Subscribe = new(gnmi.SubscriptionList)
				}
				msg.Subscribe.Prefix, err = path.CreatePrefix(prefix, "")
			default:
				return fmt.Errorf("option Prefix: %w: %T", ErrInvalidMsgType, msg)
			}
		case *gnmi.Notification:
			msg.Prefix, err = path.CreatePrefix(prefix, "")
		default:
			return fmt.Errorf("option Prefix: %w: %T", ErrInvalidMsgType, msg)
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidValue, err)
		}
		return nil
	}
}

// Target creates a GNMIOption that set the gnmi Prefix target to the supplied string value.
// The proto.Message can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Target(target string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		if target == "" {
			return nil
		}
		var err error
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			if msg.Prefix == nil {
				msg.Prefix = new(gnmi.Path)
			}
			msg.Prefix.Target = target
		case *gnmi.SetRequest:
			if msg.Prefix == nil {
				msg.Prefix = new(gnmi.Path)
			}
			msg.Prefix.Target = target
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				if msg.Subscribe == nil {
					msg.Subscribe = new(gnmi.SubscriptionList)
				}
				if msg.Subscribe.Prefix == nil {
					msg.Subscribe.Prefix = new(gnmi.Path)
				}
				msg.Subscribe.Prefix.Target = target
			default:
				return fmt.Errorf("option Target: %w: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option Target: %w: %T", ErrInvalidMsgType, msg)
		}
		return err
	}
}

// Path creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.Subscription.
func Path(p string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		var err error
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			gp, err := path.ParsePath(p)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidValue, err)
			}
			if len(msg.Path) == 0 {
				msg.Path = make([]*gnmi.Path, 0)
			}
			msg.Path = append(msg.Path, gp)
		case *gnmi.Update:
			msg.Path, err = path.ParsePath(p)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidValue, err)
			}
		case *gnmi.UpdateResult:
			msg.Path, err = path.ParsePath(p)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidValue, err)
			}
		case *gnmi.Subscription:
			msg.Path, err = path.ParsePath(p)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidValue, err)
			}
		default:
			return fmt.Errorf("option Path: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Encoding creates a GNMIOption that adds the encoding type to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Encoding(encoding string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		enc, ok := gnmi.Encoding_value[strings.ToUpper(encoding)]
		if !ok {
			return fmt.Errorf("option Encoding: %w: %s", ErrInvalidValue, encoding)
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			msg.Encoding = gnmi.Encoding(enc)
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				if msg.Subscribe == nil {
					msg.Subscribe = new(gnmi.SubscriptionList)
				}
				msg.Subscribe.Encoding = gnmi.Encoding(enc)
			}
		default:
			return fmt.Errorf("option Encoding: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// EncodingJSON creates a GNMIOption that sets the encoding type to JSON in a gnmi.GetRequest or
// gnmi.SubscribeRequest.
func EncodingJSON() func(msg proto.Message) error {
	return Encoding("JSON")
}

// EncodingBYTES creates a GNMIOption that sets the encoding type to BYTES in a gnmi.GetRequest or
// gnmi.SubscribeRequest.
func EncodingBYTES() func(msg proto.Message) error {
	return Encoding("BYTES")
}

// EncodingPROTO creates a GNMIOption that sets the encoding type to PROTO in a gnmi.GetRequest or
// gnmi.SubscribeRequest.
func EncodingPROTO() func(msg proto.Message) error {
	return Encoding("PROTO")
}

// EncodingASCII creates a GNMIOption that sets the encoding type to ASCII in a gnmi.GetRequest or
// gnmi.SubscribeRequest.
func EncodingASCII() func(msg proto.Message) error {
	return Encoding("ASCII")
}

// EncodingJSON_IETF creates a GNMIOption that sets the encoding type to JSON_IETF in a gnmi.GetRequest or
// gnmi.SubscribeRequest.
func EncodingJSON_IETF() func(msg proto.Message) error {
	return Encoding("JSON_IETF")
}

// EncodingCustom creates a GNMIOption that adds the encoding type to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
// Unlike Encoding, this GNMIOption does not validate if the provided encoding is defined by the gNMI spec.
func EncodingCustom(enc int) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			msg.Encoding = gnmi.Encoding(enc)
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				if msg.Subscribe == nil {
					msg.Subscribe = new(gnmi.SubscriptionList)
				}
				msg.Subscribe.Encoding = gnmi.Encoding(enc)
			}
		default:
			return fmt.Errorf("option EncodingCustom: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// DataType creates a GNMIOption that adds the data type to the supplied proto.Message
// which must be a *gnmi.GetRequest.
func DataType(datat string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		if datat == "" {
			return nil
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			dt, ok := gnmi.GetRequest_DataType_value[strings.ToUpper(datat)]
			if !ok {
				return fmt.Errorf("option DataType: %w: %s", ErrInvalidValue, datat)
			}
			msg.Type = gnmi.GetRequest_DataType(dt)
		default:
			return fmt.Errorf("option DataType: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// DataTypeALL creates a GNMIOption that sets the gnmi.GetRequest data type to ALL
func DataTypeALL() func(msg proto.Message) error {
	return DataType("ALL")
}

// DataTypeCONFIG creates a GNMIOption that sets the gnmi.GetRequest data type to CONFIG
func DataTypeCONFIG() func(msg proto.Message) error {
	return DataType("CONFIG")
}

// DataTypeSTATE creates a GNMIOption that sets the gnmi.GetRequest data type to STATE
func DataTypeSTATE() func(msg proto.Message) error {
	return DataType("STATE")
}

// DataTypeOPERATIONAL creates a GNMIOption that sets the gnmi.GetRequest data type to OPERATIONAL
func DataTypeOPERATIONAL() func(msg proto.Message) error {
	return DataType("OPERATIONAL")
}

// UseModel creates a GNMIOption that add a gnmi.DataModel to a gnmi.GetRequest or gnmi.SubscribeRequest
// based on the name, org and version strings provided.
func UseModel(name, org, version string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			if len(msg.UseModels) == 0 {
				msg.UseModels = make([]*gnmi.ModelData, 0)
			}
			msg.UseModels = append(msg.UseModels,
				&gnmi.ModelData{
					Name:         name,
					Organization: org,
					Version:      version,
				})

		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				if msg.Subscribe == nil {
					msg.Subscribe = new(gnmi.SubscriptionList)
				}
				if len(msg.Subscribe.UseModels) == 0 {
					msg.Subscribe.UseModels = make([]*gnmi.ModelData, 0)
				}
				msg.Subscribe.UseModels = append(msg.Subscribe.UseModels,
					&gnmi.ModelData{
						Name:         name,
						Organization: org,
						Version:      version,
					})
			}
		default:
			return fmt.Errorf("option UseModel: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Update creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message,
// the supplied message must be a *gnmi.SetRequest.
func Update(opts ...GNMIOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SetRequest:
			if len(msg.Update) == 0 {
				msg.Update = make([]*gnmi.Update, 0)
			}
			upd := new(gnmi.Update)
			err := apply(upd, opts...)
			if err != nil {
				return err
			}
			msg.Update = append(msg.Update, upd)
		case *gnmi.Notification:
			if len(msg.Update) == 0 {
				msg.Update = make([]*gnmi.Update, 0)
			}
			upd := new(gnmi.Update)
			err := apply(upd, opts...)
			if err != nil {
				return err
			}
			msg.Update = append(msg.Update, upd)
		default:
			return fmt.Errorf("option Update: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Replace creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest.
func Replace(opts ...GNMIOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SetRequest:
			if len(msg.Update) == 0 {
				msg.Update = make([]*gnmi.Update, 0)
			}
			upd := new(gnmi.Update)
			err := apply(upd, opts...)
			if err != nil {
				return err
			}
			msg.Replace = append(msg.Replace, upd)
		default:
			return fmt.Errorf("option Replace: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Value creates a GNMIOption that creates a *gnmi.TypedValue and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.Update.
// If a map is supplied as `data interface{}` it has to be a map[string]interface{}.
func Value(data interface{}, encoding string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		var err error
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Update:
			msg.Val, err = value(data, encoding)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("option Value: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

func value(data interface{}, encoding string) (*gnmi.TypedValue, error) {
	switch data := data.(type) {
	case []interface{}, []string:
		switch strings.ToLower(encoding) {
		case encodingJSON:
			b, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(b, " \r\n\t"),
				}}, nil
		case encodingJSON_IETF:
			b, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(b, " \r\n\t"),
				}}, nil
		default:
			return gvalue.FromScalar(data)
		}
	case map[string]interface{}:
		switch strings.ToLower(encoding) {
		case "":
			encoding = encodingJSON
			fallthrough
		case encodingJSON:
			b, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(b, " \r\n\t"),
				}}, nil
		case encodingJSON_IETF:
			b, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(b, " \r\n\t"),
				}}, nil
		}
	case string:
		switch strings.ToLower(encoding) {
		case "json":
			var b []byte
			var err error
			bval := json.RawMessage(data)
			if json.Valid(bval) {
				b, err = json.Marshal(bval)
			} else {
				b, err = json.Marshal(data)
			}
			//b, err = json.Marshal(data)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(b, " \r\n\t"),
				}}, nil
		case "json_ietf":
			var b []byte
			var err error
			bval := json.RawMessage(data)
			if json.Valid(bval) {
				b, err = json.Marshal(bval)
			} else {
				b, err = json.Marshal(data)
			}
			//b, err = json.Marshal(data)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(b, " \r\n\t"),
				}}, nil
		case "ascii":
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{
					AsciiVal: data,
				}}, nil
		case "bool":
			bval, err := strconv.ParseBool(data)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_BoolVal{
					BoolVal: bval,
				}}, nil
		case "bytes":
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_BytesVal{
					BytesVal: []byte(data),
				}}, nil
		case "decimal":
			return nil, fmt.Errorf("decimal type not implemented")
		case "float":
			f, err := strconv.ParseFloat(data, 32)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_FloatVal{
					FloatVal: float32(f),
				}}, nil
		case "int":
			k, err := strconv.ParseInt(data, 10, 64)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_IntVal{
					IntVal: k,
				}}, nil
		case "uint":
			u, err := strconv.ParseUint(data, 10, 64)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_UintVal{
					UintVal: u,
				}}, nil
		case "string":
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_StringVal{
					StringVal: data,
				}}, nil
		default:
			return nil, fmt.Errorf("invalid encoding %s", encoding)
		}
	case *gnmi.TypedValue:
		return data, nil
	case *gnmi.TypedValue_AnyVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_AsciiVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_BoolVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_BytesVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_DecimalVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_FloatVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_IntVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_JsonIetfVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_JsonVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_LeaflistVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_ProtoBytes:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_StringVal:
		return &gnmi.TypedValue{Value: data}, nil
	case *gnmi.TypedValue_UintVal:
		return &gnmi.TypedValue{Value: data}, nil
	default:
		v, err := gvalue.FromScalar(data)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidValue, err)
		}
		return v, nil
	}
	return nil, fmt.Errorf("unexpected value type and encoding %w: %T and %s", ErrInvalidValue, data, encoding)
}

// Delete creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest. The *gnmi.Path is added the .Delete list.
func Delete(p string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SetRequest:
			gp, err := path.ParsePath(p)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidValue, err)
			}
			if len(msg.Delete) == 0 {
				msg.Delete = make([]*gnmi.Path, 0)
			}
			msg.Delete = append(msg.Delete, gp)
		case *gnmi.Notification:
			gp, err := path.ParsePath(p)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidValue, err)
			}
			if len(msg.Delete) == 0 {
				msg.Delete = make([]*gnmi.Path, 0)
			}
			msg.Delete = append(msg.Delete, gp)
		default:
			return fmt.Errorf("option Delete: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SubscriptionListMode creates a GNMIOption that sets the SubscribeRequest Mode.
// The variable mode must be one of "once", "poll" or "stream".
// The supplied proto.Message must be a *gnmi.SubscribeRequest with RequestType Subscribe.
func SubscriptionListMode(mode string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				if msg.Subscribe == nil {
					msg.Subscribe = new(gnmi.SubscriptionList)
				}
				gmode, ok := gnmi.SubscriptionList_Mode_value[strings.ToUpper(mode)]
				if !ok {
					return fmt.Errorf("option SubscriptionListMode: %w: %s", ErrInvalidValue, mode)
				}
				msg.Subscribe.Mode = gnmi.SubscriptionList_Mode(gmode)
			default:
				return fmt.Errorf("option SubscriptionListMode: %w: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option SubscriptionListMode: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SubscriptionListModeSTREAM creates a GNMIOption that sets the Subscription List Mode to STREAM
func SubscriptionListModeSTREAM() func(msg proto.Message) error {
	return SubscriptionListMode("STREAM")
}

// SubscriptionListModeONCE creates a GNMIOption that sets the Subscription List Mode to ONCE
func SubscriptionListModeONCE() func(msg proto.Message) error {
	return SubscriptionListMode("ONCE")
}

// SubscriptionListModePOLL creates a GNMIOption that sets the Subscription List Mode to POLL
func SubscriptionListModePOLL() func(msg proto.Message) error {
	return SubscriptionListMode("POLL")
}

// Qos creates a GNMIOption that sets the QosMarking field in a *gnmi.SubscribeRequest with RequestType Subscribe.
func Qos(qos uint32) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				if msg.Subscribe == nil {
					msg.Subscribe = new(gnmi.SubscriptionList)
				}
				msg.Subscribe.Qos = &gnmi.QOSMarking{Marking: qos}
			default:
				return fmt.Errorf("option Qos: %w: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option Qos: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// AllowAggregation creates a GNMIOption that sets the AllowAggregation field in a *gnmi.SubscribeRequest with RequestType Subscribe.
func AllowAggregation(b bool) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				if msg.Subscribe == nil {
					msg.Subscribe = new(gnmi.SubscriptionList)
				}
				msg.Subscribe.AllowAggregation = b
			default:
				return fmt.Errorf("option AllowAggregation: %w: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option AllowAggregation: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// UpdatesOnly creates a GNMIOption that sets the UpdatesOnly field in a *gnmi.SubscribeRequest with RequestType Subscribe.
func UpdatesOnly(b bool) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				if msg.Subscribe == nil {
					msg.Subscribe = new(gnmi.SubscriptionList)
				}
				msg.Subscribe.UpdatesOnly = b
			default:
				return fmt.Errorf("option UpdatesOnly: %w: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option UpdatesOnly: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// UpdatesOnly creates a GNMIOption that creates a *gnmi.Subscription based on the supplied GNMIOption(s) and adds it the
// supplied proto.Mesage which must be of type *gnmi.SubscribeRequest with RequestType Subscribe.
func Subscription(opts ...GNMIOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				if msg.Subscribe == nil {
					msg.Subscribe = new(gnmi.SubscriptionList)
				}
				if len(msg.Subscribe.Subscription) == 0 {
					msg.Subscribe.Subscription = make([]*gnmi.Subscription, 0)
				}
				sub := new(gnmi.Subscription)
				err := apply(sub, opts...)
				if err != nil {
					return err
				}
				msg.Subscribe.Subscription = append(msg.Subscribe.Subscription, sub)
			default:
				return fmt.Errorf("option Subscription: %w: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option Subscription: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SubscriptionMode creates a GNMIOption that sets the Subscription mode in a proto.Message of type *gnmi.Subscription.
func SubscriptionMode(mode string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Subscription:
			gmode, ok := gnmi.SubscriptionMode_value[strings.ToUpper(strings.ReplaceAll(mode, "-", "_"))]
			if !ok {
				return fmt.Errorf("option SubscriptionMode: %w: %s", ErrInvalidValue, mode)
			}
			msg.Mode = gnmi.SubscriptionMode(gmode)
		default:
			return fmt.Errorf("option SubscriptionMode: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SubscriptionModeTARGET_DEFINED creates a GNMIOption that sets the subscription mode to TARGET_DEFINED
func SubscriptionModeTARGET_DEFINED() func(msg proto.Message) error {
	return SubscriptionMode("TARGET_DEFINED")
}

// SubscriptionModeON_CHANGE creates a GNMIOption that sets the subscription mode to ON_CHANGE
func SubscriptionModeON_CHANGE() func(msg proto.Message) error {
	return SubscriptionMode("ON_CHANGE")
}

// SubscriptionModeSAMPLE creates a GNMIOption that sets the subscription mode to SAMPLE
func SubscriptionModeSAMPLE() func(msg proto.Message) error {
	return SubscriptionMode("SAMPLE")
}

// SampleInterval creates a GNMIOption that sets the SampleInterval in a proto.Message of type *gnmi.Subscription.
func SampleInterval(d time.Duration) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Subscription:
			msg.SampleInterval = uint64(d.Nanoseconds())
		default:
			return fmt.Errorf("option SampleInterval: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// HeartbeatInterval creates a GNMIOption that sets the HeartbeatInterval in a proto.Message of type *gnmi.Subscription.
func HeartbeatInterval(d time.Duration) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Subscription:
			msg.HeartbeatInterval = uint64(d.Nanoseconds())
		default:
			return fmt.Errorf("option HeartbeatInterval: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SuppressRedundant creates a GNMIOption that sets the SuppressRedundant in a proto.Message of type *gnmi.Subscription.
func SuppressRedundant(s bool) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Subscription:
			msg.SuppressRedundant = s
		default:
			return fmt.Errorf("option SuppressRedundant: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Notification creates a GNMIOption that builds a gnmi.Notification from the supplied GNMIOptions and adds it
// to the supplied proto.Message
func Notification(opts ...GNMIOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetResponse:
			if len(msg.Notification) == 0 {
				msg.Notification = make([]*gnmi.Notification, 0)
			}
			notif := new(gnmi.Notification)
			err := apply(notif, opts...)
			if err != nil {
				return err
			}
			msg.Notification = append(msg.Notification, notif)
		case *gnmi.SubscribeResponse:
			switch msg := msg.Response.(type) {
			case *gnmi.SubscribeResponse_Update:
				notif := new(gnmi.Notification)
				err := apply(notif, opts...)
				if err != nil {
					return err
				}
				msg.Update = notif
			}
		default:
			return fmt.Errorf("option Notification: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Timestamp sets the supplied timestamp in a gnmi.Notification message
func Timestamp(t int64) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Notification:
			msg.Timestamp = t
		case *gnmi.SetResponse:
			msg.Timestamp = t
		default:
			return fmt.Errorf("option Timestamp: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// TimestampNow is the same as Timestamp(time.Now().UnixNano())
func TimestampNow() func(msg proto.Message) error {
	return Timestamp(time.Now().UnixNano())
}

// Atomic sets the .Atomic field in a gnmi.Notification message
func Atomic(b bool) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Notification:
			msg.Atomic = b
		default:
			return fmt.Errorf("option Atomic: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// UpdateResult creates a GNMIOption that creates a gnmi.UpdateResult and adds it to
// a proto.Message of type gnmi.SetResponse.
func UpdateResult(opts ...GNMIOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SetResponse:
			if len(msg.Response) == 0 {
				msg.Response = make([]*gnmi.UpdateResult, 0)
			}
			updRes := new(gnmi.UpdateResult)
			err := apply(updRes, opts...)
			if err != nil {
				return err
			}
			msg.Response = append(msg.Response, updRes)
		default:
			return fmt.Errorf("option UpdateResult: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Operation creates a GNMIOption that sets the gnmi.UpdateResult_Operation
// value in a gnmi.UpdateResult.
func Operation(oper string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.UpdateResult:
			setOper, ok := gnmi.UpdateResult_Operation_value[strings.ToUpper(oper)]
			if !ok {
				return fmt.Errorf("option Operation: %w: %s", ErrInvalidValue, oper)
			}
			msg.Op = gnmi.UpdateResult_Operation(setOper)
		default:
			return fmt.Errorf("option Operation: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// OperationINVALID creates a GNMIOption that sets the gnmi.SetResponse Operation to INVALID
func OperationINVALID() func(msg proto.Message) error {
	return Operation("INVALID")
}

// OperationDELETE creates a GNMIOption that sets the gnmi.SetResponse Operation to DELETE
func OperationDELETE() func(msg proto.Message) error {
	return Operation("DELETE")
}

// OperationREPLACE creates a GNMIOption that sets the gnmi.SetResponse Operation to REPLACE
func OperationREPLACE() func(msg proto.Message) error {
	return Operation("REPLACE")
}

// OperationUPDATE creates a GNMIOption that sets the gnmi.SetResponse Operation to UPDATE
func OperationUPDATE() func(msg proto.Message) error {
	return Operation("UPDATE")
}

// UnionReplace creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest.
func UnionReplace(opts ...GNMIOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		if msg == nil {
			return ErrInvalidMsgType
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SetRequest:
			if len(msg.UnionReplace) == 0 {
				msg.UnionReplace = make([]*gnmi.Update, 0)
			}
			upd := new(gnmi.Update)
			err := apply(upd, opts...)
			if err != nil {
				return err
			}
			msg.UnionReplace = append(msg.UnionReplace, upd)
		default:
			return fmt.Errorf("option UnionReplace: %w: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}
