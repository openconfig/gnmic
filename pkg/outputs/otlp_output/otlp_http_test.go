// © 2026 NVIDIA Corporation
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of NVIDIA's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package otlp_output

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

func TestMTLSHarness_ClientCanReachServer(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	client := srv.NewClient()
	resp, err := client.Get(srv.URL + "/ping")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestMTLSHarness_ClientWithoutCertIsRejected(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	// Client with TLS but NO client cert → handshake should fail.
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: srv.CAPool()},
	}}
	_, err := client.Get(srv.URL + "/ping")
	require.Error(t, err, "server must reject client with no cert")
}

// mTLSTestServer is an httptest server that requires a client cert
// issued by its embedded CA. Cert/key files are materialized to tmpdir
// so they can be passed through TLSConfig.CaFile / CertFile / KeyFile.
type mTLSTestServer struct {
	*httptest.Server
	caPool      *x509.CertPool
	caPEMPath   string
	cliCertPEM  []byte
	cliKeyPEM   []byte
	cliCertPath string
	cliKeyPath  string
}

func newMTLSTestServer(t *testing.T, handler http.HandlerFunc) *mTLSTestServer {
	t.Helper()

	caCert, caKey := mustGenCA(t)
	serverCert, serverKey := mustGenLeaf(t, caCert, caKey, "localhost", true)
	clientCert, clientKey := mustGenLeaf(t, caCert, caKey, "gnmic-test-client", false)

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	srvTLSCert, err := tls.X509KeyPair(pemCert(serverCert), pemKey(serverKey))
	require.NoError(t, err)

	srv := httptest.NewUnstartedServer(handler)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{srvTLSCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
	}
	// Suppress expected TLS-handshake-rejection noise on stderr during tests
	// like TestMTLSHarness_ClientWithoutCertIsRejected. Future tests reusing
	// this harness benefit automatically.
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	cliCertPath := filepath.Join(dir, "client.crt")
	cliKeyPath := filepath.Join(dir, "client.key")
	require.NoError(t, os.WriteFile(caPath, pemCert(caCert), 0o600))
	require.NoError(t, os.WriteFile(cliCertPath, pemCert(clientCert), 0o600))
	require.NoError(t, os.WriteFile(cliKeyPath, pemKey(clientKey), 0o600))

	return &mTLSTestServer{
		Server:      srv,
		caPool:      caPool,
		caPEMPath:   caPath,
		cliCertPEM:  pemCert(clientCert),
		cliKeyPEM:   pemKey(clientKey),
		cliCertPath: cliCertPath,
		cliKeyPath:  cliKeyPath,
	}
}

func (s *mTLSTestServer) CAPool() *x509.CertPool { return s.caPool }
func (s *mTLSTestServer) CAPath() string         { return s.caPEMPath }
func (s *mTLSTestServer) ClientCertPath() string { return s.cliCertPath }
func (s *mTLSTestServer) ClientKeyPath() string  { return s.cliKeyPath }

// NewClient returns a plain http.Client with the server CA trusted and
// the client cert loaded — useful for harness self-tests.
func (s *mTLSTestServer) NewClient() *http.Client {
	cliCert, _ := tls.X509KeyPair(s.cliCertPEM, s.cliKeyPEM)
	return &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:      s.caPool,
			Certificates: []tls.Certificate{cliCert},
			MinVersion:   tls.VersionTLS12,
		},
	}}
}

func mustGenCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "gnmic-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}

func mustGenLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, isServer bool) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		// ECDSA keys: KeyUsageDigitalSignature only. KeyUsageKeyEncipherment
		// is RSA-specific (RFC 5246) and meaningless for EC keys; the Go stdlib
		// generate_cert.go documents this explicitly.
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
	if isServer {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		// httptest.Server.URL is https://127.0.0.1:<port> (or [::1] for v6),
		// so the server cert must have IP SANs as well as the DNS SAN.
		tmpl.DNSNames = []string{"localhost"}
		tmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	} else {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}

func pemCert(c *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
}

func pemKey(k *ecdsa.PrivateKey) []byte {
	der, _ := x509.MarshalECPrivateKey(k)
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
}

