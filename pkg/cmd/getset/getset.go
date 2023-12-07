// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package getset

import (
	"github.com/openconfig/gnmic/pkg/app"
	"github.com/spf13/cobra"
)

// getCmd represents the get command
func New(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "getset",
		Aliases: []string{"gas", "gs"},
		Short:   "run gnmi get then set on targets",
		Annotations: map[string]string{
			"--get":    "XPATH",
			"--prefix": "PREFIX",
			"--type":   "STORE",
		},
		PreRunE:      gApp.GetSetPreRunE,
		RunE:         gApp.GetSetRunE,
		SilenceUsage: true,
	}
	gApp.InitGetSetFlags(cmd)
	return cmd
}
