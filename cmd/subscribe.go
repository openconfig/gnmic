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
	"github.com/spf13/cobra"
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
		PreRunE:      gApp.SubscribePreRunE,
		RunE:         gApp.SubscribeRunE,
		SilenceUsage: true,
	}
	gApp.InitSubscribeFlags(cmd)
	return cmd
}
