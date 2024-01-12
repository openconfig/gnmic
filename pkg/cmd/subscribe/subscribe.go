// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package subscribe

import (
	"github.com/spf13/cobra"

	"github.com/openconfig/gnmic/pkg/app"
)

var (
	// Modes is the list of supported subscription modes.
	Modes = [][2]string{
		{"once", "a single request/response channel. The target creates the relevant update messages, transmits them, and subsequently closes the RPC"},
		{"stream", "long-lived subscriptions which continue to transmit updates relating to the set of paths that are covered within the subscription indefinitely"},
		{"poll", "on-demand retrieval of data items via long-lived RPCs"},
	}

	// StreamModes is the list of supported streaming modes.
	StreamModes = [][2]string{
		{"target-defined", "the target MUST determine the best type of subscription to be created on a per-leaf basis"},
		{"sample", "the value of the data item(s) MUST be sent once per sample interval to the client"},
		{"on-change", "data updates are only sent when the value of the data item changes"},
	}
)

// New create the subscribe command tree.
func New(gApp *app.App) *cobra.Command {
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
		PreRunE: gApp.SubscribePreRunE,
		RunE:    gApp.SubscribeRunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			gApp.CleanupPlugins()
		},
		SilenceUsage: true,
	}
	gApp.InitSubscribeFlags(cmd)
	return cmd
}
