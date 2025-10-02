package certs

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// saveCert writes a certificate to a file.
func saveCert(filename string, cert *x509.Certificate) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

// saveKey writes a private key to a file.
func saveKey(filename string, key *rsa.PrivateKey) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// loadCA reads a CA certificate and private key from disk.
func loadCA(certPath, keyPath string) (*x509.Certificate, *rsa.PrivateKey, error) {
	caCertPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA certificate file: %w. Please run 'certs create-ca' first", err)
	}
	caKeyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA private key file: %w. Please run 'certs create-ca' first", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA private key PEM")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA private key: %w", err)
	}

	return caCert, caKey, nil
}
