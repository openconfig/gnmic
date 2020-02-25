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
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/google/gnxi/utils/xpath"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/metadata"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "run gnmi get on targets",

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
		req := &gnmi.GetRequest{
			UseModels: make([]*gnmi.ModelData, 0),
			Path:      make([]*gnmi.Path, 0),
		}
		model := viper.GetString("get-model")
		if model != "" {
			req.UseModels = append(req.UseModels, &gnmi.ModelData{Name: model})
		}
		prefix := viper.GetString("get-prefix")
		if prefix != "" {
			gnmiPrefix, err := xpath.ToGNMIPath(prefix)
			if err != nil {
				return fmt.Errorf("prefix parse error: %v", err)
			}
			req.Prefix = gnmiPrefix
		}
		paths := viper.GetStringSlice("get-path")
		for _, p := range paths {
			gnmiPath, err := xpath.ToGNMIPath(p)
			if err != nil {
				return fmt.Errorf("path parse error: %v", err)
			}
			req.Path = append(req.Path, gnmiPath)
		}
		dataType := viper.GetString("get-type")
		if dataType != "" {
			dti, ok := gnmi.GetRequest_DataType_value[dataType]
			if !ok {
				return fmt.Errorf("unknown data type %s", dataType)
			}
			req.Type = gnmi.GetRequest_DataType(dti)
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
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx = metadata.AppendToOutgoingContext(ctx, "username", username, "password", password)

				response, err := client.Get(ctx, req)
				if err != nil {
					log.Printf("error sending get request: %v", err)
					return
				}
				printPrefix := fmt.Sprintf("[%s] ", address)
				for _, notif := range response.Notification {
					fmt.Printf("%stimestamp: %d\n", printPrefix, notif.Timestamp)
					fmt.Printf("%sprefix: %s\n", printPrefix, gnmiPathToXPath(notif.Prefix))
					fmt.Printf("%salias: %s\n", printPrefix, notif.Alias)
					for _, upd := range notif.Update {
						if upd.Val == nil {
							continue
						}
						var value interface{}
						var jsondata []byte
						switch val := upd.Val.Value.(type) {
						case *gnmi.TypedValue_AsciiVal:
							value = val.AsciiVal
						case *gnmi.TypedValue_BoolVal:
							value = val.BoolVal
						case *gnmi.TypedValue_BytesVal:
							value = val.BytesVal
						case *gnmi.TypedValue_DecimalVal:
							value = val.DecimalVal
						case *gnmi.TypedValue_FloatVal:
							value = val.FloatVal
						case *gnmi.TypedValue_IntVal:
							value = val.IntVal
						case *gnmi.TypedValue_StringVal:
							value = val.StringVal
						case *gnmi.TypedValue_UintVal:
							value = val.UintVal
						case *gnmi.TypedValue_JsonIetfVal:
							jsondata = val.JsonIetfVal
						case *gnmi.TypedValue_JsonVal:
							jsondata = val.JsonVal
						}
						if jsondata != nil {
							err = json.Unmarshal(jsondata, &value)
							if err != nil {
								log.Printf("error unmarshling jsonVal '%s'", string(jsondata))
								continue
							}
							data, err := json.MarshalIndent(value, printPrefix, "  ")
							if err != nil {
								log.Printf("error marshling jsonVal '%s'", value)
								continue
							}
							fmt.Printf("%s%s: (%T) %s\n", printPrefix, gnmiPathToXPath(upd.Path), upd.Val.Value, data)
						} else if value != nil {
							fmt.Printf("%s%s: (%T) %s\n", printPrefix, gnmiPathToXPath(upd.Path), upd.Val.Value, value)
						}
					}
				}
				fmt.Println()
			}(addr)
		}
		wg.Wait()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)

	getCmd.Flags().StringSliceP("path", "", []string{"/"}, "get request paths")
	getCmd.Flags().StringP("prefix", "", "", "get request prefix")
	getCmd.Flags().StringP("model", "", "", "get request model")
	getCmd.Flags().StringP("type", "t", "ALL", "the type of data that is requested from the target. one of: ALL, CONFIG, STATE, OPERATIONAL")
	viper.BindPFlag("get-path", getCmd.Flags().Lookup("path"))
	viper.BindPFlag("get-prefix", getCmd.Flags().Lookup("prefix"))
	viper.BindPFlag("get-model", getCmd.Flags().Lookup("model"))
	viper.BindPFlag("get-type", getCmd.Flags().Lookup("type"))
}
