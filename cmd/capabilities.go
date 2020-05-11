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
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/viper"
	"google.golang.org/grpc/metadata"

	"github.com/spf13/cobra"
)

var printVersion bool

// capabilitiesCmd represents the capabilities command
var capabilitiesCmd = &cobra.Command{
	Use:     "capabilities",
	Aliases: []string{"c", "cap"},
	Short:   "query targets gnmi capabilities",

	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		addresses := viper.GetStringSlice("address")
		if len(addresses) == 0 {
			fmt.Println("no grpc server address specified")
			return nil
		}
		username := viper.GetString("username")
		if username == "" {
			if username, err = readUsername(); err != nil {
				return err
			}
		}
		password := viper.GetString("password")
		if password == "" {
			if password, err = readPassword(); err != nil {
				return err
			}
		}
		wg := new(sync.WaitGroup)
		wg.Add(len(addresses))
		req := &gnmi.CapabilityRequest{}
		for _, addr := range addresses {
			go func(address string) {
				defer wg.Done()
				_, _, err := net.SplitHostPort(address)
				if err != nil {
					if strings.Contains(err.Error(), "missing port in address") {
						address = net.JoinHostPort(address, defaultGrpcPort)
					} else {
						log.Printf("error parsing address '%s': %v", address, err)
						return
					}
				}
				conn, err := createGrpcConn(address)
				if err != nil {
					log.Printf("connection to %s failed: %v", address, err)
					return
				}
				client := gnmi.NewGNMIClient(conn)

				// grpcQos := gnmi.QOSMarking{
				// 	Marking: qos,
				// }
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx = metadata.AppendToOutgoingContext(ctx, "username", username, "password", password)

				response, err := client.Capabilities(ctx, req)
				if err != nil {
					log.Printf("error sending capabilities request: %v", err)
					return
				}
				printPrefix := ""
				if len(addresses) > 1 && !viper.GetBool("no-prefix") {
					printPrefix = fmt.Sprintf("[%s] ", address)
				}
				fmt.Printf("%sgNMI_Version: %s\n", printPrefix, response.GNMIVersion)
				if viper.GetBool("version") {
					return
				}
				fmt.Printf("%ssupported models:\n", printPrefix)
				for _, sm := range response.SupportedModels {
					fmt.Printf("%s  - %s, %s, %s\n", printPrefix, sm.GetName(), sm.GetOrganization(), sm.GetVersion())
				}
				fmt.Printf("%ssupported encodings:\n", printPrefix)
				for _, se := range response.SupportedEncodings {
					fmt.Printf("%s  - %s\n", printPrefix, se.String())
				}
				fmt.Println()
			}(addr)
		}
		wg.Wait()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(capabilitiesCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// capabilitiesCmd.PersistentFlags().String("foo", "", "A help for foo")
	capabilitiesCmd.Flags().BoolVarP(&printVersion, "version", "", false, "show gnmi version only")
	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// capabilitiesCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	viper.BindPFlag("version", capabilitiesCmd.Flags().Lookup("version"))
}
