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
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	gitURL  = ""
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show gnmic version",

	Run: func(cmd *cobra.Command, args []string) {
		if viper.GetString("format") != "json" {
			fmt.Printf("version : %s\n", version)
			fmt.Printf(" commit : %s\n", commit)
			fmt.Printf("   date : %s\n", date)
			fmt.Printf(" gitURL : %s\n", gitURL)
			fmt.Printf("   docs : https://gnmic.kmrd.dev\n")
			return
		}
		b, err := json.Marshal(map[string]string{
			"version": version,
			"commit":  commit,
			"date":    date,
			"gitURL":  gitURL,
			"docs":    "https://gnmic.kmrd.dev",
		}) // need indent? use jq
		if err != nil {
			logger.Printf("failed: %v", err)
			if !viper.GetBool("log") {
				fmt.Printf("failed: %v\n", err)
			}
			return
		}
		fmt.Println(string(b))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
