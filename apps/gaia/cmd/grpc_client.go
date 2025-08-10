package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// getClientConn establishes a secure gRPC connection to the daemon.
func getClientConn(ctx context.Context) (*grpc.ClientConn, error) {
	// For this example, we hardcode the daemon address and cert paths.
	// In a real application, these would be configurable.
	daemonAddress := "localhost:50051"
	caCertFile := "./certs/ca.crt"
	clientCertFile := "./certs/client.crt"
	clientKeyFile := "./certs/client.key"

	// Load client's certificate and key
	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("could not load client key pair: %w", err)
	}

	// Load the trusted CA certificate
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("could not read CA certificate: %w", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("could not append CA certificate to pool")
	}

	// Create TLS credentials for the client
	creds := credentials.NewTLS(&tls.Config{
		ServerName:   "localhost", // Must match the server's cert Common Name
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
	})

	// Establish a secure connection
	conn, err := grpc.DialContext(ctx, daemonAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to dial daemon: %w", err)
	}

	return conn, nil
}
