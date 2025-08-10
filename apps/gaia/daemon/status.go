package daemon

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stain-win/gaia/apps/gaia/config" // Import the config package
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// DaemonStatusMsg is a custom message for checking daemon status
type DaemonStatusMsg struct {
	Status string
	Err    error
}

// getDaemonStatus is a private helper function to make the synchronous gRPC call.
// This function now needs to be called with a configuration.
func getDaemonStatus(cfg *config.Config) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Pass the config to the getClientConn function.
	conn, err := getClientConn(ctx, cfg)
	if err != nil {
		return StatusStopped, err
	}
	defer conn.Close()

	client := pb.NewGaiaAdminClient(conn)
	res, err := client.GetStatus(ctx, &pb.GetStatusRequest{})
	if err != nil {
		return StatusStopped, err
	}

	return res.Status, nil
}

// getClientConn establishes a secure gRPC connection to the daemon.
// This method now correctly takes a context and config parameter.
func getClientConn(ctx context.Context, cfg *config.Config) (*grpc.ClientConn, error) {
	// Use the configuration values instead of hardcoded paths.
	daemonAddress := fmt.Sprintf("localhost:%s", cfg.GRPCPort)
	caCertFile := cfg.CACertFile
	clientCertFile := cfg.GaiaClientCertFile // Assuming client certs are in the same dir
	clientKeyFile := cfg.GaianClientKeyFile  // as the server cert for the CLI to use

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

	creds := credentials.NewTLS(&tls.Config{
		ServerName:   "localhost",
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
	})

	// Use grpc.NewClient instead of grpc.DialContext (deprecated)
	conn, err := grpc.NewClient(daemonAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return conn, nil
}

// CheckDaemonStatus is the function called by the TUI to check the daemon's status.
// It returns a tea.Cmd that will send a DaemonStatusMsg back to the TUI model.
func CheckDaemonStatus(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		status, err := getDaemonStatus(cfg)
		return DaemonStatusMsg{Status: status, Err: err}
	}
}
