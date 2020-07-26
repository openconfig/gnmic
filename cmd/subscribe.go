// Copyright © 2020 Karim Radhouani <medkarimrdi@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	_ "github.com/karimra/gnmic/outputs/all"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// subscribeCmd represents the subscribe command
var subscribeCmd = &cobra.Command{
	Use:     "subscribe",
	Aliases: []string{"sub"},
	Short:   "subscribe to gnmi updates on targets",

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		setupCloseHandler(cancel)
		debug := viper.GetBool("debug")
		targetsConfig, err := createTargets()
		if err != nil {
			return fmt.Errorf("failed getting targets config: %v", err)
		}
		if debug {
			logger.Printf("targets: %s", targetsConfig)
		}
		subscriptionsConfig, err := getSubscriptions()
		if err != nil {
			return fmt.Errorf("failed getting subscriptions config: %v", err)
		}
		if debug {
			logger.Printf("subscriptions: %s", subscriptionsConfig)
		}
		outs, err := getOutputs()
		if err != nil {
			return err
		}
		if debug {
			logger.Printf("outputs: %+v", outs)
		}

		cfg := &collector.Config{
			PrometheusAddress:   viper.GetString("prometheus-address"),
			Debug:               viper.GetBool("debug"),
			Format:              viper.GetString("format"),
			TargetReceiveBuffer: viper.GetUint("target-buffer-size"),
		}

		coll := collector.NewCollector(ctx, cfg, targetsConfig, subscriptionsConfig, outs, createCollectorDialOpts(), logger)

		wg := new(sync.WaitGroup)
		wg.Add(len(coll.Targets))
		for name := range coll.Targets {
			go func(tn string) {
				defer wg.Done()
				if err = coll.Subscribe(tn); err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						logger.Printf("failed to initialize target '%s' timeout (%s) reached", tn, targetsConfig[tn].Timeout)
						return
					}
					logger.Printf("failed to initialize target '%s': %v", tn, err)
				}
			}(name)
		}
		wg.Wait()
		polledTargetsSubscriptions := coll.PolledSubscriptionsTargets()
		if len(polledTargetsSubscriptions) > 0 {
			pollTargets := make([]string, 0, len(polledTargetsSubscriptions))
			for t := range polledTargetsSubscriptions {
				pollTargets = append(pollTargets, t)
			}
			sort.Slice(pollTargets, func(i, j int) bool {
				return pollTargets[i] < pollTargets[j]
			})
			s := promptui.Select{
				Label:        "select target to poll",
				Items:        pollTargets,
				HideSelected: true,
			}
			waitChan := make(chan struct{}, 1)
			waitChan <- struct{}{}
			mo := &collector.MarshalOptions{
				Multiline: true,
				Indent:    "  ",
				Format:    viper.GetString("format")}
			go func() {
				for {
					select {
					case <-waitChan:
						_, name, err := s.Run()
						if err != nil {
							fmt.Printf("failed selecting target to poll: %v\n", err)
							continue
						}
						ss := promptui.Select{
							Label:        "select subscription to poll",
							Items:        polledTargetsSubscriptions[name],
							HideSelected: true,
						}
						_, subName, err := ss.Run()
						if err != nil {
							fmt.Printf("failed selecting subscription to poll: %v\n", err)
							continue
						}
						response, err := coll.TargetPoll(name, subName)
						if err != nil && err != io.EOF {
							fmt.Printf("target '%s', subscription '%s': poll response error:%v\n", name, subName, err)
							continue
						}
						if response == nil {
							fmt.Printf("received empty response from target '%s'\n", name)
							continue
						}
						switch rsp := response.Response.(type) {
						case *gnmi.SubscribeResponse_SyncResponse:
							fmt.Printf("received sync response '%t' from '%s'\n", rsp.SyncResponse, name)
							waitChan <- struct{}{}
							continue
						}
						b, err := mo.Marshal(response, nil)
						if err != nil {
							fmt.Printf("target '%s', subscription '%s': poll response formatting error:%v\n", name, subName, err)
							fmt.Println(string(b))
							waitChan <- struct{}{}
							continue
						}
						fmt.Println(string(b))
						waitChan <- struct{}{}
					case <-ctx.Done():
						return
					}
				}
			}()
		}
		coll.Start()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(subscribeCmd)
	subscribeCmd.SilenceUsage = true
	subscribeCmd.Flags().StringP("prefix", "", "", "subscribe request prefix")
	subscribeCmd.Flags().StringSliceP("path", "", []string{""}, "subscribe request paths")
	//subscribeCmd.MarkFlagRequired("path")
	subscribeCmd.Flags().Int32P("qos", "q", 20, "qos marking")
	subscribeCmd.Flags().BoolP("updates-only", "", false, "only updates to current state should be sent")
	subscribeCmd.Flags().StringP("mode", "", "stream", "one of: once, stream, poll")
	subscribeCmd.Flags().StringP("stream-mode", "", "target-defined", "one of: on-change, sample, target-defined")
	subscribeCmd.Flags().DurationP("sample-interval", "i", 0,
		"sample interval as a decimal number and a suffix unit, such as \"10s\" or \"1m30s\"")
	subscribeCmd.Flags().BoolP("suppress-redundant", "", false, "suppress redundant update if the subscribed value didn't not change")
	subscribeCmd.Flags().DurationP("heartbeat-interval", "", 0, "heartbeat interval in case suppress-redundant is enabled")
	subscribeCmd.Flags().StringSliceP("model", "", []string{""}, "subscribe request used model(s)")
	subscribeCmd.Flags().Bool("quiet", false, "suppress stdout printing")
	subscribeCmd.Flags().StringP("target", "", "", "subscribe request target")
	//
	viper.BindPFlag("subscribe-prefix", subscribeCmd.LocalFlags().Lookup("prefix"))
	viper.BindPFlag("subscribe-path", subscribeCmd.LocalFlags().Lookup("path"))
	viper.BindPFlag("subscribe-qos", subscribeCmd.LocalFlags().Lookup("qos"))
	viper.BindPFlag("subscribe-updates-only", subscribeCmd.LocalFlags().Lookup("updates-only"))
	viper.BindPFlag("subscribe-mode", subscribeCmd.LocalFlags().Lookup("mode"))
	viper.BindPFlag("subscribe-stream-mode", subscribeCmd.LocalFlags().Lookup("stream-mode"))
	viper.BindPFlag("subscribe-sample-interval", subscribeCmd.LocalFlags().Lookup("sample-interval"))
	viper.BindPFlag("subscribe-suppress-redundant", subscribeCmd.LocalFlags().Lookup("suppress-redundant"))
	viper.BindPFlag("subscribe-heartbeat-interval", subscribeCmd.LocalFlags().Lookup("heartbeat-interval"))
	viper.BindPFlag("subscribe-sub-model", subscribeCmd.LocalFlags().Lookup("model"))
	viper.BindPFlag("subscribe-quiet", subscribeCmd.LocalFlags().Lookup("quiet"))
	viper.BindPFlag("subscribe-target", subscribeCmd.LocalFlags().Lookup("target"))
}

