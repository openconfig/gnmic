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
		PreRunE:      gApp.SetPreRunE,
		RunE:         gApp.SetRunE,
		SilenceUsage: true,
	}
	gApp.InitSetFlags(cmd)
	return cmd
}
