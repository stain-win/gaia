package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	pb "github.com/stain-win/gaia/libs/go/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a high-level Gaia client for interacting with the Gaia daemon.
type Client struct {
	conn   *grpc.ClientConn
	client pb.GaiaClientClient
}

// Config holds the configuration required to connect to the Gaia daemon.
type Config struct {
	// Address of the Gaia gRPC server (e.g., "localhost:50051").
	Address string
	// CACertFile is the path to the CA certificate file.
	CACertFile string
	// ClientCertFile is the path to the client's certificate file.
	ClientCertFile string
	// ClientKeyFile is the path to the client's private key file.
	ClientKeyFile string
	// Timeout is the timeout for the initial connection.
	Timeout time.Duration
	// Insecure allows connecting without TLS. For development only.
	Insecure bool
}

// NewClient creates a new Gaia client with the provided configuration.
func NewClient(cfg Config) (*Client, error) {
	var opts []grpc.DialOption

	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// Load CA cert
		caCert, err := os.ReadFile(cfg.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}

		// Load client cert and key
		clientCert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      caCertPool,
			MinVersion:   tls.VersionTLS12,
		}

		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	}

	// Set default timeout if not provided
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Gaia daemon: %w", err)
	}

	// Verify connection with a ListSecrets call (empty namespace)
	// This confirms mTLS is working and daemon is reachable
	client := pb.NewGaiaClientClient(conn)
	// We use a short timeout for the verification check
	verifyCtx, verifyCancel := context.WithTimeout(ctx, 2*time.Second)
	defer verifyCancel()

	_, err = client.ListSecrets(verifyCtx, &pb.ClientListSecretsRequest{})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to verify connection to Gaia daemon: %w", err)
	}

	return &Client{
		conn:   conn,
		client: client,
	}, nil
}

// Close closes the client's connection to the Gaia daemon.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetSecret fetches a single secret for the authenticated client from a specific namespace.
func (c *Client) GetSecret(ctx context.Context, namespace, id string) (string, error) {
	resp, err := c.client.GetSecret(ctx, &pb.GetSecretRequest{
		Namespace: namespace,
		Id:        id,
	})
	if err != nil {
		return "", err
	}
	return resp.Value, nil
}

// LoadEnv fetches all secrets available to the client and loads them into the
// current process's environment.
//
// The environment variables are formatted as GAIA_NAMESPACE_KEY.
func (c *Client) LoadEnv(ctx context.Context) error {
	secrets, err := c.ListSecrets(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch secrets: %w", err)
	}

	for namespace, kv := range secrets {
		for key, value := range kv {
			envVarName := fmt.Sprintf("GAIA_%s_%s", namespace, key)
			envVarName = strings.ToUpper(envVarName)
			envVarName = strings.ReplaceAll(envVarName, "-", "_")
			if err := os.Setenv(envVarName, value); err != nil {
				return fmt.Errorf("failed to set env var %s: %w", envVarName, err)
			}
		}
	}
	return nil
}

// ListSecrets fetches all secrets for the authenticated client.
// If a namespace is provided, it filters secrets to only that namespace.
// Returns secrets from the client's own namespaces plus the common namespace.
func (c *Client) ListSecrets(ctx context.Context, namespace ...string) (map[string]map[string]string, error) {
	req := &pb.ClientListSecretsRequest{}
	if len(namespace) > 0 && namespace[0] != "" {
		req.Namespace = namespace[0]
	}

	resp, err := c.client.ListSecrets(ctx, req)
	if err != nil {
		return nil, err
	}

	secrets := make(map[string]map[string]string)
	for _, ns := range resp.GetNamespaces() {
		secrets[ns.Name] = make(map[string]string)
		for _, s := range ns.Secrets {
			secrets[ns.Name][s.Id] = s.Value
		}
	}
	return secrets, nil
}
