package client

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	pb "github.com/stain-win/gaia/libs/go/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
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

// GetCommonSecrets fetches secrets from the "common" area.
// If a namespace is provided, it fetches secrets only for that namespace.
// If no namespace is provided, it fetches secrets from all namespaces in the common area.
func (c *Client) GetCommonSecrets(ctx context.Context, namespace ...string) (map[string]map[string]string, error) {
	req := &pb.GetCommonSecretsRequest{}
	if len(namespace) > 0 && namespace[0] != "" {
		ns := namespace[0]
		req.Namespace = &ns
	}

	resp, err := c.client.GetCommonSecrets(ctx, req)
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

// LoadEnv fetches all secrets from the "common" area and loads them into the
// current process's environment.
//
// The environment variables are formatted as GAIA_NAMESPACE_KEY.
func (c *Client) LoadEnv(ctx context.Context) error {
	secrets, err := c.GetCommonSecrets(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch common secrets: %w", err)
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

// GetStatus checks the current operational status of the Gaia daemon.
func (c *Client) GetStatus(ctx context.Context) (string, error) {
	resp, err := c.client.GetStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return "", err
	}
	return resp.Status, nil
}

// GetNamespaces lists all namespaces the authenticated client has access to.
func (c *Client) GetNamespaces(ctx context.Context) ([]string, error) {
	resp, err := c.client.GetNamespaces(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.Namespaces, nil
}
