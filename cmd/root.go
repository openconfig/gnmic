// Copyright © 2020 NAME HERE <EMAIL ADDRESS>
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
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gnmiClient",
	Short: "run gnmi rpcs from the terminal",

	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gnmiClient.yaml)")
	rootCmd.PersistentFlags().StringSliceP("address", "a", []string{}, "comma separated gnmi targets addresses")
	rootCmd.PersistentFlags().StringP("username", "u", "", "username")
	rootCmd.PersistentFlags().StringP("password", "p", "", "password")
	rootCmd.PersistentFlags().StringP("encoding", "e", "JSON", "one of: JSON, BYTES, PROTO, ASCII, JSON_IETF.")
	rootCmd.PersistentFlags().BoolP("insecure", "", false, "insecure connection")
	rootCmd.PersistentFlags().Int32P("qos", "q", 20, "qos marking")
	rootCmd.PersistentFlags().StringP("tls-cert", "", "", "tls certificate")
	rootCmd.PersistentFlags().StringP("tls-key", "", "", "tls key")

	rootCmd.PersistentFlags().BoolP("debug", "d", false, "debug mode")
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".gnmiClient" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".gnmiClient")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
