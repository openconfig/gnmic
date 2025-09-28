// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"

	"github.com/openconfig/gnmic/pkg/api/types"
)

const (
	SubscriptionMode_STREAM               = "STREAM"
	SubscriptionMode_ONCE                 = "ONCE"
	SubscriptionMode_POLL                 = "POLL"
	SubscriptionStreamMode_TARGET_DEFINED = "TARGET_DEFINED"
	SubscriptionStreamMode_ON_CHANGE      = "ON_CHANGE"
	SubscriptionStreamMode_SAMPLE         = "SAMPLE"
)
const (
	subscriptionDefaultMode       = SubscriptionMode_STREAM
	subscriptionDefaultStreamMode = SubscriptionStreamMode_TARGET_DEFINED
	subscriptionDefaultEncoding   = "JSON"
)

var ErrConfig = errors.New("config error")

func (c *Config) GetSubscriptions(cmd *cobra.Command) (map[string]*types.SubscriptionConfig, error) {
	if len(c.LocalFlags.SubscribePath) > 0 && len(c.LocalFlags.SubscribeName) > 0 {
		return nil, fmt.Errorf("flags --path and --name cannot be mixed")
	}
	// subscriptions from cli flags
	if len(c.LocalFlags.SubscribePath) > 0 {
		return c.subscriptionConfigFromFlags(cmd)
	}
	// subscriptions from file
	subDef := c.FileConfig.GetStringMap("subscriptions")
	if c.Debug {
		c.logger.Printf("subscriptions map: %#v", subDef)
	}
	// decode subscription config
	for sn, s := range subDef {
		switch s := s.(type) {
		case map[string]any:
			sub, err := c.decodeSubscriptionConfig(sn, s, cmd)
			if err != nil {
				return nil, err
			}
			c.Subscriptions[sn] = sub
		default:
			return nil, fmt.Errorf("%w: subscriptions map: unexpected type %T", ErrConfig, s)
		}
	}

	// named subscription
	if len(c.LocalFlags.SubscribeName) == 0 {
		if c.Debug {
			c.logger.Printf("subscriptions: %s", c.Subscriptions)
		}
		err := validateSubscriptionsConfig(c.Subscriptions)
		if err != nil {
			return nil, err
		}
		return c.Subscriptions, nil
	}
	filteredSubscriptions := make(map[string]*types.SubscriptionConfig)
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
	err := validateSubscriptionsConfig(filteredSubscriptions)
	if err != nil {
		return nil, err
	}
	return filteredSubscriptions, nil
}

func (c *Config) subscriptionConfigFromFlags(cmd *cobra.Command) (map[string]*types.SubscriptionConfig, error) {
	sub := &types.SubscriptionConfig{
		Name:      fmt.Sprintf("default-%d", time.Now().Unix()),
		Models:    []string{},
		Prefix:    c.LocalFlags.SubscribePrefix,
		Target:    c.LocalFlags.SubscribeTarget,
		SetTarget: c.LocalFlags.SubscribeSetTarget,
		Paths:     c.LocalFlags.SubscribePath,
		Mode:      c.LocalFlags.SubscribeMode,
		Depth:     c.LocalFlags.SubscribeDepth,
	}
	// if globalFlagIsSet(cmd, "encoding") {
	// 	sub.Encoding = &c.Encoding
	// }
	if flagIsSet(cmd, "qos") {
		sub.Qos = &c.LocalFlags.SubscribeQos
	}
	sub.StreamMode = c.LocalFlags.SubscribeStreamMode
	if flagIsSet(cmd, "heartbeat-interval") {
		sub.HeartbeatInterval = &c.LocalFlags.SubscribeHeartbeatInterval
	}
	if flagIsSet(cmd, "sample-interval") {
		sub.SampleInterval = &c.LocalFlags.SubscribeSampleInterval
	}
	sub.SuppressRedundant = c.LocalFlags.SubscribeSuppressRedundant
	sub.UpdatesOnly = c.LocalFlags.SubscribeUpdatesOnly
	sub.Models = c.LocalFlags.SubscribeModel
	if flagIsSet(cmd, "history-snapshot") {
		snapshot, err := time.Parse(time.RFC3339Nano, c.LocalFlags.SubscribeHistorySnapshot)
		if err != nil {
			return nil, fmt.Errorf("history-snapshot: %v", err)
		}
		sub.History = &types.HistoryConfig{
			Snapshot: snapshot,
		}
	}
	if flagIsSet(cmd, "history-start") && flagIsSet(cmd, "history-end") {
		start, err := time.Parse(time.RFC3339Nano, c.LocalFlags.SubscribeHistoryStart)
		if err != nil {
			return nil, fmt.Errorf("history-start: %v", err)
		}
		end, err := time.Parse(time.RFC3339Nano, c.LocalFlags.SubscribeHistoryEnd)
		if err != nil {
			return nil, fmt.Errorf("history-end: %v", err)
		}
		sub.History = &types.HistoryConfig{
			Start: start,
			End:   end,
		}
	}
	c.Subscriptions[sub.Name] = sub
	if c.Debug {
		c.logger.Printf("subscriptions: %s", c.Subscriptions)
	}
	return c.Subscriptions, nil
}

