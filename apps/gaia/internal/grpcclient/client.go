package grpcclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stain-win/gaia/apps/gaia/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ClientOptions contains optional parameters for creating a gRPC client connection.
type ClientOptions struct {
	// UserAgent is the user agent string to send with gRPC requests.
	// If empty, no user agent will be set.
	UserAgent string

	// ServerName is the expected server name in the certificate.
	// Defaults to "localhost" if empty.
	ServerName string
}

// NewAdminConnection establishes a secure gRPC connection to the Gaia daemon
// using the admin/gaia client certificate.
//
// This function handles mTLS setup using:
// - CA certificate for server verification
// - Client certificate and key for client authentication
//
// The connection uses the daemon address and TLS configuration from the provided config.
func NewAdminConnection(cfg *config.Config, opts *ClientOptions) (*grpc.ClientConn, error) {
	if opts == nil {
		opts = &ClientOptions{}
	}

	// Build paths to certificates
	caCertFile := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.CACert)
	clientCertFile := filepath.Join(cfg.TLS.CertsDirectory, cfg.GaiaClientCertFile)
	clientKeyFile := filepath.Join(cfg.TLS.CertsDirectory, cfg.GaiaClientKeyFile)

	// Load client certificate and key
	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("client certificate not found at '%s'\n\n"+
				"To fix this:\n"+
				"  1. Run: gaia setup (recommended)\n"+
				"  2. Or run: gaia certs generate\n\n"+
				"Expected certificate files:\n"+
				"  - %s\n"+
				"  - %s\n\n"+
				"Looking in directory: %s",
				clientCertFile, cfg.GaiaClientCertFile, cfg.GaiaClientKeyFile, cfg.TLS.CertsDirectory)
		}
		return nil, fmt.Errorf("failed to load client certificate from '%s': %w", clientCertFile, err)
	}

	// Load CA certificate
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("CA certificate not found at '%s'\n\n"+
				"To fix this:\n"+
				"  1. Run: gaia setup (recommended)\n"+
				"  2. Or run: gaia certs generate\n\n"+
				"Looking in directory: %s",
				caCertFile, cfg.TLS.CertsDirectory)
		}
		return nil, fmt.Errorf("failed to read CA certificate from '%s': %w", caCertFile, err)
	}

	// Create certificate pool with CA
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate to pool")
	}

	// Set server name (default to localhost)
	serverName := opts.ServerName
	if serverName == "" {
		serverName = "localhost"
	}

	// Configure TLS credentials
	tlsConfig := &tls.Config{
		ServerName:   serverName,
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
	}
	creds := credentials.NewTLS(tlsConfig)

	// Build gRPC dial options
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}

	// Add user agent if provided
	if opts.UserAgent != "" {
		dialOpts = append(dialOpts, grpc.WithUserAgent(opts.UserAgent))
	}

	// Create connection
	conn, err := grpc.NewClient(cfg.Daemon.ListenAddr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	return conn, nil
}

// NewConnection is a convenience function that creates a connection with default options.
func NewConnection(cfg *config.Config) (*grpc.ClientConn, error) {
	return NewAdminConnection(cfg, nil)
}
