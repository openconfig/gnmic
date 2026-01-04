package targets_manager

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/store"
	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// smaller scoped config than the one used when loading the config from the file
type tunnelServerConfig struct {
	Address       string           `mapstructure:"address,omitempty" json:"address,omitempty"`
	TLS           *types.TLSConfig `mapstructure:"tls,omitempty"`
	EnableMetrics bool             `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
	Debug         bool             `mapstructure:"debug,omitempty" json:"debug,omitempty"`
}

// TODO: watch tunnel server config and reconcile
type tunnelServer struct {
	config *tunnelServerConfig

	grpcTunnelSrv *grpc.Server
	tunServer     *tunnel.Server
	store         store.Store[any]
	logger        *slog.Logger
	reg           *prometheus.Registry
}

func newTunnelServer(s store.Store[any], reg *prometheus.Registry) *tunnelServer {
	ts := &tunnelServer{
		grpcTunnelSrv: grpc.NewServer(),
		store:         s,
		reg:           reg,
	}

	return ts
}

func (ts *tunnelServer) gRPCTunnelServerOpts() ([]grpc.ServerOption, error) {
	opts := make([]grpc.ServerOption, 0)
	if ts.config == nil {
		return opts, nil
	}
	if ts.config.EnableMetrics && ts.reg != nil {
		grpcMetrics := grpc_prometheus.NewServerMetrics()
		opts = append(opts,
			grpc.StreamInterceptor(grpcMetrics.StreamServerInterceptor()),
			grpc.UnaryInterceptor(grpcMetrics.UnaryServerInterceptor()),
		)
		ts.reg.MustRegister(grpcMetrics)
	}

	if ts.config.TLS == nil {
		return opts, nil
	}

	tlscfg, err := utils.NewTLSConfig(
		ts.config.TLS.CaFile,
		ts.config.TLS.CertFile,
		ts.config.TLS.KeyFile,
		ts.config.TLS.ClientAuth,
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

func (ts *tunnelServer) startTunnelServer(ctx context.Context) error {
	tscfg, found, err := ts.store.Get("tunnel-server", "tunnel-server")
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if tscfg == nil {
		return nil
	}
	logger := logging.NewLogger(ts.store, "component", "tunnel-server")
	ts.logger = logger
	originalConfig, ok := tscfg.(*config.TunnelServer)
	if !ok {
		return fmt.Errorf("tunnel-server config is malfomatted")
	}
	ts.config = &tunnelServerConfig{
		Address:       originalConfig.Address,
		TLS:           originalConfig.TLS,
		EnableMetrics: originalConfig.EnableMetrics,
		Debug:         originalConfig.Debug,
	}

	ts.logger.Info("building tunnel server")
	ts.tunServer, err = tunnel.NewServer(tunnel.ServerConfig{
		AddTargetHandler:    ts.addTargetHandler,
		DeleteTargetHandler: ts.deleteTargetHandler,
		RegisterHandler:     ts.registerHandler,
		Handler:             ts.serverHandler,
	})
	if err != nil {
		return err
	}
	opts, err := ts.gRPCTunnelServerOpts()
	if err != nil {
		return err
	}
	ts.grpcTunnelSrv = grpc.NewServer(opts...)
	tpb.RegisterTunnelServer(ts.grpcTunnelSrv, ts.tunServer)
	var l net.Listener
	network := "tcp"
	addr := ts.config.Address
	if strings.HasPrefix(ts.config.Address, "unix://") {
		network = "unix"
		addr = strings.TrimPrefix(addr, "unix://")
	}

	ctx, cancel := context.WithCancel(ctx)
	for {
		var err error
		l, err = net.Listen(network, addr)
		if err != nil {
			ts.logger.Error("failed to start gRPC tunnel server listener", "error", err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	go func() {
		ts.logger.Info("starting gRPC tunnel server")
		err := ts.grpcTunnelSrv.Serve(l)
		if err != nil {
			ts.logger.Error("gRPC tunnel server shutdown", "error", err)
		}
		cancel()
	}()
	defer ts.grpcTunnelSrv.Stop()
	for range ctx.Done() {
	}
	return ctx.Err()
}

// Tunnel Server handlers

// subscribe handler
func (ts *tunnelServer) addTargetHandler(tt tunnel.Target) error {
	ts.logger.Info("tunnel server target register request", "target", tt)
	tc := ts.getTunnelTargetMatch(tt)
	if tc == nil {
		ts.logger.Info("target ignored", "target", tt)
		return nil
	}
	ts.logger.Info("target matched", "target", tc)
	_, err := ts.store.Set("targets", tc.Name, tc)
	if err != nil {
		return err
	}
	return nil
}

// delete handler
func (ts *tunnelServer) deleteTargetHandler(tt tunnel.Target) error {
	ts.logger.Info("tunnel server target deregister request", "target", tt)
	_, _, err := ts.store.Delete("targets", tt.ID)
	if err != nil {
		ts.logger.Error("failed to delete tunneltarget from configStore", "error", err)
	}
	return nil
}

func (ts *tunnelServer) registerHandler(ss tunnel.ServerSession) error {
	return nil
}

func (ts *tunnelServer) serverHandler(ss tunnel.ServerSession, rwc io.ReadWriteCloser) error {
	return nil
}

func (ts *tunnelServer) getTunnelTargetMatch(tt tunnel.Target) *types.TargetConfig {
	matchingConfigs, err := ts.store.List("tunnel-target-matches", func(key string, value any) bool {
		switch tm := value.(type) {
		case *config.TunnelTargetMatch:
			// check if the registering target matches corresponding ID
			ok, err := regexp.MatchString(tm.ID, tt.ID)
			if err != nil {
				ts.logger.Error("regex eval failed with string", "error", err, "id", tm.ID, "target", tt.ID)
				return false
			}
			if !ok {
				return false
			}
			// check if the registering target matches corresponding type
			ok, err = regexp.MatchString(tm.Type, tt.Type)
			if err != nil {
				ts.logger.Error("regex eval failed with string", "error", err, "type", tm.Type, "target", tt.Type)
				return false
			}
			if !ok {
				return false
			}
			// target has a match,
			tc := new(types.TargetConfig)
			*tc = tm.Config
			tc.Name = tt.ID
			tc.TunnelTargetType = tt.Type
			err = config.SetTargetConfigDefaults(ts.store, tc)
			if err != nil {
				ts.logger.Error("failed to set target config defaults", "error", err, "id", tt.ID, "type", tt.Type)
				return false
			}
		}
		return false
	})
	if err != nil {
		ts.logger.Error("failed to list tunnel target matches", "error", err)
		return nil
	}
	if len(matchingConfigs) == 0 {
		return nil
	}
	// get keys and sort them
	keys := make([]string, 0, len(matchingConfigs))
	for key := range matchingConfigs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// take the first match and set the target config defaults
	mconfig := matchingConfigs[keys[0]].(*config.TunnelTargetMatch)
	tc := new(types.TargetConfig)
	*tc = mconfig.Config
	tc.Name = tt.ID
	tc.TunnelTargetType = tt.Type
	err = config.SetTargetConfigDefaults(ts.store, tc)
	if err != nil {
		ts.logger.Error("failed to set target config defaults", "error", err, "id", tt.ID, "type", tt.Type)
		return nil
	}

	return tc
}
