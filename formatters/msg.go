package formatters

import (
	"encoding/json"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
)

type NotificationRspMsg struct {
	Meta             map[string]interface{} `json:"meta,omitempty"`
	Source           string                 `json:"source,omitempty"`
	SystemName       string                 `json:"system-name,omitempty"`
	SubscriptionName string                 `json:"subscription-name,omitempty"`
	Timestamp        int64                  `json:"timestamp,omitempty"`
	Time             *time.Time             `json:"time,omitempty"`
	Prefix           string                 `json:"prefix,omitempty"`
	Target           string                 `json:"target,omitempty"`
	Updates          []update               `json:"updates,omitempty"`
	Deletes          []string               `json:"deletes,omitempty"`
}
type update struct {
	Path   string
	Values map[string]interface{} `json:"values,omitempty"`
}
type capRequest struct {
	Extensions []string `json:"extensions,omitempty"`
}
type capResponse struct {
	Version         string   `json:"version,omitempty"`
	SupportedModels []model  `json:"supported-models,omitempty"`
	Encodings       []string `json:"encodings,omitempty"`
}
type model struct {
	Name         string `json:"name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Version      string `json:"version,omitempty"`
}

type getRqMsg struct {
	Prefix   string   `json:"prefix,omitempty"`
	Target   string   `json:"target,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Encoding string   `json:"encoding,omitempty"`
	DataType string   `json:"data-type,omitempty"`
	Models   []model  `json:"models,omitempty"`
}

type setRspMsg struct {
	Source    string            `json:"source,omitempty"`
	Timestamp int64             `json:"timestamp,omitempty"`
	Time      time.Time         `json:"time,omitempty"`
	Prefix    string            `json:"prefix,omitempty"`
	Target    string            `json:"target,omitempty"`
	Results   []updateResultMsg `json:"results,omitempty"`
}

type updateResultMsg struct {
	Operation string `json:"operation,omitempty"`
	Path      string `json:"path,omitempty"`
	Target    string `json:"target,omitempty"`
}

type setReqMsg struct {
	Prefix  string      `json:"prefix,omitempty"`
	Target  string      `json:"target,omitempty"`
	Delete  []string    `json:"delete,omitempty"`
	Replace []updateMsg `json:"replace,omitempty"`
	Update  []updateMsg `json:"update,omitempty"`
	// extension is not implemented
}

type updateMsg struct {
	Path string `json:"path,omitempty"`
	Val  string `json:"val,omitempty"`
}

type subscribeReq struct {
	Subscribe subscribe         `json:"subscribe,omitempty"`
	Poll      *poll             `json:"poll,omitempty"`
	Aliases   map[string]string `json:"aliases,omitempty"`
}
type poll struct{}
type subscribe struct {
	Target           string         `json:"target,omitempty"`
	Prefix           string         `json:"prefix,omitempty"`
	Subscriptions    []subscription `json:"subscriptions,omitempty"`
	UseAliases       bool           `json:"use-aliases,omitempty"`
	Qos              uint32         `json:"qos,omitempty"`
	Mode             string         `json:"mode,omitempty"`
	AllowAggregation bool           `json:"allow-aggregation,omitempty"`
	UseModels        []model        `json:"use-models,omitempty"`
	Encoding         string         `json:"encoding,omitempty"`
	UpdatesOnly      bool           `json:"updates-only,omitempty"`
}
type subscription struct {
	Path              string `json:"path,omitempty"`
	Mode              string `json:"mode,omitempty"`
	SampleInterval    uint64 `json:"sample-interval,omitempty"`
	SuppressRedundant bool   `json:"suppress-redundant,omitempty"`
	HeartbeatInterval uint64 `json:"heartbeat-interval,omitempty"`
}

func getValue(updValue *gnmi.TypedValue) (interface{}, error) {
	if updValue == nil {
		return nil, nil
	}
	var value interface{}
	var jsondata []byte
	switch updValue.Value.(type) {
	case *gnmi.TypedValue_AsciiVal:
		value = updValue.GetAsciiVal()
	case *gnmi.TypedValue_BoolVal:
		value = updValue.GetBoolVal()
	case *gnmi.TypedValue_BytesVal:
		value = updValue.GetBytesVal()
	case *gnmi.TypedValue_DecimalVal:
		//lint:ignore SA1019 still need DecimalVal for backward compatibility
		value = updValue.GetDecimalVal()
	case *gnmi.TypedValue_FloatVal:
		//lint:ignore SA1019 still need GetFloatVal for backward compatibility
		value = updValue.GetFloatVal()
	case *gnmi.TypedValue_IntVal:
		value = updValue.GetIntVal()
	case *gnmi.TypedValue_StringVal:
		value = updValue.GetStringVal()
	case *gnmi.TypedValue_UintVal:
		value = updValue.GetUintVal()
	case *gnmi.TypedValue_JsonIetfVal:
		jsondata = updValue.GetJsonIetfVal()
	case *gnmi.TypedValue_JsonVal:
		jsondata = updValue.GetJsonVal()
	case *gnmi.TypedValue_LeaflistVal:
		value = updValue.GetLeaflistVal()
	case *gnmi.TypedValue_ProtoBytes:
		value = updValue.GetProtoBytes()
	case *gnmi.TypedValue_AnyVal:
		value = updValue.GetAnyVal()
	}
	if value == nil && len(jsondata) != 0 {
		err := json.Unmarshal(jsondata, &value)
		if err != nil {
			return nil, err
		}
	}
	return value, nil
}
