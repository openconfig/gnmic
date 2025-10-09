// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"github.com/spf13/cobra"

	"github.com/openconfig/gnmic/pkg/app"
	"github.com/openconfig/gnmic/pkg/collector"
)

// New create the collector command tree.
func New(gApp *app.App) *cobra.Command {
	c := collector.New(gApp.Context(), gApp.Store)
	cmd := &cobra.Command{
		Use:     "collect",
		Aliases: []string{"coll", "collector"},
		Short:   "collect gNMI telemetry from targets",
		PreRunE: c.CollectorPreRunE,
		RunE:    c.CollectorRunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			gApp.CleanupPlugins()
		},
		SilenceUsage: true,
	}
	return cmd
}
