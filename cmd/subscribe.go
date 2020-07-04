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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	_ "github.com/karimra/gnmic/outputs/all"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/prototext"
)

type msg struct {
	Meta             map[string]interface{} `json:"meta,omitempty"`
	Source           string                 `json:"source,omitempty"`
	SystemName       string                 `json:"system-name,omitempty"`
	SubscriptionName string                 `json:"subscription-name,omitempty"`
	Timestamp        int64                  `json:"timestamp,omitempty"`
	Time             *time.Time             `json:"time,omitempty"`
	Prefix           string                 `json:"prefix,omitempty"`
	Updates          []*update              `json:"updates,omitempty"`
	Deletes          []string               `json:"deletes,omitempty"`
}
type update struct {
	Path   string
	Values map[string]interface{} `json:"values,omitempty"`
}

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
		defer func() {
			for _, outputs := range outs {
				for _, o := range outputs {
					o.Close()
				}
			}
		}()
		cfg := &collector.Config{
			PrometheusAddress: viper.GetString("prometheus-address"),
			Debug:             viper.GetBool("debug"),
		}

		coll := collector.NewCollector(ctx, cfg, targetsConfig, subscriptionsConfig, outs, createCollectorDialOpts(), logger)

		wg := new(sync.WaitGroup)
		wg.Add(len(targetsConfig))
		for tName := range coll.Targets {
			go func(tn string) {
				defer wg.Done()
				err = coll.Subscribe(tn)
				if err != nil {
					logger.Printf("failed subscribing to target '%s': %v", tn, err)
				}
			}(tName)
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
						b, err := coll.FormatMsg(nil, response)
						if err != nil {
							fmt.Printf("target '%s', subscription '%s': poll response formatting error:%v\n", name, subName, err)
							continue
						}
						dst := new(bytes.Buffer)
						err = json.Indent(dst, b, "", "  ")
						if err != nil {
							fmt.Printf("failed to indent poll response from '%s': %v\n", name, err)
							fmt.Println(string(b))
							waitChan <- struct{}{}
							continue
						}
						fmt.Println(dst.String())
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
	subscribeCmd.Flags().StringP("prefix", "", "", "subscribe request prefix")
	subscribeCmd.Flags().StringSliceP("path", "", []string{""}, "subscribe request paths")
	//subscribeCmd.MarkFlagRequired("path")
	subscribeCmd.Flags().Int32P("qos", "q", 20, "qos marking")
	subscribeCmd.Flags().BoolP("updates-only", "", false, "only updates to current state should be sent")
	subscribeCmd.Flags().StringP("subscription-mode", "", "stream", "one of: once, stream, poll")
	subscribeCmd.Flags().StringP("stream-subscription-mode", "", "target-defined", "one of: on-change, sample, target-defined")
	subscribeCmd.Flags().StringP("sampling-interval", "i", "10s",
		"sampling interval as a decimal number and a suffix unit, such as \"10s\" or \"1m30s\", minimum is 1s")
	subscribeCmd.Flags().BoolP("suppress-redundant", "", false, "suppress redundant update if the subscribed value didn't not change")
	subscribeCmd.Flags().StringP("heartbeat-interval", "", "0s", "heartbeat interval in case suppress-redundant is enabled")
	subscribeCmd.Flags().StringSliceP("model", "", []string{""}, "subscribe request used model(s)")
	subscribeCmd.Flags().BoolP("quiet", "", false, "suppress stdout printing")
	//
	viper.BindPFlag("subscribe-prefix", subscribeCmd.LocalFlags().Lookup("prefix"))
	viper.BindPFlag("subscribe-path", subscribeCmd.LocalFlags().Lookup("path"))
	viper.BindPFlag("subscribe-qos", subscribeCmd.LocalFlags().Lookup("qos"))
	viper.BindPFlag("subscribe-updates-only", subscribeCmd.LocalFlags().Lookup("updates-only"))
	viper.BindPFlag("subscribe-subscription-mode", subscribeCmd.LocalFlags().Lookup("subscription-mode"))
	viper.BindPFlag("subscribe-stream-subscription-mode", subscribeCmd.LocalFlags().Lookup("stream-subscription-mode"))
	viper.BindPFlag("subscribe-sampling-interval", subscribeCmd.LocalFlags().Lookup("sampling-interval"))
	viper.BindPFlag("subscribe-suppress-redundant", subscribeCmd.LocalFlags().Lookup("suppress-redundant"))
	viper.BindPFlag("subscribe-heartbeat-interval", subscribeCmd.LocalFlags().Lookup("heartbeat-interval"))
	viper.BindPFlag("subscribe-sub-model", subscribeCmd.LocalFlags().Lookup("model"))
	viper.BindPFlag("subscribe-quiet", subscribeCmd.LocalFlags().Lookup("quiet"))
}