func (c *Config) decodeSubscriptionConfig(sn string, s any, cmd *cobra.Command) (*types.SubscriptionConfig, error) {
	sub := new(types.SubscriptionConfig)
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
	if err := c.setSubscriptionFieldsFromFlags(sub, cmd); err != nil {
		return nil, err
	}
	expandSubscriptionEnv(sub)
	return sub, nil
}

func (c *Config) setSubscriptionFieldsFromFlags(sub *types.SubscriptionConfig, cmd *cobra.Command) error {
	if sub.SampleInterval == nil && flagIsSet(cmd, "sample-interval") {
		sub.SampleInterval = &c.LocalFlags.SubscribeSampleInterval
	}
	if sub.HeartbeatInterval == nil && flagIsSet(cmd, "heartbeat-interval") {
		sub.HeartbeatInterval = &c.LocalFlags.SubscribeHeartbeatInterval
	}
	// if sub.Encoding == nil && globalFlagIsSet(cmd, "encoding") {
	// 	sub.Encoding = &c.Encoding
	// }
	if sub.Mode == "" {
		sub.Mode = c.LocalFlags.SubscribeMode
	}
	if strings.ToUpper(sub.Mode) == SubscriptionMode_STREAM && sub.StreamMode == "" {
		sub.StreamMode = c.LocalFlags.SubscribeStreamMode
	}
	if sub.Qos == nil && flagIsSet(cmd, "qos") {
		sub.Qos = &c.LocalFlags.SubscribeQos
	}
	if flagIsSet(cmd, "depth") {
		sub.Depth = c.LocalFlags.SubscribeDepth
	}
	if sub.History == nil && flagIsSet(cmd, "history-snapshot") {
		snapshot, err := time.Parse(time.RFC3339Nano, c.LocalFlags.SubscribeHistorySnapshot)
		if err != nil {
			return fmt.Errorf("history-snapshot: %v", err)
		}
		sub.History = &types.HistoryConfig{
			Snapshot: snapshot,
		}
	}
	if sub.History == nil && flagIsSet(cmd, "history-start") && flagIsSet(cmd, "history-end") {
		start, err := time.Parse(time.RFC3339Nano, c.LocalFlags.SubscribeHistoryStart)
		if err != nil {
			return fmt.Errorf("history-start: %v", err)
		}
		end, err := time.Parse(time.RFC3339Nano, c.LocalFlags.SubscribeHistoryEnd)
		if err != nil {
			return fmt.Errorf("history-end: %v", err)
		}
		sub.History = &types.HistoryConfig{
			Start: start,
			End:   end,
		}
	}
	return nil
}

func (c *Config) GetSubscriptionsFromFile() []*types.SubscriptionConfig {
	subs, err := c.GetSubscriptions(nil)
	if err != nil {
		return nil
	}
	subscriptions := make([]*types.SubscriptionConfig, 0)
	for _, sub := range subs {
		subscriptions = append(subscriptions, sub)
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		return subscriptions[i].Name < subscriptions[j].Name
	})
	return subscriptions
}

func validateSubscriptionsConfig(subs map[string]*types.SubscriptionConfig) error {
	var hasPoll bool
	var hasOnce bool
	var hasStream bool
	for _, sc := range subs {
		switch strings.ToUpper(sc.Mode) {
		case "POLL":
			hasPoll = true
		case "ONCE":
			hasOnce = true
		case "STREAM":
			hasStream = true
		}
	}
	if hasPoll && hasOnce || hasPoll && hasStream {
		return errors.New("subscriptions with mode Poll cannot be mixed with Stream or Once")
	}
	return nil
}

func expandSubscriptionEnv(sc *types.SubscriptionConfig) {
	sc.Name = os.ExpandEnv(sc.Name)
	for i := range sc.Models {
		sc.Models[i] = os.ExpandEnv(sc.Models[i])
	}
	sc.Prefix = os.ExpandEnv(sc.Prefix)
	sc.Target = os.ExpandEnv(sc.Target)
	for i := range sc.Paths {
		sc.Paths[i] = os.ExpandEnv(sc.Paths[i])
	}
	sc.Mode = os.ExpandEnv(sc.Mode)
	sc.StreamMode = os.ExpandEnv(sc.StreamMode)
	if sc.Encoding != nil {
		sc.Encoding = pointer.ToString(os.ExpandEnv(*sc.Encoding))
	}
}
