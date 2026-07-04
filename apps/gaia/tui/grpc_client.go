package tui

import (
	"context"

	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/internal/grpcclient"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"google.golang.org/grpc"
)

// getAdminClientConn establishes a secure gRPC connection to the daemon.
// This is a wrapper around the centralized grpcclient package with TUI-specific user agent.
func getAdminClientConn(cfg *config.Config) (*grpc.ClientConn, error) {
	return grpcclient.NewAdminConnection(cfg, &grpcclient.ClientOptions{
		UserAgent: GaiaTui,
	})
}

func GetDaemonStatus(cfg *config.Config) (string, error) {
	conn, err := getAdminClientConn(cfg)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewGaiaAdminClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.GRPCClientTimeout)
	defer cancel()

	res, err := client.GetStatus(ctx, &pb.GetStatusRequest{})
	if err != nil {
		return "nil", err
	}

	return res.Status, nil
}
