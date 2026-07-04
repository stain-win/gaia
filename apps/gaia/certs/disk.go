package certs

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// saveCert writes a certificate to a file with explicit 0644 permissions.
func saveCert(filename string, cert *x509.Certificate) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return pem.Encode(file, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

// saveKey writes an RSA private key to a file.
func saveKey(filename string, key *rsa.PrivateKey) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return pem.Encode(file, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// savePrivateKey writes a private key (ECDSA or RSA) to a file.
func savePrivateKey(filename string, key interface{}) error {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	var block *pem.Block
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		keyBytes, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return fmt.Errorf("failed to marshal ECDSA private key: %w", err)
		}
		block = &pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: keyBytes,
		}
	case *rsa.PrivateKey:
		block = &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(k),
		}
	default:
		return fmt.Errorf("unsupported private key type: %T", key)
	}

	return pem.Encode(file, block)
}

// loadCA reads a CA certificate and private key from the disk.
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

	// Try PKCS#1 first (traditional RSA format), then fall back to PKCS#8
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		// Fallback to PKCS#8 (modern format that supports multiple key types)
		pkcs8Key, pkcs8Err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
		if pkcs8Err != nil {
			return nil, nil, fmt.Errorf("failed to parse CA private key (tried PKCS#1 and PKCS#8): %w", err)
		}
		var ok bool
		caKey, ok = pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("CA private key is not RSA (got %T)", pkcs8Key)
		}
	}

	return caCert, caKey, nil
}
