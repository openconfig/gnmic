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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// capabilitiesCmd represents the capabilities command
func newCapabilitiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "capabilities",
		Aliases: []string{"cap"},
		Short:   "query targets gnmi capabilities",
		PreRun: func(cmd *cobra.Command, args []string) {
			gApp.Config.SetLocalFlagsFromFile(cmd)
		},
		RunE: gApp.CapRun,
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			initCapabilitiesFlags(cmd)
		},
		SilenceUsage: true,
	}
	initCapabilitiesFlags(cmd)
	return cmd
}

func initCapabilitiesFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&gApp.Config.LocalFlags.CapabilitiesVersion, "version", "", false, "show gnmi version only")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		gApp.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}