func TestResolveMetricsURL(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		tls      bool
		want     string
		wantErr  bool
	}{
		{"bare_host_port_tls", "panoptes.example.com:4318", true, "https://panoptes.example.com:4318/v1/metrics", false},
		{"bare_host_port_plain", "localhost:4318", false, "http://localhost:4318/v1/metrics", false},
		{"full_https_url_with_path", "https://panoptes.example.com/api/v1/metrics", true, "https://panoptes.example.com/api/v1/metrics", false},
		{"full_http_url_no_path", "http://localhost:4318", false, "http://localhost:4318/v1/metrics", false},
		{"full_https_url_no_path_appends_default", "https://panoptes.example.com:4318", true, "https://panoptes.example.com:4318/v1/metrics", false},
		{"full_url_with_root_slash_path", "https://panoptes.example.com/", true, "https://panoptes.example.com/v1/metrics", false},
		{"empty_endpoint", "", false, "", true},
		{"url_with_whitespace", " https://x.com ", true, "https://x.com/v1/metrics", false},
		// Decision-path: explicit URL with malformed structure must reach url.Parse failure.
		{"malformed_full_url", "http://[::1", false, "", true},
		// Decision-path: synthesized bare endpoints must also be validated, otherwise garbage
		// like "foo bar:4318" survives Init and only fails much later inside http.NewRequestWithContext.
		{"bare_endpoint_with_space_rejected", "foo bar:4318", false, "", true},
		{"bare_endpoint_with_control_char_rejected", "foo\nbar:4318", false, "", true},
		// Decision-path: per the OTLP exporter spec, only http and https schemes are valid.
		{"unsupported_scheme_ftp", "ftp://example.com:4318", false, "", true},
		{"unsupported_scheme_grpc", "grpc://example.com:4317", false, "", true},
		// Decision-path: port-only ":4318" must NOT pass — SplitHostPort gives host=="".
		{"port_only_bare_rejected", ":4318", false, "", true},
		// Decision-path: bare endpoint must include a port; OTLP has no convention
		// for defaulting bare HTTP endpoints to 80/443, so silently doing so would
		// surprise operators who omitted the port by mistake.
		{"bare_no_port_rejected", "panoptes", false, "", true},
		{"bare_trailing_colon_rejected", "panoptes:", false, "", true},
		// Decision-path: bare endpoint must be host:port only; paths require a full URL.
		{"bare_endpoint_with_path_rejected", "localhost:4318/custom", false, "", true},
		// Decision-path: userinfo in URL must be rejected — auth goes through Headers config.
		{"userinfo_rejected", "https://user:pass@host:4318", true, "", true},
		// Positive: IPv6 bare and full URLs must work end-to-end.
		{"ipv6_bare_with_brackets", "[::1]:4318", false, "http://[::1]:4318/v1/metrics", false},
		{"ipv6_full_url", "https://[::1]:4318", true, "https://[::1]:4318/v1/metrics", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveMetricsURL(tc.endpoint, tc.tls)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// newInitHTTPHelperOutput is a tiny helper for tests that exercise initHTTPFor
// directly (no transport publication required). Keeps test bodies short.
func newInitHTTPHelperOutput() *otlpOutput {
	o := &otlpOutput{}
	o.state = new(atomic.Pointer[outputState])
	o.logger = logging.DiscardLogger()
	return o
}

func TestInitHTTPFor_WithMTLS(t *testing.T) {
	var (
		gotContentType string
		gotOrgID       string
	)
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotOrgID = r.Header.Get("X-Scope-OrgID")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := newInitHTTPHelperOutput()
	cfg := &config{
		Endpoint: srv.URL,
		Protocol: "http",
		TLS: &types.TLSConfig{
			CaFile:   srv.CAPath(),
			CertFile: srv.ClientCertPath(),
			KeyFile:  srv.ClientKeyPath(),
		},
		Headers: map[string]string{"X-Scope-OrgID": "tenant-1"},
	}

	hs, err := o.initHTTPFor(cfg)
	require.NoError(t, err)
	require.NotNil(t, hs)
	require.NotNil(t, hs.client)
	require.Contains(t, hs.endpoint, "/v1/metrics")

	// Headers are built per-request now, so verify they arrive at the server.
	o.state.Store(&outputState{cfg: cfg, transport: &transportState{httpState: hs}})
	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
	require.Equal(t, "application/x-protobuf", gotContentType)
	require.Equal(t, "tenant-1", gotOrgID)
}

func TestInitHTTPFor_NoTLSBlock(t *testing.T) {
	o := newInitHTTPHelperOutput()
	cfg := &config{Endpoint: "localhost:4318", Protocol: "http"}

	hs, err := o.initHTTPFor(cfg)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:4318/v1/metrics", hs.endpoint)
}

// Strict-coherence guard: configuring TLS while pinning a plaintext URL is
// almost always a misconfiguration that silently disables mTLS. Reject at Init.
func TestInitHTTPFor_PlaintextURLWithTLSBlockErrors(t *testing.T) {
	o := newInitHTTPHelperOutput()
	cfg := &config{
		Endpoint: "http://panoptes.observability.svc:4318",
		Protocol: "http",
		TLS:      &types.TLSConfig{CaFile: "/some/ca.crt"},
	}

	_, err := o.initHTTPFor(cfg)
	require.Error(t, err, "must reject http:// URL when tls block is configured")
	require.Contains(t, err.Error(), "tls")
}

// Mirror case stays permissive: https:// URL with no tls block uses system roots.
func TestInitHTTPFor_HTTPSURLWithoutTLSBlockAllowed(t *testing.T) {
	o := newInitHTTPHelperOutput()
	cfg := &config{Endpoint: "https://panoptes.example.com/v1/metrics", Protocol: "http"}

	hs, err := o.initHTTPFor(cfg)
	require.NoError(t, err)
	require.Equal(t, "https://panoptes.example.com/v1/metrics", hs.endpoint)
}

// Decision-path: createTLSConfigFor failure must propagate as a clear Init error.
func TestInitHTTPFor_TLSCreateFails(t *testing.T) {
	o := newInitHTTPHelperOutput()
	cfg := &config{
		Endpoint: "https://panoptes.example.com:4318",
		Protocol: "http",
		TLS: &types.TLSConfig{
			CaFile:   "/nonexistent/path/ca.crt",
			CertFile: "/nonexistent/path/client.crt",
			KeyFile:  "/nonexistent/path/client.key",
		},
	}

	_, err := o.initHTTPFor(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TLS")
}

// Decision-path: empty endpoint reaching resolveMetricsURL must surface as Init error.
func TestInitHTTPFor_EmptyEndpointErrors(t *testing.T) {
	o := newInitHTTPHelperOutput()
	cfg := &config{Endpoint: "", Protocol: "http"}

	_, err := o.initHTTPFor(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint is required")
}

// Decision-path: a reload that changes only Headers must take effect on the
// next batch without a transport rebuild. The old design cached headers in
// httpClientState and silently ignored such reloads.
func TestSendHTTP_HeaderReloadTakesEffect(t *testing.T) {
	var gotOrgID string
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotOrgID = r.Header.Get("X-Scope-OrgID")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	withCfg(o, func(c *config) {
		c.Headers = map[string]string{"X-Scope-OrgID": "tenant-old"}
	})
	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
	require.Equal(t, "tenant-old", gotOrgID)

	// Simulate a header-only reload — same transport, new headers.
	withCfg(o, func(c *config) {
		c.Headers = map[string]string{"X-Scope-OrgID": "tenant-new"}
	})
	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
	require.Equal(t, "tenant-new", gotOrgID, "header-only reload must take effect on the next batch")
}

// User-supplied headers cannot override the OTLP-required Content-Type or
// (when compression is on) Content-Encoding. Document and enforce this contract.
func TestSendHTTP_UserHeadersCannotOverrideProtocolHeaders(t *testing.T) {
	var gotContentType string
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	withCfg(o, func(c *config) {
		c.Headers = map[string]string{"Content-Type": "application/json"} // bogus override attempt
	})
	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
	require.Equal(t, "application/x-protobuf", gotContentType, "user Headers must not override Content-Type")
}

func TestSendHTTP_SuccessfulExport(t *testing.T) {
	var gotBody []byte
	var gotContentType, gotOrgID string

	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, otlpHTTPMetricsPath, r.URL.Path)
		gotContentType = r.Header.Get("Content-Type")
		gotOrgID = r.Header.Get("X-Scope-OrgID")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := &otlpOutput{}
	o.state = new(atomic.Pointer[outputState])
	o.logger = logging.DiscardLogger()

	cfg := &config{
		Endpoint: srv.URL,
		Protocol: "http",
		Timeout:  5 * time.Second,
		TLS: &types.TLSConfig{
			CaFile:   srv.CAPath(),
			CertFile: srv.ClientCertPath(),
			KeyFile:  srv.ClientKeyPath(),
		},
		Headers: map[string]string{"X-Scope-OrgID": "tenant-1"},
	}

	hs, err := o.initHTTPFor(cfg)
	require.NoError(t, err)
	o.state.Store(&outputState{cfg: cfg, transport: &transportState{httpState: hs}})

	// Minimal valid ExportMetricsServiceRequest.
	req := validExportRequest()
	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), req))

	require.Equal(t, "application/x-protobuf", gotContentType)
	require.Equal(t, "tenant-1", gotOrgID)

	// Round-trip check: unmarshal body and compare.
	var roundtrip metricsv1.ExportMetricsServiceRequest
	require.NoError(t, proto.Unmarshal(gotBody, &roundtrip))
}

// withCfg applies a mutation to a copy of the current config and atomically
// publishes a new outputState that shares the same *transportState pointer.
// Test-only convenience — production reloads via Update(). Direct mutation
// of state.Load().cfg is an unsynchronized write that defeats atomic.Pointer;
// this helper does it safely. Sharing the transport pointer matches Update's
// no-rebuild path (and avoids the WaitGroup-copy issue that would arise from
// allocating a fresh transportState for every cfg-only test mutation).
func withCfg(o *otlpOutput, mutate func(*config)) {
	oldState := o.state.Load()
	cfg := *oldState.cfg
	mutate(&cfg)
	newState := &outputState{
		cfg:       &cfg,
		transport: oldState.transport,
	}
	o.state.Store(newState)
}

