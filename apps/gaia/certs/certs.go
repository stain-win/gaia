// Package certs provides certificate management for the Gaia daemon.
// It handles creation and management of the Certificate Authority (CA),
// server certificates, and client certificates for mutual TLS authentication.
package certs

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stain-win/gaia/apps/gaia/config"
)

// GenerateCA creates a new self-signed Certificate Authority and saves the certificate and private key.
func GenerateCA(cfg *config.Config, commonName string) error {
	if err := os.MkdirAll(cfg.TLS.CertsDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	caKey, caCert, err := generateCA(commonName, cfg.TLS.CertExpiryDays)
	if err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}

	certPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.CACert)
	if err := saveCert(certPath, caCert); err != nil {
		return fmt.Errorf("failed to save CA certificate: %w", err)
	}

	keyPath := filepath.Join(cfg.TLS.CertsDirectory, "ca.key")
	if err := saveKey(keyPath, caKey); err != nil {
		return fmt.Errorf("failed to save CA key: %w", err)
	}

	return nil
}

// GenerateServerCertificate creates a server certificate signed by the CA.
func GenerateServerCertificate(cfg *config.Config, serverName string) error {
	caCertPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.CACert)
	caKeyPath := filepath.Join(cfg.TLS.CertsDirectory, "ca.key")

	caCert, caKey, err := loadCA(caCertPath, caKeyPath)
	if err != nil {
		return err
	}

	serverKey, serverCert, err := generateCert(serverName, caKey, caCert, true, cfg.TLS.CertExpiryDays)
	if err != nil {
		return fmt.Errorf("failed to generate server certificate: %w", err)
	}

	certPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.ServerCert)
	if err := saveCert(certPath, serverCert); err != nil {
		return fmt.Errorf("failed to save server certificate: %w", err)
	}

	keyPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.ServerKey)
	if err := saveKey(keyPath, serverKey); err != nil {
		return fmt.Errorf("failed to save server key: %w", err)
	}

	return nil
}

// GenerateClientCertificate creates a client certificate signed by the CA using ECDSA.
// This uses ECDSA (P-256 curve) for better performance and smaller key sizes
// while maintaining equivalent security to RSA 2048-bit keys.
func GenerateClientCertificate(cfg *config.Config, clientName string) error {
	caCertPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.CACert)
	caKeyPath := filepath.Join(cfg.TLS.CertsDirectory, "ca.key")

	caCert, caKey, err := loadCA(caCertPath, caKeyPath)
	if err != nil {
		return err
	}

	clientKey, clientCert, err := generateClientCert(clientName, caKey, caCert, cfg.TLS.CertExpiryDays)
	if err != nil {
		return fmt.Errorf("failed to generate client certificate: %w", err)
	}

	certPath := filepath.Join(cfg.TLS.CertsDirectory, clientName+".crt")
	if err := saveCert(certPath, clientCert); err != nil {
		return fmt.Errorf("failed to save client certificate: %w", err)
	}

	keyPath := filepath.Join(cfg.TLS.CertsDirectory, clientName+".key")
	if err := savePrivateKey(keyPath, clientKey); err != nil {
		return fmt.Errorf("failed to save client key: %w", err)
	}

	return nil
}

// GenerateClientCertificateData generates client certificate data in memory.
func GenerateClientCertificateData(clientName string, caCert *x509.Certificate, caKey *rsa.PrivateKey, validityDays int) (certPEM, keyPEM []byte, err error) {
	return generateClientCertData(clientName, caCert, caKey, validityDays)
}
