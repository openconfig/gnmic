// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/fullstorydev/grpcurl"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	nokiasros "github.com/karimra/sros-dialout"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/app"
	"github.com/openconfig/gnmic/pkg/outputs"
)

// New returns the listen command tree.
func New(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "listens for telemetry dialout updates from the node",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			gApp.Config.SetLocalFlagsFromFile(cmd)
			if len(gApp.Config.Address) == 0 {
				return fmt.Errorf("no address specified")
			}
			if len(gApp.Config.Address) > 1 {
				fmt.Fprintf(os.Stderr, "multiple addresses specified, listening only on %s\n", gApp.Config.Address[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			server := new(dialoutTelemetryServer)
			server.ctx = ctx

			opts := []grpc.ServerOption{
				grpc.MaxConcurrentStreams(gApp.Config.LocalFlags.ListenMaxConcurrentStreams),
			}
			if gApp.Config.MaxMsgSize > 0 {
				opts = append(opts, grpc.MaxRecvMsgSize(gApp.Config.MaxMsgSize))
			}

			if gApp.Config.LocalFlags.ListenPrometheusAddress != "" {
				server.reg = prometheus.NewRegistry()
				grpcMetrics := grpc_prometheus.NewServerMetrics()
				opts = append(opts,
					grpc.StreamInterceptor(grpcMetrics.StreamServerInterceptor()),
				)
				server.reg.MustRegister(grpcMetrics)
			}

			if len(gApp.Config.ProtoFile) > 0 {
				gApp.Logger.Printf("loading proto files...")
				descSource, err := grpcurl.DescriptorSourceFromProtoFiles(gApp.Config.ProtoDir, gApp.Config.ProtoFile...)
				if err != nil {
					gApp.Logger.Printf("failed to load proto files: %v", err)
					return err
				}
				server.rootDesc, err = descSource.FindSymbol("Nokia.SROS.root")
				if err != nil {
					gApp.Logger.Printf("could not get symbol 'Nokia.SROS.root': %v", err)
					return err
				}
				gApp.Logger.Printf("loaded proto files")
			}

			server.Outputs = make(map[string]outputs.Output)
			outCfgs, err := gApp.Config.GetOutputs()
			if err != nil {
				return err
			}

			for name, outConf := range outCfgs {
				if outType, ok := outConf["type"]; ok {
					if initializer, ok := outputs.Outputs[outType.(string)]; ok {
						out := initializer()
						go out.Init(ctx, name, outConf,
							outputs.WithLogger(gApp.Logger),
							outputs.WithName(gApp.Config.InstanceName),
							outputs.WithClusterName(gApp.Config.ClusterName),
							outputs.WithRegistry(server.reg),
							outputs.WithConfigStore(gApp.Store),
						)
						server.Outputs[name] = out
					}
				}
			}

			defer func() {
				for _, o := range server.Outputs {
					o.Close()
				}
			}()

			server.listener, err = net.Listen("tcp", gApp.Config.Address[0])
			if err != nil {
				return err
			}
			gApp.Logger.Printf("waiting for connections on %s", gApp.Config.Address[0])

			if gApp.Config.TLSKey != "" && gApp.Config.TLSCert != "" {
				tlsConfig, err := utils.NewTLSConfig(
					gApp.Config.TLSCa,
					gApp.Config.TLSCert,
					gApp.Config.TLSKey,
					"request",
					false,
					true,
				)
				if err != nil {
					return err
				}
				opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConfig)))
			}

			server.grpcServer = grpc.NewServer(opts...)
			nokiasros.RegisterDialoutTelemetryServer(server.grpcServer, server)

			if gApp.Config.LocalFlags.ListenPrometheusAddress != "" {
				grpc_prometheus.Register(server.grpcServer)

				httpServer := &http.Server{
					Handler: promhttp.HandlerFor(server.reg, promhttp.HandlerOpts{}),
					Addr:    gApp.Config.LocalFlags.ListenPrometheusAddress,
				}
				go func() {
					if err := httpServer.ListenAndServe(); err != nil {
						gApp.Logger.Printf("Unable to start prometheus http server.")
					}
				}()
				defer httpServer.Close()
			}
			server.gApp = gApp
			server.grpcServer.Serve(server.listener)
			defer server.grpcServer.Stop()
			return nil
		},
		SilenceUsage: true,
	}
	cmd.Flags().Uint32P("max-concurrent-streams", "", 256, "max concurrent streams gnmic can receive per transport")
	cmd.Flags().StringP("prometheus-address", "", "", "prometheus server address")
	gApp.Config.FileConfig.BindPFlag("listen-max-concurrent-streams", cmd.LocalFlags().Lookup("max-concurrent-streams"))
	gApp.Config.FileConfig.BindPFlag("listen-prometheus-address", cmd.LocalFlags().Lookup("prometheus-address"))
	return cmd
}