// newHTTPTestOutput is shared by every test that drives sendHTTP through a real
// httptest server. Defining it here in Task 5 so subsequent tasks can use it
// without re-introducing.
func newHTTPTestOutput(t *testing.T, srv *mTLSTestServer) *otlpOutput {
	t.Helper()
	o := &otlpOutput{}
	o.state = new(atomic.Pointer[outputState])
	o.logger = logging.DiscardLogger()
	cfg := &config{
		Endpoint: srv.URL,
		Protocol: "http",
		Timeout:  2 * time.Second,
		TLS: &types.TLSConfig{
			CaFile:   srv.CAPath(),
			CertFile: srv.ClientCertPath(),
			KeyFile:  srv.ClientKeyPath(),
		},
	}
	hs, err := o.initHTTPFor(cfg)
	require.NoError(t, err)
	o.state.Store(&outputState{cfg: cfg, transport: &transportState{httpState: hs}})
	return o
}

// Decision-path: defensive guard for "Init never ran or partially failed".
// Constructs an outputState with cfg.Protocol == "http" but no httpState —
// exactly the shape that would be invalid in production but possible from
// hand-built test scaffolding.
func TestSendHTTP_NoStateInitialized(t *testing.T) {
	o := &otlpOutput{}
	o.state = new(atomic.Pointer[outputState])
	o.logger = logging.DiscardLogger()
	state := &outputState{cfg: &config{Protocol: "http", Endpoint: "https://x"}, transport: &transportState{}}
	o.state.Store(state)

	err := o.sendHTTP(context.Background(), state, &metricsv1.ExportMetricsServiceRequest{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not initialized")
}

// Decision-path: cfg.Timeout == 0 skips the WithTimeout branch.
// We start an mTLS server, send normally, and just verify success — exercising
// the no-timeout path. Hangs would be caught by the test runner's overall timeout.
func TestSendHTTP_NoTimeoutConfigured(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	withCfg(o, func(c *config) { c.Timeout = 0 })

	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
}

// Decision-path: client.Do failure (transport error). Stop the server first so
// the dial fails — this exercises the "HTTP export failed" branch without
// hitting the response classification code.
func TestSendHTTP_DialFailure(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	o := newHTTPTestOutput(t, srv)
	srv.Close() // close before the call so dial fails

	err := o.sendHTTP(context.Background(), o.state.Load(), validExportRequest())
	require.Error(t, err)
}

// Decision-path: proto.Marshal fails when a string field contains invalid UTF-8
// (proto3 requires UTF-8). The server handler must NOT be reached because the
// failure is local to the marshal step.
func TestSendHTTP_MarshalRejectsInvalidUTF8(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server must not receive a request when proto.Marshal fails")
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	invalid := string([]byte{0xff, 0xfe, 0xfd}) // not valid UTF-8
	// Structurally valid (validateRequest doesn't check UTF-8) so the
	// failure is isolated to the marshal step.
	req := validExportRequest()
	req.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].Name = invalid
	err := o.sendHTTP(context.Background(), o.state.Load(), req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "marshal OTLP request")
}

// Table-driven decision-path coverage for isPermanentHTTPError.
// Per OTLP spec (https://opentelemetry.io/docs/specs/otlp/), retryable status
// codes are 429, 502, 503, 504. All other 4xx and 5xx are permanent.
// Transport-level errors (errors.As fails to find *httpExportError) are
// treated as retryable — typically transient network blips.
func TestIsPermanentHTTPError_Table(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		permanent bool
	}{
		// Transport / wrapped non-HTTP errors → retryable.
		{"transport_error", fmt.Errorf("dial tcp: connection refused"), false},
		{"nil_error", nil, false}, // nil isn't permanent (caller checks err != nil first)

		// Retryable per OTLP spec.
		{"status_429_too_many_requests", &httpExportError{status: 429}, false},
		{"status_502_bad_gateway", &httpExportError{status: 502}, false},
		{"status_503_service_unavailable", &httpExportError{status: 503}, false},
		{"status_504_gateway_timeout", &httpExportError{status: 504}, false},

		// Permanent (4xx and 5xx not in retryable set).
		{"status_400_bad_request", &httpExportError{status: 400}, true},
		{"status_401_unauthorized", &httpExportError{status: 401}, true},
		{"status_403_forbidden", &httpExportError{status: 403}, true},
		{"status_404_not_found", &httpExportError{status: 404}, true},
		{"status_408_request_timeout_permanent_per_otlp", &httpExportError{status: 408}, true},
		{"status_500_internal_server_error", &httpExportError{status: 500}, true},
		{"status_501_not_implemented", &httpExportError{status: 501}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.permanent, isPermanentHTTPError(tc.err))
		})
	}
}

// Sanity check that the integration with a live server matches the table:
// permanent 400 surfaces as permanent, retryable 503 does not, and sendHTTP
// itself never retries (the loop is in sendBatch).
func TestSendHTTP_PermanentErrorNotRetried(t *testing.T) {
	var calls int
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest) // 400
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	err := o.sendHTTP(context.Background(), o.state.Load(), validExportRequest())
	require.Error(t, err)
	require.True(t, isPermanentHTTPError(err))
	require.Equal(t, 1, calls, "sendHTTP itself should not retry")
}

func TestSendHTTP_RetryableError(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // 503
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	err := o.sendHTTP(context.Background(), o.state.Load(), validExportRequest())
	require.Error(t, err)
	require.False(t, isPermanentHTTPError(err))
}

// Table-driven decision-path coverage for parseRetryAfter (RFC 7231 §7.1.3).
func TestParseRetryAfter_Table(t *testing.T) {
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"empty", "", 0},
		{"whitespace_only", "   ", 0},
		{"zero_seconds", "0", 0},
		{"five_seconds", "5", 5 * time.Second},
		{"surrounded_by_whitespace", "  5  ", 5 * time.Second},
		{"negative_seconds_clamped_to_zero", "-1", 0},
		{"unparseable", "not-a-number", 0},
		{"http_date_in_future", now.Add(10 * time.Second).UTC().Format(http.TimeFormat), 10 * time.Second},
		{"http_date_in_past", now.Add(-time.Hour).UTC().Format(http.TimeFormat), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRetryAfter(tc.in, now)
			// HTTP-date precision is seconds; allow a tiny tolerance to absorb formatting jitter.
			diff := got - tc.want
			if diff < 0 {
				diff = -diff
			}
			require.Less(t, diff, time.Second, "got=%v want=%v", got, tc.want)
		})
	}
}

func TestSendHTTP_PartialSuccessReturnsError(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Build a 200 response containing a PartialSuccess with rejected points.
		respProto := &metricsv1.ExportMetricsServiceResponse{
			PartialSuccess: &metricsv1.ExportMetricsPartialSuccess{
				RejectedDataPoints: 7,
				ErrorMessage:       "schema validation failed for 7 points",
			},
		}
		body, err := proto.Marshal(respProto)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	err := o.sendHTTP(context.Background(), o.state.Load(), validExportRequest())
	require.Error(t, err, "PartialSuccess with rejected points must surface as an error (parity with gRPC)")
	require.Contains(t, err.Error(), "rejected 7")
}