func getOutputs() (map[string][]outputs.Output, error) {
	outDef := viper.GetStringMap("outputs")
	if len(outDef) == 0 && !viper.GetBool("quiet") {
		stdoutConfig := map[string]interface{}{
			"type":      "file",
			"file-type": "stdout",
			"format":    viper.GetString("format"),
		}
		outDef["stdout"] = []interface{}{stdoutConfig}
	}
	outputDestinations := make(map[string][]outputs.Output)
	for name, d := range outDef {
		dl := convert(d)
		switch outs := dl.(type) {
		case []interface{}:
			for _, ou := range outs {
				switch ou := ou.(type) {
				case map[string]interface{}:
					if outType, ok := ou["type"]; ok {
						if initalizer, ok := outputs.Outputs[outType.(string)]; ok {
							format, ok := ou["format"]
							if !ok || (ok && format == "") {
								ou["format"] = viper.GetString("format")
							}
							o := initalizer()
							err := o.Init(ou, logger)
							if err != nil {
								return nil, err
							}
							if outputDestinations[name] == nil {
								outputDestinations[name] = make([]outputs.Output, 0)
							}
							outputDestinations[name] = append(outputDestinations[name], o)
							continue
						}
						logger.Printf("unknown output type '%s'", outType)
						continue
					}
					logger.Printf("missing output 'type' under %v", ou)
				default:
					logger.Printf("unknown configuration format expecting a map[string]interface{}: got %T : %v", d, d)
				}
			}
		default:
			return nil, fmt.Errorf("unknown configuration format: %T : %v", d, d)
		}
	}
	return outputDestinations, nil
}

func getSubscriptions() (map[string]*collector.SubscriptionConfig, error) {
	subscriptions := make(map[string]*collector.SubscriptionConfig)
	paths := viper.GetStringSlice("subscribe-path")
	hi := viper.GetDuration("subscribe-heartbeat-interval")
	si := viper.GetDuration("subscribe-sample-interval")
	qos := viper.GetUint32("subscribe-qos")
	if len(paths) > 0 {
		sub := new(collector.SubscriptionConfig)
		sub.Name = "default"
		sub.Paths = paths
		sub.Prefix = viper.GetString("subscribe-prefix")
		sub.Target = viper.GetString("subscribe-target")
		sub.Mode = viper.GetString("subscribe-mode")
		sub.Encoding = viper.GetString("encoding")
		sub.Qos = &qos
		sub.StreamMode = viper.GetString("subscribe-stream-mode")
		sub.HeartbeatInterval = &hi
		sub.SampleInterval = &si
		sub.SuppressRedundant = viper.GetBool("subscribe-suppress-redundant")
		sub.UpdatesOnly = viper.GetBool("subscribe-updates-only")
		sub.Models = viper.GetStringSlice("models")
		subscriptions["default"] = sub
		return subscriptions, nil
	}
	subDef := viper.GetStringMap("subscriptions")
	if viper.GetBool("debug") {
		logger.Printf("subscription map: %v+", subDef)
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
			sub.Qos = &qos
		}
		subscriptions[sn] = sub
	}
	return subscriptions, nil
}
