// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// certReloader holds a cached TLS certificate loaded from a cert/key file pair.
// It reloads the certificate from disk only when the files' modification times
// have changed since the last load, enabling zero-downtime certificate rotation.
type certReloader struct {
	mu        sync.Mutex
	cert      *tls.Certificate
	lastMtime time.Time
	certFile  string
	keyFile   string
}

func newCertReloader(certFile, keyFile string) (*certReloader, error) {
	r := &certReloader{
		certFile: certFile,
		keyFile:  keyFile,
	}
	// Eagerly validate: fail fast if files are missing or malformed.
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// reload reads cert and key from disk, parses the key pair, and updates the
// cache. It records the max mtime of the two files for future change detection.
// Callers that hold r.mu must not call this; it acquires no lock itself.
func (r *certReloader) reload() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, 2)
	var certBytes, keyBytes []byte

	wg := new(sync.WaitGroup)
	wg.Add(2)
	go func() {
		defer wg.Done()
		var err error
		certBytes, err = ReadLocalFile(ctx, r.certFile)
		if err != nil {
			errCh <- err
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		keyBytes, err = ReadLocalFile(ctx, r.keyFile)
		if err != nil {
			errCh <- err
		}
	}()
	wg.Wait()
	close(errCh)
	for err := range errCh {
		return err
	}

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return err
	}
	r.cert = &cert

	// Record max mtime so we can skip reloads when nothing has changed.
	if certStat, err := os.Stat(r.certFile); err == nil {
		r.lastMtime = certStat.ModTime()
	}
	if keyStat, err := os.Stat(r.keyFile); err == nil && keyStat.ModTime().After(r.lastMtime) {
		r.lastMtime = keyStat.ModTime()
	}
	return nil
}

// getCertificate returns a cached or freshly loaded certificate. It performs
// two cheap os.Stat calls on every invocation; a full file read only happens
// when one of the files has been modified since the last load.
func (r *certReloader) getCertificate() (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	certStat, certErr := os.Stat(r.certFile)
	keyStat, keyErr := os.Stat(r.keyFile)

	if certErr != nil || keyErr != nil {
		// Stat failed; fall back to the cached certificate if one exists.
		if r.cert != nil {
			return r.cert, nil
		}
		if certErr != nil {
			return nil, certErr
		}
		return nil, keyErr
	}

	latestMtime := certStat.ModTime()
	if keyStat.ModTime().After(latestMtime) {
		latestMtime = keyStat.ModTime()
	}

	// Nothing changed since the last load — return the cached certificate.
	if r.cert != nil && !latestMtime.After(r.lastMtime) {
		return r.cert, nil
	}

	// A file has changed — reload from disk.
	if err := r.reload(); err != nil {
		// Reload failed (e.g. file is mid-write). Return the cached certificate
		// so that the current TLS handshake can still proceed.
		if r.cert != nil {
			return r.cert, nil
		}
		return nil, err
	}
	return r.cert, nil
}

// caReloader holds a cached *x509.CertPool loaded from a CA file or directory.
// It reloads only when the path's modification time has changed.
type caReloader struct {
	mu        sync.Mutex
	pool      *x509.CertPool
	lastMtime time.Time
	caPath    string
}

// getPool returns a cached or freshly loaded CA cert pool.
func (r *caReloader) getPool() *x509.CertPool {
	r.mu.Lock()
	defer r.mu.Unlock()

	stat, err := os.Stat(r.caPath)
	if err != nil {
		// Stat failed; fall back to the last known-good pool.
		return r.pool
	}

	mtime := stat.ModTime()
	if r.pool != nil && !mtime.After(r.lastMtime) {
		return r.pool
	}

	pool, err := LoadCACertificates(r.caPath)
	if err != nil {
		// Reload failed (e.g. file is mid-write); use the cached pool.
		return r.pool
	}
	r.pool = pool
	r.lastMtime = mtime
	return r.pool
}

