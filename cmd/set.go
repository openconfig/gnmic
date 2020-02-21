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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/metadata"
)

// setCmd represents the set command
var setCmd = &cobra.Command{
	Use:   "set",
	Short: "run gnmi set on targets",

	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		addresses := viper.GetStringSlice("address")
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
		for _, addr := range addresses {
			go func(address string) {
				defer wg.Done()
				ipa, _, err := net.SplitHostPort(address)
				if err != nil {
					if strings.Contains(err.Error(), "missing port in address") {
						address = net.JoinHostPort(ipa, defaultGrpcPort)
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
				// 	Marking: viper.GetUint32("qos"),
				// }
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx = metadata.AppendToOutgoingContext(ctx, "username", username, "password", password)
				req := &gnmi.SetRequest{}
				response, err := client.Set(ctx, req)
				if err != nil {
					log.Printf("error sending set request: %v", err)
					return
				}
				fmt.Println(response)
			}(addr)
		}
		wg.Wait()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// setCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// setCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
