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
	"sync"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/logging"
	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zestor-dev/zestor/store"
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

	// track currently connected tunnel targets so we can reconcile when
	// tunnel-target-matches are created, updated, or deleted.
	mu               sync.RWMutex
	connectedTargets map[string]tunnel.Target // key = target ID
}

func newTunnelServer(s store.Store[any], reg *prometheus.Registry) *tunnelServer {
	ts := &tunnelServer{
		grpcTunnelSrv:    grpc.NewServer(),
		store:            s,
		reg:              reg,
		connectedTargets: make(map[string]tunnel.Target),
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
	if originalConfig == nil {
		return nil
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

	// watch tunnel-target-matches for CRUD operations and reconcile connected targets
	var matchesCh <-chan *store.Event[any]
	var matchesCancel func()
	for {
		var err error
		matchesCh, matchesCancel, err = ts.store.Watch("tunnel-target-matches")
		if err != nil {
			ts.logger.Error("failed to watch tunnel-target-matches", "error", err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	go ts.watchTunnelTargetMatches(ctx, matchesCh, matchesCancel)

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

// addTargetHandler is called when a tunnel target connects (registers)
func (ts *tunnelServer) addTargetHandler(tt tunnel.Target) error {
	ts.logger.Info("tunnel server target register request", "target", tt)

	// track the connected target so we can reconcile when matches change
	ts.mu.Lock()
	ts.connectedTargets[tt.ID] = tt
	ts.mu.Unlock()

	tc := ts.getTunnelTargetMatch(tt)
	if tc == nil {
		ts.logger.Info("target ignored, not matching any rule", "target", tt)
		return nil
	}
	ts.logger.Info("target matched", "target", tc)
	_, err := ts.store.Set("targets", tc.Name, tc)
	if err != nil {
		return err
	}
	return nil
}

// deleteTargetHandler is called when a tunnel target disconnects (deregisters)
func (ts *tunnelServer) deleteTargetHandler(tt tunnel.Target) error {
	ts.logger.Info("tunnel server target deregister request", "target", tt)

	// remove from connected targets tracking
	ts.mu.Lock()
	delete(ts.connectedTargets, tt.ID)
	ts.mu.Unlock()

	_, _, err := ts.store.Delete("targets", tt.ID)
	if err != nil {
		ts.logger.Error("failed to delete tunnel target from configStore", "error", err)
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

// watchTunnelTargetMatches watches for changes to tunnel-target-matches and
// reconciles all connected tunnel targets when a match is created, updated, or deleted.
func (ts *tunnelServer) watchTunnelTargetMatches(ctx context.Context, ch <-chan *store.Event[any], cancel func()) {
	defer cancel()
	ts.logger.Info("starting tunnel-target-matches watcher")
	for {
		select {
		case <-ctx.Done():
			ts.logger.Info("tunnel-target-matches watcher stopped")
			return
		case ev, ok := <-ch:
			if !ok {
				ts.logger.Info("tunnel-target-matches watch channel closed")
				return
			}
			ts.logger.Info("tunnel-target-match changed, reconciling connected targets",
				"eventType", ev.EventType,
				"matchID", ev.Name,
			)
			ts.reconcileConnectedTargets()
		}
	}
}

// reconcileConnectedTargets re-evaluates all connected tunnel targets against
// the current set of tunnel-target-matches. This is called when a match rule
// is created, updated, or deleted.
//
// For each connected target:
//   - If it matches a rule: upsert the target config (create or update)
//   - If it doesn't match any rule: delete the target config
//
// We hold the lock for the entire reconciliation to prevent a race where a target
// deregisters (and gets deleted from the store) while we're processing it, which
// would cause us to recreate an orphaned target config.
func (ts *tunnelServer) reconcileConnectedTargets() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.logger.Info("reconciling connected tunnel targets", "count", len(ts.connectedTargets))

	for _, tt := range ts.connectedTargets {
		tc := ts.getTunnelTargetMatch(tt)
		if tc != nil {
			// target matches a rule ==> upsert the target config
			ts.logger.Debug("tunnel target matches rule, upserting config",
				"targetID", tt.ID,
				"targetType", tt.Type,
			)
			_, err := ts.store.Set("targets", tc.Name, tc)
			if err != nil {
				ts.logger.Error("failed to upsert tunnel target config",
					"targetID", tt.ID,
					"error", err,
				)
			}
		} else {
			// target no longer matches any rule ==> delete the target config
			ts.logger.Debug("tunnel target no longer matches any rule, deleting config",
				"targetID", tt.ID,
				"targetType", tt.Type,
			)
			_, _, err := ts.store.Delete("targets", tt.ID)
			if err != nil {
				ts.logger.Error("failed to delete tunnel target config",
					"targetID", tt.ID,
					"error", err,
				)
			}
		}
	}

	ts.logger.Info("tunnel target reconciliation complete", "count", len(ts.connectedTargets))
}