type dialoutTelemetryServer struct {
	listener   net.Listener
	grpcServer *grpc.Server
	rootDesc   desc.Descriptor

	Outputs map[string]outputs.Output

	ctx context.Context

	gApp *app.App
	reg  *prometheus.Registry
}

func (s *dialoutTelemetryServer) Publish(stream nokiasros.DialoutTelemetry_PublishServer) error {
	peer, ok := peer.FromContext(stream.Context())
	if ok && s.gApp.Config.Debug {
		b, err := json.Marshal(peer)
		if err != nil {
			s.gApp.Logger.Printf("failed to marshal peer data: %v", err)
		} else {
			s.gApp.Logger.Printf("received Publish RPC from peer=%s", string(b))
		}
	}
	md, ok := metadata.FromIncomingContext(stream.Context())
	if ok && s.gApp.Config.Debug {
		b, err := json.Marshal(md)
		if err != nil {
			s.gApp.Logger.Printf("failed to marshal context metadata: %v", err)
		} else {
			s.gApp.Logger.Printf("received http2_header=%s", string(b))
		}
	}
	outMeta := outputs.Meta{}
	if sn, ok := md["subscription-name"]; ok {
		if len(sn) > 0 {
			outMeta["subscription-name"] = sn[0]
		}
	} else {
		s.gApp.Logger.Println("could not find subscription-name in http2 headers")
	}
	outMeta["source"] = peer.Addr.String()
	if systemName, ok := md["system-name"]; ok {
		if len(systemName) > 0 {
			outMeta["system-name"] = systemName[0]
		}
	} else {
		s.gApp.Logger.Println("could not find system-name in http2 headers")
	}
	for {
		subResp, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				s.gApp.Logger.Printf("gRPC dialout receive error: %v", err)
			}
			break
		}
		err = stream.Send(&nokiasros.PublishResponse{})
		if err != nil {
			s.gApp.Logger.Printf("error sending publish response to server: %v", err)
		}
		switch resp := subResp.Response.(type) {
		case *gnmi.SubscribeResponse_Update:
			if s.rootDesc != nil {
				for _, update := range resp.Update.Update {
					switch update.Val.Value.(type) {
					case *gnmi.TypedValue_ProtoBytes:
						m := dynamic.NewMessage(s.rootDesc.GetFile().FindMessage("Nokia.SROS.root"))
						err := m.Unmarshal(update.Val.GetProtoBytes())
						if err != nil {
							s.gApp.Logger.Printf("failed to unmarshal m: %v", err)
						}
						jsondata, err := m.MarshalJSON()
						if err != nil {
							s.gApp.Logger.Printf("failed to marshal dynamic proto msg: %v", err)
							continue
						}
						if s.gApp.Config.Debug {
							s.gApp.Logger.Printf("json format=%s", string(jsondata))
						}
						update.Val.Value = &gnmi.TypedValue_JsonVal{JsonVal: jsondata}
					}
				}
			}
			for _, o := range s.Outputs {
				go o.Write(s.ctx, subResp, outMeta)
			}

		case *gnmi.SubscribeResponse_SyncResponse:
			s.gApp.Logger.Printf("received sync response=%+v from %s", resp.SyncResponse, outMeta["source"])
		}
	}
	return nil
}
