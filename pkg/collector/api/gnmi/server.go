// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

// Package gnmiserver implements the collector's northbound gNMI server.
//
// It exposes the same gNMI interface as the `subscribe` command's gNMI
// server (see pkg/app/gnmi_server.go):
//   - Subscribe RPCs are served from the collector cache, which is kept in
//     sync with the configured targets/subscriptions by the outputs manager.
//   - Get and Set RPCs are relayed to the target(s) selected via the request
//     Prefix.Target field.
//   - Get requests with origin `gnmic` are served from the collector's
//     configuration store (`targets` and `subscriptions` paths).
//
// It is configured with the same `gnmi-server` config section used by the
// `subscribe` command.
package gnmiserver

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/openconfig/gnmic/pkg/api/server"
	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/collector/env"
	targets_manager "github.com/openconfig/gnmic/pkg/collector/managers/targets"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/config"
	handlers "github.com/openconfig/gnmic/pkg/gnmiserver"
	"github.com/openconfig/gnmic/pkg/logging"
)

// Server is the collector's northbound gNMI server.
type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	store          *collstore.Store
	targetsManager *targets_manager.TargetsManager
	cache          cache.Cache

	cfg    *config.GNMIServer
	logger *slog.Logger
	reg    *prometheus.Registry
}

// NewServer creates a new collector gNMI server.
// The server is only effectively started (listening) if the `gnmi-server`
// section is present in the configuration store when Start is called.
func NewServer(ctx context.Context, store *collstore.Store, tm *targets_manager.TargetsManager, reg *prometheus.Registry) *Server {
	return &Server{
		ctx:            ctx,
		store:          store,
		targetsManager: tm,
		reg:            reg,
	}
}

// Start reads the `gnmi-server` configuration from the store and, if present,
// starts the gNMI server. `c` is the cache the Subscribe RPCs are served
// from; it is populated by the outputs manager.
func (s *Server) Start(c cache.Cache, wg *sync.WaitGroup) error {
	s.logger = logging.NewLogger(s.store.Config, "component", "gnmi-server")

	cfg, err := s.getConfig()
	if err != nil {
		s.logger.Error("failed to get gnmi-server config", "error", err)
		return err
	}
	if cfg == nil {
		s.logger.Info("gnmi-server config not found, skipping gNMI server")
		return nil
	}
	env.ExpandGNMIServerEnv(cfg)
	cfg.SetDefaults()
	if cfg.TLS != nil {
		if err := cfg.TLS.Validate(); err != nil {
			return fmt.Errorf("gnmi-server TLS config error: %w", err)
		}
	}
	s.cfg = cfg
	s.cache = c
	if s.cache == nil {
		s.logger.Warn("no cache available, Subscribe RPCs will not be served")
	}

	h := handlers.New(
		handlers.Config{
			DefaultSampleInterval: cfg.DefaultSampleInterval,
			MinSampleInterval:     cfg.MinSampleInterval,
			MinHeartbeatInterval:  cfg.MinHeartbeatInterval,
			Debug:                 cfg.Debug,
		},
		s.cache,
		s.logger,
		s.selectTargets,
		s.handleInternalGet,
	)
	opts := []server.Option{
		server.WithLogger(s.logger),
		server.WithCapabilitiesHandler(h.Capabilities),
		server.WithGetHandler(h.Get),
		server.WithSubscribeHandler(h.Subscribe),
		server.WithRegistry(s.reg),
	}
	// when the server is read-only, the Set handler is not registered
	// and Set RPCs are rejected with an `Unimplemented` status.
	if cfg.IsReadOnly() {
		s.logger.Info("gNMI server is read-only, Set RPCs are disabled")
	} else {
		opts = append(opts, server.WithSetHandler(h.Set))
	}

	s.logger.Info("starting gNMI server", "address", cfg.Address)
	srv, err := server.New(server.Config{
		Address:              cfg.Address,
		MaxUnaryRPC:          cfg.MaxUnaryRPC,
		MaxStreamingRPC:      cfg.MaxSubscriptions,
		MaxRecvMsgSize:       cfg.MaxRecvMsgSize,
		MaxSendMsgSize:       cfg.MaxSendMsgSize,
		MaxConcurrentStreams: cfg.MaxConcurrentStreams,
		TCPKeepalive:         cfg.TCPKeepalive,
		Keepalive:            cfg.GRPCKeepalive.Convert(),
		RateLimit:            cfg.RateLimit,
		Timeout:              cfg.Timeout,
		HealthEnabled:        true,
		TLS:                  cfg.TLS,
	}, opts...)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.cancel = cancel

	if cfg.ServiceRegistration != nil {
		go s.registerService(ctx)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		err := srv.Start(ctx)
		if err != nil {
			logging.LogErrUnlessCanceled(s.logger, err, "gNMI server exited")
		}
	}()
	return nil
}

// Stop stops the gNMI server.
func (s *Server) Stop() {
	if s.cancel == nil {
		return
	}
	s.logger.Info("stopping gNMI server")
	s.cancel()
}

func (s *Server) getConfig() (*config.GNMIServer, error) {
	v, ok, err := s.store.Config.Get("gnmi-server", "gnmi-server")
	if err != nil {
		return nil, err
	}
	if !ok || v == nil {
		return nil, nil
	}
	cfg, ok := v.(*config.GNMIServer)
	if !ok {
		return nil, fmt.Errorf("invalid gnmi-server config type: %T", v)
	}
	// the store may hold a typed nil pointer when the config
	// file does not define a gnmi-server section.
	if cfg == nil {
		return nil, nil
	}
	return cfg, nil
}

func (s *Server) getClusteringConfig() *config.Clustering {
	v, ok, err := s.store.Config.Get("clustering", "clustering")
	if err != nil || !ok || v == nil {
		return nil
	}
	cfg, ok := v.(*config.Clustering)
	if !ok {
		return nil
	}
	return cfg
}

func (s *Server) instanceName() string {
	v, ok, err := s.store.Config.Get("global-flags", "global-flags")
	if err != nil || !ok || v == nil {
		return ""
	}
	switch gf := v.(type) {
	case config.GlobalFlags:
		return gf.InstanceName
	case *config.GlobalFlags:
		return gf.InstanceName
	}
	return ""
}
