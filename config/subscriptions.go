package config

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
)

func (c *Config) GetSubscriptions(cmd *cobra.Command) (map[string]*collector.SubscriptionConfig, error) {
	if len(c.LocalFlags.SubscribePath) > 0 && len(c.LocalFlags.SubscribeName) > 0 {
		return nil, fmt.Errorf("flags --path and --name cannot be mixed")
	}
	if len(c.LocalFlags.SubscribePath) > 0 {
		sub := new(collector.SubscriptionConfig)
		sub.Name = fmt.Sprintf("default-%d", time.Now().Unix())
		sub.Paths = c.LocalFlags.SubscribePath
		sub.Prefix = c.LocalFlags.SubscribePrefix
		sub.Target = c.LocalFlags.SubscribeTarget
		sub.Mode = c.LocalFlags.SubscribeMode
		sub.Encoding = c.Encoding
		if flagIsSet(cmd, "qos") {
			sub.Qos = &c.LocalFlags.SubscribeQos
		}
		sub.StreamMode = c.LocalFlags.SubscribeStreamMode
		if flagIsSet(cmd, "heartbeat-interval") {
			sub.HeartbeatInterval = &c.LocalFlags.SubscribeHeartbearInterval
		}
		if flagIsSet(cmd, "sample-interval") {
			sub.SampleInterval = &c.LocalFlags.SubscribeSampleInterval
		}
		sub.SuppressRedundant = c.LocalFlags.SubscribeSuppressRedundant
		sub.UpdatesOnly = c.LocalFlags.SubscribeUpdatesOnly
		sub.Models = c.LocalFlags.SubscribeModel
		c.Subscriptions[sub.Name] = sub
		if c.Debug {
			c.logger.Printf("subscriptions: %s", c.Subscriptions)
		}
		return c.Subscriptions, nil
	}
	subDef := c.FileConfig.GetStringMap("subscriptions")
	if c.Debug {
		c.logger.Printf("subscriptions map: %v+", subDef)
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
		c.setSubscriptionDefaults(sub, cmd)
		c.Subscriptions[sn] = sub
	}
	if len(c.LocalFlags.SubscribeName) == 0 {
		if c.Debug {
			c.logger.Printf("subscriptions: %s", c.Subscriptions)
		}
		return c.Subscriptions, nil
	}
	filteredSubscriptions := make(map[string]*collector.SubscriptionConfig)
	notFound := make([]string, 0)
	for _, name := range c.LocalFlags.SubscribeName {
		if s, ok := c.Subscriptions[name]; ok {
			filteredSubscriptions[name] = s
		} else {
			notFound = append(notFound, name)
		}
	}
	if len(notFound) > 0 {
		return nil, fmt.Errorf("named subscription(s) not found in config file: %v", notFound)
	}
	if c.Debug {
		c.logger.Printf("subscriptions: %s", filteredSubscriptions)
	}
	return filteredSubscriptions, nil
}

func (c *Config) setSubscriptionDefaults(sub *collector.SubscriptionConfig, cmd *cobra.Command) {
	if sub.SampleInterval == nil {
		if flagIsSet(cmd, "sample-interval") {
			sub.SampleInterval = &c.LocalFlags.SubscribeSampleInterval
		}
	}
	if sub.HeartbeatInterval == nil {
		if flagIsSet(cmd, "heartbeat-interval") {
			sub.HeartbeatInterval = &c.LocalFlags.SubscribeHeartbearInterval
		}
	}
	if sub.Encoding == "" {
		sub.Encoding = c.Encoding
	}
	if sub.Mode == "" {
		sub.Mode = c.LocalFlags.SubscribeMode
	}
	if strings.ToUpper(sub.Mode) == "STREAM" && sub.StreamMode == "" {
		sub.StreamMode = c.LocalFlags.SubscribeStreamMode
	}
	if sub.Qos == nil {
		if flagIsSet(cmd, "qos") {
			sub.Qos = &c.LocalFlags.SubscribeQos
		}
	}
}

func (c *Config) GetSubscriptionsFromFile() []*collector.SubscriptionConfig {
	subs, err := c.GetSubscriptions(nil)
	if err != nil {
		return nil
	}
	subscriptions := make([]*collector.SubscriptionConfig, 0)
	for _, sub := range subs {
		subscriptions = append(subscriptions, sub)
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		return subscriptions[i].Name < subscriptions[j].Name
	})
	return subscriptions
}