// Decision-path: PartialSuccess present but RejectedDataPoints == 0 must NOT
// surface as an error. This is the "informational" PartialSuccess case (e.g.
// server reporting metadata about a fully-accepted batch).
func TestSendHTTP_PartialSuccessZeroRejected(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		respProto := &metricsv1.ExportMetricsServiceResponse{
			PartialSuccess: &metricsv1.ExportMetricsPartialSuccess{
				RejectedDataPoints: 0,
				ErrorMessage:       "informational",
			},
		}
		body, _ := proto.Marshal(respProto)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
}

// Decision-path: empty 200 body must succeed (the early-return short-circuit).
func TestSendHTTP_EmptyResponseBody(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // no body written
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
}

// Decision-path: malformed (non-protobuf) 200 body must NOT cause an error;
// the transport-level success is authoritative. EnableMetrics is off in this
// variant, so the malformed-responses counter branch is skipped.
func TestSendHTTP_MalformedResponseBody_DebugOff(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is definitely not a protobuf"))
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	require.False(t, o.state.Load().cfg.Debug, "precondition: debug must be off for this variant")
	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
}

// Same scenario with log capture: the unmarshal failure must be logged.
// Level and metric assertions live in
// TestSendHTTP_MalformedResponseWarnsAndIncrementsMetric.
func TestSendHTTP_MalformedResponseBody_DebugOn(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not a protobuf"))
	})
	defer srv.Close()

	var logbuf bytes.Buffer
	o := newHTTPTestOutput(t, srv)
	o.logger = slog.New(slog.NewTextHandler(&logbuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	withCfg(o, func(c *config) { c.Debug = true })

	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
	require.Contains(t, logbuf.String(), "failed to unmarshal OTLP response body")
}

// validExportRequest returns the smallest request that passes validateRequest,
// for tests exercising transport behavior rather than payload content.
func validExportRequest() *metricsv1.ExportMetricsServiceRequest {
	return &metricsv1.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{{
			Resource: &resourcepb.Resource{},
			ScopeMetrics: []*metricspb.ScopeMetrics{{
				Scope: &commonpb.InstrumentationScope{},
				Metrics: []*metricspb.Metric{{
					Name: "test_metric",
					Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{
						DataPoints: []*metricspb.NumberDataPoint{{
							TimeUnixNano: 1,
							Value:        &metricspb.NumberDataPoint_AsDouble{AsDouble: 1},
						}},
					}},
				}},
			}},
		}},
	}
}

