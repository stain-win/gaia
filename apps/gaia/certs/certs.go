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
