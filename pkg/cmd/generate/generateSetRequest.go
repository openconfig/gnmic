// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"github.com/openconfig/gnmic/pkg/app"
	"github.com/spf13/cobra"
)

// newGenerateSetRequestCmd represents the generate set-request command
func newGenerateSetRequestCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set-request",
		Aliases: []string{"sr", "sreq", "srq"},
		Short:   "generate Set Request file",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			gApp.Config.SetLocalFlagsFromFile(cmd)
			return nil
		},
		RunE:         gApp.GenerateSetRequestRunE,
		SilenceUsage: true,
	}
	gApp.InitGenerateSetRequestFlags(cmd)
	return cmd
}
