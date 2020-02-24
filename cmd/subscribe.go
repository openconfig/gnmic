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
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/gnxi/utils/xpath"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/metadata"
)

type msg struct {
	//Source    string                 `json:"source,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
	Prefix    string                 `json:"prefix,omitempty"`
	Values    map[string]interface{} `json:"values,omitempty"`
}

// subscribeCmd represents the subscribe command
var subscribeCmd = &cobra.Command{
	Use:     "subscribe",
	Aliases: []string{"sub"},
	Short:   "subscribe to gnmi updates on targets",

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
		subscReq, err := createSubscribeRequest()
		if err != nil {
			return err
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
				printPrefix := fmt.Sprintf("[%s] ", address)
				conn, err := createGrpcConn(address)
				if err != nil {
					log.Printf("connection to %s failed: %v", address, err)
					return
				}
				client := gnmi.NewGNMIClient(conn)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx = metadata.AppendToOutgoingContext(ctx, "username", username, "password", password)

				subscribeClient, err := client.Subscribe(ctx)
				if err != nil {
					log.Printf("error creating subscribe client: %v", err)
					return
				}
				err = subscribeClient.Send(subscReq)
				if err != nil {
					log.Printf("subscribe error: %v", err)
					return
				}
				switch subscReq.GetSubscribe().Mode {
				case gnmi.SubscriptionList_ONCE, gnmi.SubscriptionList_STREAM:
					for {
						subscribeRsp, err := subscribeClient.Recv()
						if err != nil {
							log.Printf("rcv error: %v", err)
							return
						}
						switch resp := subscribeRsp.Response.(type) {
						case *gnmi.SubscribeResponse_Update:
							msg := &msg{
								Values: make(map[string]interface{}),
							}
							fmt.Printf("%supdate received at %s\n", printPrefix, time.Now().Format(time.RFC3339Nano))
							msg.Timestamp = resp.Update.Timestamp
							msg.Prefix = gnmiPathToXPath(resp.Update.Prefix)
							for _, u := range resp.Update.Update {
								pathElems := make([]string, 0, len(u.Path.Elem))
								for _, pElem := range u.Path.Elem {
									pathElems = append(pathElems, pElem.GetName())
								}
								var value interface{}
								var jsondata []byte
								switch val := u.Val.Value.(type) {
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
								if value == nil {
									err = json.Unmarshal(jsondata, &value)
									if err != nil {
										log.Printf("error unmarshling jsonVal '%s'", string(jsondata))
										continue
									}
								}
								msg.Values[strings.Join(pathElems, "/")] = value
							}
							dMsg, err := json.MarshalIndent(msg, printPrefix, "  ")
							if err != nil {
								log.Printf("error marshling json msg:%v", err)
								continue
							}
							fmt.Printf("%s%s\n", printPrefix, string(dMsg))
						case *gnmi.SubscribeResponse_SyncResponse:
							fmt.Printf("%ssync response: %+v\n", printPrefix, resp.SyncResponse)
							if subscReq.GetSubscribe().Mode == gnmi.SubscriptionList_ONCE {
								return
							}
						}
						fmt.Println()
					}
				case gnmi.SubscriptionList_POLL:
					for {
						subscribeRsp, err := subscribeClient.Recv()
						if err != nil {
							log.Printf("rcv error: %v", err)
							return
						}
						switch resp := subscribeRsp.Response.(type) {
						case *gnmi.SubscribeResponse_Update:
						case *gnmi.SubscribeResponse_SyncResponse:
							fmt.Printf("%ssync response: %+v\n", printPrefix, resp.SyncResponse)
						}

					}
				}
			}(addr)
		}
		if subscReq.GetSubscribe().Mode == gnmi.SubscriptionList_POLL {

		}
		wg.Wait()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(subscribeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// subscribeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// subscribeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	subscribeCmd.Flags().StringP("prefix", "", "", "subscribe request prefix")
	subscribeCmd.Flags().StringSliceP("path", "", []string{"/"}, "subscribe request paths")
	subscribeCmd.Flags().Int32P("qos", "q", 20, "qos marking")
	subscribeCmd.Flags().BoolP("updates-only", "", false, "only updates to current state should be sent")
	subscribeCmd.Flags().StringP("subscription-mode", "", "stream", "one of: once, stream, poll")
	subscribeCmd.Flags().StringP("stream-subscription-mode", "", "target-defined", "one of: on-change, sample, target-defined")
	subscribeCmd.Flags().StringP("sampling-interval", "i", "10s",
		"sampling interval as a decimal number and a suffix unit, such as \"10s\" or \"1m30s\", minimum is 1s")
	subscribeCmd.Flags().BoolP("suppress-redundant", "", false, "suppress redundant update if the subscribed value didnt not change")
	subscribeCmd.Flags().StringP("heartbeat-interval", "", "0s", "heartbeat interval in case suppress-redundant is enabled")
	//
	viper.BindPFlag("sub-prefix", subscribeCmd.Flags().Lookup("prefix"))
	viper.BindPFlag("sub-path", subscribeCmd.Flags().Lookup("path"))
	viper.BindPFlag("qos", subscribeCmd.Flags().Lookup("qos"))
	viper.BindPFlag("updates-only", subscribeCmd.Flags().Lookup("updates-only"))
	viper.BindPFlag("subscription-mode", subscribeCmd.Flags().Lookup("subscription-mode"))
	viper.BindPFlag("stream-subscription-mode", subscribeCmd.Flags().Lookup("stream-subscription-mode"))
	viper.BindPFlag("sampling-interval", subscribeCmd.Flags().Lookup("sampling-interval"))
	viper.BindPFlag("suppress-redundant", subscribeCmd.Flags().Lookup("suppress-redundant"))
	viper.BindPFlag("heartbeat-interval", subscribeCmd.Flags().Lookup("heartbeat-interval"))
}

func createSubscribeRequest() (*gnmi.SubscribeRequest, error) {
	paths := viper.GetStringSlice("sub-path")
	if len(paths) == 0 {
		return nil, errors.New("no path provided")
	}
	gnmiPrefix, err := xpath.ToGNMIPath(viper.GetString("sub-prefix"))
	if err != nil {
		return nil, fmt.Errorf("prefix parse error: %v", err)
	}
	encodingVal, ok := gnmi.Encoding_value[strings.Replace(strings.ToUpper(viper.GetString("encoding")), "-", "_", -1)]
	if !ok {
		return nil, fmt.Errorf("invalid encoding type '%s'", viper.GetString("encoding"))
	}
	modeVal, ok := gnmi.SubscriptionList_Mode_value[strings.ToUpper(viper.GetString("subscription-mode"))]
	if !ok {
		return nil, fmt.Errorf("invalid subscription list type '%s'", viper.GetString("subscription-mode"))
	}
	qos := &gnmi.QOSMarking{Marking: viper.GetUint32("qos")}
	samplingInterval, err := time.ParseDuration(viper.GetString("sampling-interval"))
	if err != nil {
		return nil, err
	}
	heartbeatInterval, err := time.ParseDuration(viper.GetString("heartbeat-interval"))
	if err != nil {
		return nil, err
	}
	subscriptions := make([]*gnmi.Subscription, len(paths))
	for i, p := range paths {
		gnmiPath, err := xpath.ToGNMIPath(p)
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		subscriptions[i] = &gnmi.Subscription{Path: gnmiPath}
		switch gnmi.SubscriptionList_Mode(modeVal) {
		case gnmi.SubscriptionList_STREAM:
			mode, ok := gnmi.SubscriptionMode_value[strings.Replace(strings.ToUpper(viper.GetString("stream-subscription-mode")), "-", "_", -1)]
			if !ok {
				return nil, fmt.Errorf("invalid streamed subscription mode %s", viper.GetString("stream-subscription-mode"))
			}
			subscriptions[i].Mode = gnmi.SubscriptionMode(mode)
			switch gnmi.SubscriptionMode(mode) {
			case gnmi.SubscriptionMode_ON_CHANGE:
				subscriptions[i].HeartbeatInterval = uint64(heartbeatInterval.Nanoseconds())
			case gnmi.SubscriptionMode_SAMPLE:
				subscriptions[i].SampleInterval = uint64(samplingInterval.Nanoseconds())
				subscriptions[i].SuppressRedundant = viper.GetBool("suppress-redundant")
				if subscriptions[i].SuppressRedundant {
					subscriptions[i].HeartbeatInterval = uint64(heartbeatInterval.Nanoseconds())
				}
			case gnmi.SubscriptionMode_TARGET_DEFINED:
				subscriptions[i].SampleInterval = uint64(samplingInterval.Nanoseconds())
				subscriptions[i].SuppressRedundant = viper.GetBool("suppress-redundant")
				if subscriptions[i].SuppressRedundant {
					subscriptions[i].HeartbeatInterval = uint64(heartbeatInterval.Nanoseconds())
				}
			}
		}
	}
	return &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: &gnmi.SubscriptionList{
				Prefix:       gnmiPrefix,
				Mode:         gnmi.SubscriptionList_Mode(modeVal),
				Encoding:     gnmi.Encoding(encodingVal),
				Subscription: subscriptions,
				Qos:          qos,
			},
		},
	}, nil

}
