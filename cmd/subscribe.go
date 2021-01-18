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
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/formatters"
	"github.com/manifoldco/promptui"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const defaultRetryTimer = 10 * time.Second

var subscriptionModes = [][2]string{
	{"once", "a single request/response channel. The target creates the relevant update messages, transmits them, and subsequently closes the RPC"},
	{"stream", "long-lived subscriptions which continue to transmit updates relating to the set of paths that are covered within the subscription indefinitely"},
	{"poll", "on-demand retrieval of data items via long-lived RPCs"},
}

var streamSubscriptionModes = [][2]string{
	{"target-defined", "the target MUST determine the best type of subscription to be created on a per-leaf basis"},
	{"sample", "the value of the data item(s) MUST be sent once per sample interval to the client"},
	{"on-change", "data updates are only sent when the value of the data item changes"},
}

// subscribeCmd represents the subscribe command
func newSubscribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "subscribe",
		Aliases: []string{"sub"},
		Short:   "subscribe to gnmi updates on targets",
		Annotations: map[string]string{
			"--path":        "XPATH",
			"--prefix":      "PREFIX",
			"--model":       "MODEL",
			"--mode":        "SUBSC_MODE",
			"--stream-mode": "STREAM_MODE",
			"--name":        "SUBSCRIPTION",
			"--output":      "OUTPUT",
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			cli.config.SetLocalFlagsFromFile(cmd)
		},
		RunE: cli.subscribeRunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			initSubscribeFlags(cmd)
		},
		SilenceUsage: true,
	}
	initSubscribeFlags(cmd)
	return cmd
}

