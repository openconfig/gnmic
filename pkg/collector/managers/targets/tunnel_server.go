package targets_manager

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/config/store"
	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// TODO: watch tunnel server config and reconcile
type tunnelServer struct {
	// config lock
	m      *sync.RWMutex
	config *config.TunnelServer

	grpcTunnelSrv *grpc.Server
	tunServer     *tunnel.Server
	store         store.Store[any]
	// ttm           *sync.RWMutex
	// tunTargets    map[tunnel.Target]struct{}
	// tunTargetCfn  map[tunnel.Target]context.CancelFunc
	logger *slog.Logger
	reg    *prometheus.Registry
}

func newTunnelServer(s store.Store[any], reg *prometheus.Registry) *tunnelServer {
	ts := &tunnelServer{
		m:             new(sync.RWMutex),
		grpcTunnelSrv: grpc.NewServer(),
		store:         s,
		// ttm:           new(sync.RWMutex),
		// tunTargets:    make(map[tunnel.Target]struct{}),
		// tunTargetCfn:  make(map[tunnel.Target]context.CancelFunc),
		logger: slog.With("component", "tunnel-server"),
		reg:    reg,
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
	var ok bool
	ts.config, ok = tscfg.(*config.TunnelServer)
	if !ok {
		return fmt.Errorf("tunnel-server config is malfomatted")
	}
	if ts.config == nil {
		return nil
	}
	ts.tunServer, err = tunnel.NewServer(tunnel.ServerConfig{
		AddTargetHandler:    ts.tunServerAddTargetSubscribeHandler,
		DeleteTargetHandler: ts.tunServerDeleteTargetHandler,
		RegisterHandler:     ts.tunServerRegisterHandler,
		Handler:             ts.tunServerHandler,
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

// handlers
// subscribe handler
func (ts *tunnelServer) tunServerAddTargetSubscribeHandler(tt tunnel.Target) error {
	ts.logger.Info("tunnel server target register request", "target", tt)
	tc := ts.getTunnelTargetMatch(tt)
	if tc == nil {
		ts.logger.Info("target ignored", "target", tt)
		return nil
	}
	// ts.ttm.Lock()
	// ts.tunTargets[tt] = struct{}{}
	// ts.ttm.Unlock()
	_, err := ts.store.Set("targets", tt.ID, tc)
	if err != nil {
		return err
	} // TODO: more ?
	return nil
}

// delete handler
func (ts *tunnelServer) tunServerDeleteTargetHandler(tt tunnel.Target) error {
	ts.logger.Info("tunnel server target deregister request", "target", tt)
	// ts.ttm.Lock()
	// defer ts.ttm.Unlock()
	// if cfn, ok := ts.tunTargetCfn[tt]; ok {
	// 	cfn()
	// 	delete(ts.tunTargetCfn, tt)
	// 	delete(ts.tunTargets, tt)
	// 	_, _, err := ts.store.Delete("targets", tt.ID)
	// 	if err != nil {
	// 		ts.logger.Error("failed to delete tunneltarget from configStore", "error", err)
	// 	}
	// }
	_, _, err := ts.store.Delete("targets", tt.ID)
	if err != nil {
		ts.logger.Error("failed to delete tunneltarget from configStore", "error", err)
	}
	return nil
}

func (ts *tunnelServer) tunServerRegisterHandler(ss tunnel.ServerSession) error {
	return nil
}

func (ts *tunnelServer) tunServerHandler(ss tunnel.ServerSession, rwc io.ReadWriteCloser) error {
	return nil
}

func (ts *tunnelServer) getTunnelTargetMatch(tt tunnel.Target) *types.TargetConfig {
	ts.m.RLock()
	defer ts.m.RUnlock()

	if len(ts.config.Targets) == 0 {
		if tt.Type != "GNMI_GNOI" {
			return nil
		}
		tc := &types.TargetConfig{Name: tt.ID, TunnelTargetType: tt.Type}
		err := config.SetTargetConfigDefaults(ts.store, tc)
		if err != nil {
			ts.logger.Error("failed to set target %q config defaults", "error", err)
			return nil
		}
		tc.Address = tc.Name
		return tc
	}
	for _, tm := range ts.config.Targets {
		// check if the discovered target matches one of the configured types
		ok, err := regexp.MatchString(tm.Type, tt.Type)
		if err != nil {
			ts.logger.Error("regex eval failed with string", "error", err, "type", tm.Type, "target", tt.Type)
			continue
		}
		if !ok {
			continue
		}
		// check if the discovered target matches corresponding ID
		ok, err = regexp.MatchString(tm.ID, tt.ID)
		if err != nil {
			ts.logger.Error("regex eval failed with string", "error", err, "id", tm.ID, "target", tt.ID)
			continue
		}
		if !ok {
			continue
		}
		// target has a match
		tc := new(types.TargetConfig)
		*tc = tm.Config
		tc.Name = tt.ID
		tc.TunnelTargetType = tt.Type
		err = config.SetTargetConfigDefaults(ts.store, tc)
		if err != nil {
			ts.logger.Error("failed to set target config defaults", "error", err, "id", tt.ID, "type", tt.Type)
			return nil
		}
		tc.Address = tc.Name
		return tc
	}
	return nil
}
