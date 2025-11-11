package daemon

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stain-win/gaia/apps/gaia/config"
)

// TestUnlockDB_InvalidPassphrase verifies that unlocking with wrong passphrase fails.
func TestUnlockDB_InvalidPassphrase(t *testing.T) {
	// Create a temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_gaia.db")

	cfg := config.NewDefaultConfig()
	cfg.Daemon.DBFile = dbPath
	cfg.TLS.CertsDirectory = tmpDir

	d := NewDaemon(cfg)

	// Initialize database with a known passphrase
	correctPassphrase := "correct-test-passphrase-12345"
	err := d.InitializeDB(correctPassphrase)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Try to unlock with wrong passphrase
	wrongPassphrase := "wrong-passphrase"
	err = d.UnlockDB(wrongPassphrase)
	if err == nil {
		t.Fatal("Expected error when unlocking with wrong passphrase, got nil")
	}
	if err.Error() != "invalid passphrase" {
		t.Errorf("Expected 'invalid passphrase' error, got: %v", err)
	}

	// Verify daemon is still locked
	if !d.isLocked {
		t.Error("Expected daemon to remain locked after failed unlock")
	}

	// Verify unlock works with correct passphrase
	// First, need to create dummy certs for the test
	createDummyCerts(t, tmpDir)

	err = d.UnlockDB(correctPassphrase)
	if err != nil {
		t.Fatalf("Failed to unlock with correct passphrase: %v", err)
	}
	if d.isLocked {
		t.Error("Expected daemon to be unlocked after correct passphrase")
	}

	// Clean up
	d.LockDB()
}

// createDummyCerts creates properly formatted CA certs for testing.
func createDummyCerts(t *testing.T, dir string) {
	t.Helper()

	// Generate a proper RSA key for testing
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate CA key: %v", err)
	}

	// Create a self-signed CA certificate
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Test CA",
			Organization: []string{"Gaia Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("Failed to create CA certificate: %v", err)
	}

	// Write CA key
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caKey),
	})
	if err := os.WriteFile(filepath.Join(dir, "ca.key"), keyPEM, 0600); err != nil {
		t.Fatalf("Failed to write test CA key: %v", err)
	}

	// Write CA certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), certPEM, 0644); err != nil {
		t.Fatalf("Failed to write test CA cert: %v", err)
	}
}
