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
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func (s *GNMIServer) serverOpts() ([]grpc.ServerOption, error) {
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
	return opts, nil
}

func (s *GNMIServer) tlsServerOpts() (grpc.ServerOption, error) {
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

func (s *GNMIServer) createTLSConfig() (*tls.Config, error) {
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
		ca, err := os.ReadFile(s.config.TLS.CaFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read client CA cert: %w", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(ca)
		tlsConfig.ClientCAs = caCertPool
	}

	return tlsConfig, nil
}

func (s *GNMIServer) readCerts(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
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
