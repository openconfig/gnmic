package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

func (c *Config) GetSubscriptions() (map[string]*collector.SubscriptionConfig, error) {
	subscriptions := make(map[string]*collector.SubscriptionConfig)
	// hi := c.LocalFlags.SubscribeHeartbearInterval
	// si := c.LocalFlags.SubscribeSampleInteral
	var qos *uint32
	// qos value is set to nil by default to enable targets which don't support qos marking
	if c.LocalFlags.SubscribeQos != nil {
		qos = c.LocalFlags.SubscribeQos
	}

	subNames := viper.GetStringSlice("subscribe-name")
	if len(c.LocalFlags.SubscribePath) > 0 && len(c.LocalFlags.SubscribeName) > 0 {
		return nil, fmt.Errorf("flags --path and --name cannot be mixed")
	}
	if len(c.LocalFlags.SubscribePath) > 0 {
		sub := new(collector.SubscriptionConfig)
		sub.Name = fmt.Sprintf("default-%d", time.Now().Unix())
		sub.Paths = c.LocalFlags.SubscribePath
		sub.Prefix = viper.GetString("subscribe-prefix")
		sub.Target = viper.GetString("subscribe-target")
		sub.Mode = viper.GetString("subscribe-mode")
		sub.Encoding = viper.GetString("encoding")
		sub.Qos = qos
		sub.StreamMode = viper.GetString("subscribe-stream-mode")
		sub.HeartbeatInterval = c.LocalFlags.SubscribeHeartbearInterval
		sub.SampleInterval = c.LocalFlags.SubscribeSampleInteral
		sub.SuppressRedundant = viper.GetBool("subscribe-suppress-redundant")
		sub.UpdatesOnly = viper.GetBool("subscribe-updates-only")
		sub.Models = viper.GetStringSlice("models")
		subscriptions["default"] = sub
		return subscriptions, nil
	}
	subDef := viper.GetStringMap("subscriptions")
	if viper.GetBool("debug") {
		c.logger.Printf("subscription map: %v+", subDef)
	}
	for sn, s := range subDef {
		sub := new(collector.SubscriptionConfig)
		decoder, err := mapstructure.NewDecoder(
			&mapstructure.DecoderConfig{
				DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
				Result:     sub,
			})
		if err != nil {
			return nil, err
		}
		err = decoder.Decode(s)
		if err != nil {
			return nil, err
		}
		sub.Name = sn

		// inherit global "subscribe-*" option if it's not set
		setSubscriptionDefaults(sub)
		subscriptions[sn] = sub
	}
	if len(subNames) == 0 {
		return subscriptions, nil
	}
	filteredSubscriptions := make(map[string]*collector.SubscriptionConfig)
	notFound := make([]string, 0)
	for _, name := range subNames {
		if s, ok := subscriptions[name]; ok {
			filteredSubscriptions[name] = s
		} else {
			notFound = append(notFound, name)
		}
	}
	if len(notFound) > 0 {
		return nil, fmt.Errorf("named subscription(s) not found in config file: %v", notFound)
	}
	return filteredSubscriptions, nil
}

func setSubscriptionDefaults(sub *collector.SubscriptionConfig) {
	hi := viper.GetDuration("subscribe-heartbeat-interval")
	si := viper.GetDuration("subscribe-sample-interval")
	if sub.SampleInterval == nil {
		sub.SampleInterval = &si
	}
	if sub.HeartbeatInterval == nil {
		sub.HeartbeatInterval = &hi
	}
	if sub.Encoding == "" {
		sub.Encoding = viper.GetString("encoding")
	}
	if sub.Mode == "" {
		sub.Mode = viper.GetString("subscribe-mode")
	}
	if strings.ToUpper(sub.Mode) == "STREAM" && sub.StreamMode == "" {
		sub.StreamMode = viper.GetString("subscribe-stream-mode")
	}
	if sub.Qos == nil {
		if viper.IsSet("subscribe-qos") {
			q := viper.GetUint32("subscribe-qos")
			sub.Qos = &q
		}
	}
}
