// © 2024 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func generatePEMs(t *testing.T) ([]byte, []byte) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey failed: %v", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Hour)
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("rand.Int failed: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"test"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate failed: %v", err)
	}

	certBuff := new(bytes.Buffer)
	keyBuff := new(bytes.Buffer)
	pem.Encode(certBuff, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	pem.Encode(keyBuff, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certBuff.Bytes(), keyBuff.Bytes()
}

func writeCerts(t *testing.T, certPath, keyPath string) {
	t.Helper()
	certBytes, keyBytes := generatePEMs(t)
	if err := os.WriteFile(certPath, certBytes, 0644); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyBytes, 0644); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}
}

func forceMtime(t *testing.T, path string, offset time.Duration) {
	t.Helper()
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	newTime := stat.ModTime().Add(offset)
	if err := os.Chtimes(path, newTime, newTime); err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}
}

func TestCertReloader_HotReload(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	writeCerts(t, certPath, keyPath)

	r, err := newCertReloader(certPath, keyPath)
	if err != nil {
		t.Fatalf("newCertReloader failed: %v", err)
	}
	cert1, err := r.getCertificate()
	if err != nil {
		t.Fatalf("first getCertificate() failed: %v", err)
	}
	if cert1 == nil {
		t.Fatal("expected a valid certificate")
	}

	writeCerts(t, certPath, keyPath)
	forceMtime(t, certPath, 1*time.Second)
	forceMtime(t, keyPath, 1*time.Second)

	cert2, err := r.getCertificate()
	if err != nil {
		t.Fatalf("second getCertificate() failed: %v", err)
	}
	if cert1 == cert2 {
		t.Fatal("expected new certificate instance after hot reload, got cached instance")
	}

	cert3, err := r.getCertificate()
	if err != nil {
		t.Fatalf("third getCertificate() failed: %v", err)
	}
	if cert2 != cert3 {
		t.Fatal("expected cached certificate, got new instance")
	}
}

func TestCertReloader_Fallback(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	writeCerts(t, certPath, keyPath)
	r, err := newCertReloader(certPath, keyPath)
	if err != nil {
		t.Fatalf("newCertReloader failed: %v", err)
	}
	
	validCert, err := r.getCertificate()
	if err != nil {
		t.Fatalf("initial getCertificate() failed: %v", err)
	}

	os.WriteFile(certPath, []byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----"), 0644)
	forceMtime(t, certPath, 1*time.Second)
	
	fallbackCert, err := r.getCertificate()
	if err != nil {
		t.Fatalf("fallback getCertificate() returned unexpected error: %v", err)
	}
	if validCert != fallbackCert {
		t.Fatal("expected cached valid certificate on fallback, got something else")
	}
}

func TestCAReloader_HotReloadAndFallback(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	
	certBytes, _ := generatePEMs(t)
	if err := os.WriteFile(caPath, certBytes, 0644); err != nil {
		t.Fatalf("failed to write ca: %v", err)
	}

	r := &caReloader{caPath: caPath}
	pool1 := r.getPool()
	if pool1 == nil {
		t.Fatal("first getPool() returned nil")
	}

	os.WriteFile(caPath, []byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----"), 0644)
	forceMtime(t, caPath, 1*time.Second)
	
	pool2 := r.getPool()
	if pool1 != pool2 {
		t.Fatal("expected cached pool on bad reload, got different pool")
	}

	os.Remove(caPath)
	pool3 := r.getPool()
	if pool1 != pool3 {
		t.Fatal("expected cached pool when Stat fails, got different pool")
	}
}

func TestCertReloader_ThreadSafety(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	writeCerts(t, certPath, keyPath)
	r, err := newCertReloader(certPath, keyPath)
	if err != nil {
		t.Fatalf("newCertReloader failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.getCertificate()
		}()
	}
	wg.Wait()
}