func formatSubscribeResponse(meta map[string]interface{}, subResp *gnmi.SubscribeResponse) ([]byte, error) {
	switch resp := subResp.Response.(type) {
	case *gnmi.SubscribeResponse_Update:
		if viper.GetString("format") == "textproto" {
			return []byte(prototext.Format(subResp)), nil
		}
		msg := new(msg)
		msg.Timestamp = resp.Update.Timestamp
		t := time.Unix(0, resp.Update.Timestamp)
		msg.Time = &t
		if meta == nil {
			meta = make(map[string]interface{})
		}
		msg.Prefix = gnmiPathToXPath(resp.Update.Prefix)
		var ok bool
		if _, ok = meta["source"]; ok {
			msg.Source = fmt.Sprintf("%s", meta["source"])
		}
		if _, ok = meta["system-name"]; ok {
			msg.SystemName = fmt.Sprintf("%s", meta["system-name"])
		}
		if _, ok = meta["subscription-name"]; ok {
			msg.SubscriptionName = fmt.Sprintf("%s", meta["subscription-name"])
		}
		for i, upd := range resp.Update.Update {
			pathElems := make([]string, 0, len(upd.Path.Elem))
			for _, pElem := range upd.Path.Elem {
				pathElems = append(pathElems, pElem.GetName())
			}
			value, err := getValue(upd.Val)
			if err != nil {
				logger.Println(err)
			}
			msg.Updates = append(msg.Updates,
				&update{
					Path:   gnmiPathToXPath(upd.Path),
					Values: make(map[string]interface{}),
				})
			msg.Updates[i].Values[strings.Join(pathElems, "/")] = value
		}
		for _, del := range resp.Update.Delete {
			msg.Deletes = append(msg.Deletes, gnmiPathToXPath(del))
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	return nil, nil
}

func getOutputs() (map[string][]outputs.Output, error) {
	outDef := viper.GetStringMap("outputs")
	if len(outDef) == 0 && !viper.GetBool("quiet") {
		stdoutConfig := map[string]interface{}{
			"type":     "file",
			"fileType": "stdout",
		}
		stdoutFile := make([]interface{}, 1)
		stdoutFile[0] = stdoutConfig
		outDef["stdout"] = stdoutFile
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
	if len(paths) > 0 {
		sub := new(collector.SubscriptionConfig)
		sub.Name = "default"
		sub.Paths = paths
		sub.Prefix = viper.GetString("subscribe-prefix")
		sub.Mode = viper.GetString("subscribe-subscription-mode")
		sub.Encoding = viper.GetString("encoding")
		sub.Qos = viper.GetUint32("qos")
		sub.StreamMode = viper.GetString("subscribe-stream-subscription-mode")
		sub.HeartbeatInterval = viper.GetDuration("subscribe-heartbeat-interval")
		sub.SampleInterval = viper.GetDuration("subscribe-sampling-interval")
		sub.SuppressRedundant = viper.GetBool("subscribe-suppress-redundant")
		sub.UpdatesOnly = viper.GetBool("subscribe-updates-only")
		subscriptions["default"] = sub
		return subscriptions, nil
	}
	subDef := viper.GetStringMap("subscriptions")
	if len(subDef) == 0 {
		return subscriptions, nil
	}
	var err error
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
		subscriptions[sn] = sub
	}
	return subscriptions, err
}
