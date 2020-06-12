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
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/gnxi/utils/xpath"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/prototext"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "run gnmi get on targets",

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		setupCloseHandler(cancel)
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
		encodingVal, ok := gnmi.Encoding_value[strings.Replace(strings.ToUpper(viper.GetString("encoding")), "-", "_", -1)]
		if !ok {
			return fmt.Errorf("invalid encoding type '%s'", viper.GetString("encoding"))
		}
		req := &gnmi.GetRequest{
			UseModels: make([]*gnmi.ModelData, 0),
			Path:      make([]*gnmi.Path, 0),
			Encoding:  gnmi.Encoding(encodingVal),
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
			gnmiPath, err := xpath.ToGNMIPath(strings.TrimSpace(p))
			if err != nil {
				return fmt.Errorf("path parse error: %v", err)
			}
			req.Path = append(req.Path, gnmiPath)
		}
		dataType := viper.GetString("get-type")
		if dataType != "" {
			dti, ok := gnmi.GetRequest_DataType_value[strings.ToUpper(dataType)]
			if !ok {
				return fmt.Errorf("unknown data type %s", dataType)
			}
			req.Type = gnmi.GetRequest_DataType(dti)
		}
		wg := new(sync.WaitGroup)
		wg.Add(len(addresses))
		lock := new(sync.Mutex)
		for _, addr := range addresses {
			go getRequest(ctx, req, addr, username, password, wg, lock)
		}
		wg.Wait()
		return nil
	},
}

func getRequest(ctx context.Context, req *gnmi.GetRequest, address, username, password string, wg *sync.WaitGroup, lock *sync.Mutex) {
	defer wg.Done()
	_, _, err := net.SplitHostPort(address)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			address = net.JoinHostPort(address, defaultGrpcPort)
		} else {
			logger.Printf("error parsing address '%s': %v", address, err)
			return
		}
	}
	conn, err := createGrpcConn(address)
	if err != nil {
		logger.Printf("connection to %s failed: %v", address, err)
		return
	}
	client := gnmi.NewGNMIClient(conn)
	nctx, cancel := context.WithCancel(ctx)
	defer cancel()
	nctx = metadata.AppendToOutgoingContext(nctx, "username", username, "password", password)
	xreq := req
	models := viper.GetStringSlice("get-model")
	if len(models) > 0 {
		spModels, unspModels, err := filterModels(nctx, client, models)
		if err != nil {
			logger.Printf("failed getting supported models from '%s': %v", address, err)
			return
		}
		if len(unspModels) > 0 {
			logger.Printf("found unsupported models for target '%s': %+v", address, unspModels)
		}
		for _, m := range spModels {
			xreq.UseModels = append(xreq.UseModels, m)
		}
	}
	logger.Printf("sending gNMI GetRequest: prefix='%v', path='%v', type='%v', encoding='%v', models='%+v', extension='%+v' to %s", xreq.Prefix, xreq.Path, xreq.Type, xreq.Encoding, xreq.UseModels, xreq.Extension, address)
	response, err := client.Get(nctx, xreq)
	if err != nil {
		logger.Printf("failed sending GetRequest to %s: %v", address, err)
		return
	}
	lock.Lock()
	printGetResponse(address, response)
	lock.Unlock()
}

func printGetResponse(address string, response *gnmi.GetResponse) {
	printPrefix := ""
	addresses := viper.GetStringSlice("address")
	if len(addresses) > 1 && !viper.GetBool("no-prefix") {
		printPrefix = fmt.Sprintf("[%s] ", address)
	}
	if viper.GetString("format") == "textproto" {
		fmt.Printf("%s\n", indent(printPrefix, prototext.Format(response)))
		return
	}
	for _, notif := range response.Notification {
		msg := new(msg)
		msg.Timestamp = notif.Timestamp
		t := time.Unix(0, notif.Timestamp)
		msg.Time = &t
		msg.Prefix = gnmiPathToXPath(notif.Prefix)
		for i, upd := range notif.Update {
			if upd.Val == nil {
				if viper.GetBool("debug") {
					logger.Printf("DEBUG: got a nil val update: %+v", upd)
				}
				continue
			}
			pathElems := make([]string, 0, len(upd.Path.Elem))
			for _, pElem := range upd.Path.Elem {
				pathElems = append(pathElems, pElem.GetName())
			}
			value, err := getValue(upd.Val)
			if err != nil {
				logger.Println(err)
			}
			msg.Updates = append(msg.Updates,
				&update{
					Path:   gnmiPathToXPath(upd.Path),
					Values: make(map[string]interface{}),
				})
			msg.Updates[i].Values[strings.Join(pathElems, "/")] = value
		}
		dMsg, err := json.MarshalIndent(msg, printPrefix, "  ")
		if err != nil {
			logger.Printf("error marshling json msg:%v", err)
			return
		}
		fmt.Printf("%s%s\n", printPrefix, string(dMsg))
	}
	fmt.Println()
}

func stringInList(s string, l []string) bool {
	for _, ss := range l {
		if s == ss {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(getCmd)

	getCmd.Flags().StringSliceP("path", "", []string{""}, "get request paths")
	getCmd.MarkFlagRequired("path")
	getCmd.Flags().StringP("prefix", "", "", "get request prefix")
	getCmd.Flags().StringSliceP("model", "", []string{""}, "get request model(s)")
	getCmd.Flags().StringP("type", "t", "ALL", "the type of data that is requested from the target. one of: ALL, CONFIG, STATE, OPERATIONAL")
	viper.BindPFlag("get-path", getCmd.Flags().Lookup("path"))
	viper.BindPFlag("get-prefix", getCmd.Flags().Lookup("prefix"))
	viper.BindPFlag("get-model", getCmd.Flags().Lookup("model"))
	viper.BindPFlag("get-type", getCmd.Flags().Lookup("type"))
}
