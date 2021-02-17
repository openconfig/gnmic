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

// setCmd represents the set command
func newSetCmd() *cobra.Command {
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
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// g.Config.SetLocalFlagsFromFile(cmd)
			// return g.Config.ValidateSetInput()
			gApp.Config.SetLocalFlagsFromFile(cmd)
			return gApp.Config.ValidateSetInput()
		},
		RunE: gApp.SetRun,
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			initSetFlags(cmd)
		},
		SilenceUsage: true,
	}
	initSetFlags(cmd)
	return cmd
}

// used to init or reset setCmd flags for gnmic-prompt mode
func initSetFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("prefix", "", "", "set request prefix")

	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.SetDelete, "delete", "", []string{}, "set request path to be deleted")

	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.SetReplace, "replace", "", []string{}, fmt.Sprintf("set request path:::type:::value to be replaced, type must be one of %v", config.ValueTypes))
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.SetUpdate, "update", "", []string{}, fmt.Sprintf("set request path:::type:::value to be updated, type must be one of %v", config.ValueTypes))

	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.SetReplacePath, "replace-path", "", []string{}, "set request path to be replaced")
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.SetUpdatePath, "update-path", "", []string{}, "set request path to be updated")
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.SetUpdateFile, "update-file", "", []string{}, "set update request value in json/yaml file")
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.SetReplaceFile, "replace-file", "", []string{}, "set replace request value in json/yaml file")
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.SetUpdateValue, "update-value", "", []string{}, "set update request value")
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.SetReplaceValue, "replace-value", "", []string{}, "set replace request value")
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.SetDelimiter, "delimiter", "", ":::", "set update/replace delimiter between path, type, value")
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.SetTarget, "target", "", "", "set request target")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		gApp.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
