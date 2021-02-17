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

	"github.com/karimra/gnmic/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

//var paths []string
var dataType = [][2]string{
	{"all", "all config/state/operational data"},
	{"config", "data that the target considers to be read/write"},
	{"state", "read-only data on the target"},
	{"operational", "read-only data on the target that is related to software processes operating on the device, or external interactions of the device"},
}

// getCmd represents the get command
func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "run gnmi get on targets",
		Annotations: map[string]string{
			"--path":   "XPATH",
			"--prefix": "PREFIX",
			"--model":  "MODEL",
			"--type":   "STORE",
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			gApp.Config.SetLocalFlagsFromFile(cmd)
			gApp.Config.LocalFlags.GetPath = config.SanitizeArrayFlagValue(gApp.Config.LocalFlags.GetPath)
			gApp.Config.LocalFlags.GetModel = config.SanitizeArrayFlagValue(gApp.Config.LocalFlags.GetModel)
		},
		RunE: gApp.GetRun,
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			initGetFlags(cmd)
		},
		SilenceUsage: true,
	}
	initGetFlags(cmd)
	return cmd
}

// used to init or reset getCmd flags for gnmic-prompt mode
func initGetFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.GetPath, "path", "", []string{}, "get request paths")
	cmd.MarkFlagRequired("path")
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.GetPrefix, "prefix", "", "", "get request prefix")
	cmd.Flags().StringSliceVarP(&gApp.Config.LocalFlags.GetModel, "model", "", []string{}, "get request models")
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.GetType, "type", "t", "ALL", "data type requested from the target. one of: ALL, CONFIG, STATE, OPERATIONAL")
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.GetTarget, "target", "", "", "get request target")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		gApp.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
