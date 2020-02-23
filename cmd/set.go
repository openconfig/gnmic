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
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/google/gnxi/utils/xpath"
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
		deletePaths := viper.GetStringSlice("delete-path")
		updatePaths := viper.GetStringSlice("update-path")
		replacePaths := viper.GetStringSlice("replace-path")
		updateValues := viper.GetStringSlice("update-value")
		replaceValues := viper.GetStringSlice("replace-value")
		updateValuesTypes := viper.GetStringSlice("update-value-type")
		replaceValuesTypes := viper.GetStringSlice("replace-value-type")
		if len(replacePaths) != len(replaceValues) {
			return errors.New("missing or extra replace values")
		}
		if len(replaceValuesTypes) != len(replaceValues) {
			return errors.New("missing or extra replace values")
		}
		if len(updatePaths) != len(updateValues) {
			return errors.New("missing or extra replace values")
		}
		if len(updateValuesTypes) != len(updateValues) {
			return errors.New("missing or extra replace values")
		}
		fmt.Println(updatePaths)
		fmt.Println(replacePaths)
		fmt.Println(deletePaths)
		if (len(deletePaths) + len(updatePaths) + len(replacePaths)) == 0 {
			return errors.New("no paths provided")
		}
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
		req := &gnmi.SetRequest{
			Delete:  make([]*gnmi.Path, 0, len(deletePaths)),
			Replace: make([]*gnmi.Update, 0, len(replacePaths)),
			Update:  make([]*gnmi.Update, 0, len(updatePaths)),
		}
		for _, p := range deletePaths {
			gnmiPath, err := xpath.ToGNMIPath(p)
			if err != nil {
				return fmt.Errorf("path parse error: %v", err)
			}
			req.Delete = append(req.Delete, gnmiPath)
		}
		for i, p := range replacePaths {
			gnmiPath, err := xpath.ToGNMIPath(p)
			if err != nil {
				return fmt.Errorf("path parse error: %v", err)
			}
			upd := &gnmi.Update{
				Path: gnmiPath,
				Val:  &gnmi.TypedValue{},
			}
			switch replaceValuesTypes[i] {
			case "string":
				upd.Val.Value = &gnmi.TypedValue_StringVal{StringVal: replaceValues[i]}
			case "int":
				v, err := strconv.Atoi(replaceValues[i])
				if err != nil {
					log.Printf("Err converting string to int: %v", err)
					continue
				}
				upd.Val.Value = &gnmi.TypedValue_IntVal{IntVal: int64(v)}
			case "uint":
				v, err := strconv.Atoi(replaceValues[i])
				if err != nil {
					log.Printf("Err converting string to uint: %v", err)
					continue
				}
				upd.Val.Value = &gnmi.TypedValue_UintVal{UintVal: uint64(v)}
			case "bool":
				upd.Val.Value = &gnmi.TypedValue_BoolVal{BoolVal: replaceValues[i] == "true"}
			case "bytes":
				upd.Val.Value = &gnmi.TypedValue_BytesVal{BytesVal: []byte{}}
			case "float":
			case "decimal":
			case "leaflist":
			case "any":
			case "json":
				upd.Val.Value = &gnmi.TypedValue_JsonVal{JsonVal: []byte(replaceValues[i])}
			case "json-ietf":
			case "ascii":
			case "protobytes":
			}
			req.Replace = append(req.Replace, upd)

		}
		for i, p := range updatePaths {
			gnmiPath, err := xpath.ToGNMIPath(p)
			if err != nil {
				return fmt.Errorf("path parse error: %v", err)
			}
			upd := &gnmi.Update{
				Path: gnmiPath,
				Val:  &gnmi.TypedValue{},
			}
			switch updateValuesTypes[i] {
			case "string":
				upd.Val.Value = &gnmi.TypedValue_StringVal{StringVal: updateValues[i]}
			case "int":
				v, err := strconv.Atoi(updateValues[i])
				if err != nil {
					log.Printf("Err converting string to int: %v", err)
					continue
				}
				upd.Val.Value = &gnmi.TypedValue_IntVal{IntVal: int64(v)}
			case "uint":
				v, err := strconv.Atoi(updateValues[i])
				if err != nil {
					log.Printf("Err converting string to uint: %v", err)
					continue
				}
				upd.Val.Value = &gnmi.TypedValue_UintVal{UintVal: uint64(v)}
			case "bool":
				upd.Val.Value = &gnmi.TypedValue_BoolVal{BoolVal: updateValues[i] == "true"}
			case "bytes":
				upd.Val.Value = &gnmi.TypedValue_BytesVal{BytesVal: []byte{}}
			case "float":
			case "decimal":
			case "leaflist":
			case "any":
			case "json":
				upd.Val.Value = &gnmi.TypedValue_JsonVal{JsonVal: []byte(updateValues[i])}
			case "json-ietf":
			case "ascii":
			case "protobytes":
			}
			req.Update = append(req.Update, upd)
			fmt.Printf("upd: %s\n", upd)
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
	setCmd.Flags().StringP("prefix", "", "", "set request prefix")
	setCmd.Flags().StringSliceP("delete-path", "", []string{""}, "set request path to be deleted")
	setCmd.Flags().StringSliceP("replace-path", "", []string{""}, "set request path to be replaced")
	setCmd.Flags().StringSliceP("update-path", "", []string{""}, "set request path to be updated")
	setCmd.Flags().StringSliceP("replace-value", "", []string{""}, "set request value to be replaced")
	setCmd.Flags().StringSliceP("update-value", "", []string{""}, "set request value to be updated")
	setCmd.Flags().StringSliceP("replace-value-type", "", []string{""}, "set request value type to be replaced")
	setCmd.Flags().StringSliceP("update-value-type", "", []string{""}, "set request value type to be updated")
	viper.BindPFlag("set-prefix", setCmd.Flags().Lookup("prefix"))
	viper.BindPFlag("delete-path", setCmd.Flags().Lookup("delete-path"))
	viper.BindPFlag("replace-path", setCmd.Flags().Lookup("replace-path"))
	viper.BindPFlag("update-path", setCmd.Flags().Lookup("update-path"))

	viper.BindPFlag("replace-value", setCmd.Flags().Lookup("replace-value"))
	viper.BindPFlag("update-value", setCmd.Flags().Lookup("update-value"))

	viper.BindPFlag("replace-value-type", setCmd.Flags().Lookup("replace-value-type"))
	viper.BindPFlag("update-value-type", setCmd.Flags().Lookup("update-value-type"))
}
