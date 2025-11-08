package tui

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stain-win/gaia/apps/gaia/config"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// getAdminClientConn establishes a secure gRPC connection to the daemon.
func getAdminClientConn(cfg *config.Config) (*grpc.ClientConn, error) {
	caCertPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.TLS.CACert)
	// Use the Gaia client certificate from config
	gaiaCertPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.GaiaClientCertFile)
	gaiaKeyPath := filepath.Join(cfg.TLS.CertsDirectory, cfg.GaiaClientKeyFile)

	clientCert, err := tls.LoadX509KeyPair(gaiaCertPath, gaiaKeyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load TUI client key pair: %w", err)
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("could not read CA certificate for TUI: %w", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("could not append CA certificate to TUI pool")
	}

	creds := credentials.NewTLS(&tls.Config{
		ServerName:   "localhost",
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
	})
	daemonAddress := cfg.Daemon.ListenAddr
	conn, err := grpc.NewClient(daemonAddress, grpc.WithTransportCredentials(creds), grpc.WithUserAgent(GaiaTui))
	if err != nil {
		return nil, fmt.Errorf("TUI failed to connect to daemon: %w", err)
	}

	return conn, nil
}

func GetDaemonStatus(cfg *config.Config) (string, error) {
	conn, err := getAdminClientConn(cfg)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	client := pb.NewGaiaAdminClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.GRPCClientTimeout)
	defer cancel()

	res, err := client.GetStatus(ctx, &pb.GetStatusRequest{})
	if err != nil {
		return "nil", err
	}

	return res.Status, nil
}
