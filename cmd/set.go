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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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

// setCmd represents the set command
var setCmd = &cobra.Command{
	Use:   "set",
	Short: "run gnmi set on targets",

	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		addresses := viper.GetStringSlice("address")
		if len(addresses) == 0 {
			return errors.New("no address provided")
		}
		if len(addresses) > 1 {
			fmt.Println("[warning] running set command on multiple targets")
		}
		prefix := viper.GetString("set-prefix")
		gnmiPrefix, err := xpath.ToGNMIPath(prefix)
		if err != nil {
			return err
		}
		deletes := viper.GetStringSlice("delete")
		updates := viper.GetStringSlice("update")
		replaces := viper.GetStringSlice("replace")
		updateFiles := viper.GetStringSlice("update-file")
		replaceFiles := viper.GetStringSlice("replace-file")
		updateValues := viper.GetStringSlice("update-value")
		replaceValues := viper.GetStringSlice("replace-value")
		useUpdateFile := len(updateFiles) > 0 && len(updateValues) == 0
		useReplaceFile := len(replaceFiles) > 0 && len(replaceValues) == 0
		if viper.GetBool("debug") {
			log.Printf("deletes(%d)=%v\n", len(deletes), deletes)
			log.Printf("updates(%d)=%v\n", len(updates), updates)
			log.Printf("replaces(%d)=%v\n", len(replaces), replaces)
			log.Printf("updates-files(%d)=%v\n", len(updateFiles), updateFiles)
			log.Printf("replaces-files(%d)=%v\n", len(replaceFiles), replaceFiles)
			log.Printf("updates-values(%d)=%v\n", len(updateValues), updateValues)
			log.Printf("replaces-values(%d)=%v\n", len(replaceValues), replaceValues)
		}
		if (len(deletes) + len(updates) + len(replaces)) == 0 {
			return errors.New("no paths provided")
		}
		if useUpdateFile {
			if len(updates) != len(updateFiles) {
				return errors.New("missing or extra update files")
			}
		} else {
			if len(updates) != len(updateValues) && len(updates) > 0 {
				return errors.New("missing or extra update values")
			}
		}
		if useReplaceFile {
			if len(replaces) != len(replaceFiles) {
				return errors.New("missing or extra replace files")
			}
		} else {
			if len(replaces) != len(replaceValues) && len(replaces) > 0 {
				return errors.New("missing or extra replace values")
			}
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
		req := &gnmi.SetRequest{
			Prefix:  gnmiPrefix,
			Delete:  make([]*gnmi.Path, 0, len(deletes)),
			Replace: make([]*gnmi.Update, 0, len(replaces)),
			Update:  make([]*gnmi.Update, 0, len(updates)),
		}
		for _, p := range deletes {
			gnmiPath, err := xpath.ToGNMIPath(p)
			if err != nil {
				log.Printf("path '%s' parse error: %v", p, err)
				continue
			}
			req.Delete = append(req.Delete, gnmiPath)
		}
		for i, p := range updates {
			gnmiPath, err := xpath.ToGNMIPath(p)
			if err != nil {
				log.Print(err)
			}
			var updateData []byte
			if useUpdateFile {
				updateData, err = ioutil.ReadFile(updateFiles[i])
				if err != nil {
					log.Printf("error reading data from file %v: skipping path '%s'", updateFiles[i], p)
					continue
				}
			} else {
				buff := new(bytes.Buffer)
				err = json.NewEncoder(buff).Encode(strings.TrimRight(strings.TrimLeft(updateValues[i], "["), "]"))
				if err != nil {
					log.Printf("error encoding json value: %v", err)
					continue
				}
				updateData = buff.Bytes()
			}
			req.Update = append(req.Update, &gnmi.Update{
				Path: gnmiPath,
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_JsonVal{
						JsonVal: bytes.Trim(updateData, " \r\n\t"),
					}}})
		}
		for i, p := range replaces {
			gnmiPath, err := xpath.ToGNMIPath(p)
			if err != nil {
				log.Print(err)
			}
			var replaceData []byte
			if useReplaceFile {
				replaceData, err = ioutil.ReadFile(replaceFiles[i])
				if err != nil {
					log.Printf("error reading data from file %v: skipping path '%s'", replaceFiles[i], p)
					continue
				}
			} else {
				buff := new(bytes.Buffer)
				err = json.NewEncoder(buff).Encode(strings.TrimRight(strings.TrimLeft(replaceValues[i], "["), "]"))
				if err != nil {
					log.Printf("error encoding json value: %v", err)
					continue
				}
				replaceData = buff.Bytes()
			}
			req.Replace = append(req.Replace, &gnmi.Update{
				Path: gnmiPath,
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_JsonVal{
						JsonVal: bytes.Trim([]byte(replaceData), " \r\n\t"),
					}}})
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
				fmt.Printf("[%s] gnmi set request :\n", address)
				fmt.Printf("[%s] gnmi set request : prefix: %v\n", address, gnmiPathToXPath(req.Prefix))
				if len(req.Delete) > 0 {
					for _, del := range req.Delete {
						fmt.Printf("[%s] gnmi set request : delete: %v\n", address, gnmiPathToXPath(del))
					}
				}
				if len(req.Update) > 0 {
					for _, upd := range req.Update {
						fmt.Printf("[%s] gnmi set request : update path : %v\n", address, gnmiPathToXPath(upd.Path))
						fmt.Printf("[%s] gnmi set request : update value: %v\n", address, upd.Val)
					}
				}
				if len(req.Replace) > 0 {
					for _, rep := range req.Replace {
						fmt.Printf("[%s] gnmi set request : replace path : %v\n", address, gnmiPathToXPath(rep.Path))
						fmt.Printf("[%s] gnmi set request : replace value: %v\n", address, rep.Val)
					}
				}
				response, err := client.Set(ctx, req)
				if err != nil {
					log.Printf("error sending set request: %v", err)
					return
				}
				fmt.Printf("[%s] gnmi set response:\n", address)
				fmt.Printf("[%s] gnmi set response: timestamp: %v\n", address, response.Timestamp)
				fmt.Printf("[%s] gnmi set response: prefix: %v\n", address, gnmiPathToXPath(response.Prefix))
				if response.Message != nil {
					fmt.Printf("[%s] gnmi set response: error: %v\n", address, response.Message.String())
				}
				for _, u := range response.Response {
					fmt.Printf("[%s] gnmi set response: result: op=%v path=%v\n", address, u.Op, gnmiPathToXPath(u.Path))
				}
			}(addr)
		}
		wg.Wait()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setCmd)

	setCmd.Flags().StringP("prefix", "", "", "set request prefix")
	setCmd.Flags().StringSliceP("delete", "", []string{""}, "set request path to be deleted")
	setCmd.Flags().StringSliceP("replace", "", []string{""}, "set request path to be replaced")
	setCmd.Flags().StringSliceP("update", "", []string{""}, "set request path to be updated")
	setCmd.Flags().StringSliceP("update-file", "", []string{""}, "set update request value in json file")
	setCmd.Flags().StringSliceP("replace-file", "", []string{""}, "set replace request value in json file")
	setCmd.Flags().StringSliceP("update-value", "", []string{""}, "set update request value")
	setCmd.Flags().StringSliceP("replace-value", "", []string{""}, "set replace request value")
	viper.BindPFlag("set-prefix", setCmd.Flags().Lookup("prefix"))
	viper.BindPFlag("delete", setCmd.Flags().Lookup("delete"))
	viper.BindPFlag("replace", setCmd.Flags().Lookup("replace"))
	viper.BindPFlag("update", setCmd.Flags().Lookup("update"))
	viper.BindPFlag("update-file", setCmd.Flags().Lookup("update-file"))
	viper.BindPFlag("replace-file", setCmd.Flags().Lookup("replace-file"))
	viper.BindPFlag("update-value", setCmd.Flags().Lookup("update-value"))
	viper.BindPFlag("replace-value", setCmd.Flags().Lookup("replace-value"))
}
