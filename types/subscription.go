package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
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
