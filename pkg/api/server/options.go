// © 2024 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	grpc_ratelimit "github.com/grpc-ecosystem/go-grpc-middleware/ratelimit"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/juju/ratelimit"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func (s *gNMIServer) serverOpts() ([]grpc.ServerOption, error) {
	opts := make([]grpc.ServerOption, 0, 1)
	credsOpts, err := s.tlsServerOpts()
	if err != nil {
		return nil, err
	}
	opts = append(opts, credsOpts)

	if s.config.Keepalive != nil {
		opts = append(opts, grpc.KeepaliveParams(*s.config.Keepalive))
	}
	if s.config.MaxRecvMsgSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(s.config.MaxRecvMsgSize))
	}
	if s.config.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(s.config.MaxSendMsgSize))
	}
	if s.config.MaxConcurrentStreams > 0 {
		opts = append(opts, grpc.MaxConcurrentStreams(s.config.MaxConcurrentStreams))
	}
	opts = append(opts, s.interceptorsOpts()...)
	return opts, nil
}

func (s *gNMIServer) interceptorsOpts() []grpc.ServerOption {
	ui := []grpc.UnaryServerInterceptor{}
	si := []grpc.StreamServerInterceptor{}
	if s.reg != nil {
		grpcMetrics := grpc_prometheus.NewServerMetrics()
		ui = append(ui, grpcMetrics.UnaryServerInterceptor())
		si = append(si, grpcMetrics.StreamServerInterceptor())
		s.reg.MustRegister(grpcMetrics)
	}
	if s.config.RateLimit > 0 {
		limiter := &rateLimiterInterceptor{
			bucket: ratelimit.NewBucket(time.Second, s.config.RateLimit),
		}
		ui = append(ui, grpc_ratelimit.UnaryServerInterceptor(limiter))
		si = append(si, grpc_ratelimit.StreamServerInterceptor(limiter))
	}
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(ui...),
		grpc.ChainStreamInterceptor(si...),
	}
}

type rateLimiterInterceptor struct {
	bucket *ratelimit.Bucket
}

func (r *rateLimiterInterceptor) Limit() bool {
	return r.bucket.TakeAvailable(1) == 0
}

func (s *gNMIServer) tlsServerOpts() (grpc.ServerOption, error) {
	if s.config.TLS == nil {
		return grpc.Creds(insecure.NewCredentials()), nil
	}
	err := s.config.TLS.Validate()
	if err != nil {
		return nil, err
	}
	tlsConfig, err := s.createTLSConfig()
	if err != nil {
		return nil, err
	}
	return grpc.Creds(credentials.NewTLS(tlsConfig)), nil
}

func (s *gNMIServer) createTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		GetCertificate: s.readCerts,
	}
	switch s.config.TLS.ClientAuth {
	default:
		tlsConfig.ClientAuth = tls.NoClientCert
	case "request":
		tlsConfig.ClientAuth = tls.RequestClientCert
	case "require":
		tlsConfig.ClientAuth = tls.RequireAnyClientCert
	case "verify-if-given":
		tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
	case "require-verify":
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if len(s.config.TLS.CaFile) != 0 {
		caCertPool, err := utils.LoadCACertificates(s.config.TLS.CaFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientCAs = caCertPool
	}

	return tlsConfig, nil
}

func (s *gNMIServer) readCerts(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
	now := time.Now()

	s.cm.Lock()
	defer s.cm.Unlock()

	if !now.After(s.lastRead.Add(time.Minute)) && s.cert != nil {
		return s.cert, nil
	}

	cert, err := os.ReadFile(s.config.TLS.CertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read defined cert file: %w", err)
	}

	key, err := os.ReadFile(s.config.TLS.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read defined key file: %w", err)
	}

	serverCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	s.cert = &serverCert
	s.lastRead = time.Now()
	return &serverCert, nil
}
