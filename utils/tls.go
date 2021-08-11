package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"math/big"
	"time"
)

func NewTLSConfig(ca, cert, key string, skipVerify bool) (*tls.Config, error) {
	if !(skipVerify || ca != "" || (cert != "" && key != "")) {
		return nil, nil
	}
	tlsConfig := &tls.Config{
		Renegotiation:      tls.RenegotiateNever,
		InsecureSkipVerify: skipVerify,
	}
	if cert != "" && key != "" {
		certificate, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
		// tlscfg.BuildNameToCertificate()
	} else {
		cert, err := SelfSignedCerts()
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	if ca != "" {
		certPool := x509.NewCertPool()
		caFile, err := ioutil.ReadFile(ca)
		if err != nil {
			return nil, err
		}
		if ok := certPool.AppendCertsFromPEM(caFile); !ok {
			return nil, errors.New("failed to append certificate")
		}
		tlsConfig.RootCAs = certPool
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
			Organization: []string{"kmrd.dev"},
		},
		DNSNames:              []string{"kmrd.dev"},
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
