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

// generateCmd represents the generate command
func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "generate",
		Aliases:           []string{"gen"},
		Short:             "generate paths or JSON/YAML objects from YANG",
		PersistentPreRunE: gApp.GeneratePreRunE,
		RunE:              gApp.GenerateRunE,
		SilenceUsage:      true,
	}
	gApp.InitGenerateFlags(cmd)
	return cmd
}
