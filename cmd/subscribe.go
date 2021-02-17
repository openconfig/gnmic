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
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	defaultRetryTimer  = 10 * time.Second
	defaultBackoff     = 100 * time.Millisecond
	defaultClusterName = "default-cluster"
)

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
			gApp.Config.SetLocalFlagsFromFile(cmd)
		},
		RunE: gApp.SubscribeRun,
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
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.SubscribePrefix, "prefix", "", "", "subscribe request prefix")
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.SubscribePath, "path", "", []string{}, "subscribe request paths")
	//cmd.MarkFlagRequired("path")
	cmd.Flags().Uint32VarP(&gApp.Config.LocalFlags.SubscribeQos, "qos", "q", 0, "qos marking")
	cmd.Flags().BoolVarP(&gApp.Config.LocalFlags.SubscribeUpdatesOnly, "updates-only", "", false, "only updates to current state should be sent")
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.SubscribeMode, "mode", "", "stream", "one of: once, stream, poll")
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.SubscribeStreamMode, "stream-mode", "", "target-defined", "one of: on-change, sample, target-defined")
	cmd.Flags().DurationVarP(&gApp.Config.LocalFlags.SubscribeSampleInterval, "sample-interval", "i", 0,
		"sample interval as a decimal number and a suffix unit, such as \"10s\" or \"1m30s\"")
	cmd.Flags().BoolVarP(&gApp.Config.LocalFlags.SubscribeSuppressRedundant, "suppress-redundant", "", false, "suppress redundant update if the subscribed value didn't not change")
	cmd.Flags().DurationVarP(&gApp.Config.LocalFlags.SubscribeHeartbearInterval, "heartbeat-interval", "", 0, "heartbeat interval in case suppress-redundant is enabled")
	cmd.Flags().StringSliceVarP(&gApp.Config.LocalFlags.SubscribeModel, "model", "", []string{}, "subscribe request used model(s)")
	cmd.Flags().BoolVar(&gApp.Config.LocalFlags.SubscribeQuiet, "quiet", false, "suppress stdout printing")
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.SubscribeTarget, "target", "", "", "subscribe request target")
	cmd.Flags().StringSliceVarP(&gApp.Config.LocalFlags.SubscribeName, "name", "n", []string{}, "reference subscriptions by name, must be defined in gnmic config file")
	cmd.Flags().StringSliceVarP(&gApp.Config.LocalFlags.SubscribeOutput, "output", "", []string{}, "reference to output groups by name, must be defined in gnmic config file")
	cmd.Flags().BoolVarP(&gApp.Config.LocalFlags.SubscribeWatchConfig, "watch-config", "", false, "watch configuration changes, add or delete subscribe targets accordingly")
	cmd.Flags().DurationVarP(&gApp.Config.LocalFlags.SubscribeBackoff, "backoff", "", 0, "backoff time between subscribe requests")
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.SubscribeClusterName, "cluster-name", "", defaultClusterName, "cluster name the gnmic instance belongs to, this is used for target loadsharing via a locker")
	cmd.Flags().DurationVarP(&gApp.Config.LocalFlags.SubscribeLockRetry, "lock-retry", "", 5*time.Second, "time to wait between target lock attempts")
	//
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		gApp.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
