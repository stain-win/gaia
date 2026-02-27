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

// closeConn closes a gRPC client connection. The error from Close is
// intentionally discarded: in a CLI context the connection is torn down
// at process exit and the error is not actionable.
func closeConn(conn *grpc.ClientConn) {
	_ = conn.Close()
}
