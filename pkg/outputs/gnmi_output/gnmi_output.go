// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package gnmi_output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"text/template"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
)

const (
	loggingPrefix           = "[gnmi_output:%s] "
	defaultMaxSubscriptions = 64
	defaultMaxGetRPC        = 64
	defaultAddress          = ":57400"
)

func init() {
	outputs.Register("gnmi", func() outputs.Output {
		return &gNMIOutput{
			cfg:    new(config),
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// gNMIOutput //
type gNMIOutput struct {
	outputs.BaseOutput
	cfg       *config
	logger    *log.Logger
	targetTpl *template.Template
	//
	srv     *server
	grpcSrv *grpc.Server
	c       *cache.Cache
	reg     *prometheus.Registry
}

type config struct {
	//Name             string `mapstructure:"name,omitempty"`
	Address          string           `mapstructure:"address,omitempty"`
	TargetTemplate   string           `mapstructure:"target-template,omitempty"`
	MaxSubscriptions int64            `mapstructure:"max-subscriptions,omitempty"`
	MaxUnaryRPC      int64            `mapstructure:"max-unary-rpc,omitempty"`
	TLS              *types.TLSConfig `mapstructure:"tls,omitempty"`
	EnableMetrics    bool             `mapstructure:"enable-metrics,omitempty"`
	Debug            bool             `mapstructure:"debug,omitempty"`
}

func (g *gNMIOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, g.cfg)
	if err != nil {
		return err
	}
	g.c = cache.New(nil)
	g.srv = g.newServer()

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}
	if options.Logger != nil && g.logger != nil {
		g.logger.SetOutput(options.Logger.Writer())
		g.logger.SetFlags(options.Logger.Flags())
	}
	g.reg = options.Registry
	g.registerMetrics()
	err = g.setDefaults()
	if err != nil {
		return err
	}
	g.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))
	if g.targetTpl == nil {
		g.targetTpl, err = gtemplate.CreateTemplate(fmt.Sprintf("%s-target-template", name), g.cfg.TargetTemplate)
		if err != nil {
			return err
		}
	}
	err = g.startGRPCServer()
	if err != nil {
		return err
	}
	g.logger.Printf("started gnmi output: %v", g)
	return nil
}

func (g *gNMIOutput) Update(ctx context.Context, cfg map[string]any) error {
	return errors.New("not implemented for this output type")
}

func (g *gNMIOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	var err error
	rsp, err = outputs.AddSubscriptionTarget(rsp, meta, "if-not-present", g.targetTpl)
	if err != nil {
		g.logger.Printf("failed to add target to the response: %v", err)
	}
	switch rsp := rsp.(type) {
	case *gnmi.SubscribeResponse:
		switch rsp := rsp.Response.(type) {
		case *gnmi.SubscribeResponse_Update:
			target := rsp.Update.GetPrefix().GetTarget()
			if target == "" {
				g.logger.Printf("response missing target")
				return
			}
			if !g.c.HasTarget(target) {
				g.c.Add(target)
				g.logger.Printf("target %q added to the local cache", target)
			}
			if g.cfg.Debug {
				g.logger.Printf("updating target %q local cache", target)
			}
			err = g.c.GnmiUpdate(rsp.Update)
			if err != nil {
				g.logger.Printf("failed to update gNMI cache: %v", err)
				return
			}
		case *gnmi.SubscribeResponse_SyncResponse:
		}
	}
}

func (g *gNMIOutput) WriteEvent(context.Context, *formatters.EventMsg) {}

func (g *gNMIOutput) Close() error {
	//g.teardown()
	g.grpcSrv.Stop()
	return nil
}

func (g *gNMIOutput) registerMetrics() {
	if !g.cfg.EnableMetrics {
		return
	}
	if g.reg == nil {
		g.logger.Printf("ERR: output metrics enabled but main registry is not initialized, enable main metrics under `api-server`")
		return
	}
	srvMetrics := grpc_prometheus.NewServerMetrics()
	srvMetrics.InitializeMetrics(g.grpcSrv)
	if err := g.reg.Register(srvMetrics); err != nil {
		g.logger.Printf("failed to register prometheus metrics: %v", err)
	}
}

func (g *gNMIOutput) String() string {
	b, err := json.Marshal(g.cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (g *gNMIOutput) setDefaults() error {
	if g.cfg.Address == "" {
		g.cfg.Address = defaultAddress
	}
	if g.cfg.TargetTemplate == "" {
		g.targetTpl = outputs.DefaultTargetTemplate
	}
	if g.cfg.MaxSubscriptions <= 0 {
		g.cfg.MaxSubscriptions = defaultMaxSubscriptions
	}
	if g.cfg.MaxUnaryRPC <= 0 {
		g.cfg.MaxUnaryRPC = defaultMaxGetRPC
	}
	return nil
}

func (g *gNMIOutput) startGRPCServer() error {
	g.srv.subscribeRPCsem = semaphore.NewWeighted(g.cfg.MaxSubscriptions)
	g.srv.unaryRPCsem = semaphore.NewWeighted(g.cfg.MaxUnaryRPC)
	g.c.SetClient(g.srv.Update)

	var l net.Listener
	var err error
	network := "tcp"
	addr := g.cfg.Address
	if strings.HasPrefix(g.cfg.Address, "unix://") {
		network = "unix"
		addr = strings.TrimPrefix(addr, "unix://")
	}
	l, err = net.Listen(network, addr)
	if err != nil {
		return err
	}
	opts, err := g.serverOpts()
	if err != nil {
		return err
	}
	g.grpcSrv = grpc.NewServer(opts...)
	gnmi.RegisterGNMIServer(g.grpcSrv, g.srv)
	go g.grpcSrv.Serve(l)
	return nil
}

func (g *gNMIOutput) serverOpts() ([]grpc.ServerOption, error) {
	opts := make([]grpc.ServerOption, 0)
	if g.cfg.EnableMetrics {
		opts = append(opts, grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor))
	}
	if g.cfg.TLS == nil {
		return opts, nil
	}

	tlscfg, err := utils.NewTLSConfig(
		g.cfg.TLS.CaFile,
		g.cfg.TLS.CertFile,
		g.cfg.TLS.KeyFile,
		g.cfg.TLS.ClientAuth,
		false,
		true,
	)
	if err != nil {
		return nil, err
	}
	if tlscfg != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlscfg)))
	}

	return opts, nil
}