// NewTLSConfig generates a *tls.Config based on given CA, certificate, key files and skipVerify flag.
// If certificate and key are missing a self signed key pair is generated.
// The certificates paths can be local or remote, http(s) and (s)ftp are supported for remote files.
// When hotReload is true and file paths are provided, the returned *tls.Config installs
// GetCertificate / GetClientCertificate callbacks that re-read the cert/key pair from
// disk on each TLS handshake — but only when the files' modification times have changed.
// A VerifyPeerCertificate callback is likewise installed to re-validate the server
// certificate against the current CA bundle on disk. This enables zero-downtime
// rotation of client certificates and CA bundles without restarting the process.
func NewTLSConfig(ca, cert, key, clientAuth string, skipVerify, genSelfSigned, hotReload bool) (*tls.Config, error) {
	if !(skipVerify || ca != "" || (cert != "" && key != "")) {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipVerify, //nolint:gosec
	}

	// set clientAuth
	switch clientAuth {
	case "":
		if ca != "" {
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
	case "request":
		tlsConfig.ClientAuth = tls.RequestClientCert
	case "require":
		tlsConfig.ClientAuth = tls.RequireAnyClientCert
	case "verify-if-given":
		tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
	case "require-verify":
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	default:
		return nil, fmt.Errorf("unknown client-auth mode: %s", clientAuth)
	}

	if cert != "" && key != "" {
		if hotReload {
			// Install callbacks that re-read cert/key from disk on each TLS handshake,
			// but only when the files' modification times have changed. This allows
			// certificate rotation without restarting the collector.
			r, err := newCertReloader(cert, key)
			if err != nil {
				return nil, err
			}
			// GetCertificate is invoked when gnmic acts as a TLS server.
			tlsConfig.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				return r.getCertificate()
			}
			// GetClientCertificate is invoked when gnmic acts as a TLS client
			// and the server requests a client certificate (mTLS).
			tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
				return r.getCertificate()
			}
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var certBytes, keyBytes []byte

			errCh := make(chan error, 2)
			wg := new(sync.WaitGroup)
			wg.Add(2)
			go func() {
				defer wg.Done()
				var err error
				certBytes, err = ReadLocalFile(ctx, cert)
				if err != nil {
					errCh <- err
					return
				}
			}()
			go func() {
				defer wg.Done()
				var err error
				keyBytes, err = ReadLocalFile(ctx, key)
				if err != nil {
					errCh <- err
					return
				}
			}()
			wg.Wait()
			close(errCh)
			for err := range errCh {
				return nil, err
			}
			certificate, err := tls.X509KeyPair(certBytes, keyBytes)
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{certificate}
		}
	} else if genSelfSigned {
		c, err := SelfSignedCerts()
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{c}
	}

	if ca != "" {
		certPool, err := LoadCACertificates(ca)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = certPool
		tlsConfig.ClientCAs = certPool

		if hotReload && !skipVerify {
			// Install a VerifyPeerCertificate callback that re-validates the
			// server's leaf certificate against the current CA pool on disk.
			// Standard chain verification against the initial RootCAs pool
			// still runs first; this is a supplemental check that enforces
			// CA bundle changes without restarting.
			caR := &caReloader{
				caPath:    ca,
				pool:      certPool,
				lastMtime: func() time.Time {
					if st, err := os.Stat(ca); err == nil {
						return st.ModTime()
					}
					return time.Time{}
				}(),
			}
			tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
				if len(rawCerts) == 0 {
					return nil
				}
				leaf, err := x509.ParseCertificate(rawCerts[0])
				if err != nil {
					return err
				}
				opts := x509.VerifyOptions{Roots: caR.getPool()}
				if len(rawCerts) > 1 {
					opts.Intermediates = x509.NewCertPool()
					for _, rawCert := range rawCerts[1:] {
						if c, err := x509.ParseCertificate(rawCert); err == nil {
							opts.Intermediates.AddCert(c)
						}
					}
				}
				_, err = leaf.Verify(opts)
				return err
			}
		}
	}
	return tlsConfig, nil
}

func SelfSignedCerts() (tls.Certificate, error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, nil
	}
	certTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"openconfig.net"},
		},
		DNSNames:              []string{"openconfig.net"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, nil
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, nil
	}
	certBuff := new(bytes.Buffer)
	keyBuff := new(bytes.Buffer)
	pem.Encode(certBuff, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	pem.Encode(keyBuff, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return tls.X509KeyPair(certBuff.Bytes(), keyBuff.Bytes())
}

// readLocalFile reads a file from the local file system,
// unmarshals the content into a map[string]*types.TargetConfig
// and returns
func ReadLocalFile(ctx context.Context, path string) ([]byte, error) {
	// read from stdin
	if path == "-" {
		return readFromStdin(ctx)
	}

	// local file
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, fmt.Errorf("%q is a directory", path)
	}
	data := make([]byte, st.Size())

	rd := bufio.NewReader(f)
	_, err = rd.Read(data)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data, nil
}

// read bytes from stdin
func readFromStdin(ctx context.Context) ([]byte, error) {
	// read from stdin
	data := make([]byte, 0, 128)
	rd := bufio.NewReader(os.Stdin)
	buf := make([]byte, 128)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			n, err := rd.Read(buf)
			if err == io.EOF {
				data = append(data, buf[:n]...)
				return data, nil
			}
			if err != nil {
				return nil, err
			}
			data = append(data, buf[:n]...)
		}
	}
}

// LoadCACertificates reads PEM-encoded CA certificates from a file and adds them to a CertPool.
// It returns the CertPool and any error encountered.
func LoadCACertificates(caPath string) (*x509.CertPool, error) {
	st, err := os.Stat(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat the cert file: %s: %w", caPath, err)
	}
	if st.IsDir() {
		files, err := os.ReadDir(caPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read the cert directory: %s: %w", caPath, err)
		}
		certPool := x509.NewCertPool()

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			err = loadCACertificatesToPool(filepath.Join(caPath, file.Name()), certPool)
			if err != nil {
				return nil, fmt.Errorf("failed to load the cert file: %s: %w", filepath.Join(caPath, file.Name()), err)
			}
		}
		return certPool, nil
	}
	// caPath is a single cert file
	certPool := x509.NewCertPool()
	err = loadCACertificatesToPool(caPath, certPool)
	if err != nil {
		return nil, fmt.Errorf("failed to load the cert file: %s: %w", caPath, err)
	}
	return certPool, nil
}

func loadCACertificatesToPool(filePath string, certPool *x509.CertPool) error {
	certPEMBlock, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read the cert file: %s: %w", filePath, err)
	}

	for {
		block, rest := pem.Decode(certPEMBlock)
		if block == nil {
			break
		}
		certPEMBlock = rest

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}

		if !cert.IsCA {
			return fmt.Errorf("file %s contains a certificate that is not a CA", filePath)
		}
		certPool.AddCert(cert)
	}
	return nil
}
