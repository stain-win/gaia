package cmd

import (
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/internal/grpcclient"
	"google.golang.org/grpc"
)

// getClientConn establishes a secure gRPC connection to the daemon.
// This is a wrapper around the centralized grpcclient package.
func getClientConn(cfg *config.Config) (*grpc.ClientConn, error) {
	return grpcclient.NewConnection(cfg)
}
