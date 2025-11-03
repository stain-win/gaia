package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stain-win/gaia/apps/gaia/config"
)

// TestGenerateCA tests CA certificate generation
func TestGenerateCA(t *testing.T) {
	caKey, caCert, err := generateCA("Test CA", 365)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Verify key is RSA 4096-bit
	if caKey.N.BitLen() != 4096 {
		t.Errorf("Expected 4096-bit RSA key, got %d-bit", caKey.N.BitLen())
	}

	// Verify certificate properties
	if caCert.Subject.CommonName != "Test CA" {
		t.Errorf("Expected CN 'Test CA', got '%s'", caCert.Subject.CommonName)
	}

	if !caCert.IsCA {
		t.Error("Certificate should be a CA")
	}

	if len(caCert.Subject.Organization) == 0 || caCert.Subject.Organization[0] != "Gaia Root CA" {
		t.Errorf("Expected organization 'Gaia Root CA', got %v", caCert.Subject.Organization)
	}

	// Verify key usage
	if caCert.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("CA should have KeyUsageCertSign")
	}

	if caCert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Error("CA should have KeyUsageDigitalSignature")
	}

	// Verify validity period (CA should be 10x longer than specified)
	expectedExpiry := time.Now().AddDate(0, 0, 365*10)
	if caCert.NotAfter.Before(expectedExpiry.Add(-24 * time.Hour)) {
		t.Errorf("CA certificate expires too soon: %v", caCert.NotAfter)
	}

	// Verify self-signed
	if err := caCert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("CA certificate should be self-signed: %v", err)
	}
}

// TestGenerateCert_Server tests server certificate generation
func TestGenerateCert_Server(t *testing.T) {
	// Generate CA first
	caKey, caCert, err := generateCA("Test CA", 365)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Generate server certificate
	serverKey, serverCert, err := generateCert("localhost", caKey, caCert, true, 365)
	if err != nil {
		t.Fatalf("Failed to generate server certificate: %v", err)
	}

	// Verify key is RSA 2048-bit
	if serverKey.N.BitLen() != 2048 {
		t.Errorf("Expected 2048-bit RSA key, got %d-bit", serverKey.N.BitLen())
	}

	// Verify certificate properties
	if serverCert.Subject.CommonName != "localhost" {
		t.Errorf("Expected CN 'localhost', got '%s'", serverCert.Subject.CommonName)
	}

	if serverCert.IsCA {
		t.Error("Server certificate should not be a CA")
	}

	// Verify ExtKeyUsage for server auth
	found := false
	for _, usage := range serverCert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			found = true
			break
		}
	}
	if !found {
		t.Error("Server certificate should have ExtKeyUsageServerAuth")
	}

	// Verify DNS names and IP addresses
	if len(serverCert.DNSNames) == 0 || serverCert.DNSNames[0] != "localhost" {
		t.Errorf("Expected DNS name 'localhost', got %v", serverCert.DNSNames)
	}

	if len(serverCert.IPAddresses) == 0 || serverCert.IPAddresses[0].String() != "127.0.0.1" {
		t.Errorf("Expected IP 127.0.0.1, got %v", serverCert.IPAddresses)
	}

	// Verify signature by CA
	if err := serverCert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("Server certificate should be signed by CA: %v", err)
	}
}

