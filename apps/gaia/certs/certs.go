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
	"os"
	"path/filepath"
	"time"
)

// GenerateTLSCertificates creates a self-signed CA, a server certificate, and a client certificate pair.
func GenerateTLSCertificates(outputDir, caName, serverName, clientName string) error {
	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 1. Generate the Root CA
	caKey, caCert, err := generateCA(caName)
	if err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}
	caCertFile := fmt.Sprintf("%s/ca.crt", outputDir)
	if err := saveCert(caCertFile, caCert); err != nil {
		return fmt.Errorf("failed to save CA certificate: %w", err)
	}
	fmt.Printf("Generated Root CA certificate: %s\n", caCertFile)

	// 2. Generate the Server Certificate
	serverKey, serverCert, err := generateCert(serverName, caKey, caCert, true)
	if err != nil {
		return fmt.Errorf("failed to generate server certificate: %w", err)
	}
	serverCertFile := fmt.Sprintf("%s/server.crt", outputDir)
	if err := saveCert(serverCertFile, serverCert); err != nil {
		return fmt.Errorf("failed to save server certificate: %w", err)
	}
	serverKeyFile := fmt.Sprintf("%s/server.key", outputDir)
	if err := saveKey(serverKeyFile, serverKey); err != nil {
		return fmt.Errorf("failed to save server key: %w", err)
	}
	fmt.Printf("Generated server certificate and key: %s, %s\n", serverCertFile, serverKeyFile)

	// 3. Generate the Client Certificate
	clientKey, clientCert, err := generateCert(clientName, caKey, caCert, false)
	if err != nil {
		return fmt.Errorf("failed to generate client certificate: %w", err)
	}
	clientCertFile := fmt.Sprintf("%s/client.crt", outputDir)
	if err := saveCert(clientCertFile, clientCert); err != nil {
		return fmt.Errorf("failed to save client certificate: %w", err)
	}
	clientKeyFile := fmt.Sprintf("%s/client.key", outputDir)
	if err := saveKey(clientKeyFile, clientKey); err != nil {
		return fmt.Errorf("failed to save client key: %w", err)
	}
	fmt.Printf("Generated client certificate and key: %s, %s\n", clientCertFile, clientKeyFile)

	return nil
}

func GenerateClientCertificateData(clientName string, caCert *x509.Certificate, caKey *rsa.PrivateKey) (certPEM, keyPEM []byte, err error) {
	// Generate a new private key for the client.
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate client key: %w", err)
	}

	// Create the certificate template for the client.
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: clientName, // This is the client's identity
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0), // Valid for 1 year
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, // Specifies this is a client cert
	}

	// Create the certificate by signing the template with the CA's private key.
	certBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create client certificate: %w", err)
	}

	// Encode the certificate to PEM format in memory.
	certBuf := new(bytes.Buffer)
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate to PEM: %w", err)
	}

	// Encode the private key to PEM format in memory.
	keyBuf := new(bytes.Buffer)
	if err := pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}); err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key to PEM: %w", err)
	}

	return certBuf.Bytes(), keyBuf.Bytes(), nil
}

// generateCA creates a self-signed Root Certificate Authority.
func generateCA(commonName string) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
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
		NotAfter:              time.Now().AddDate(10, 0, 0), // Valid for 10 years
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
func generateCert(commonName string, caKey *rsa.PrivateKey, caCert *x509.Certificate, isServer bool) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0), // Valid for 1 year
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
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

