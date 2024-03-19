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
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"

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
	// RPCs rate limit
	RateLimit int64
	// TLS config
	TLS *types.TLSConfig
}

type gNMIServer struct {
	gnmi.UnimplementedGNMIServer

	config Config
	logger *log.Logger
	reg    *prometheus.Registry
	//
	unarySem  *semaphore.Weighted
	streamSem *semaphore.Weighted
	// gnmi handlers
	capabilitiesHandler CapabilitiesHandler
	getHandler          GetHandler
	setHandler          SetHandler
	subscribeHandler    SubscribeHandler
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

type Option func(*gNMIServer)

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

func New(c Config, opts ...Option) (*gNMIServer, error) {
	err := c.setDefaults()
	if err != nil {
		return nil, err
	}
	s := &gNMIServer{
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

func (s *gNMIServer) Start(ctx context.Context) error {
	var networkType = "tcp"
	var addr = s.config.Address
	if indx := strings.Index(addr, "://"); indx > 0 {
		networkType = addr[:indx]
		addr = addr[indx+3:]
	}
	lc := &net.ListenConfig{
		KeepAlive: s.config.TCPKeepalive,
	}
	var l net.Listener
	var err error
	for {
		l, err = lc.Listen(ctx, networkType, addr)
		if err != nil {
			err = errors.Wrap(err, "cannot listen")
			s.logger.Print(err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	opts, err := s.serverOpts()
	if err != nil {
		return err
	}
	// create a gRPC server object
	gs := grpc.NewServer(opts...)
	// register reflection
	reflection.Register(gs)
	// register gnmi service to the grpc server
	gnmi.RegisterGNMIServer(gs, s)

	if s.config.HealthEnabled {
		hs := health.NewServer()
		healthpb.RegisterHealthServer(gs, hs)
		hs.SetServingStatus("gNMI", healthpb.HealthCheckResponse_SERVING)
	}

	s.logger.Printf("starting gRPC server...")
	err = gs.Serve(l)
	if err != nil {
		s.logger.Printf("gRPC serve failed: %v", err)
		return err
	}
	return nil
}

func (s *gNMIServer) Capabilities(ctx context.Context, req *gnmi.CapabilityRequest) (*gnmi.CapabilityResponse, error) {
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

func (s *gNMIServer) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
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

func (s *gNMIServer) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
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

func (s *gNMIServer) Subscribe(stream gnmi.GNMI_SubscribeServer) error {
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

func (s *gNMIServer) acquireUnarySem(ctx context.Context) error {
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

func (s *gNMIServer) releaseUnarySem() {
	if s.config.MaxUnaryRPC <= 0 {
		return
	}
	s.unarySem.Release(1)
}

func (s *gNMIServer) acquireStreamSem(ctx context.Context) error {
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

func (s *gNMIServer) releaseStreamSem() {
	if s.config.MaxStreamingRPC <= 0 {
		return
	}
	s.streamSem.Release(1)
}

// opts
func WithLogger(l *log.Logger) func(*gNMIServer) {
	return func(s *gNMIServer) {
		s.logger = l
	}
}

func WithRegistry(reg *prometheus.Registry) func(*gNMIServer) {
	return func(s *gNMIServer) {
		s.reg = reg
	}
}

func WithCapabilitiesHandler(h CapabilitiesHandler) func(*gNMIServer) {
	return func(s *gNMIServer) {
		s.capabilitiesHandler = h
	}
}

func WithGetHandler(h GetHandler) func(*gNMIServer) {
	return func(s *gNMIServer) {
		s.getHandler = h
	}
}

func WithSetHandler(h SetHandler) func(*gNMIServer) {
	return func(s *gNMIServer) {
		s.setHandler = h
	}
}

func WithSubscribeHandler(h SubscribeHandler) func(*gNMIServer) {
	return func(s *gNMIServer) {
		s.subscribeHandler = h
	}
}
