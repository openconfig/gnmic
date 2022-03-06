package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/karimra/gnmic/utils"
	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
	"github.com/openconfig/grpctunnel/tunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func (a *App) initTunnelServer() error {
	if !a.Config.UseTunnelServer {
		return nil
	}
	err := a.Config.GetTunnelServer()
	if err != nil {
		return err
	}
	go func() {
		err = a.startTunnelServer()
		if err != nil {
			a.Logger.Printf("failed to start tunnel server: %v", err)
		}
	}()
	return nil
}

func (a *App) startTunnelServer() error {
	if a.Config.TunnelServer == nil {
		return nil
	}
	var err error
	a.tunServer, err = tunnel.NewServer(
		tunnel.ServerConfig{
			AddTargetHandler:    a.tunServerAddTargetHandler,
			DeleteTargetHandler: a.tunServerDeleteTargetHandler,
			RegisterHandler:     a.tunServerRegisterHandler,
			Handler:             a.tunServerHandler,
		})
	if err != nil {
		a.Logger.Printf("failed to create a tunnel server: %v", err)
		return err

	}
	// create tunnel server options
	opts, err := a.gRPCTunnelServerOpts()
	if err != nil {
		a.Logger.Printf("failed to build gRPC tunnel server options: %v", err)
		return err
	}
	a.grpcTunnelSrv = grpc.NewServer(opts...)
	// register the tunnel service with the grpc server
	tpb.RegisterTunnelServer(a.grpcTunnelSrv, a.tunServer)
	//
	var l net.Listener
	network := "tcp"
	addr := a.Config.TunnelServer.Address
	if strings.HasPrefix(a.Config.TunnelServer.Address, "unix://") {
		network = "unix"
		addr = strings.TrimPrefix(addr, "unix://")
	}

	ctx, cancel := context.WithCancel(a.ctx)
	for {
		l, err = net.Listen(network, addr)
		if err != nil {
			a.Logger.Printf("failed to start gRPC tunnel server listener: %v", err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	go func() {
		err = a.grpcTunnelSrv.Serve(l)
		if err != nil {
			a.Logger.Printf("gRPC tunnel server shutdown: %v", err)
		}
		cancel()
	}()
	defer a.grpcTunnelSrv.Stop()
	for range ctx.Done() {
	}
	return ctx.Err()
}

func (a *App) gRPCTunnelServerOpts() ([]grpc.ServerOption, error) {
	opts := make([]grpc.ServerOption, 0)
	if a.Config.TunnelServer.EnableMetrics && a.reg != nil {
		grpcMetrics := grpc_prometheus.NewServerMetrics()
		opts = append(opts,
			grpc.StreamInterceptor(grpcMetrics.StreamServerInterceptor()),
			grpc.UnaryInterceptor(grpcMetrics.UnaryServerInterceptor()),
		)
		a.reg.MustRegister(grpcMetrics)
	}

	tlscfg, err := utils.NewTLSConfig(
		a.Config.TunnelServer.CaFile,
		a.Config.TunnelServer.CertFile,
		a.Config.TunnelServer.KeyFile,
		a.Config.TunnelServer.SkipVerify,
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

func (a *App) tunServerAddTargetHandler(tt tunnel.Target) error {
	a.Logger.Printf("tunnel server discovered target %+v", tt)
	a.ttm.Lock()
	a.tunTargets[tt.ID] = tt
	a.ttm.Unlock()
	return nil
}

func (a *App) tunServerDeleteTargetHandler(tt tunnel.Target) error {
	a.Logger.Printf("tunnel server target %+v deregistered", tt)
	a.ttm.Lock()
	a.tunTargetCfn[tt.ID]()
	delete(a.tunTargetCfn, tt.ID)
	delete(a.tunTargets, tt.ID)
	a.ttm.Unlock()
	return nil
}

func (a *App) tunServerRegisterHandler(ss tunnel.ServerSession) error {
	return nil
}

func (a *App) tunServerHandler(ss tunnel.ServerSession, rwc io.ReadWriteCloser) error {
	return nil
}

// tunDialerFn is used to build a grpc Option that sets a custom dialer for tunnel targets.
func (a *App) tunDialerFn(ctx context.Context, tName string) func(context.Context, string) (net.Conn, error) {
	return func(_ context.Context, _ string) (net.Conn, error) {
		a.Logger.Printf("dialing tunnel connection for target %q", tName)
		a.ttm.RLock()
		tt, ok := a.tunTargets[tName]
		a.ttm.RUnlock()
		if !ok {
			return nil, fmt.Errorf("unknown tunnel target %q", tName)
		}
		return tunnel.ServerConn(ctx, a.tunServer, &tt)
	}
}
