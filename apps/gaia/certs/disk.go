package certs

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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

// savePrivateKey writes a client private key (ECDSA or RSA) to a file, owner-only
// (0600). See RelaxKeyACLMask for why callers should invoke it afterward when the
// key needs to be consumed by a separate service account.
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

// RelaxKeyACLMask is a best-effort, Linux-only fixup for a specific POSIX ACL
// quirk: when a directory carries a default ACL (e.g. an operator-configured
// "setfacl -d -m g:docker:rX certs/" granting a service account read access to
// newly-created client certs), the named entries (user:foo, group:docker, ...)
// are correctly copied onto any new file — but the new file's ACL *mask* entry
// is computed as the default ACL's mask ANDed with the requested mode's group
// bits. Since private keys are written with mode 0600 (group bits zero), that
// mask collapses to zero and silently makes every named entry's *effective*
// permission nothing, even though the entries themselves look right in getfacl.
//
// Rather than compensate by widening the file's own mode bits (which would
// grant the owning group blanket read access in every deployment, including
// ones with no ACL scheme at all), this only loosens the mask enough to let
// whatever entries the operator already configured actually take effect.
// It intentionally does not touch or add any ACL entries itself. Any failure
// here (missing setfacl, non-Linux, no ACL support on the filesystem, no
// default ACL present) is silently ignored — the key is already safely
// readable by its owner regardless of whether this succeeds.
func RelaxKeyACLMask(filename string) {
	if runtime.GOOS != "linux" {
		return
	}
	if _, err := exec.LookPath("setfacl"); err != nil {
		return
	}
	_ = exec.Command("setfacl", "-m", "mask::r--", filename).Run()
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
