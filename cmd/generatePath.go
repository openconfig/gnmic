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

// newGeneratePathCmd represents the generate path command
func newGeneratePathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "path",
		Short:        "generate xpath(s) from yang models",
		PreRunE:      gApp.GeneratePathPreRunE,
		RunE:         gApp.GeneratePathRunE,
		SilenceUsage: true,
	}
	gApp.InitGeneratePathFlags(cmd)
	return cmd
}
