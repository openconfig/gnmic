package collector

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/gnxi/utils/xpath"
	"github.com/openconfig/gnmi/proto/gnmi"
)

// SubscriptionConfig //
type SubscriptionConfig struct {
	Name              string        `mapstructure:"name,omitempty"`
	Models            []string      `mapstructure:"models,omitempty"`
	Prefix            string        `mapstructure:"prefix,omitempty"`
	Target            string        `mapstructure:"target,omitempty"`
	Paths             []string      `mapstructure:"paths,omitempty"`
	Mode              string        `mapstructure:"mode,omitempty"`
	StreamMode        string        `mapstructure:"stream-mode,omitempty"`
	Encoding          string        `mapstructure:"encoding,omitempty"`
	Qos               uint32        `mapstructure:"qos,omitempty"`
	SampleInterval    time.Duration `mapstructure:"sample-interval,omitempty"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat-interval,omitempty"`
	SuppressRedundant bool          `mapstructure:"suppress-redundant,omitempty"`
	UpdatesOnly       bool          `mapstructure:"updates-only,omitempty"`
}

// String //
func (sc *SubscriptionConfig) String() string {
	b, err := json.Marshal(sc)
	if err != nil {
		return ""
	}
	return string(b)
}

func (sc *SubscriptionConfig) setDefaults() error {
	if len(sc.Paths) == 0 {
		return fmt.Errorf("missing path(s) in subscription '%s'", sc.Name)
	}
	if sc.Mode == "" {
		sc.Mode = "STREAM"
	}
	if sc.Mode == "STREAM" && sc.StreamMode == "" {
		sc.StreamMode = "TARGET_DEFINED"
	}
	if sc.Encoding == "" {
		sc.Encoding = "JSON"
	}
	if sc.Qos == 0 {
		sc.Qos = 20
	}
	if sc.StreamMode == "SAMPLE" && sc.SampleInterval == 0 {
		sc.SampleInterval = 10 * time.Second
	}
	return nil
}

// CreateSubscribeRequest validates the SubscriptionConfig and creates gnmi.SubscribeRequest
func (sc *SubscriptionConfig) CreateSubscribeRequest() (*gnmi.SubscribeRequest, error) {
	if err := sc.setDefaults(); err != nil {
		return nil, err
	}
	gnmiPrefix, err := CreatePrefix(sc.Prefix, sc.Target)
	if err != nil {
		return nil, fmt.Errorf("prefix parse error: %v", err)
	}
	encodingVal, ok := gnmi.Encoding_value[strings.Replace(strings.ToUpper(sc.Encoding), "-", "_", -1)]
	if !ok {
		return nil, fmt.Errorf("subscription '%s' invalid encoding type '%s'", sc.Name, sc.Encoding)
	}
	modeVal, ok := gnmi.SubscriptionList_Mode_value[strings.ToUpper(sc.Mode)]
	if !ok {
		return nil, fmt.Errorf("subscription '%s' invalid subscription list type '%s'", sc.Name, sc.Mode)
	}
	qos := &gnmi.QOSMarking{Marking: sc.Qos}

	subscriptions := make([]*gnmi.Subscription, len(sc.Paths))
	for i, p := range sc.Paths {
		gnmiPath, err := xpath.ToGNMIPath(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("path '%s' parse error: %v", p, err)
		}
		subscriptions[i] = &gnmi.Subscription{Path: gnmiPath}
		switch gnmi.SubscriptionList_Mode(modeVal) {
		case gnmi.SubscriptionList_STREAM:
			mode, ok := gnmi.SubscriptionMode_value[strings.Replace(strings.ToUpper(sc.StreamMode), "-", "_", -1)]
			if !ok {
				return nil, fmt.Errorf("invalid streamed subscription mode %s", sc.Mode)
			}
			subscriptions[i].Mode = gnmi.SubscriptionMode(mode)
			switch gnmi.SubscriptionMode(mode) {
			case gnmi.SubscriptionMode_ON_CHANGE:
				subscriptions[i].HeartbeatInterval = uint64(sc.HeartbeatInterval.Nanoseconds())
			case gnmi.SubscriptionMode_SAMPLE, gnmi.SubscriptionMode_TARGET_DEFINED:
				subscriptions[i].SampleInterval = uint64(sc.SampleInterval.Nanoseconds())
				subscriptions[i].SuppressRedundant = sc.SuppressRedundant
				if subscriptions[i].SuppressRedundant {
					subscriptions[i].HeartbeatInterval = uint64(sc.HeartbeatInterval.Nanoseconds())
				}
			}
		}
	}
	models := make([]*gnmi.ModelData, 0, len(sc.Models))
	for _, m := range sc.Models {
		models = append(models, &gnmi.ModelData{Name: m})
	}
	return &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: &gnmi.SubscriptionList{
				Prefix:       gnmiPrefix,
				Mode:         gnmi.SubscriptionList_Mode(modeVal),
				Encoding:     gnmi.Encoding(encodingVal),
				Subscription: subscriptions,
				Qos:          qos,
				UpdatesOnly:  sc.UpdatesOnly,
				UseModels:    models,
			},
		},
	}, nil
}

// SubscribeResponse //
type SubscribeResponse struct {
	SubscriptionName string
	Response         *gnmi.SubscribeResponse
}
