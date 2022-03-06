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
		PreRunE:      gApp.GetPreRunE,
		RunE:         gApp.GetRun,
		SilenceUsage: true,
	}
	gApp.InitGetFlags(cmd)
	return cmd
}
