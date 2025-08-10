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

// StatusMsg is a custom message for checking daemon status
type StatusMsg struct {
	Status string
	Err    error
}

// getDaemonStatus is a private helper function to make the synchronous gRPC call.
func getDaemonStatus(cfg *config.Config) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

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
func getClientConn(_ context.Context, cfg *config.Config) (*grpc.ClientConn, error) {
	daemonAddress := fmt.Sprintf("localhost:%s", cfg.GRPCPort)
	caCertFile := cfg.CACertFile
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

	conn, err := grpc.NewClient(daemonAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return conn, nil
}

// CheckDaemonStatus is the function called by the TUI to check the daemon's status.
func CheckDaemonStatus(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		status, err := getDaemonStatus(cfg)
		return StatusMsg{Status: status, Err: err}
	}
}
