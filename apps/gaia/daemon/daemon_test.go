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

// setupTestDaemon creates and initializes a test daemon with the given passphrase, ready for use.
func setupTestDaemon(t *testing.T, passphrase string) *Daemon {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_gaia.db")

	cfg := config.NewDefaultConfig()
	cfg.Daemon.DBFile = dbPath
	cfg.TLS.CertsDirectory = tmpDir

	d := NewDaemon(cfg)

	if err := d.InitializeDB(passphrase); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	createDummyCerts(t, tmpDir)

	if err := d.UnlockDB(passphrase); err != nil {
		t.Fatalf("Failed to unlock database: %v", err)
	}

	return d
}

// TestRotatePassword_Success verifies the full rotation flow: add secrets, rotate, verify decryption.
func TestRotatePassword_Success(t *testing.T) {
	oldPass := "OldSecurePassphrase123!"
	newPass := "NewSecurePassphrase456!"

	d := setupTestDaemon(t, oldPass)
	defer d.LockDB()

	// Add some secrets
	if err := d.AddSecret("common", "common", "key1", "secret-value-1"); err != nil {
		t.Fatalf("Failed to add secret: %v", err)
	}
	if err := d.AddSecret("common", "common", "key2", "secret-value-2"); err != nil {
		t.Fatalf("Failed to add secret: %v", err)
	}

	// Rotate password
	rotated, backupPath, err := d.RotatePassword(oldPass, newPass)
	if err != nil {
		t.Fatalf("RotatePassword failed: %v", err)
	}

	if rotated != 2 {
		t.Errorf("Expected 2 secrets rotated, got %d", rotated)
	}

	if backupPath == "" {
		t.Error("Expected non-empty backup path")
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup file does not exist at %s", backupPath)
	}

	// Verify secrets are still accessible with the new key (use ListSecrets which doesn't enforce policies)
	allSecrets, err := d.ListSecrets("common")
	if err != nil {
		t.Fatalf("Failed to list secrets after rotation: %v", err)
	}
	commonSecrets, ok := allSecrets["common"]
	if !ok {
		t.Fatal("Expected 'common' namespace in secrets")
	}
	if commonSecrets["key1"] != "secret-value-1" {
		t.Errorf("Expected 'secret-value-1', got '%s'", commonSecrets["key1"])
	}
	if commonSecrets["key2"] != "secret-value-2" {
		t.Errorf("Expected 'secret-value-2', got '%s'", commonSecrets["key2"])
	}

	// Verify we can lock and unlock with the new passphrase
	d.LockDB()
	createDummyCerts(t, d.config.TLS.CertsDirectory)
	if err := d.UnlockDB(newPass); err != nil {
		t.Fatalf("Failed to unlock with new passphrase: %v", err)
	}

	// Verify old passphrase no longer works
	d.LockDB()
	createDummyCerts(t, d.config.TLS.CertsDirectory)
	if err := d.UnlockDB(oldPass); err == nil {
		t.Fatal("Expected error when unlocking with old passphrase, got nil")
	}
}

// TestRotatePassword_WrongCurrentPassphrase verifies rotation fails cleanly with wrong passphrase.
func TestRotatePassword_WrongCurrentPassphrase(t *testing.T) {
	d := setupTestDaemon(t, "CorrectPassphrase123!")
	defer d.LockDB()

	if err := d.AddSecret("common", "common", "key1", "value1"); err != nil {
		t.Fatalf("Failed to add secret: %v", err)
	}

	_, _, err := d.RotatePassword("WrongPassphrase999!", "NewPassphrase456!")
	if err == nil {
		t.Fatal("Expected error for wrong current passphrase, got nil")
	}

	// Verify the secret is still accessible (no mutation occurred)
	allSecrets, err := d.ListSecrets("common")
	if err != nil {
		t.Fatalf("Secret should still be accessible after failed rotation: %v", err)
	}
	commonSecrets := allSecrets["common"]
	if commonSecrets["key1"] != "value1" {
		t.Errorf("Expected 'value1', got '%s'", commonSecrets["key1"])
	}
}

// TestRotatePassword_SamePassphrase verifies rotation is rejected when old == new.
func TestRotatePassword_SamePassphrase(t *testing.T) {
	pass := "SamePassphrase123!"
	d := setupTestDaemon(t, pass)
	defer d.LockDB()

	_, _, err := d.RotatePassword(pass, pass)
	if err == nil {
		t.Fatal("Expected error when new passphrase equals current, got nil")
	}
}

// TestRotatePassword_WeakNewPassphrase verifies weak new passphrases are rejected.
func TestRotatePassword_WeakNewPassphrase(t *testing.T) {
	d := setupTestDaemon(t, "StrongPassphrase123!")
	defer d.LockDB()

	_, _, err := d.RotatePassword("StrongPassphrase123!", "weak")
	if err == nil {
		t.Fatal("Expected error for weak new passphrase, got nil")
	}
}

// TestRotatePassword_DaemonLocked verifies rotation fails when daemon is locked.
func TestRotatePassword_DaemonLocked(t *testing.T) {
	pass := "TestPassphrase123!"
	d := setupTestDaemon(t, pass)
	d.LockDB()

	_, _, err := d.RotatePassword(pass, "NewPassphrase456!")
	if err == nil {
		t.Fatal("Expected error when daemon is locked, got nil")
	}
}

// TestRotatePassword_EmptyDatabase verifies rotation works with zero secrets.
func TestRotatePassword_EmptyDatabase(t *testing.T) {
	oldPass := "OldPassphrase123!"
	newPass := "NewPassphrase456!"

	d := setupTestDaemon(t, oldPass)
	defer d.LockDB()

	rotated, backupPath, err := d.RotatePassword(oldPass, newPass)
	if err != nil {
		t.Fatalf("RotatePassword failed on empty database: %v", err)
	}

	if rotated != 0 {
		t.Errorf("Expected 0 secrets rotated, got %d", rotated)
	}

	if backupPath == "" {
		t.Error("Expected non-empty backup path")
	}

	// Verify unlock works with new passphrase
	d.LockDB()
	createDummyCerts(t, d.config.TLS.CertsDirectory)
	if err := d.UnlockDB(newPass); err != nil {
		t.Fatalf("Failed to unlock with new passphrase after empty rotation: %v", err)
	}
}

// TestRotatePassword_BackupCreated verifies backup file is created and is valid.
func TestRotatePassword_BackupCreated(t *testing.T) {
	d := setupTestDaemon(t, "TestPassphrase123!")
	defer d.LockDB()

	_, backupPath, err := d.RotatePassword("TestPassphrase123!", "NewPassphrase456!")
	if err != nil {
		t.Fatalf("RotatePassword failed: %v", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("Backup file not found: %v", err)
	}

	if info.Size() == 0 {
		t.Error("Backup file is empty")
	}

	// Verify backup file permissions are restrictive
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected backup file permissions 0600, got %o", info.Mode().Perm())
	}
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