// saveCert writes a certificate to a file.
func saveCert(filename string, cert *x509.Certificate) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Printf("failed to close file: %v\n", err)
		}
	}(file)

	return pem.Encode(file, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

// saveKey writes a private key to a file.
func saveKey(filename string, key *rsa.PrivateKey) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Printf("failed to close file: %v\n", err)
		}
	}(file)

	return pem.Encode(file, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// GenerateCA creates a new self-signed Certificate Authority and saves the
// certificate and private key to the specified output directory.
func GenerateCA(outputDir, commonName string) error {
	// 1. Create the output directory if it doesn't exist.
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 2. Generate a new private key for the CA.
	caKey, err := rsa.GenerateKey(rand.Reader, 4096) // Using 4096 bits for a root CA is a good practice.
	if err != nil {
		return fmt.Errorf("failed to generate CA private key: %w", err)
	}

	// 3. Create the template for the CA certificate.
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"Gaia Root CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // Valid for 10 years
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// 4. Create the self-signed certificate.
	caCertBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	// 5. Save the public certificate to ca.crt.
	certPath := filepath.Join(outputDir, "ca.crt")
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create certificate file: %w", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes}); err != nil {
		return fmt.Errorf("failed to write certificate to file: %w", err)
	}

	// 6. Save the private key to ca.key.
	keyPath := filepath.Join(outputDir, "ca.key")
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600) // 0600 permissions for the private key
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer keyFile.Close()
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caKey)}); err != nil {
		return fmt.Errorf("failed to write private key to file: %w", err)
	}

	return nil
}

// GenerateServerCertificate creates a new server certificate and private key signed
// by the existing CA in the specified directory.
func GenerateServerCertificate(outputDir, serverName string) error {
	// 1. Load the existing CA certificate and private key from disk.
	caCertPath := filepath.Join(outputDir, "ca.crt")
	caKeyPath := filepath.Join(outputDir, "ca.key")

	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate file: %w. Please run 'certs create-ca' first", err)
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read CA private key file: %w. Please run 'certs create-ca' first", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return fmt.Errorf("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return fmt.Errorf("failed to decode CA private key PEM")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA private key: %w", err)
	}

	// 2. Generate a new private key for the server.
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate server private key: %w", err)
	}

	// 3. Create the template for the server certificate.
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName: serverName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0), // Valid for 1 year
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		// Include IP addresses and DNS names for which the certificate is valid.
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:    []string{"localhost", serverName},
	}

	// 4. Create the server certificate by signing it with the CA.
	serverCertBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to create server certificate: %w", err)
	}

	// 5. Save the server's public certificate to server.crt.
	certPath := filepath.Join(outputDir, "server.crt")
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create server certificate file: %w", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: serverCertBytes}); err != nil {
		return fmt.Errorf("failed to write server certificate to file: %w", err)
	}

	// 6. Save the server's private key to server.key.
	keyPath := filepath.Join(outputDir, "server.key")
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create server private key file: %w", err)
	}
	defer keyFile.Close()
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)}); err != nil {
		return fmt.Errorf("failed to write server private key to file: %w", err)
	}

	return nil
}

func GenerateClientCertificate(outputDir, clientName string) error {
	// 1. Load the existing CA certificate and private key from disk.
	caCertPath := filepath.Join(outputDir, "ca.crt")
	caKeyPath := filepath.Join(outputDir, "ca.key")

	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate file: %w. Please run 'certs create-ca' first", err)
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read CA private key file: %w. Please run 'certs create-ca' first", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return fmt.Errorf("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return fmt.Errorf("failed to decode CA private key PEM")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA private key: %w", err)
	}

	// 2. Generate a new private key for the client.
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate client private key: %w", err)
	}

	// 3. Create the template for the client certificate.
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName: clientName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0), // Valid for 1 year
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, // Specify this is for client auth
	}

	// 4. Create the client certificate by signing it with the CA.
	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to create client certificate: %w", err)
	}

	// 5. Save the client's public certificate.
	certPath := filepath.Join(outputDir, clientName+".crt")
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create client certificate file: %w", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: clientCertBytes}); err != nil {
		return fmt.Errorf("failed to write client certificate to file: %w", err)
	}

	// 6. Save the client's private key.
	keyPath := filepath.Join(outputDir, clientName+".key")
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create client private key file: %w", err)
	}
	defer keyFile.Close()
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}); err != nil {
		return fmt.Errorf("failed to write client private key to file: %w", err)
	}

	return nil
}
