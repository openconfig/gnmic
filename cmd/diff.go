// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "diff",
		Aliases:      []string{"compare"},
		Short:        "run a diff comparison between targets",
		PreRunE:      gApp.DiffPreRunE,
		RunE:         gApp.DiffRunE,
		SilenceUsage: true,
	}
	gApp.InitDiffFlags(cmd)
	cmd.AddCommand(newDiffSetRequestCmd())
	return cmd
}

// newDiffSetRequestCmd creates a new diff setrequest command.
func newDiffSetRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "setrequest",
		Short:        "run a diff comparison between two setrequests in textproto format",
		RunE:         gApp.DiffSetRequestRunE,
		SilenceUsage: true,
	}
	gApp.InitDiffSetRequestFlags(cmd)
	return cmd
}
