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
	"github.com/spf13/cobra"
)

// pathCmd represents the path command
func newPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "generate gnmi or xpath style from yang file",
		Annotations: map[string]string{
			"--file": "YANG",
			"--dir":  "DIR",
		},
		PreRunE: gApp.PathPreRunE,
		RunE:    gApp.PathRunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			gApp.InitPathFlags(cmd)
		},
		SilenceUsage: true,
	}
	gApp.InitPathFlags(cmd)
	return cmd
}