// Parity with sendGRPC: structurally invalid requests must be rejected by
// validateRequest before anything goes on the wire.
func TestSendHTTP_InvalidRequestRejectedBeforeSend(t *testing.T) {
	var serverHit atomic.Bool
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		serverHit.Store(true)
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	err := o.sendHTTP(context.Background(), o.state.Load(), &metricsv1.ExportMetricsServiceRequest{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "request validation failed")
	require.False(t, serverHit.Load(), "invalid request must not reach the collector")
}

// A 2xx response whose body doesn't unmarshal as an OTLP response is still
// accepted (transport-level success is authoritative, retrying would duplicate
// data), but must be visible to operators: Warn log + malformed-responses counter.
func TestSendHTTP_MalformedResponseWarnsAndIncrementsMetric(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not a protobuf"))
	})
	defer srv.Close()

	var logbuf bytes.Buffer
	o := newHTTPTestOutput(t, srv)
	o.logger = slog.New(slog.NewTextHandler(&logbuf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	withCfg(o, func(c *config) {
		c.EnableMetrics = true
		c.Name = "test-malformed-output"
	})

	before := testutil.ToFloat64(otlpMalformedResponses.WithLabelValues("test-malformed-output"))
	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
	after := testutil.ToFloat64(otlpMalformedResponses.WithLabelValues("test-malformed-output"))
	require.Equal(t, float64(1), after-before, "malformed response must increment the counter")
	require.Contains(t, logbuf.String(), "level=WARN", "unmarshal failure must be logged at Warn")
	require.Contains(t, logbuf.String(), "failed to unmarshal OTLP response body")
}

// Decision-path: when EnableMetrics is set, PartialSuccess rejections must
// increment the otlpRejectedDataPoints counter (parity with gRPC path).
func TestSendHTTP_PartialSuccessIncrementsMetric(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		respProto := &metricsv1.ExportMetricsServiceResponse{
			PartialSuccess: &metricsv1.ExportMetricsPartialSuccess{
				RejectedDataPoints: 3,
				ErrorMessage:       "rejected 3",
			},
		}
		body, _ := proto.Marshal(respProto)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	withCfg(o, func(c *config) {
		c.EnableMetrics = true
		c.Name = "test-metric-output"
	})

	cfgName := o.state.Load().cfg.Name
	before := testutil.ToFloat64(otlpRejectedDataPoints.WithLabelValues(cfgName))
	err := o.sendHTTP(context.Background(), o.state.Load(), validExportRequest())
	require.Error(t, err)
	after := testutil.ToFloat64(otlpRejectedDataPoints.WithLabelValues(cfgName))
	require.Equal(t, float64(3), after-before, "metric must advance by RejectedDataPoints")
}

func TestSendHTTP_RetryAfterHeaderHonored(t *testing.T) {
	start := time.Now()
	var callTimes []time.Time
	var calls int
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		callTimes = append(callTimes, time.Now())
		calls++
		if calls <= 1 {
			w.Header().Set("Retry-After", "1") // 1 second
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	withCfg(o, func(c *config) {
		c.MaxRetries = 3
		c.resourceTagSet = map[string]bool{}
	})

	// Build one event and run through the public sendBatch path so we exercise the retry loop.
	events := []*formatters.EventMsg{{
		Name: "test", Timestamp: time.Now().UnixNano(),
		Tags:   map[string]string{"source": "x"},
		Values: map[string]interface{}{"value": int64(1)},
	}}
	o.sendBatch(context.Background(), events)

	require.GreaterOrEqual(t, calls, 2, "should retry at least once")
	require.GreaterOrEqual(t, time.Since(start), 1*time.Second, "must wait >=1s per Retry-After")
}

func TestSendHTTP_PermanentErrorStopsRetryLoop(t *testing.T) {
	var calls int
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	withCfg(o, func(c *config) {
		c.MaxRetries = 5
		c.resourceTagSet = map[string]bool{}
	})

	events := []*formatters.EventMsg{{
		Name: "test", Timestamp: time.Now().UnixNano(),
		Tags:   map[string]string{"source": "x"},
		Values: map[string]interface{}{"value": int64(1)},
	}}
	o.sendBatch(context.Background(), events)

	require.Equal(t, 1, calls, "permanent 400 must not retry")
}

// Table-driven decision-path coverage for effectiveRetrySleep.
//
// gRPC and the Retry-After-honoring path are deterministic — assert exact
// values. The HTTP no-Retry-After path uses exponential backoff with full
// jitter, so we assert the result lies in the canonical [0, base) range
// where base = min(maxSleep, 2^attempt * 100ms).
func TestEffectiveRetrySleep_Table(t *testing.T) {
	const testCap = 30 * time.Second

	type bound struct {
		min, max time.Duration // inclusive lower, exclusive upper
	}
	cases := []struct {
		name       string
		protocol   string
		attempt    int
		retryAfter time.Duration
		// Exactly one of `want` or `bound` must be set.
		want  time.Duration
		bound *bound
	}{
		// gRPC: deterministic linear backoff regardless of Retry-After.
		{name: "grpc_attempt0_no_retry_after", protocol: "grpc", attempt: 0, retryAfter: 0, want: 100 * time.Millisecond},
		{name: "grpc_attempt2_no_retry_after", protocol: "grpc", attempt: 2, retryAfter: 0, want: 300 * time.Millisecond},
		{name: "grpc_ignores_retry_after", protocol: "grpc", attempt: 0, retryAfter: 5 * time.Second, want: 100 * time.Millisecond},

		// HTTP with Retry-After: deterministic.
		{name: "http_retry_after_under_cap_used", protocol: "http", attempt: 0, retryAfter: 5 * time.Second, want: 5 * time.Second},
		{name: "http_retry_after_at_cap_used", protocol: "http", attempt: 0, retryAfter: testCap, want: testCap},
		{name: "http_retry_after_above_cap_clamped", protocol: "http", attempt: 0, retryAfter: 5 * time.Minute, want: testCap},

		// HTTP without Retry-After: exponential backoff with full jitter — assert range.
		// base for attempt=0 is 100ms, attempt=1 is 200ms, attempt=2 is 400ms.
		{name: "http_attempt0_no_retry_after", protocol: "http", attempt: 0, retryAfter: 0, bound: &bound{0, 100 * time.Millisecond}},
		{name: "http_attempt1_no_retry_after", protocol: "http", attempt: 1, retryAfter: 0, bound: &bound{0, 200 * time.Millisecond}},
		{name: "http_attempt2_no_retry_after", protocol: "http", attempt: 2, retryAfter: 0, bound: &bound{0, 400 * time.Millisecond}},
		{name: "http_zero_retry_after_falls_back", protocol: "http", attempt: 0, retryAfter: 0, bound: &bound{0, 100 * time.Millisecond}},
		{name: "http_negative_retry_after_falls_back", protocol: "http", attempt: 0, retryAfter: -time.Second, bound: &bound{0, 100 * time.Millisecond}},
		// Large attempt: base saturates at maxSleep cap. 2^9 * 100ms = 51.2s > 30s cap → bound is [0, testCap).
		{name: "http_high_attempt_clamped_to_cap", protocol: "http", attempt: 9, retryAfter: 0, bound: &bound{0, testCap}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := effectiveRetrySleep(tc.protocol, tc.attempt, tc.retryAfter, testCap)
			if tc.bound != nil {
				require.GreaterOrEqual(t, got, tc.bound.min, "jitter lower bound")
				require.Less(t, got, tc.bound.max, "jitter exclusive upper bound")
				return
			}
			require.Equal(t, tc.want, got)
		})
	}
}

// TestUpdate_HTTPHeaderOnlyReload_NextBatchCarriesNewHeader drives the public
// Update() API through the no-rebuild path (only cfg.Headers changed) and
// verifies that the new header value reaches the server on the very next batch.
//
// Decision-path: needsTransportRebuild == false → cfg-only swap. Headers are
// read per-request from state.cfg.Headers (Task 15c invariant), so a reload
// that changes only the headers map takes effect without rebuilding the
// HTTP transport.
func TestUpdate_HTTPHeaderOnlyReload_NextBatchCarriesNewHeader(t *testing.T) {
	var (
		mu     sync.Mutex
		orgIDs []string
	)
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		orgIDs = append(orgIDs, r.Header.Get("X-Scope-OrgID"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	tlsCfgMap := map[string]interface{}{
		"ca-file":   srv.CAPath(),
		"cert-file": srv.ClientCertPath(),
		"key-file":  srv.ClientKeyPath(),
	}
	baseCfgMap := map[string]interface{}{
		"endpoint":   srv.URL,
		"protocol":   "http",
		"timeout":    "2s",
		"batch-size": 1,
		"interval":   "100ms",
		"tls":        tlsCfgMap,
		"headers":    map[string]interface{}{"X-Scope-OrgID": "tenant-A"},
	}

	o := &otlpOutput{}
	require.NoError(t, o.Init(context.Background(), "test-header-reload", baseCfgMap,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))
	defer o.Close()

	// Send batch A — expect tenant-A.
	o.WriteEvent(context.Background(), &formatters.EventMsg{
		Name: "test", Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{"source": "x"}, Values: map[string]interface{}{"value": int64(1)},
	})
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(orgIDs) >= 1
	}, 2*time.Second, 20*time.Millisecond, "batch A must arrive at server")

	mu.Lock()
	firstID := orgIDs[len(orgIDs)-1]
	mu.Unlock()
	require.Equal(t, "tenant-A", firstID, "first batch must carry tenant-A header")

	// Build a cfg map identical except for the header value (no transport rebuild).
	updatedCfgMap := map[string]interface{}{
		"endpoint":   srv.URL,
		"protocol":   "http",
		"timeout":    "2s",
		"batch-size": 1,
		"interval":   "100ms",
		"tls":        tlsCfgMap,
		"headers":    map[string]interface{}{"X-Scope-OrgID": "tenant-B"},
	}
	require.NoError(t, o.Update(context.Background(), updatedCfgMap))

	// Send batch B — expect tenant-B.
	countBefore := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(orgIDs)
	}()
	o.WriteEvent(context.Background(), &formatters.EventMsg{
		Name: "test", Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{"source": "x"}, Values: map[string]interface{}{"value": int64(1)},
	})
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(orgIDs) > countBefore
	}, 2*time.Second, 20*time.Millisecond, "batch B must arrive after header-only reload")

	mu.Lock()
	lastID := orgIDs[len(orgIDs)-1]
	mu.Unlock()
	require.Equal(t, "tenant-B", lastID, "batch after header-only Update must carry tenant-B header")
}

// Statistical sanity for full-jitter distribution: many samples must span the
// [0, base) range (not all clustered near zero or near base). 100 samples is
// sufficient to detect a stuck PRNG without making the test flaky.
func TestEffectiveRetrySleep_JitterDistribution(t *testing.T) {
	const testCap = 30 * time.Second
	const samples = 100
	const attempt = 4 // base = 1.6s; plenty of room for the distribution to spread

	base := time.Duration(1<<attempt) * 100 * time.Millisecond
	var minSeen, maxSeen time.Duration = base, 0
	for i := 0; i < samples; i++ {
		got := effectiveRetrySleep("http", attempt, 0, testCap)
		require.GreaterOrEqual(t, got, time.Duration(0))
		require.Less(t, got, base)
		if got < minSeen {
			minSeen = got
		}
		if got > maxSeen {
			maxSeen = got
		}
	}
	// Across 100 samples we expect to see at least the lower 25% and upper 25%
	// of the range. If the PRNG is stuck this fails loudly.
	require.Less(t, minSeen, base/4, "samples should span the lower quartile")
	require.Greater(t, maxSeen, base*3/4, "samples should span the upper quartile")
}

// Decision-path: retryAfterFromError must return 0 for plain (non-typed) errors.
// The Retry-After-honoring tests cover the typed-error path.
func TestRetryAfterFromError_NonHTTPError(t *testing.T) {
	require.Equal(t, time.Duration(0), retryAfterFromError(fmt.Errorf("plain error")))
	require.Equal(t, time.Duration(0), retryAfterFromError(nil))
}

