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
	"strconv"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"

	"github.com/openconfig/gnmic/pkg/api"
	"github.com/openconfig/gnmic/pkg/types"
)

const (
	subscriptionDefaultMode       = "STREAM"
	subscriptionDefaultStreamMode = "TARGET_DEFINED"
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
	if flagIsSet(cmd, "depth") {
		depth, err := strconv.ParseUint(c.LocalFlags.SubscribeDepth, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("depth: %v", err)
		}
		temp := uint32(depth)
		sub.Depth = &temp
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
	if strings.ToUpper(sub.Mode) == "STREAM" && sub.StreamMode == "" {
		sub.StreamMode = c.LocalFlags.SubscribeStreamMode
	}
	if sub.Qos == nil && flagIsSet(cmd, "qos") {
		sub.Qos = &c.LocalFlags.SubscribeQos
	}
	if sub.History == nil && flagIsSet(cmd, "history-snapshot") {
		snapshot, err := time.Parse(time.RFC3339Nano, c.LocalFlags.SubscribeHistorySnapshot)
		if err != nil {
			return fmt.Errorf("history-snapshot: %v", err)
		}
		sub.History = &types.HistoryConfig{
			Snapshot: snapshot,
		}
		return nil
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
		return nil
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

func (c *Config) CreateSubscribeRequest(sc *types.SubscriptionConfig, tc *types.TargetConfig) (*gnmi.SubscribeRequest, error) {
	err := validateAndSetDefaults(sc)
	if err != nil {
		return nil, err
	}
	gnmiOpts, err := c.subscriptionOpts(sc, tc)
	if err != nil {
		return nil, err
	}
	return api.NewSubscribeRequest(gnmiOpts...)
}

func (c *Config) subscriptionOpts(sc *types.SubscriptionConfig, tc *types.TargetConfig) ([]api.GNMIOption, error) {
	gnmiOpts := make([]api.GNMIOption, 0, 4)

	gnmiOpts = append(gnmiOpts,
		api.Prefix(sc.Prefix),
		api.SubscriptionListMode(sc.Mode),
		api.UpdatesOnly(sc.UpdatesOnly),
	)
	// encoding
	switch {
	case sc.Encoding != nil:
		gnmiOpts = append(gnmiOpts, api.Encoding(*sc.Encoding))
	case tc != nil && tc.Encoding != nil:
		gnmiOpts = append(gnmiOpts, api.Encoding(*tc.Encoding))
	default:
		gnmiOpts = append(gnmiOpts, api.Encoding(c.Encoding))
	}

	// history extension
	if sc.History != nil {
		if !sc.History.Snapshot.IsZero() {
			gnmiOpts = append(gnmiOpts, api.Extension_HistorySnapshotTime(sc.History.Snapshot))
		}
		if !sc.History.Start.IsZero() && !sc.History.End.IsZero() {
			gnmiOpts = append(gnmiOpts, api.Extension_HistoryRange(sc.History.Start, sc.History.End))
		}
	}

	// depth extension
	if sc.Depth != nil {
		gnmiOpts = append(gnmiOpts, api.Extension_DepthLevel(*sc.Depth))
	}

	// QoS
	if sc.Qos != nil {
		gnmiOpts = append(gnmiOpts, api.Qos(*sc.Qos))
	}

	// add models
	for _, m := range sc.Models {
		gnmiOpts = append(gnmiOpts, api.UseModel(m, "", ""))
	}

	// add target opt
	if sc.Target != "" {
		gnmiOpts = append(gnmiOpts, api.Target(sc.Target))
	} else if sc.SetTarget {
		gnmiOpts = append(gnmiOpts, api.Target(tc.Name))
	}
	// add gNMI subscriptions
	// multiple stream subscriptions
	if len(sc.StreamSubscriptions) > 0 {
		for _, ssc := range sc.StreamSubscriptions {
			streamGNMIOpts, err := streamSubscriptionOpts(ssc)
			if err != nil {
				return nil, err
			}
			gnmiOpts = append(gnmiOpts, streamGNMIOpts...)
		}
	}

	for _, p := range sc.Paths {
		subGnmiOpts := make([]api.GNMIOption, 0, 2)
		switch gnmi.SubscriptionList_Mode(gnmi.SubscriptionList_Mode_value[strings.ToUpper(sc.Mode)]) {
		case gnmi.SubscriptionList_STREAM:
			switch gnmi.SubscriptionMode(gnmi.SubscriptionMode_value[strings.Replace(strings.ToUpper(sc.StreamMode), "-", "_", -1)]) {
			case gnmi.SubscriptionMode_ON_CHANGE:
				if sc.HeartbeatInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
				}
				subGnmiOpts = append(subGnmiOpts, api.SubscriptionMode(sc.StreamMode))
			case gnmi.SubscriptionMode_SAMPLE, gnmi.SubscriptionMode_TARGET_DEFINED:
				if sc.SampleInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.SampleInterval(*sc.SampleInterval))
				}
				subGnmiOpts = append(subGnmiOpts, api.SuppressRedundant(sc.SuppressRedundant))
				if sc.SuppressRedundant && sc.HeartbeatInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
				}
				subGnmiOpts = append(subGnmiOpts, api.SubscriptionMode(sc.StreamMode))
			default:
				return nil, fmt.Errorf("%w: subscription %s unknown stream subscription mode %s", ErrConfig, sc.Name, sc.StreamMode)
			}
		default:
			// poll and once subscription modes
		}
		//
		subGnmiOpts = append(subGnmiOpts, api.Path(p))
		gnmiOpts = append(gnmiOpts,
			api.Subscription(subGnmiOpts...),
		)
	}

	return gnmiOpts, nil
}

