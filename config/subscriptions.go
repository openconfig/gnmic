package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/karimra/gnmic/api"
	"github.com/karimra/gnmic/types"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
)

const (
	subscriptionDefaultMode       = "STREAM"
	subscriptionDefaultStreamMode = "TARGET_DEFINED"
	subscriptionDefaultEncoding   = "JSON"
)

func (c *Config) GetSubscriptions(cmd *cobra.Command) (map[string]*types.SubscriptionConfig, error) {
	if len(c.LocalFlags.SubscribePath) > 0 && len(c.LocalFlags.SubscribeName) > 0 {
		return nil, fmt.Errorf("flags --path and --name cannot be mixed")
	}
	// subscriptions from cli flags
	if len(c.LocalFlags.SubscribePath) > 0 {
		sub := new(types.SubscriptionConfig)
		sub.Name = fmt.Sprintf("default-%d", time.Now().Unix())
		sub.Paths = c.LocalFlags.SubscribePath
		sub.Prefix = c.LocalFlags.SubscribePrefix
		sub.Target = c.LocalFlags.SubscribeTarget
		sub.SetTarget = c.LocalFlags.SubscribeSetTarget
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
		if flagIsSet(cmd, "history-snapshot") {
			sub.History = &types.HistoryConfig{
				Snapshot: c.LocalFlags.SubscribeHistorySnapshot,
			}
		}
		if flagIsSet(cmd, "history-start") && flagIsSet(cmd, "history-end") {
			sub.History = &types.HistoryConfig{
				Start: c.LocalFlags.SubscribeHistoryStart,
				End:   c.LocalFlags.SubscribeHistoryEnd,
			}
		}
		c.Subscriptions[sub.Name] = sub
		if c.Debug {
			c.logger.Printf("subscriptions: %s", c.Subscriptions)
		}
		return c.Subscriptions, nil
	}
	// subscriptions from file
	subDef := c.FileConfig.GetStringMap("subscriptions")
	if c.Debug {
		c.logger.Printf("subscriptions map: %v+", subDef)
	}
	for sn, s := range subDef {
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
		c.setSubscriptionDefaults(sub, cmd)
		expandSubscriptionEnv(sub)
		c.Subscriptions[sn] = sub
	}
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

func (c *Config) setSubscriptionDefaults(sub *types.SubscriptionConfig, cmd *cobra.Command) {
	if sub.SampleInterval == nil && flagIsSet(cmd, "sample-interval") {
		sub.SampleInterval = &c.LocalFlags.SubscribeSampleInterval
	}
	if sub.HeartbeatInterval == nil && flagIsSet(cmd, "heartbeat-interval") {
		sub.HeartbeatInterval = &c.LocalFlags.SubscribeHeartbearInterval
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
	if sub.Qos == nil && flagIsSet(cmd, "qos") {
		sub.Qos = &c.LocalFlags.SubscribeQos
	}
	if sub.History == nil && flagIsSet(cmd, "history-snapshot") {
		sub.History = &types.HistoryConfig{
			Snapshot: c.LocalFlags.SubscribeHistorySnapshot,
		}
		return
	}
	if sub.History == nil && flagIsSet(cmd, "history-start") && flagIsSet(cmd, "history-end") {
		sub.History = &types.HistoryConfig{
			Start: c.LocalFlags.SubscribeHistoryStart,
			End:   c.LocalFlags.SubscribeHistoryEnd,
		}
		return
	}
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

func (*Config) CreateSubscribeRequest(sc *types.SubscriptionConfig, target string) (*gnmi.SubscribeRequest, error) {
	err := setDefaults(sc)
	if err != nil {
		return nil, err
	}
	gnmiOpts := make([]api.GNMIOption, 0)
	gnmiOpts = append(gnmiOpts,
		api.Prefix(sc.Prefix),
		api.Encoding(sc.Encoding),
		api.SubscriptionListMode(sc.Mode),
		api.UpdatesOnly(sc.UpdatesOnly),
	)
	// history extension
	if sc.History != nil {
		if sc.History.Snapshot != "" {
			gnmiOpts = append(gnmiOpts, api.Extension_HistorySnapshotTime(sc.History.Snapshot))
		}
		if sc.History.Start != "" && sc.History.End != "" {
			gnmiOpts = append(gnmiOpts, api.Extension_HistoryRange(sc.History.Start, sc.History.End))
		}
	}
	if sc.Qos != nil {
		gnmiOpts = append(gnmiOpts, api.Qos(*sc.Qos))
	}
	// add target opt
	if sc.Target != "" {
		gnmiOpts = append(gnmiOpts, api.Target(sc.Target))
	} else if sc.SetTarget {
		gnmiOpts = append(gnmiOpts, api.Target(target))
	}
	// add gNMI subscriptions
	for _, p := range sc.Paths {
		subGnmiOpts := make([]api.GNMIOption, 0, 2)
		switch gnmi.SubscriptionList_Mode(gnmi.SubscriptionList_Mode_value[strings.ToUpper(sc.Mode)]) {
		case gnmi.SubscriptionList_STREAM:
			subGnmiOpts = append(subGnmiOpts, api.SubscriptionMode(sc.StreamMode))
			switch gnmi.SubscriptionMode(gnmi.SubscriptionMode_value[strings.Replace(strings.ToUpper(sc.StreamMode), "-", "_", -1)]) {
			case gnmi.SubscriptionMode_ON_CHANGE:
				if sc.HeartbeatInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
				}
			case gnmi.SubscriptionMode_SAMPLE, gnmi.SubscriptionMode_TARGET_DEFINED:
				if sc.SampleInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.SampleInterval(*sc.SampleInterval))
				}
				subGnmiOpts = append(subGnmiOpts, api.SuppressRedundant(sc.SuppressRedundant))
				if sc.SuppressRedundant && sc.HeartbeatInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
				}
			default:
				return nil, fmt.Errorf("unknown stream subscription mode %s", sc.StreamMode)
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
	for _, m := range sc.Models {
		gnmiOpts = append(gnmiOpts, api.UseModel(m, "", ""))
	}
	return api.NewSubscribeRequest(gnmiOpts...)
}

func setDefaults(sc *types.SubscriptionConfig) error {
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
	sc.Encoding = os.ExpandEnv(sc.Encoding)
}
