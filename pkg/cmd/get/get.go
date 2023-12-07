// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package get

import (
	"github.com/openconfig/gnmic/pkg/app"
	"github.com/spf13/cobra"
)

var DataType = [][2]string{
	{"all", "all config/state/operational data"},
	{"config", "data that the target considers to be read/write"},
	{"state", "read-only data on the target"},
	{"operational", "read-only data on the target that is related to software processes operating on the device, or external interactions of the device"},
}

// getCmd represents the get command
func New(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "run gnmi get on targets",
		Annotations: map[string]string{
			"--path":   "XPATH",
			"--prefix": "PREFIX",
			"--model":  "MODEL",
			"--type":   "STORE",
		},
		PreRunE:      gApp.GetPreRunE,
		RunE:         gApp.GetRun,
		SilenceUsage: true,
	}
	gApp.InitGetFlags(cmd)
	return cmd
}
