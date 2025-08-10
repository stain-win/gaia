package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/stain-win/gaia/apps/gaia/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// getClientConn establishes a secure gRPC connection to the daemon.
// This method is updated to use the provided configuration.
func getClientConn(ctx context.Context, cfg *config.Config) (*grpc.ClientConn, error) {
	// Use values from the configuration struct instead of hardcoded paths.
	daemonAddress := fmt.Sprintf("localhost:%s", cfg.GRPCPort)
	caCertFile := cfg.CACertFile

	// These values should not come from the daemon's config. For a client,
	// they should be sourced from the client's own configuration.
	// We'll use hardcoded paths for now, but this would be configurable in a real app.
	clientCertFile := cfg.GaiaClientCertFile
	clientKeyFile := cfg.GaianClientKeyFile

	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("could not load client key pair: %w", err)
	}

	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("could not read CA certificate: %w", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("could not append CA certificate to pool")
	}

	creds := credentials.NewTLS(&tls.Config{
		ServerName:   "localhost",
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
	})

	// Use grpc.NewClient() which is the non-blocking, recommended method.
	// The context for the timeout will be passed to the RPC call itself.
	conn, err := grpc.NewClient(daemonAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return conn, nil
}
