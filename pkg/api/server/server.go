// © 2024 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type Config struct {
	// gRPC server address
	Address string
	// MaxUnaryRPC defines the max number of inflight
	// Unary RPCs (Cap, Get, Set,...).
	// if negative or unset, there is not limit.
	MaxUnaryRPC int64
	// MaxStreamingRPC defines the max number of inflight
	// streaming RPCs (Subscribe,...).
	// if negative or unset, there is not limit.
	MaxStreamingRPC int64
	// MaxRecvMsgSize defines the max message
	// size in bytes the server can receive.
	// If this is not set, it defaults to 4MB.
	MaxRecvMsgSize int
	// MaxSendMsgSize defines the the max message
	// size in bytes the server can send.
	// If this is not set, the default is `math.MaxInt32`.
	MaxSendMsgSize int
	// MaxConcurrentStreams defines the max number of
	// concurrent streams to each ServerTransport.
	MaxConcurrentStreams uint32
	// TCPKeepalive set the TCP keepalive time and
	// interval, if unset it is enabled based on
	// the protocol used and the OS.
	// If negative it is disabled.
	TCPKeepalive time.Duration
	// Keepalive params
	Keepalive *keepalive.ServerParameters
	// enable gRPC Health RPCs
	HealthEnabled bool
	// unary RPC request timeout
	Timeout time.Duration
	// TLS config
	TLS *types.TLSConfig
}

type GNMIServer struct {
	config Config
	logger *log.Logger

	unarySem  *semaphore.Weighted
	streamSem *semaphore.Weighted
	// gnmi handlers
	capabilitiesHandler CapabilitiesHandler
	getHandler          GetHandler
	setHandler          SetHandler
	subscribeHandler    SubscribeHandler
	// health handlers
	checkHandler CheckHandler
	watchHandler WatchHandler
	// cached certificate
	cm   *sync.Mutex
	cert *tls.Certificate
	// certificate last read time
	lastRead time.Time
}

// gNMI Handlers
type CapabilitiesHandler func(ctx context.Context, req *gnmi.CapabilityRequest) (*gnmi.CapabilityResponse, error)

type GetHandler func(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error)

type SetHandler func(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error)

type SubscribeHandler func(req *gnmi.SubscribeRequest, stream gnmi.GNMI_SubscribeServer) error

// Health Handlers
type CheckHandler func(context.Context, *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error)

type WatchHandler func(*healthpb.HealthCheckRequest, healthpb.Health_WatchServer) error

type Option func(*GNMIServer)

func defaultCapabilitiesHandlerFunc(ctx context.Context, req *gnmi.CapabilityRequest) (*gnmi.CapabilityResponse, error) {
	return &gnmi.CapabilityResponse{
		GNMIVersion: "0.10.0",
	}, nil
}

func (c *Config) setDefaults() error {
	if c.Address == "" {
		return errors.New("missing address")
	}
	if c.Timeout <= 0 {
		c.Timeout = 2 * time.Minute
	}
	return nil
}

func New(c Config, opts ...Option) (*GNMIServer, error) {
	err := c.setDefaults()
	if err != nil {
		return nil, err
	}
	s := &GNMIServer{
		config: c,
		cm:     new(sync.Mutex),
	}
	if c.MaxUnaryRPC > 0 {
		s.unarySem = semaphore.NewWeighted(c.MaxUnaryRPC)
	}
	if c.MaxStreamingRPC > 0 {
		s.streamSem = semaphore.NewWeighted(c.MaxStreamingRPC)
	}
	for _, o := range opts {
		o(s)
	}
	if s.capabilitiesHandler == nil {
		s.capabilitiesHandler = defaultCapabilitiesHandlerFunc
	}
	return s, nil
}

func (s *GNMIServer) Start(ctx context.Context) error {
	var networkType = "tcp"
	var addr = s.config.Address
	if indx := strings.Index(addr, "://"); indx > 0 {
		networkType = addr[:indx]
		addr = addr[indx+3:]
	}
	lc := &net.ListenConfig{
		KeepAlive: s.config.TCPKeepalive,
	}
	l, err := lc.Listen(ctx, networkType, addr)
	if err != nil {
		return errors.Wrap(err, "cannot listen")
	}
	opts, err := s.serverOpts()
	if err != nil {
		return err
	}
	// create a gRPC server object
	GNMIServer := grpc.NewServer(opts...)

	// register gnmi service to the grpc server
	gnmi.RegisterGNMIServer(GNMIServer, s)

	// register health service to the grpc server
	if s.config.HealthEnabled {
		healthpb.RegisterHealthServer(GNMIServer, s)
	}

	// register reflection
	reflection.Register(GNMIServer)

	s.logger.Printf("starting gRPC server...")
	err = GNMIServer.Serve(l)
	if err != nil {
		s.logger.Printf("gRPC serve failed: %v", err)
		return err
	}
	return nil
}

