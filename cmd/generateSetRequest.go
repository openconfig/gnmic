/*
Copyright © 2021 Karim Radhouani <medkarimrdi@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// newGenerateSetRequestCmd represents the generate set-request command
func newGenerateSetRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set-request",
		Aliases: []string{"sr", "sreq", "srq"},
		Short:   "generate Set Request file",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			gApp.Config.SetLocalFlagsFromFile(cmd)
			return nil
		},
		RunE:         gApp.GenerateSetRequestRunE,
		SilenceUsage: true,
	}
	gApp.InitGenerateSetRequestFlags(cmd)
	return cmd
}
