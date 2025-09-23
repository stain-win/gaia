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
	if err := os.MkdirAll(cfg.CertsDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	caKey, caCert, err := generateCA(commonName, cfg.CertExpiryDays)
	if err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}

	certPath := filepath.Join(cfg.CertsDirectory, cfg.CACertFile)
	if err := saveCert(certPath, caCert); err != nil {
		return fmt.Errorf("failed to save CA certificate: %w", err)
	}

	keyPath := filepath.Join(cfg.CertsDirectory, "ca.key") // Assuming ca.key is the standard name
	if err := saveKey(keyPath, caKey); err != nil {
		return fmt.Errorf("failed to save CA key: %w", err)
	}

	fmt.Printf("Generated Root CA: %s and %s\n", certPath, keyPath)
	return nil
}

// GenerateServerCertificate creates a server certificate signed by the CA.
func GenerateServerCertificate(cfg *config.Config, serverName string) error {
	caCertPath := filepath.Join(cfg.CertsDirectory, cfg.CACertFile)
	caKeyPath := filepath.Join(cfg.CertsDirectory, "ca.key")

	caCert, caKey, err := loadCA(caCertPath, caKeyPath)
	if err != nil {
		return err
	}

	serverKey, serverCert, err := generateCert(serverName, caKey, caCert, true, cfg.CertExpiryDays)
	if err != nil {
		return fmt.Errorf("failed to generate server certificate: %w", err)
	}

	certPath := filepath.Join(cfg.CertsDirectory, cfg.ServerCertFile)
	if err := saveCert(certPath, serverCert); err != nil {
		return fmt.Errorf("failed to save server certificate: %w", err)
	}

	keyPath := filepath.Join(cfg.CertsDirectory, cfg.ServerKeyFile)
	if err := saveKey(keyPath, serverKey); err != nil {
		return fmt.Errorf("failed to save server key: %w", err)
	}

	fmt.Printf("Generated server certificate: %s and %s\n", certPath, keyPath)
	return nil
}

// GenerateClientCertificate creates a client certificate signed by the CA.
func GenerateClientCertificate(cfg *config.Config, clientName string) error {
	caCertPath := filepath.Join(cfg.CertsDirectory, cfg.CACertFile)
	caKeyPath := filepath.Join(cfg.CertsDirectory, "ca.key")

	caCert, caKey, err := loadCA(caCertPath, caKeyPath)
	if err != nil {
		return err
	}

	clientKey, clientCert, err := generateCert(clientName, caKey, caCert, false, cfg.CertExpiryDays)
	if err != nil {
		return fmt.Errorf("failed to generate client certificate: %w", err)
	}

	certPath := filepath.Join(cfg.CertsDirectory, clientName+".crt")
	if err := saveCert(certPath, clientCert); err != nil {
		return fmt.Errorf("failed to save client certificate: %w", err)
	}

	keyPath := filepath.Join(cfg.CertsDirectory, clientName+".key")
	if err := saveKey(keyPath, clientKey); err != nil {
		return fmt.Errorf("failed to save client key: %w", err)
	}

	fmt.Printf("Generated client certificate: %s and %s\n", certPath, keyPath)
	return nil
}

// GenerateClientCertificateData generates client certificate data in memory.
func GenerateClientCertificateData(clientName string, caCert *x509.Certificate, caKey *rsa.PrivateKey, validityDays int) (certPEM, keyPEM []byte, err error) {
	return generateClientCertData(clientName, caCert, caKey, validityDays)
}
