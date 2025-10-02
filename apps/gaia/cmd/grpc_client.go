package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stain-win/gaia/apps/gaia/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// getClientConn establishes a secure gRPC connection to the daemon.
func getClientConn(ctx context.Context, cfg *config.Config) (*grpc.ClientConn, error) {
	daemonAddress := fmt.Sprintf("%s:%s", cfg.GRPCServerName, cfg.GRPCPort)
	caCertFile := filepath.Join(cfg.CertsDirectory, cfg.CACertFile)
	clientCertFile := filepath.Join(cfg.CertsDirectory, cfg.GaiaClientCertFile)
	clientKeyFile := filepath.Join(cfg.CertsDirectory, cfg.GaianClientKeyFile)

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

	conn, err := grpc.NewClient(daemonAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return conn, nil
}
