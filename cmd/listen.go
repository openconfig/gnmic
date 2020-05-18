/*
Copyright © 2020 Karim Radhouani <medkarimrdi@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"io"
	"net"

	nokiasros "github.com/karimra/sros-dialout"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// listenCmd represents the listen command
var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "listens for telemetry dialout updates from the node",

	RunE: func(cmd *cobra.Command, args []string) error {
		server := new(dialoutTelemetryServer)
		address := viper.GetString("address")
		var err error
		server.listener, err = net.Listen("tcp", address)
		if err != nil {
			return err
		}
		var opts []grpc.ServerOption
		if viper.GetInt("max-msg-size") > 0 {
			opts = append(opts, grpc.MaxRecvMsgSize(viper.GetInt("max-msg-size")))
		}
		server.grpcServer = grpc.NewServer(opts...)
		//
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listenCmd)

}

type dialoutTelemetryServer struct {
	listener   net.Listener
	grpcServer *grpc.Server
}

func (s *dialoutTelemetryServer) Publish(stream nokiasros.DialoutTelemetry_PublishServer) {
	for {
		peer, ok := peer.FromContext(stream.Context())
		if ok {
			logger.Printf("received dialout connection from peer: %+v", peer)
		}
		md, ok := metadata.FromIncomingContext(stream.Context())
		if ok {
			logger.Printf("received dialout Publish:::metadata = %+v", md)
		}

		subResp, err := stream.Recv()
		logger.Printf("%+v", subResp)
		if err != nil {
			if err != io.EOF {
				logger.Printf(fmt.Errorf("GRPC dialout receive error: %v", err))
			}
			break
		}
		err = stream.Send(&nokiasros.PublishResponse{})
		if err != nil {
			logger.Debugf("error sending publish response to server: %v", err)
		}
		subName := ""
		if sn, ok := md["subscription-name"]; ok {
			if len(sn) > 0 {
				subName = sn[0]
			}
		}
		logger.Debugf("subscription-name: %v", subName)

	}
}