// used to init or reset subscribeCmd flags for gnmic-prompt mode
func initSubscribeFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&cli.config.LocalFlags.SubscribePrefix, "prefix", "", "", "subscribe request prefix")
	cmd.Flags().StringArrayVarP(&cli.config.LocalFlags.SubscribePath, "path", "", []string{}, "subscribe request paths")
	//cmd.MarkFlagRequired("path")
	cmd.Flags().Uint32VarP(&cli.config.LocalFlags.SubscribeQos, "qos", "q", 0, "qos marking")
	cmd.Flags().BoolVarP(&cli.config.LocalFlags.SubscribeUpdatesOnly, "updates-only", "", false, "only updates to current state should be sent")
	cmd.Flags().StringVarP(&cli.config.LocalFlags.SubscribeMode, "mode", "", "stream", "one of: once, stream, poll")
	cmd.Flags().StringVarP(&cli.config.LocalFlags.SubscribeStreamMode, "stream-mode", "", "target-defined", "one of: on-change, sample, target-defined")
	cmd.Flags().DurationVarP(&cli.config.LocalFlags.SubscribeSampleInterval, "sample-interval", "i", 0,
		"sample interval as a decimal number and a suffix unit, such as \"10s\" or \"1m30s\"")
	cmd.Flags().BoolVarP(&cli.config.LocalFlags.SubscribeSuppressRedundant, "suppress-redundant", "", false, "suppress redundant update if the subscribed value didn't not change")
	cmd.Flags().DurationVarP(&cli.config.LocalFlags.SubscribeHeartbearInterval, "heartbeat-interval", "", 0, "heartbeat interval in case suppress-redundant is enabled")
	cmd.Flags().StringSliceVarP(&cli.config.LocalFlags.SubscribeModel, "model", "", []string{}, "subscribe request used model(s)")
	cmd.Flags().BoolVar(&cli.config.LocalFlags.SubscribeQuiet, "quiet", false, "suppress stdout printing")
	cmd.Flags().StringVarP(&cli.config.LocalFlags.SubscribeTarget, "target", "", "", "subscribe request target")
	cmd.Flags().StringSliceVarP(&cli.config.LocalFlags.SubscribeName, "name", "n", []string{}, "reference subscriptions by name, must be defined in gnmic config file")
	cmd.Flags().StringSliceVarP(&cli.config.LocalFlags.SubscribeOutput, "output", "", []string{}, "reference to output groups by name, must be defined in gnmic config file")
	cmd.Flags().BoolVarP(&cli.config.LocalFlags.SubscribeWatchConfig, "watch-config", "", false, "watch configuration changes, add or delete subscribe targets accordingly")
	//
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		cli.config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func (c *CLI) subscribeRunE(cmd *cobra.Command, args []string) error {
	gctx, gcancel = context.WithCancel(context.Background())
	setupCloseHandler(gcancel)
	inputsConfig, err := c.config.GetInputs()
	if err != nil {
		return fmt.Errorf("failed getting inputs config: %v", err)
	}
	targetsConfig, err := c.config.GetTargets()
	if (errors.Is(err, config.ErrNoTargetsFound) && !c.config.LocalFlags.SubscribeWatchConfig) ||
		(!errors.Is(err, config.ErrNoTargetsFound) && err != nil) {
		return fmt.Errorf("failed getting targets config: %v", err)
	}

	subscriptionsConfig, err := c.config.GetSubscriptions(cmd)
	if err != nil {
		return fmt.Errorf("failed getting subscriptions config: %v", err)
	}
	outs, err := c.config.GetOutputs()
	if err != nil {
		return err
	}
	epconfig, err := c.config.GetEventProcessors()
	if err != nil {
		return err
	}

	if c.collector == nil {
		cfg := &collector.Config{
			PrometheusAddress:   c.config.Globals.PrometheusAddress,
			Debug:               c.config.Globals.Debug,
			Format:              c.config.Globals.Format,
			TargetReceiveBuffer: c.config.Globals.TargetBufferSize,
			RetryTimer:          c.config.Globals.Retry,
		}

		c.collector = collector.NewCollector(cfg, targetsConfig,
			collector.WithDialOptions(createCollectorDialOpts()),
			collector.WithSubscriptions(subscriptionsConfig),
			collector.WithOutputs(outs),
			collector.WithLogger(c.logger),
			collector.WithEventProcessors(epconfig),
			collector.WithInputs(inputsConfig),
		)
	} else {
		// prompt mode
		for name, outCfg := range outs {
			err = c.collector.AddOutput(name, outCfg)
			if err != nil {
				c.logger.Printf("%v", err)
			}
		}
		for _, sc := range subscriptionsConfig {
			err = c.collector.AddSubscriptionConfig(sc)
			if err != nil {
				c.logger.Printf("%v", err)
			}
		}
		for _, tc := range targetsConfig {
			c.collector.AddTarget(tc)
			if err != nil {
				c.logger.Printf("%v", err)
			}
		}
	}

	c.collector.InitOutputs(gctx)
	c.collector.InitInputs(gctx)
	go c.collector.Start(gctx)

	c.wg.Add(len(c.collector.Targets))
	for _, target := range c.collector.Targets {
		go c.subscribe(gctx, target.Config)
	}
	c.wg.Wait()
	polledTargetsSubscriptions := c.collector.PolledSubscriptionsTargets()
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
		mo := &formatters.MarshalOptions{
			Multiline: true,
			Indent:    "  ",
			Format:    cli.config.Globals.Format,
		}
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
					response, err := c.collector.TargetPoll(name, subName)
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
				case <-gctx.Done():
					return
				}
			}
		}()
	}
	if c.config.LocalFlags.SubscribeWatchConfig {
		go c.watchConfig()
	}

	if cli.promptMode {
		return nil
	}
	for range gctx.Done() {
		return gctx.Err()
	}
	return nil
}

func (c *CLI) subscribe(ctx context.Context, tConf *collector.TargetConfig) {
	defer c.wg.Done()
	var err error
	for {
		err = c.collector.Subscribe(gctx, tConf.Name)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				c.logger.Printf("failed to initialize target '%s' timeout (%s) reached", tConf.Name, tConf.Timeout)
			} else {
				c.logger.Printf("failed to initialize target '%s': %v", tConf.Name, err)
			}
			c.logger.Printf("retrying target %s in %s", tConf.Name, tConf.RetryTimer)
			time.Sleep(tConf.RetryTimer)
			continue
		}
		return
	}
}
