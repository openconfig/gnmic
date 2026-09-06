package cluster_manager

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
)

func TestNewAPIClient_noTLSConfig(t *testing.T) {
	for name, cfg := range map[string]*config.Clustering{
		"nil clustering": nil,
		"nil tls":        {ClusterName: "lab"},
	} {
		t.Run(name, func(t *testing.T) {
			client, err := newAPIClient(cfg)
			if err != nil {
				t.Fatalf("newAPIClient: %v", err)
			}
			if client.Timeout != apiClientTimeout {
				t.Fatalf("timeout = %s, want %s", client.Timeout, apiClientTimeout)
			}
			if client.Transport != nil {
				t.Fatalf("expected default transport, got %#v", client.Transport)
			}
		})
	}
}

func TestNewAPIClient_invalidCert(t *testing.T) {
	_, err := newAPIClient(&config.Clustering{
		TLS: &types.TLSConfig{
			CertFile: filepath.Join(t.TempDir(), "missing.pem"),
			KeyFile:  filepath.Join(t.TempDir(), "missing-key.pem"),
		},
	})
	if err == nil {
		t.Fatal("expected error for missing cert files")
	}
}

func TestNewAPIClient_mTLS(t *testing.T) {
	caFile, certFile, keyFile, serverCert, caPool := writeTestCerts(t)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	client, err := newAPIClient(&config.Clustering{
		TLS: &types.TLSConfig{
			CaFile:   caFile,
			CertFile: certFile,
			KeyFile:  keyFile,
		},
	})
	if err != nil {
		t.Fatalf("newAPIClient: %v", err)
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("request with clustering TLS certs failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	// without a client certificate the member API must reject the request
	noCertClient, err := newAPIClient(&config.Clustering{
		TLS: &types.TLSConfig{CaFile: caFile},
	})
	if err != nil {
		t.Fatalf("newAPIClient: %v", err)
	}
	resp, err = noCertClient.Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected request without client certificate to fail")
	}
}

// writeTestCerts generates a CA, a server certificate and a client certificate,
// writes the CA and client pair to disk and returns their paths together with
// the server certificate and a pool containing the CA.
func writeTestCerts(t *testing.T) (caFile, certFile, keyFile string, serverCert tls.Certificate, caPool *x509.CertPool) {
	t.Helper()
	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	newCert := func(template *x509.Certificate) (certPEM, keyPEM []byte) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
		return certPEM, keyPEM
	}

	serverCertPEM, serverKeyPEM := newCert(&x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	})
	clientCertPEM, clientKeyPEM := newCert(&x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})

	serverCert, err = tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	caPool = x509.NewCertPool()
	caPool.AddCert(caCert)

	caFile = filepath.Join(dir, "ca.pem")
	certFile = filepath.Join(dir, "client-cert.pem")
	keyFile = filepath.Join(dir, "client-key.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	for f, b := range map[string][]byte{
		caFile:   caPEM,
		certFile: clientCertPEM,
		keyFile:  clientKeyPEM,
	} {
		if err := os.WriteFile(f, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return caFile, certFile, keyFile, serverCert, caPool
}