func (s *GNMIServer) Capabilities(ctx context.Context, req *gnmi.CapabilityRequest) (*gnmi.CapabilityResponse, error) {
	if s.capabilitiesHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method Capabilities not implemented")
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireUnarySem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.releaseUnarySem()
	return s.capabilitiesHandler(ctx, req)
}

func (s *GNMIServer) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	if s.getHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireUnarySem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.releaseUnarySem()
	return s.getHandler(ctx, req)
}

func (s *GNMIServer) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	if s.setHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method Set not implemented")
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireUnarySem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.releaseUnarySem()
	return s.setHandler(ctx, req)
}

func (s *GNMIServer) Subscribe(stream gnmi.GNMI_SubscribeServer) error {
	if s.subscribeHandler == nil {
		return status.Errorf(codes.Unimplemented, "method Subscribe not implemented")
	}
	ctx := stream.Context()
	err := s.acquireStreamSem(ctx)
	if err != nil {
		return err
	}
	defer s.releaseStreamSem()
	//
	pr, _ := peer.FromContext(ctx)
	s.logger.Printf("received subscribe request from peer %s", pr.Addr)

	req, err := stream.Recv()
	switch {
	case err == io.EOF:
		return nil
	case err != nil:
		return err
	case req.GetSubscribe() == nil:
		return status.Errorf(codes.InvalidArgument, "the subscribe request must contain a subscription definition")
	}
	return s.subscribeHandler(req, stream)
}

func (s *GNMIServer) acquireUnarySem(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		if s.config.MaxUnaryRPC <= 0 {
			return nil
		}
		return s.unarySem.Acquire(ctx, 1)
	}
}

func (s *GNMIServer) releaseUnarySem() {
	if s.config.MaxUnaryRPC <= 0 {
		return
	}
	s.unarySem.Release(1)
}

func (s *GNMIServer) acquireStreamSem(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		if s.config.MaxStreamingRPC <= 0 {
			return nil
		}
		return s.streamSem.Acquire(ctx, 1)
	}
}

func (s *GNMIServer) releaseStreamSem() {
	if s.config.MaxStreamingRPC <= 0 {
		return
	}
	s.streamSem.Release(1)
}

// health RPCs
// Check implements `service Health`.
func (s *GNMIServer) Check(ctx context.Context, in *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	if s.checkHandler == nil {
		return nil, status.Error(codes.Unimplemented, "method Check not implemented")
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.acquireUnarySem(ctx)
	if err != nil {
		return nil, err
	}
	defer s.releaseUnarySem()
	return s.checkHandler(ctx, in)
}

// Watch implements `service Health`.
func (s *GNMIServer) Watch(in *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	if s.watchHandler == nil {
		return status.Error(codes.Unimplemented, "method Watch not implemented")
	}
	err := s.acquireStreamSem(stream.Context())
	if err != nil {
		return err
	}
	defer s.releaseStreamSem()
	return s.watchHandler(in, stream)
}

// opts
func WithLogger(l *log.Logger) func(*GNMIServer) {
	return func(s *GNMIServer) {
		s.logger = l
	}
}

func WithGetHandler(h GetHandler) func(*GNMIServer) {
	return func(s *GNMIServer) {
		s.SetGetHandler(h)
	}
}

func WithSetHandler(h SetHandler) func(*GNMIServer) {
	return func(s *GNMIServer) {
		s.SetSetHandler(h)
	}
}

func WithSubscribeHandler(h SubscribeHandler) func(*GNMIServer) {
	return func(s *GNMIServer) {
		s.SetSubscribeHandler(h)
	}
}

func WithCheckHandler(h CheckHandler) func(*GNMIServer) {
	return func(s *GNMIServer) {
		s.checkHandler = h
	}
}

func WithWatchHandler(h WatchHandler) func(*GNMIServer) {
	return func(s *GNMIServer) {
		s.watchHandler = h
	}
}

func (s *GNMIServer) SetGetHandler(h GetHandler) {
	s.getHandler = h
}

func (s *GNMIServer) SetSetHandler(h SetHandler) {
	s.setHandler = h
}

func (s *GNMIServer) SetSubscribeHandler(h SubscribeHandler) {
	s.subscribeHandler = h
}