// Decision-path: a Retry-After sleep must be interruptible by ctx cancellation
// so Close() during a retry doesn't pin the worker.
func TestSendBatch_ContextCancelledDuringRetryWait(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Always 503 with a long Retry-After so the loop wants to sleep >=1s.
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	const name = "cancelled-retry-accounting"
	withCfg(o, func(c *config) {
		c.MaxRetries = 5
		c.EnableMetrics = true
		c.Name = name
		c.resourceTagSet = map[string]bool{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond) // give first attempt time to fail and start sleeping
		cancel()
	}()

	events := []*formatters.EventMsg{{
		Name: "test", Timestamp: time.Now().UnixNano(),
		Tags:   map[string]string{"source": "x"},
		Values: map[string]interface{}{"value": int64(1)},
	}}

	start := time.Now()
	o.sendBatch(ctx, events)
	elapsed := time.Since(start)

	// Cancellation should land well before the 5-second Retry-After elapses.
	require.Less(t, elapsed, 2*time.Second, "ctx cancel must interrupt the retry sleep")

	// The cancelled batch was dropped — it must be counted as failed, not
	// silently lost from the accounting.
	require.Equal(t, float64(len(events)),
		testutil.ToFloat64(otlpNumberOfFailedEvents.WithLabelValues(name, "send_failed")),
		"batch dropped by cancellation must be counted as failed")
}

func TestOTLP_InitHTTPTransport_Succeeds(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := &otlpOutput{}
	cfgMap := map[string]interface{}{
		"type":     "otlp",
		"endpoint": srv.URL,
		"protocol": "http",
		"tls": map[string]interface{}{
			"ca-file":   srv.CAPath(),
			"cert-file": srv.ClientCertPath(),
			"key-file":  srv.ClientKeyPath(),
		},
	}
	err := o.Init(context.Background(), "test-otlp", cfgMap,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err, "Init with protocol: http must succeed once initHTTPFor is wired")
	defer o.Close()
}

// Verifies that Init calls validateConfig. We deliberately pick a failure
// mode that ONLY validateConfig surfaces — counter-pattern compilation —
// so this test can't pass via the existing Init transport switch or any
// other accidental gate. If an implementer forgets the validateConfig call
// in Init, this test fails: the bad pattern would otherwise be accepted at
// startup and silently produce a runtime issue later.
func TestOTLP_InitRunsValidateConfig(t *testing.T) {
	o := &otlpOutput{}
	cfg := map[string]interface{}{
		"endpoint":         "localhost:4317",
		"protocol":         "grpc",             // valid — passes every other Init gate
		"counter-patterns": []interface{}{"("}, // unclosed paren; only validateConfig compiles regexes
	}
	err := o.Init(context.Background(), "test-otlp", cfg,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid counter-pattern")
}

// Decision-path: validateConfig must reject protocols outside the allowed set.
func TestValidateConfig_RejectsBadProtocol(t *testing.T) {
	o := &otlpOutput{}
	cfg := &config{
		Endpoint: "localhost:4317",
		Protocol: "tcp", // not grpc/http
	}
	o.setDefaultsFor(cfg) // run defaults first so other fields are sane
	cfg.Protocol = "tcp"  // re-apply after setDefaultsFor (which would replace "" with "grpc")
	err := o.validateConfig(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported protocol")
}

// Table-driven decision-path coverage for needsTransportRebuild.
// (Compression-diff case is added in Task 10 once the field exists.)
func TestNeedsTransportRebuild_Table(t *testing.T) {
	base := func() *config {
		return &config{
			Endpoint: "localhost:4318",
			Protocol: "http",
			TLS:      &types.TLSConfig{CaFile: "/ca"},
		}
	}
	withTLS := func(c *config, ca string) *config { c.TLS = &types.TLSConfig{CaFile: ca}; return c }

	cases := []struct {
		name string
		old  *config
		nw   *config
		want bool
	}{
		{"both_nil", nil, nil, true},
		{"old_nil", nil, base(), true},
		{"new_nil", base(), nil, true},
		{"endpoint_changed", base(), func() *config { c := base(); c.Endpoint = "other:4318"; return c }(), true},
		{"protocol_changed", base(), func() *config { c := base(); c.Protocol = "grpc"; return c }(), true},
		{"tls_ca_changed", base(), withTLS(base(), "/different-ca"), true},
		{"tls_added", func() *config { c := base(); c.TLS = nil; return c }(), base(), true},
		{"tls_removed", base(), func() *config { c := base(); c.TLS = nil; return c }(), true},
		{"identical_no_rebuild", base(), base(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, needsTransportRebuild(tc.old, tc.nw))
		})
	}
}

func TestOTLP_Close_HTTPReleasesConnections(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := &otlpOutput{}
	cfgMap := map[string]interface{}{
		"type":     "otlp",
		"endpoint": srv.URL,
		"protocol": "http",
		"tls": map[string]interface{}{
			"ca-file":   srv.CAPath(),
			"cert-file": srv.ClientCertPath(),
			"key-file":  srv.ClientKeyPath(),
		},
	}
	require.NoError(t, o.Init(context.Background(), "test-otlp", cfgMap,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))

	// Close must not error and must not panic even though grpcState is nil.
	require.NoError(t, o.Close())

	// Second Close should be idempotent.
	require.NoError(t, o.Close())
}

// Decision-path: Close after initFields ran but before any transport state was
// stored — exercises every Swap(nil) path returning nil, the cancelFn nil
// guard, and the eventCh nil guard. Does NOT test the genuinely-zero-value
// case (initFields never ran) — Close is only required to be safe on the
// post-initFields surface, which is what every real Init call produces.
//
// Note: otlpOutput.wg is *sync.WaitGroup (pointer), not a value, so calling
// Close without running initFields first would nil-deref. We call initFields
// directly to mirror the post-Init shape; this is package-private so the test
// can reach it.
func TestOTLP_Close_AfterInitFieldsButBeforeTransportStored(t *testing.T) {
	o := &otlpOutput{}
	o.initFields()
	o.logger = logging.DiscardLogger()

	require.NoError(t, o.Close())
}

func TestSendHTTP_GzipCompression(t *testing.T) {
	var gotEncoding string
	var gotBody []byte
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		gr, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		gotBody, _ = io.ReadAll(gr)
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	withCfg(o, func(c *config) { c.Compression = "gzip" })
	cfg := o.state.Load().cfg
	hs, err := o.initHTTPFor(cfg)
	require.NoError(t, err)
	o.state.Store(&outputState{cfg: cfg, transport: &transportState{httpState: hs}})

	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
	require.Equal(t, "gzip", gotEncoding)

	var roundtrip metricsv1.ExportMetricsServiceRequest
	require.NoError(t, proto.Unmarshal(gotBody, &roundtrip))
}

// Default-on verification: a config with protocol:http and no compression set
// should end up with Compression=="gzip" after setDefaultsFor runs, and the
// resulting send should carry Content-Encoding: gzip.
func TestSendHTTP_GzipIsDefaultForHTTP(t *testing.T) {
	var gotEncoding string
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := &otlpOutput{}
	cfgMap := map[string]interface{}{
		"type":     "otlp",
		"endpoint": srv.URL,
		"protocol": "http",
		// compression intentionally omitted — default should kick in
		"tls": map[string]interface{}{
			"ca-file":   srv.CAPath(),
			"cert-file": srv.ClientCertPath(),
			"key-file":  srv.ClientKeyPath(),
		},
	}
	require.NoError(t, o.Init(context.Background(), "test-otlp", cfgMap,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))
	defer o.Close()

	require.Equal(t, "gzip", o.state.Load().cfg.Compression, "default must populate Compression for http")
	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
	require.Equal(t, "gzip", gotEncoding)
}

// Opt-out verification: explicit "none" disables gzip even when protocol is http.
func TestSendHTTP_CompressionNoneOptOut(t *testing.T) {
	var gotEncoding string
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := &otlpOutput{}
	cfgMap := map[string]interface{}{
		"type":        "otlp",
		"endpoint":    srv.URL,
		"protocol":    "http",
		"compression": "none",
		"tls": map[string]interface{}{
			"ca-file":   srv.CAPath(),
			"cert-file": srv.ClientCertPath(),
			"key-file":  srv.ClientKeyPath(),
		},
	}
	require.NoError(t, o.Init(context.Background(), "test-otlp", cfgMap,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))
	defer o.Close()

	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
	require.Equal(t, "", gotEncoding, "compression: none must produce no Content-Encoding header")
}

// Decision-path: validateConfig must reject compression values outside the allowed set.
func TestValidateConfig_RejectsBadCompression(t *testing.T) {
	o := &otlpOutput{}
	cfg := &config{
		Endpoint:    "localhost:4318",
		Protocol:    "http",
		Compression: "snappy", // not none/gzip
	}
	o.setDefaultsFor(cfg)
	cfg.Compression = "snappy" // re-apply in case defaults changed it
	err := o.validateConfig(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "compression")
}

// Decision-path: needsTransportRebuild must trigger on compression change.
// (Complements the table in Task 8 with the case that became valid in Task 10.)
func TestNeedsTransportRebuild_CompressionDiff(t *testing.T) {
	base := &config{Endpoint: "x:4318", Protocol: "http", Compression: "gzip"}
	changed := &config{Endpoint: "x:4318", Protocol: "http", Compression: "none"}
	require.True(t, needsTransportRebuild(base, changed))
	require.True(t, needsTransportRebuild(changed, base))
}

// Decision-path: PartialSuccess responses must NOT be retried (per OTLP spec).
// Drives sendBatch (NOT sendHTTP directly) to exercise the retry-loop dispatch.
func TestSendBatch_HTTPPartialSuccessNotRetried(t *testing.T) {
	var calls int
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		respProto := &metricsv1.ExportMetricsServiceResponse{
			PartialSuccess: &metricsv1.ExportMetricsPartialSuccess{
				RejectedDataPoints: 5,
				ErrorMessage:       "schema validation failed",
			},
		}
		body, _ := proto.Marshal(respProto)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	withCfg(o, func(c *config) {
		c.MaxRetries = 5
		c.resourceTagSet = map[string]bool{}
	})

	events := []*formatters.EventMsg{{
		Name: "test", Timestamp: time.Now().UnixNano(),
		Tags:   map[string]string{"source": "x"},
		Values: map[string]interface{}{"value": int64(1)},
	}}
	o.sendBatch(context.Background(), events)

	require.Equal(t, 1, calls, "PartialSuccess must not retry — server already accepted points")
}

// Decision-path: isPartialSuccessError detects the typed sentinel.
func TestIsPartialSuccessError(t *testing.T) {
	require.True(t, isPartialSuccessError(&partialSuccessError{rejected: 3, errorMessage: "x"}))
	require.False(t, isPartialSuccessError(fmt.Errorf("plain error")))
	require.False(t, isPartialSuccessError(nil))
	// httpExportError must NOT match — it's a different category.
	require.False(t, isPartialSuccessError(&httpExportError{status: 503}))
}

// Decision-path: an in-flight batch holding the OLD outputState must be able
// to finish even if Update swaps to a new state mid-flight. Under the wall-
// clock heuristic this would have dropped batches whose lifetime exceeded the
// fixed 60s window. Under the WaitGroup model, cleanup waits for actual users.
func TestUpdate_TransportNotClosedWhileBatchInFlight(t *testing.T) {
	// Server intentionally hangs on the first request (simulates a slow
	// upstream). The hang is released by closing 'release' from the test.
	release := make(chan struct{})
	var hangCount atomic.Int32
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if hangCount.Add(1) == 1 {
			<-release
		}
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	withCfg(o, func(c *config) {
		c.Timeout = 10 * time.Second // generous so the hang can persist
	})

	// Start an in-flight sendHTTP via sendBatch. Use a goroutine so the
	// caller doesn't block on the server hang.
	state := o.state.Load()
	state.transport.inFlight.Add(1)
	sendDone := make(chan error, 1)
	go func() {
		defer state.transport.inFlight.Done()
		sendDone <- o.sendHTTP(context.Background(), state, validExportRequest())
	}()

	// Wait until the server has received the request (so the goroutine is
	// actually mid-send and holding the state reference).
	require.Eventually(t, func() bool { return hangCount.Load() == 1 },
		2*time.Second, 25*time.Millisecond, "server should receive the first request")

	// Simulate a transport-rebuild reload by manually publishing a new state.
	// This mirrors what Update's rebuild path does: build new state, lock-
	// Swap-unlock, async Wait+cleanup.
	newCfg := *o.state.Load().cfg
	newHS, err := o.initHTTPFor(&newCfg)
	require.NoError(t, err)
	newState := &outputState{cfg: &newCfg, transport: &transportState{httpState: newHS}}

	o.stateMu.Lock()
	oldState := o.state.Swap(newState)
	o.stateMu.Unlock()

	// Track when cleanup actually fires.
	cleanupDone := make(chan struct{})
	go func() {
		oldState.transport.inFlight.Wait()
		oldState.transport.cleanup()
		close(cleanupDone)
	}()

	// Critical: cleanupDone must NOT have fired yet — the in-flight goroutine
	// is still hung on the server response. The 100ms window is comfortably
	// shorter than the test's 2s overall budget.
	select {
	case <-cleanupDone:
		t.Fatal("cleanup fired while batch was still in-flight — would close the transport under it")
	case <-time.After(100 * time.Millisecond):
		// expected: cleanup is still waiting
	}

	// Release the server hang. The goroutine completes, inFlight drains to
	// zero, cleanup fires.
	close(release)
	require.NoError(t, <-sendDone, "in-flight send should complete after release")

	select {
	case <-cleanupDone:
		// expected: cleanup fired once the WaitGroup drained
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup never fired after the in-flight batch completed")
	}
}

// Decision-path: a config-only reload between an in-flight batch and a
// transport-rebuild reload must not let the rebuild close a transport that
// the original state is still using. The fix shares *transportState across
// config-only reloads so a single inFlight WaitGroup tracks all batches on
// that transport.
//
// Two-step scenario:
//  1. Init publishes S1 = {cfg1, T1}. Batch A starts on S1, increments
//     T1.inFlight.
//  2. Config-only reload publishes S2 = {cfg2, T1} — same *transportState
//     pointer. Batch A is still running, holding T1.inFlight.
//  3. Rebuild reload publishes S3 = {cfg3, T2}. The async cleanup goroutine
//     waits on oldState.transport.inFlight which == T1.inFlight, so it
//     correctly blocks until batch A finishes before closing T1.
func TestUpdate_TwoStepReload_TransportSharedAcrossConfigOnly(t *testing.T) {
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)

	// S1: published by newHTTPTestOutput.
	s1 := o.state.Load()

	// Simulate batch A starting on S1: take a count on the transport's
	// inFlight WaitGroup. The (Load, Add) pair would be serialized via
	// stateMu in production sendBatch; we mirror that here.
	o.stateMu.Lock()
	s1.transport.inFlight.Add(1)
	o.stateMu.Unlock()

	// Config-only reload: same Headers tweak, transport pointer reused.
	// This matches Update's no-rebuild path and the withCfg helper, both of
	// which share the *transportState pointer rather than allocating a new
	// one (which would create a fresh, empty inFlight WaitGroup).
	cfg2 := *s1.cfg
	cfg2.Headers = map[string]string{"X-Scope-OrgID": "tenant-config-only"}
	s2 := &outputState{cfg: &cfg2, transport: s1.transport}
	o.stateMu.Lock()
	o.state.Store(s2)
	o.stateMu.Unlock()

	// Sanity: same transport pointer across S1 and S2.
	require.Same(t, s1.transport, s2.transport,
		"config-only reload must share *transportState — not allocate a new one")

	// Rebuild reload: build a new transportState. Cleanup of the old state
	// must Wait on s1.transport.inFlight (== s2.transport.inFlight), which
	// is non-zero because batch A is still running.
	cfg3 := *s2.cfg
	newHS, err := o.initHTTPFor(&cfg3)
	require.NoError(t, err)
	s3 := &outputState{cfg: &cfg3, transport: &transportState{httpState: newHS}}

	o.stateMu.Lock()
	oldState := o.state.Swap(s3)
	o.stateMu.Unlock()

	// The rebuild's async cleanup mirrors Update's rebuild path.
	cleanupDone := make(chan struct{})
	go func() {
		oldState.transport.inFlight.Wait()
		oldState.transport.cleanup()
		close(cleanupDone)
	}()

	// Cleanup must be blocked: batch A is still in flight on the shared
	// transport. With the buggy pre-fix code, oldState (== s2) had a fresh
	// empty inFlight, Wait() returned immediately, and cleanup tore down T1
	// while batch A was still using it.
	select {
	case <-cleanupDone:
		t.Fatal("cleanup completed before in-flight batch finished — transport could be closed under an active batch")
	case <-time.After(50 * time.Millisecond):
		// good: still blocked
	}

	// Releasing batch A unblocks cleanup.
	s1.transport.inFlight.Done()
	select {
	case <-cleanupDone:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup never fired after the in-flight batch completed")
	}
}

// Security: redirects must never be followed. A redirecting (or compromised)
// collector could otherwise pull the metrics payload and user-defined headers
// (which Go forwards cross-host, unlike Authorization) to another origin —
// 307/308 replay the POST body. CheckRedirect returns ErrUseLastResponse, so
// the 3xx surfaces as a response and classifies as a permanent error.
func TestSendHTTP_RedirectNotFollowed(t *testing.T) {
	var redirectTargetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	var originHits atomic.Int32
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		originHits.Add(1)
		w.Header().Set("Location", target.URL+"/v1/metrics")
		w.WriteHeader(http.StatusPermanentRedirect)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	err := o.sendHTTP(context.Background(), o.state.Load(), validExportRequest())
	require.Error(t, err)

	var hee *httpExportError
	require.ErrorAs(t, err, &hee)
	require.Equal(t, http.StatusPermanentRedirect, hee.status)
	require.True(t, isPermanentHTTPError(err), "3xx must classify as permanent, not retryable")
	require.Equal(t, int32(1), originHits.Load(), "origin must receive exactly one request")
	require.Equal(t, int32(0), redirectTargetHits.Load(), "redirect target must never be contacted")
}

// Contract: Content-Encoding is protocol-owned in both directions. With
// compression: none, a user-supplied Content-Encoding (any case — Header.Set
// canonicalizes) must be stripped, or the raw protobuf body is mislabelled
// and the collector tries to gunzip it.
func TestSendHTTP_UserContentEncodingStrippedWhenNoCompression(t *testing.T) {
	var gotEncoding string
	var gotBody []byte
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	withCfg(o, func(c *config) {
		c.Compression = "none"
		// lowercase on purpose: canonicalization must not defeat the strip
		c.Headers = map[string]string{"content-encoding": "gzip"}
	})

	require.NoError(t, o.sendHTTP(context.Background(), o.state.Load(), validExportRequest()))
	require.Empty(t, gotEncoding, "user Content-Encoding must be stripped when compression is none")
	var roundtrip metricsv1.ExportMetricsServiceRequest
	require.NoError(t, proto.Unmarshal(gotBody, &roundtrip), "body must be raw (uncompressed) protobuf")
}

// Accounting: PartialSuccess is request-level success — the server durably
// accepted everything except the rejected points. The batch counts toward
// sent_events (even when every point was rejected); the loss signal lives in
// rejected_data_points_total, and failed_events stays untouched. Data-loss
// alerting must watch the rejected counter, not the sent/failed pair.
func TestSendBatch_PartialSuccessCountsSent(t *testing.T) {
	respBody, err := proto.Marshal(&metricsv1.ExportMetricsServiceResponse{
		PartialSuccess: &metricsv1.ExportMetricsPartialSuccess{
			RejectedDataPoints: 2,
			ErrorMessage:       "all points rejected",
		},
	})
	require.NoError(t, err)

	var hits atomic.Int32
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(respBody)
	})
	defer srv.Close()

	o := newHTTPTestOutput(t, srv)
	const name = "partial-success-accounting"
	withCfg(o, func(c *config) {
		c.EnableMetrics = true
		c.Name = name
		c.MaxRetries = 3
		c.resourceTagSet = map[string]bool{}
	})

	events := []*formatters.EventMsg{
		{
			Name: "test", Timestamp: time.Now().UnixNano(),
			Tags:   map[string]string{"source": "x"},
			Values: map[string]interface{}{"a": int64(1)},
		},
		{
			Name: "test", Timestamp: time.Now().UnixNano(),
			Tags:   map[string]string{"source": "x"},
			Values: map[string]interface{}{"b": int64(2)},
		},
	}
	o.sendBatch(context.Background(), events)

	require.Equal(t, int32(1), hits.Load(), "partial success must not be retried")
	require.Equal(t, float64(len(events)),
		testutil.ToFloat64(otlpNumberOfSentEvents.WithLabelValues(name)),
		"partially-successful batch must count as sent")
	require.Equal(t, float64(0),
		testutil.ToFloat64(otlpNumberOfFailedEvents.WithLabelValues(name, "send_failed")),
		"partially-successful batch must not count as failed")
	require.Equal(t, float64(2),
		testutil.ToFloat64(otlpRejectedDataPoints.WithLabelValues(name)),
		"rejected points must be counted on the dedicated counter")
}
