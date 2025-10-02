package certs

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// generateCA creates a self-signed Root Certificate Authority.
func generateCA(commonName string, validityDays int) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"Gaia Root CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, validityDays*10), // CA valid for 10x longer
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	return key, cert, nil
}

// generateCert creates a certificate signed by the given CA.
func generateCert(commonName string, caKey *rsa.PrivateKey, caCert *x509.Certificate, isServer bool, validityDays int) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(0, 0, validityDays),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}
	if isServer {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		template.DNSNames = []string{"localhost"}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return key, cert, nil
}

// generateClientCertData creates a new client certificate and returns the PEM-encoded data.
func generateClientCertData(clientName string, caCert *x509.Certificate, caKey *rsa.PrivateKey, validityDays int) (certPEM, keyPEM []byte, err error) {
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate client key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: clientName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, validityDays),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create client certificate: %w", err)
	}

	certBuf := new(bytes.Buffer)
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate to PEM: %w", err)
	}

	keyBuf := new(bytes.Buffer)
	if err := pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key to PEM: %w", err)
	}

	return certBuf.Bytes(), keyBuf.Bytes(), nil
}