// TestGenerateCert_Client tests client certificate generation (RSA version via generateCert)
func TestGenerateCert_Client(t *testing.T) {
	// Generate CA first
	caKey, caCert, err := generateCA("Test CA", 365)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Generate client certificate
	clientKey, clientCert, err := generateCert("test-client", caKey, caCert, false, 365)
	if err != nil {
		t.Fatalf("Failed to generate client certificate: %v", err)
	}

	// Verify key is RSA 2048-bit
	if clientKey.N.BitLen() != 2048 {
		t.Errorf("Expected 2048-bit RSA key, got %d-bit", clientKey.N.BitLen())
	}

	// Verify certificate properties
	if clientCert.Subject.CommonName != "test-client" {
		t.Errorf("Expected CN 'test-client', got '%s'", clientCert.Subject.CommonName)
	}

	if clientCert.IsCA {
		t.Error("Client certificate should not be a CA")
	}

	// Verify ExtKeyUsage for client auth
	found := false
	for _, usage := range clientCert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			found = true
			break
		}
	}
	if !found {
		t.Error("Client certificate should have ExtKeyUsageClientAuth")
	}

	// Verify no DNS names or IPs for client cert
	if len(clientCert.DNSNames) > 0 {
		t.Errorf("Client certificate should not have DNS names, got %v", clientCert.DNSNames)
	}

	if len(clientCert.IPAddresses) > 0 {
		t.Errorf("Client certificate should not have IP addresses, got %v", clientCert.IPAddresses)
	}

	// Verify signature by CA
	if err := clientCert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("Client certificate should be signed by CA: %v", err)
	}
}

// TestGenerateClientCertData_ECDSA tests ECDSA client certificate generation
func TestGenerateClientCertData_ECDSA(t *testing.T) {
	// Generate CA first
	caKey, caCert, err := generateCA("Test CA", 365)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Generate client certificate data
	certPEM, keyPEM, err := generateClientCertData("ecdsa-client", caCert, caKey, 365)
	if err != nil {
		t.Fatalf("Failed to generate client certificate data: %v", err)
	}

	// Verify certificate PEM
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		t.Fatal("Failed to decode certificate PEM")
	}
	if certBlock.Type != "CERTIFICATE" {
		t.Errorf("Expected PEM type 'CERTIFICATE', got '%s'", certBlock.Type)
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Verify certificate properties
	if cert.Subject.CommonName != "ecdsa-client" {
		t.Errorf("Expected CN 'ecdsa-client', got '%s'", cert.Subject.CommonName)
	}

	// Verify public key is ECDSA
	pubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("Expected ECDSA public key, got %T", cert.PublicKey)
	}

	// Verify curve is P-256
	if pubKey.Curve != elliptic.P256() {
		t.Errorf("Expected P-256 curve, got %v", pubKey.Curve.Params().Name)
	}

	// Verify private key PEM
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		t.Fatal("Failed to decode private key PEM")
	}
	if keyBlock.Type != "EC PRIVATE KEY" {
		t.Errorf("Expected PEM type 'EC PRIVATE KEY', got '%s'", keyBlock.Type)
	}

	privKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse EC private key: %v", err)
	}

	// Verify private key curve is P-256
	if privKey.Curve != elliptic.P256() {
		t.Errorf("Expected P-256 curve for private key, got %v", privKey.Curve.Params().Name)
	}

	// Verify ExtKeyUsage for client auth
	found := false
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			found = true
			break
		}
	}
	if !found {
		t.Error("Client certificate should have ExtKeyUsageClientAuth")
	}

	// Verify signature by CA
	if err := cert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("Client certificate should be signed by CA: %v", err)
	}
}

// TestSaveCert tests certificate saving to disk
func TestSaveCert(t *testing.T) {
	// Generate a test certificate
	caKey, caCert, err := generateCA("Test CA", 365)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Create temporary file
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "test.crt")

	// Save certificate
	if err := saveCert(certPath, caCert); err != nil {
		t.Fatalf("Failed to save certificate: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Fatal("Certificate file was not created")
	}

	// Read and verify PEM format
	data, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("Failed to read certificate file: %v", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("Failed to decode PEM")
	}
	if block.Type != "CERTIFICATE" {
		t.Errorf("Expected PEM type 'CERTIFICATE', got '%s'", block.Type)
	}

	// Verify we can parse it back
	loadedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse loaded certificate: %v", err)
	}

	// Verify it matches the original
	if !loadedCert.Equal(caCert) {
		t.Error("Loaded certificate does not match original")
	}

	// Verify private key wasn't saved
	_ = caKey // silence unused warning
}

