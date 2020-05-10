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
	"strconv"
	"strings"
	"sync"

	"github.com/google/gnxi/utils/xpath"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/metadata"
)

var vTypes = []string{"json", "json_ietf", "string", "int", "uint", "bool", "decimal", "float", "bytes", "ascii"}

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
		updates := viper.GetString("update")
		replaces := viper.GetString("replace")

		updatePaths := viper.GetStringSlice("update-path")
		replacePaths := viper.GetStringSlice("replace-path")
		updateFiles := viper.GetStringSlice("update-file")
		replaceFiles := viper.GetStringSlice("replace-file")
		updateValues := viper.GetStringSlice("update-value")
		replaceValues := viper.GetStringSlice("replace-value")
		delimiter := viper.GetString("delimiter")
		if (len(deletes)+len(updates)+len(replaces)) == 0 && (len(updatePaths)+len(replacePaths)) == 0 {
			return errors.New("no paths provided")
		}
		inlineUpdates := len(updates) > 0
		inlineReplaces := len(replaces) > 0
		useUpdateFile := len(updateFiles) > 0 && len(updateValues) == 0
		useReplaceFile := len(replaceFiles) > 0 && len(replaceValues) == 0
		updateTypes := make([]string, 0)
		replaceTypes := make([]string, 0)

		if viper.GetBool("debug") {
			log.Printf("deletes(%d)=%v\n", len(deletes), deletes)
			log.Printf("updates(%d)=%v\n", len(updates), updates)
			log.Printf("replaces(%d)=%v\n", len(replaces), replaces)
			log.Printf("delimiter=%v\n", delimiter)
			log.Printf("updates-paths(%d)=%v\n", len(updatePaths), updatePaths)
			log.Printf("replaces-paths(%d)=%v\n", len(replacePaths), replacePaths)
			log.Printf("updates-files(%d)=%v\n", len(updateFiles), updateFiles)
			log.Printf("replaces-files(%d)=%v\n", len(replaceFiles), replaceFiles)
			log.Printf("updates-values(%d)=%v\n", len(updateValues), updateValues)
			log.Printf("replaces-values(%d)=%v\n", len(replaceValues), replaceValues)
		}
		if inlineUpdates && !useUpdateFile {
			updateSlice := strings.Split(updates, delimiter)
			if len(updateSlice) < 3 {
				return fmt.Errorf("'%s' invalid inline update format: %v", updates, err)
			}
			updatePaths = append(updatePaths, updateSlice[0])
			updateTypes = append(updateTypes, updateSlice[1])
			updateValues = append(updateValues, strings.Join(updateSlice[2:], delimiter))
		}
		if inlineReplaces && !useReplaceFile {
			replaceSlice := strings.Split(replaces, delimiter)
			if len(replaceSlice) < 3 {
				return fmt.Errorf("'%s' invalid inline replace format: %v", replaces, err)
			}
			replacePaths = append(replacePaths, replaceSlice[0])
			replaceTypes = append(replaceTypes, replaceSlice[1])
			replaceValues = append(replaceValues, strings.Join(replaceSlice[2:], delimiter))
		}

		if useUpdateFile && !inlineUpdates {
			if len(updatePaths) != len(updateFiles) {
				return errors.New("missing or extra update files")
			}
		} else {
			if len(updatePaths) != len(updateValues) && len(updates) > 0 {
				return errors.New("missing or extra update values")
			}
		}
		if useReplaceFile && !inlineReplaces {
			if len(replacePaths) != len(replaceFiles) {
				return errors.New("missing or extra replace files")
			}
		} else {
			if len(replacePaths) != len(replaceValues) && len(replaces) > 0 {
				return errors.New("missing or extra replace values")
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

		for i, p := range updatePaths {
			gnmiPath, err := xpath.ToGNMIPath(p)
			if err != nil {
				log.Print(err)
			}
			value := new(gnmi.TypedValue)
			if useUpdateFile {
				var updateData []byte
				updateData, err = ioutil.ReadFile(updateFiles[i])
				if err != nil {
					log.Printf("error reading data from file %v: skipping path '%s'", updateFiles[i], p)
					continue
				}
				value.Value = &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(updateData, " \r\n\t"),
				}
			} else {
				var vType string
				if inlineUpdates {
					if len(updateTypes) > i {
						vType = updateTypes[i]
					} else {
						vType = "json"
					}
				}
				switch vType {
				case "json":
					buff := new(bytes.Buffer)
					err = json.NewEncoder(buff).Encode(strings.TrimRight(strings.TrimLeft(updateValues[i], "["), "]"))
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_JsonVal{
						JsonVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
					}
				case "json_ietf":
					buff := new(bytes.Buffer)
					err = json.NewEncoder(buff).Encode(strings.TrimRight(strings.TrimLeft(updateValues[i], "["), "]"))
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_JsonIetfVal{
						JsonIetfVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
					}
				case "ascii":
					value.Value = &gnmi.TypedValue_AsciiVal{
						AsciiVal: updateValues[i],
					}
				case "bool":
					bval, err := strconv.ParseBool(updateValues[i])
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_BoolVal{
						BoolVal: bval,
					}
				case "bytes":
					value.Value = &gnmi.TypedValue_BytesVal{
						BytesVal: []byte(updateValues[i]),
					}
				case "decimal":
					dVal := &gnmi.Decimal64{}
					value.Value = &gnmi.TypedValue_DecimalVal{
						DecimalVal: dVal,
					}
					log.Println("decimal type not implemented")
					return nil
				case "float":
					f, err := strconv.ParseFloat(updateValues[i], 32)
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_FloatVal{
						FloatVal: float32(f),
					}
				case "int":
					k, err := strconv.ParseInt(updateValues[i], 10, 64)
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_IntVal{
						IntVal: k,
					}
				case "uint":
					u, err := strconv.ParseUint(updateValues[i], 10, 64)
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_UintVal{
						UintVal: u,
					}
				case "string":
					value.Value = &gnmi.TypedValue_StringVal{
						StringVal: updateValues[i],
					}
				default:
					return fmt.Errorf("unknown type '%s', must be one of: %v", vType, vTypes)
				}
			}
			req.Update = append(req.Update, &gnmi.Update{
				Path: gnmiPath,
				Val:  value,
			})
		}
		for i, p := range replacePaths {
			gnmiPath, err := xpath.ToGNMIPath(p)
			if err != nil {
				log.Print(err)
			}
			value := new(gnmi.TypedValue)
			if useReplaceFile {
				var replaceData []byte
				replaceData, err = ioutil.ReadFile(replaceFiles[i])
				if err != nil {
					log.Printf("error reading data from file %v: skipping path '%s'", replaceFiles[i], p)
					continue
				}
				value.Value = &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(replaceData, " \r\n\t"),
				}
			} else {
				var vType string
				if inlineReplaces {
					if len(replaceTypes) > i {
						vType = replaceTypes[i]
					} else {
						vType = "json"
					}
				}
				switch vType {
				case "json":
					buff := new(bytes.Buffer)
					err = json.NewEncoder(buff).Encode(strings.TrimRight(strings.TrimLeft(replaceValues[i], "["), "]"))
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_JsonVal{
						JsonVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
					}
				case "json_ietf":
					buff := new(bytes.Buffer)
					err = json.NewEncoder(buff).Encode(strings.TrimRight(strings.TrimLeft(replaceValues[i], "["), "]"))
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_JsonIetfVal{
						JsonIetfVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
					}
				case "ascii":
					value.Value = &gnmi.TypedValue_AsciiVal{
						AsciiVal: replaceValues[i],
					}
				case "bool":
					bval, err := strconv.ParseBool(replaceValues[i])
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_BoolVal{
						BoolVal: bval,
					}
				case "bytes":
					value.Value = &gnmi.TypedValue_BytesVal{
						BytesVal: []byte(replaceValues[i]),
					}
				case "decimal":
					dVal := &gnmi.Decimal64{}
					value.Value = &gnmi.TypedValue_DecimalVal{
						DecimalVal: dVal,
					}
					log.Println("decimal type not implemented")
					return nil
				case "float":
					f, err := strconv.ParseFloat(replaceValues[i], 32)
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_FloatVal{
						FloatVal: float32(f),
					}
				case "int":
					i, err := strconv.ParseInt(replaceValues[i], 10, 64)
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_IntVal{
						IntVal: i,
					}
				case "uint":
					i, err := strconv.ParseUint(replaceValues[i], 10, 64)
					if err != nil {
						return err
					}
					value.Value = &gnmi.TypedValue_UintVal{
						UintVal: i,
					}
				case "string":
					value.Value = &gnmi.TypedValue_StringVal{
						StringVal: replaceValues[i],
					}
				default:
					return fmt.Errorf("unknown type '%s', must be one of: %v", vType, vTypes)
				}
			}
			req.Replace = append(req.Replace, &gnmi.Update{
				Path: gnmiPath,
				Val:  value,
			})
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
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx = metadata.AppendToOutgoingContext(ctx, "username", username, "password", password)
				printPrefix := ""
				if len(addresses) > 1 && !viper.GetBool("no-prefix") {
					printPrefix = fmt.Sprintf("[%s] ", address)
				}
				fmt.Printf("%sgnmi set request :\n", printPrefix)
				fmt.Printf("%sgnmi set request : prefix: %v\n", printPrefix, gnmiPathToXPath(req.Prefix))
				if len(req.Delete) > 0 {
					for _, del := range req.Delete {
						fmt.Printf("%sgnmi set request : delete: %v\n", printPrefix, gnmiPathToXPath(del))
					}
				}
				if len(req.Update) > 0 {
					for _, upd := range req.Update {
						fmt.Printf("%sgnmi set request : update path : %v\n", printPrefix, gnmiPathToXPath(upd.Path))
						fmt.Printf("%sgnmi set request : update value: %v\n", printPrefix, upd.Val)
					}
				}
				if len(req.Replace) > 0 {
					for _, rep := range req.Replace {
						fmt.Printf("%sgnmi set request : replace path : %v\n", printPrefix, gnmiPathToXPath(rep.Path))
						fmt.Printf("%sgnmi set request : replace value: %v\n", printPrefix, rep.Val)
					}
				}
				response, err := client.Set(ctx, req)
				if err != nil {
					log.Printf("error sending set request: %v", err)
					return
				}
				fmt.Printf("%sgnmi set response:\n", printPrefix)
				fmt.Printf("%sgnmi set response: timestamp: %v\n", printPrefix, response.Timestamp)
				fmt.Printf("%sgnmi set response: prefix: %v\n", printPrefix, gnmiPathToXPath(response.Prefix))
				if response.Message != nil {
					fmt.Printf("%sgnmi set response: error: %v\n", printPrefix, response.Message.String())
				}
				for _, u := range response.Response {
					fmt.Printf("%sgnmi set response: result: op=%v path=%v\n", printPrefix, u.Op, gnmiPathToXPath(u.Path))
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

	setCmd.Flags().StringSliceP("delete", "", []string{}, "set request path to be deleted")

	setCmd.Flags().StringP("replace", "", "", fmt.Sprintf("set request path:::type:::value to be replaced, type must be one of %v", vTypes))
	setCmd.Flags().StringP("update", "", "", fmt.Sprintf("set request path:::type:::value to be updated, type must be one of %v", vTypes))

	setCmd.Flags().StringSliceP("replace-path", "", []string{""}, "set request path to be replaced")
	setCmd.Flags().StringSliceP("update-path", "", []string{""}, "set request path to be updated")
	setCmd.Flags().StringSliceP("update-file", "", []string{""}, "set update request value in json file")
	setCmd.Flags().StringSliceP("replace-file", "", []string{""}, "set replace request value in json file")
	setCmd.Flags().StringSliceP("update-value", "", []string{""}, "set update request value")
	setCmd.Flags().StringSliceP("replace-value", "", []string{""}, "set replace request value")
	setCmd.Flags().StringP("delimiter", "", ":::", "set update/replace delimiter between path,type,value")

	viper.BindPFlag("set-prefix", setCmd.Flags().Lookup("prefix"))
	viper.BindPFlag("delete", setCmd.Flags().Lookup("delete"))
	viper.BindPFlag("replace", setCmd.Flags().Lookup("replace"))
	viper.BindPFlag("update", setCmd.Flags().Lookup("update"))
	viper.BindPFlag("update-path", setCmd.Flags().Lookup("update-path"))
	viper.BindPFlag("replace-path", setCmd.Flags().Lookup("replace-path"))
	viper.BindPFlag("update-file", setCmd.Flags().Lookup("update-file"))
	viper.BindPFlag("replace-file", setCmd.Flags().Lookup("replace-file"))
	viper.BindPFlag("update-value", setCmd.Flags().Lookup("update-value"))
	viper.BindPFlag("replace-value", setCmd.Flags().Lookup("replace-value"))
	viper.BindPFlag("delimiter", setCmd.Flags().Lookup("delimiter"))
}
