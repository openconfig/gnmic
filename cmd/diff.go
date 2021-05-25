// Copyright © 2021 Karim Radhouani <medkarimrdi@gmail.com>
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
	"github.com/karimra/gnmic/config"
	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "diff",
		Aliases: []string{"compare"},
		Short:   "run a diff comparison between targets",
		PreRun: func(cmd *cobra.Command, args []string) {
			gApp.Config.SetLocalFlagsFromFile(cmd)
			if len(gApp.Config.LocalFlags.DiffPath) == 0 {
				gApp.Config.LocalFlags.DiffPath = []string{"/"}
			}
			gApp.Config.LocalFlags.DiffPath = config.SanitizeArrayFlagValue(gApp.Config.LocalFlags.DiffPath)
			gApp.Config.LocalFlags.DiffModel = config.SanitizeArrayFlagValue(gApp.Config.LocalFlags.DiffModel)
			gApp.Config.LocalFlags.DiffCompare = config.SanitizeArrayFlagValue(gApp.Config.LocalFlags.DiffCompare)
		},
		RunE:         gApp.DiffRun,
		SilenceUsage: true,
	}
	gApp.InitDiffFlags(cmd)
	return cmd
}
