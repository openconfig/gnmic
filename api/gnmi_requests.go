package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	gvalue "github.com/openconfig/gnmi/value"
	"google.golang.org/protobuf/proto"
)

type GNMIOption func(proto.Message) error

var ErrInvalidMsgType = errors.New("invalid message type")

// NewCapabilitiesRequest creates a new *gnmi.CapabilityeRequest using the priovided GNMIOption list.
// returns an error in case one of the options is invalid
func NewCapabilitiesRequest(opts ...GNMIOption) (*gnmi.CapabilityRequest, error) {
	req := new(gnmi.CapabilityRequest)
	var err error
	for _, o := range opts {
		err = o(req)
		if err != nil {
			return nil, err
		}
	}
	return req, nil
}

// NewGetRequest creates a new *gnmi.GetRequest using the priovided GNMIOption list.
// returns an error in case one of the options is invalid
func NewGetRequest(opts ...GNMIOption) (*gnmi.GetRequest, error) {
	getReq := new(gnmi.GetRequest)
	var err error
	for _, o := range opts {
		err = o(getReq)
		if err != nil {
			return nil, err
		}
	}
	return getReq, nil
}

// NewSetRequest creates a new *gnmi.SetRequest using the priovided GNMIOption list.
// returns an error in case one of the options is invalid
func NewSetRequest(opts ...GNMIOption) (*gnmi.SetRequest, error) {
	req := new(gnmi.SetRequest)
	var err error
	for _, o := range opts {
		err = o(req)
		if err != nil {
			return nil, err
		}
	}
	return req, nil
}

// NewSubscribeRequest creates a new *gnmi.SubscribeRequest using the priovided GNMIOption list.
// returns an error in case one of the options is invalid
func NewSubscribeRequest(opts ...GNMIOption) (*gnmi.SubscribeRequest, error) {
	req := &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: new(gnmi.SubscriptionList),
		},
	}
	var err error
	for _, o := range opts {
		err = o(req)
		if err != nil {
			return nil, err
		}
	}
	return req, nil
}

// NewSubscribePollRequest creates a new *gnmi.SubscribeRequest with request type Poll
// using the priovided GNMIOption list.
// returns an error in case one of the options is invalid
func NewSubscribePollRequest(opts ...GNMIOption) *gnmi.SubscribeRequest {
	return &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Poll{
			Poll: new(gnmi.Poll),
		},
	}
}

// Messages options

// Extention creates a GNMIOption that applies the suplied gnmi_ext.Extension to the provided
// proto.Message.
func Extension(ext *gnmi_ext.Extension) func(msg proto.Message) error {
	return func(msg proto.Message) error {
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
		case *gnmi.SetRequest:
			if len(msg.Extension) == 0 {
				msg.Extension = make([]*gnmi_ext.Extension, 0)
			}
			msg.Extension = append(msg.Extension, ext)
		case *gnmi.SubscribeRequest:
			if len(msg.Extension) == 0 {
				msg.Extension = make([]*gnmi_ext.Extension, 0)
			}
			msg.Extension = append(msg.Extension, ext)
		}
		return nil
	}
}

// Prefix creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message (as a Path Prefix)
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Prefix(prefix string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		var err error
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			msg.Prefix, err = utils.CreatePrefix(prefix, "")
		case *gnmi.SetRequest:
			msg.Prefix, err = utils.CreatePrefix(prefix, "")
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				msg.Subscribe.Prefix, err = utils.CreatePrefix(prefix, "")
			default:
				return fmt.Errorf("option Prefix: %v: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option Prefix: %v: %T", ErrInvalidMsgType, msg)
		}
		return err
	}
}

// Target creates a GNMIOption that set the gnmi Prefix target to the supplied string value.
// The proto.Message can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Target(target string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
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
				if msg.Subscribe.Prefix == nil {
					msg.Subscribe.Prefix = new(gnmi.Path)
				}
				msg.Subscribe.Prefix.Target = target
			default:
				return fmt.Errorf("option Target: %v: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option Target: %v: %T", ErrInvalidMsgType, msg)
		}
		return err
	}
}