func streamSubscriptionOpts(sc *types.SubscriptionConfig) ([]api.GNMIOption, error) {
	gnmiOpts := make([]api.GNMIOption, 0)
	for _, p := range sc.Paths {
		subGnmiOpts := make([]api.GNMIOption, 0, 2)
		switch gnmi.SubscriptionMode(gnmi.SubscriptionMode_value[strings.Replace(strings.ToUpper(sc.StreamMode), "-", "_", -1)]) {
		case gnmi.SubscriptionMode_ON_CHANGE:
			if sc.HeartbeatInterval != nil {
				subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
			}
			subGnmiOpts = append(subGnmiOpts, api.SubscriptionMode(sc.StreamMode))
		case gnmi.SubscriptionMode_SAMPLE, gnmi.SubscriptionMode_TARGET_DEFINED:
			if sc.SampleInterval != nil {
				subGnmiOpts = append(subGnmiOpts, api.SampleInterval(*sc.SampleInterval))
			}
			subGnmiOpts = append(subGnmiOpts, api.SuppressRedundant(sc.SuppressRedundant))
			if sc.SuppressRedundant && sc.HeartbeatInterval != nil {
				subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
			}
			subGnmiOpts = append(subGnmiOpts, api.SubscriptionMode(sc.StreamMode))
		default:
			return nil, fmt.Errorf("%w: subscription %s unknown stream subscription mode %s", ErrConfig, sc.Name, sc.StreamMode)
		}

		subGnmiOpts = append(subGnmiOpts, api.Path(p))
		gnmiOpts = append(gnmiOpts,
			api.Subscription(subGnmiOpts...),
		)
	}
	return gnmiOpts, nil
}

func validateAndSetDefaults(sc *types.SubscriptionConfig) error {
	numPaths := len(sc.Paths)
	numStreamSubs := len(sc.StreamSubscriptions)
	if sc.Prefix == "" && numPaths == 0 && numStreamSubs == 0 {
		return fmt.Errorf("%w: missing path(s) in subscription %q", ErrConfig, sc.Name)
	}

	if numPaths > 0 && numStreamSubs > 0 {
		return fmt.Errorf("%w: subscription %q: cannot set 'paths' and 'stream-subscriptions' at the same time", ErrConfig, sc.Name)
	}

	// validate subscription Mode
	switch strings.ToUpper(sc.Mode) {
	case "":
		sc.Mode = subscriptionDefaultMode
	case "ONCE", "POLL":
		if numStreamSubs > 0 {
			return fmt.Errorf("%w: subscription %q: cannot set 'stream-subscriptions' and 'mode'", ErrConfig, sc.Name)
		}
	case "STREAM":
	default:
		return fmt.Errorf("%w: subscription %s: unknown subscription mode %q", ErrConfig, sc.Name, sc.Mode)
	}
	// validate encoding
	if sc.Encoding != nil {
		switch strings.ToUpper(strings.ReplaceAll(*sc.Encoding, "-", "_")) {
		case "":
			sc.Encoding = pointer.ToString(subscriptionDefaultEncoding)
		case "JSON":
		case "BYTES":
		case "PROTO":
		case "ASCII":
		case "JSON_IETF":
		default:
			// allow integer encoding values
			_, err := strconv.Atoi(*sc.Encoding)
			if err != nil {
				return fmt.Errorf("%w: subscription %s: unknown encoding type %q", ErrConfig, sc.Name, *sc.Encoding)
			}
		}
	}

	// validate subscription stream mode
	if strings.ToUpper(sc.Mode) == "STREAM" {
		if len(sc.StreamSubscriptions) == 0 {
			switch strings.ToUpper(strings.ReplaceAll(sc.StreamMode, "-", "_")) {
			case "":
				sc.StreamMode = subscriptionDefaultStreamMode
			case "TARGET_DEFINED":
			case "SAMPLE":
			case "ON_CHANGE":
			default:
				return fmt.Errorf("%w: subscription %s: unknown stream-mode type %q", ErrConfig, sc.Name, sc.StreamMode)
			}
			return nil
		}

		// stream subscriptions
		for i, scs := range sc.StreamSubscriptions {
			if scs.Mode != "" {
				return fmt.Errorf("%w: subscription %s/%d: 'mode' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.Prefix != "" {
				return fmt.Errorf("%w: subscription %s/%d: 'prefix' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.Target != "" {
				return fmt.Errorf("%w: subscription %s/%d: 'target' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.SetTarget {
				return fmt.Errorf("%w: subscription %s/%d: 'set-target' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.Encoding != nil {
				return fmt.Errorf("%w: subscription %s/%d: 'encoding' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.History != nil {
				return fmt.Errorf("%w: subscription %s/%d: 'history' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.Models != nil {
				return fmt.Errorf("%w: subscription %s/%d: 'models' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.UpdatesOnly {
				return fmt.Errorf("%w: subscription %s/%d: 'updates-only' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.StreamSubscriptions != nil {
				return fmt.Errorf("%w: subscription %s/%d: 'subscriptions' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.Qos != nil {
				return fmt.Errorf("%w: subscription %s/%d: 'qos' attribute cannot be set", ErrConfig, sc.Name, i)
			}

			switch strings.ReplaceAll(strings.ToUpper(scs.StreamMode), "-", "_") {
			case "":
				scs.StreamMode = subscriptionDefaultStreamMode
			case "TARGET_DEFINED":
			case "SAMPLE":
			case "ON_CHANGE":
			default:
				return fmt.Errorf("%w: subscription %s/%d: unknown subscription stream mode %q", ErrConfig, sc.Name, i, scs.StreamMode)
			}
		}
	}
	return nil
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
