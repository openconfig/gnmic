// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package set

import (
	"github.com/openconfig/gnmic/pkg/app"
	"github.com/spf13/cobra"
)

// New creates the set command tree.
func New(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "run gnmi set on targets",
		Annotations: map[string]string{
			"--delete":       "XPATH",
			"--prefix":       "PREFIX",
			"--replace":      "XPATH",
			"--replace-file": "FILE",
			"--replace-path": "XPATH",
			"--update":       "XPATH",
			"--update-file":  "FILE",
			"--update-path":  "XPATH",
		},
		PreRunE:      gApp.SetPreRunE,
		RunE:         gApp.SetRunE,
		SilenceUsage: true,
	}
	gApp.InitSetFlags(cmd)
	return cmd
}