// Path creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.Subscription.
func Path(path string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			p, err := utils.ParsePath(path)
			if err != nil {
				return err
			}
			if len(msg.Path) == 0 {
				msg.Path = make([]*gnmi.Path, 0)
			}
			msg.Path = append(msg.Path, p)
		case *gnmi.Update:
			var err error
			msg.Path, err = utils.ParsePath(path)
			if err != nil {
				return err
			}
		case *gnmi.Subscription:
			var err error
			msg.Path, err = utils.ParsePath(path)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("option Path: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Encoding creates a GNMIOption that adds the encoding type to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Encoding(encoding string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		enc, ok := gnmi.Encoding_value[strings.ToUpper(encoding)]
		if !ok {
			return fmt.Errorf("invalid encoding type %s", encoding)
		}
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			msg.Encoding = gnmi.Encoding(enc)
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				msg.Subscribe.Encoding = gnmi.Encoding(enc)
			}
		default:
			return fmt.Errorf("option Encoding: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// DataType creates a GNMIOption that adds the data type to the supplied proto.Message
// which must be a *gnmi.GetRequest.
func DataType(datat string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.GetRequest:
			dt, ok := gnmi.GetRequest_DataType_value[strings.ToUpper(datat)]
			if !ok {
				return fmt.Errorf("invalid data type %s", datat)
			}
			msg.Type = gnmi.GetRequest_DataType(dt)
		default:
			return fmt.Errorf("option DataType: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Update creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message,
// the supplied message must be a *gnmi.SetRequest.
func Update(opts ...GNMIOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SetRequest:
			if len(msg.Update) == 0 {
				msg.Update = make([]*gnmi.Update, 0)
			}
			upd := new(gnmi.Update)
			var err error
			for _, o := range opts {
				if err = o(upd); err != nil {
					return err
				}
			}
			msg.Update = append(msg.Update, upd)
		default:
			return fmt.Errorf("option Update: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Replace creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest.
func Replace(opts ...GNMIOption) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SetRequest:
			if len(msg.Update) == 0 {
				msg.Update = make([]*gnmi.Update, 0)
			}
			upd := new(gnmi.Update)
			var err error
			for _, o := range opts {
				if err = o(upd); err != nil {
					return err
				}
			}
			msg.Replace = append(msg.Replace, upd)
		default:
			return fmt.Errorf("option Replace: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Value creates a GNMIOption that creates a *gnmi.TypedValue and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.Update.
// If a map is supplied as `data interface{}` it has to be a map[string]interface{}.
func Value(data interface{}, encoding string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		var err error
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Update:
			msg.Val, err = value(data, encoding)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("option Value: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

func value(data interface{}, encoding string) (*gnmi.TypedValue, error) {
	var err error
	switch data := data.(type) {
	case []interface{}, []string:
		switch strings.ToLower(encoding) {
		case "json":
			b, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(b, " \r\n\t"),
				}}, nil
		case "json_ietf":
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
			encoding = "json"
			fallthrough
		case "json":
			b, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(b, " \r\n\t"),
				}}, nil
		case "json_ietf":
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
			// data = strings.TrimRight(strings.TrimLeft(data, "["), "]")
			buff := new(bytes.Buffer)
			bval := json.RawMessage(data)
			if json.Valid(bval) {
				err = json.NewEncoder(buff).Encode(bval)
			} else {
				err = json.NewEncoder(buff).Encode(data)
			}
			if err != nil {
				return nil, err
			}

			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
				}}, nil
		case "json_ietf":
			buff := new(bytes.Buffer)
			bval := json.RawMessage(data)
			if json.Valid(bval) {
				err = json.NewEncoder(buff).Encode(bval)
			} else {
				err = json.NewEncoder(buff).Encode(data)
			}
			if err != nil {
				return nil, err
			}
			return &gnmi.TypedValue{
				Value: &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
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
	default:
		return gvalue.FromScalar(data)
	}
	return nil, fmt.Errorf("unexpected value type: %T", data)
}

// Delete creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest. The *gnmi.Path is added the .Delete list.
func Delete(path string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SetRequest:
			p, err := utils.ParsePath(path)
			if err != nil {
				return err
			}
			if len(msg.Delete) == 0 {
				msg.Delete = make([]*gnmi.Path, 0)
			}
			msg.Delete = append(msg.Delete, p)
		default:
			return fmt.Errorf("option Delete: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SubscriptionListMode creates a GNMIOption that sets the SubscribeRequest Mode.
// The variable mode must be one of "once", "poll" or "stream".
// The supplied proto.Message must be a *gnmi.SubscribeRequest with RequestType Subscribe.
func SubscriptionListMode(mode string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				gmode, ok := gnmi.SubscriptionList_Mode_value[strings.ToUpper(mode)]
				if !ok {
					return fmt.Errorf("invalid subscription list mode: %s", mode)
				}
				msg.Subscribe.Mode = gnmi.SubscriptionList_Mode(gmode)
			default:
				return fmt.Errorf("option SubscriptionListMode: %v: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option SubscriptionListMode: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// Qos creates a GNMIOption that sets the QosMarking field in a *gnmi.SubscribeRequest with RequestType Subscribe.
func Qos(qos uint32) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				msg.Subscribe.Qos = &gnmi.QOSMarking{Marking: qos}
			default:
				return fmt.Errorf("option Qos: %v: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option Qos: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// UseAliases creates a GNMIOption that sets the UsesAliases field in a *gnmi.SubscribeRequest with RequestType Subscribe.
func UseAliases(b bool) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.SubscribeRequest:
			switch msg := msg.Request.(type) {
			case *gnmi.SubscribeRequest_Subscribe:
				msg.Subscribe.UseAliases = b
			default:
				return fmt.Errorf("option UseAliases: %v: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option UseAliases: %v: %T", ErrInvalidMsgType, msg)
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
				msg.Subscribe.AllowAggregation = b
			default:
				return fmt.Errorf("option AllowAggregation: %v: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option AllowAggregation: %v: %T", ErrInvalidMsgType, msg)
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
				msg.Subscribe.UpdatesOnly = b
			default:
				return fmt.Errorf("option UpdatesOnly: %v: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option UpdatesOnly: %v: %T", ErrInvalidMsgType, msg)
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
				if len(msg.Subscribe.Subscription) == 0 {
					msg.Subscribe.Subscription = make([]*gnmi.Subscription, 0)
				}
				sub := new(gnmi.Subscription)
				var err error
				for _, o := range opts {
					if err = o(sub); err != nil {
						return err
					}
				}
				msg.Subscribe.Subscription = append(msg.Subscribe.Subscription, sub)
			default:
				return fmt.Errorf("option Subscription: %v: %T", ErrInvalidMsgType, msg)
			}
		default:
			return fmt.Errorf("option Subscription: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SubscriptionMode creates a GNMIOption that sets the Subscription mode in a proto.Message of type *gnmi.Subscription.
func SubscriptionMode(mode string) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Subscription:
			gmode, ok := gnmi.SubscriptionMode_value[strings.ToUpper(strings.ReplaceAll(mode, "-", "_"))]
			if !ok {
				return fmt.Errorf("invalid subscription mode: %s", mode)
			}
			msg.Mode = gnmi.SubscriptionMode(gmode)
		default:
			return fmt.Errorf("option SubscriptionMode: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SampleInterval creates a GNMIOption that sets the SampleInterval in a proto.Message of type *gnmi.Subscription.
func SampleInterval(d time.Duration) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Subscription:
			msg.SampleInterval = uint64(d.Nanoseconds())
		default:
			return fmt.Errorf("option SampleInterval: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// HeartbeatInterval creates a GNMIOption that sets the HeartbeatInterval in a proto.Message of type *gnmi.Subscription.
func HeartbeatInterval(d time.Duration) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Subscription:
			msg.HeartbeatInterval = uint64(d.Nanoseconds())
		default:
			return fmt.Errorf("option HeartbeatInterval: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}

// SuppressRedundant creates a GNMIOption that sets the SuppressRedundant in a proto.Message of type *gnmi.Subscription.
func SuppressRedundant(s bool) func(msg proto.Message) error {
	return func(msg proto.Message) error {
		switch msg := msg.ProtoReflect().Interface().(type) {
		case *gnmi.Subscription:
			msg.SuppressRedundant = s
		default:
			return fmt.Errorf("option SuppressRedundant: %v: %T", ErrInvalidMsgType, msg)
		}
		return nil
	}
}