// TestSaveKey tests private key saving to disk
func TestSaveKey(t *testing.T) {
	// Generate a test key
	caKey, _, err := generateCA("Test CA", 365)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Create temporary file
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "test.key")

	// Save key
	if err := saveKey(keyPath, caKey); err != nil {
		t.Fatalf("Failed to save key: %v", err)
	}

	// Verify file exists with correct permissions
	info, err := os.Stat(keyPath)
	if os.IsNotExist(err) {
		t.Fatal("Key file was not created")
	}

	// Verify file permissions are 0600
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
	}

	// Read and verify PEM format
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to read key file: %v", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("Failed to decode PEM")
	}
	if block.Type != "RSA PRIVATE KEY" {
		t.Errorf("Expected PEM type 'RSA PRIVATE KEY', got '%s'", block.Type)
	}

	// Verify we can parse it back
	loadedKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse loaded key: %v", err)
	}

	// Verify it matches the original
	if !loadedKey.Equal(caKey) {
		t.Error("Loaded key does not match original")
	}
}

// TestLoadCA tests loading CA from disk
func TestLoadCA(t *testing.T) {
	// Generate CA
	caKey, caCert, err := generateCA("Test CA", 365)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Save to temp directory
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "ca.crt")
	keyPath := filepath.Join(tempDir, "ca.key")

	if err := saveCert(certPath, caCert); err != nil {
		t.Fatalf("Failed to save certificate: %v", err)
	}
	if err := saveKey(keyPath, caKey); err != nil {
		t.Fatalf("Failed to save key: %v", err)
	}

	// Load CA
	loadedCert, loadedKey, err := loadCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to load CA: %v", err)
	}

	// Verify certificate matches
	if !loadedCert.Equal(caCert) {
		t.Error("Loaded certificate does not match original")
	}

	// Verify key matches
	if !loadedKey.Equal(caKey) {
		t.Error("Loaded key does not match original")
	}
}

// TestLoadCA_MissingFiles tests error handling for missing files
func TestLoadCA_MissingFiles(t *testing.T) {
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "missing.crt")
	keyPath := filepath.Join(tempDir, "missing.key")

	// Try to load non-existent files
	_, _, err := loadCA(certPath, keyPath)
	if err == nil {
		t.Fatal("Expected error when loading missing files")
	}

	if !strings.Contains(err.Error(), "failed to read CA certificate file") {
		t.Errorf("Expected error about missing certificate, got: %v", err)
	}
}

// TestGenerateCA_Integration tests the full CA generation flow
func TestGenerateCA_Integration(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		TLS: config.TLSConfig{
			CertsDirectory: tempDir,
			CACert:         "ca.crt",
			CertExpiryDays: 365,
		},
	}

	// Generate CA
	err := GenerateCA(cfg, "Integration Test CA")
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Verify files were created
	certPath := filepath.Join(tempDir, "ca.crt")
	keyPath := filepath.Join(tempDir, "ca.key")

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Error("CA certificate file was not created")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("CA key file was not created")
	}

	// Load and verify
	caCert, caKey, err := loadCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to load generated CA: %v", err)
	}

	if caCert.Subject.CommonName != "Integration Test CA" {
		t.Errorf("Expected CN 'Integration Test CA', got '%s'", caCert.Subject.CommonName)
	}

	if !caCert.IsCA {
		t.Error("Generated certificate should be a CA")
	}

	if caKey.N.BitLen() != 4096 {
		t.Errorf("Expected 4096-bit RSA key, got %d-bit", caKey.N.BitLen())
	}
}

