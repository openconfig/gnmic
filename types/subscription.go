package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/proto/gnmi"
)

const (
	subscriptionDefaultMode       = "STREAM"
	subscriptionDefaultStreamMode = "TARGET_DEFINED"
	subscriptionDefaultEncoding   = "JSON"
)

// SubscriptionConfig //
type SubscriptionConfig struct {
	Name              string         `mapstructure:"name,omitempty" json:"name,omitempty"`
	Models            []string       `mapstructure:"models,omitempty" json:"models,omitempty"`
	Prefix            string         `mapstructure:"prefix,omitempty" json:"prefix,omitempty"`
	Target            string         `mapstructure:"target,omitempty" json:"target,omitempty"`
	SetTarget         bool           `mapstructure:"set-target,omitempty" json:"set-target,omitempty"`
	Paths             []string       `mapstructure:"paths,omitempty" json:"paths,omitempty"`
	Mode              string         `mapstructure:"mode,omitempty" json:"mode,omitempty"`
	StreamMode        string         `mapstructure:"stream-mode,omitempty" json:"stream-mode,omitempty"`
	Encoding          string         `mapstructure:"encoding,omitempty" json:"encoding,omitempty"`
	Qos               *uint32        `mapstructure:"qos,omitempty" json:"qos,omitempty"`
	SampleInterval    *time.Duration `mapstructure:"sample-interval,omitempty" json:"sample-interval,omitempty"`
	HeartbeatInterval *time.Duration `mapstructure:"heartbeat-interval,omitempty" json:"heartbeat-interval,omitempty"`
	SuppressRedundant bool           `mapstructure:"suppress-redundant,omitempty" json:"suppress-redundant,omitempty"`
	UpdatesOnly       bool           `mapstructure:"updates-only,omitempty" json:"updates-only,omitempty"`
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
		sc.Mode = subscriptionDefaultMode
	}
	if strings.ToUpper(sc.Mode) == "STREAM" && sc.StreamMode == "" {
		sc.StreamMode = subscriptionDefaultStreamMode
	}
	if sc.Encoding == "" {
		sc.Encoding = subscriptionDefaultEncoding
	}
	return nil
}

// CreateSubscribeRequest validates the SubscriptionConfig and creates gnmi.SubscribeRequest
func (sc *SubscriptionConfig) CreateSubscribeRequest(target string) (*gnmi.SubscribeRequest, error) {
	if err := sc.setDefaults(); err != nil {
		return nil, err
	}
	gnmiPrefix, err := sc.createPrefix(target)
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
	var qos *gnmi.QOSMarking
	if sc.Qos != nil {
		qos = &gnmi.QOSMarking{Marking: *sc.Qos}
	}

	subscriptions := make([]*gnmi.Subscription, len(sc.Paths))
	for i, p := range sc.Paths {
		gnmiPath, err := utils.ParsePath(strings.TrimSpace(p))
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
				if sc.HeartbeatInterval != nil {
					subscriptions[i].HeartbeatInterval = uint64(sc.HeartbeatInterval.Nanoseconds())
				}
			case gnmi.SubscriptionMode_SAMPLE, gnmi.SubscriptionMode_TARGET_DEFINED:
				if sc.SampleInterval != nil {
					subscriptions[i].SampleInterval = uint64(sc.SampleInterval.Nanoseconds())
				}
				subscriptions[i].SuppressRedundant = sc.SuppressRedundant
				if subscriptions[i].SuppressRedundant && sc.HeartbeatInterval != nil {
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

func (sc *SubscriptionConfig) createPrefix(target string) (*gnmi.Path, error) {
	if sc.Target != "" {
		return utils.CreatePrefix(sc.Prefix, sc.Target)
	}
	if sc.SetTarget {
		return utils.CreatePrefix(sc.Prefix, target)
	}
	return utils.CreatePrefix(sc.Prefix, "")
}

// SubscribeResponse //
type SubscribeResponse struct {
	SubscriptionName   string
	SubscriptionConfig *SubscriptionConfig
	Response           *gnmi.SubscribeResponse
}

func (sc *SubscriptionConfig) PathsString() string {
	return fmt.Sprintf("- %s", strings.Join(sc.Paths, "\n- "))
}

func (sc *SubscriptionConfig) PrefixString() string {
	if sc.Prefix == "" {
		return "NA"
	}
	return sc.Prefix
}

func (sc *SubscriptionConfig) ModeString() string {
	if strings.ToLower(sc.Mode) == "stream" {
		return fmt.Sprintf("%s/%s", strings.ToLower(sc.Mode), strings.ToLower(sc.StreamMode))
	}
	return strings.ToLower(sc.Mode)
}

func (sc *SubscriptionConfig) SampleIntervalString() string {
	if strings.ToLower(sc.Mode) == "stream" && strings.ToLower(sc.StreamMode) == "sample" {
		return sc.SampleInterval.String()
	}
	return "NA"
}

func (sc *SubscriptionConfig) ModelsString() string {
	return fmt.Sprintf("- %s", strings.Join(sc.Models, "\n- "))
}

func (sc *SubscriptionConfig) QosString() string {
	if sc.Qos == nil {
		return "NA"
	}
	return fmt.Sprintf("%d", *sc.Qos)
}

func (sc *SubscriptionConfig) HeartbeatIntervalString() string {
	return sc.HeartbeatInterval.String()
}

func (sc *SubscriptionConfig) SuppressRedundantString() string {
	return fmt.Sprintf("%t", sc.SuppressRedundant)
}

func (sc *SubscriptionConfig) UpdatesOnlyString() string {
	return fmt.Sprintf("%t", sc.UpdatesOnly)
}