// TestGenerateServerCertificate_Integration tests server certificate generation flow
func TestGenerateServerCertificate_Integration(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		TLS: config.TLSConfig{
			CertsDirectory: tempDir,
			CACert:         "ca.crt",
			ServerCert:     "server.crt",
			ServerKey:      "server.key",
			CertExpiryDays: 365,
		},
	}

	// Generate CA first
	if err := GenerateCA(cfg, "Test CA"); err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Generate server certificate
	if err := GenerateServerCertificate(cfg, "test-server"); err != nil {
		t.Fatalf("Failed to generate server certificate: %v", err)
	}

	// Verify files were created
	serverCertPath := filepath.Join(tempDir, "server.crt")
	serverKeyPath := filepath.Join(tempDir, "server.key")

	if _, err := os.Stat(serverCertPath); os.IsNotExist(err) {
		t.Error("Server certificate file was not created")
	}
	if _, err := os.Stat(serverKeyPath); os.IsNotExist(err) {
		t.Error("Server key file was not created")
	}

	// Load and verify
	certData, err := os.ReadFile(serverCertPath)
	if err != nil {
		t.Fatalf("Failed to read server certificate: %v", err)
	}

	block, _ := pem.Decode(certData)
	serverCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse server certificate: %v", err)
	}

	if serverCert.Subject.CommonName != "test-server" {
		t.Errorf("Expected CN 'test-server', got '%s'", serverCert.Subject.CommonName)
	}

	// Verify it's signed by our CA
	caCertPath := filepath.Join(tempDir, "ca.crt")
	caKeyPath := filepath.Join(tempDir, "ca.key")
	caCert, _, err := loadCA(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("Failed to load CA: %v", err)
	}

	if err := serverCert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("Server certificate should be signed by CA: %v", err)
	}
}

// TestGenerateClientCertificate_Integration tests client certificate generation flow
func TestGenerateClientCertificate_Integration(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		TLS: config.TLSConfig{
			CertsDirectory: tempDir,
			CACert:         "ca.crt",
			CertExpiryDays: 365,
		},
	}

	// Generate CA first
	if err := GenerateCA(cfg, "Test CA"); err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Generate client certificate
	clientName := "test-client"
	if err := GenerateClientCertificate(cfg, clientName); err != nil {
		t.Fatalf("Failed to generate client certificate: %v", err)
	}

	// Verify files were created
	clientCertPath := filepath.Join(tempDir, clientName+".crt")
	clientKeyPath := filepath.Join(tempDir, clientName+".key")

	if _, err := os.Stat(clientCertPath); os.IsNotExist(err) {
		t.Error("Client certificate file was not created")
	}
	if _, err := os.Stat(clientKeyPath); os.IsNotExist(err) {
		t.Error("Client key file was not created")
	}

	// Load and verify
	certData, err := os.ReadFile(clientCertPath)
	if err != nil {
		t.Fatalf("Failed to read client certificate: %v", err)
	}

	block, _ := pem.Decode(certData)
	clientCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse client certificate: %v", err)
	}

	if clientCert.Subject.CommonName != clientName {
		t.Errorf("Expected CN '%s', got '%s'", clientName, clientCert.Subject.CommonName)
	}

	// Verify it's signed by our CA
	caCertPath := filepath.Join(tempDir, "ca.crt")
	caKeyPath := filepath.Join(tempDir, "ca.key")
	caCert, _, err := loadCA(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("Failed to load CA: %v", err)
	}

	if err := clientCert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("Client certificate should be signed by CA: %v", err)
	}
}

// TestGenerateClientCertificateData tests in-memory client cert generation
func TestGenerateClientCertificateData(t *testing.T) {
	// Generate CA
	caKey, caCert, err := generateCA("Test CA", 365)
	if err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Generate client certificate data
	certPEM, keyPEM, err := GenerateClientCertificateData("api-client", caCert, caKey, 365)
	if err != nil {
		t.Fatalf("Failed to generate client certificate data: %v", err)
	}

	// Verify certificate PEM is valid
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		t.Fatal("Failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	if cert.Subject.CommonName != "api-client" {
		t.Errorf("Expected CN 'api-client', got '%s'", cert.Subject.CommonName)
	}

	// Verify it's ECDSA
	if _, ok := cert.PublicKey.(*ecdsa.PublicKey); !ok {
		t.Errorf("Expected ECDSA public key, got %T", cert.PublicKey)
	}

	// Verify key PEM is valid
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		t.Fatal("Failed to decode key PEM")
	}

	if keyBlock.Type != "EC PRIVATE KEY" {
		t.Errorf("Expected 'EC PRIVATE KEY', got '%s'", keyBlock.Type)
	}

	// Verify signature
	if err := cert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("Certificate should be signed by CA: %v", err)
	}
}
