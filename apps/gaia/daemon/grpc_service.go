package daemon

import (
	"context"
	"errors"

	"github.com/stain-win/gaia/apps/gaia/proto"
)

// gaiaAdminServer implements the GaiaAdmin gRPC service.
type gaiaAdminServer struct {
	proto.UnimplementedGaiaAdminServer
	d *Daemon
}

// NewAdminServer creates a new server for the GaiaAdmin service.
func NewAdminServer(d *Daemon) proto.GaiaAdminServer {
	return &gaiaAdminServer{d: d}
}

// NewClientServer creates a new server for the GaiaClient service.
func NewClientServer(d *Daemon) proto.GaiaClientServer {
	return &gaiaClientServer{daemon: d}
}

// AddSecret handles the AddSecret RPC call.
func (s *gaiaAdminServer) AddSecret(ctx context.Context, req *proto.AddSecretRequest) (*proto.AddSecretResponse, error) {
	if s.d.isLocked {
		return nil, errors.New("daemon is in a locked state, cannot add secrets")
	}
	err := s.d.AddSecret(req.Namespace, req.Id, req.Value)
	if err != nil {
		return &proto.AddSecretResponse{Success: false}, err
	}
	return &proto.AddSecretResponse{Success: true}, nil
}

// RevokeCert handles the RevokeCert RPC call.
func (s *gaiaAdminServer) RevokeCert(ctx context.Context, req *proto.RevokeCertRequest) (*proto.RevokeCertResponse, error) {
	// TODO: Implement certificate revocation logic here.
	return &proto.RevokeCertResponse{Success: false}, errors.New("not implemented")
}

// GetStatus handles the GetStatus RPC call.
func (s *gaiaAdminServer) GetStatus(ctx context.Context, req *proto.GetStatusRequest) (*proto.GetStatusResponse, error) {
	return &proto.GetStatusResponse{Status: s.d.Status()}, nil
}

// GetSecret handles the GetSecret RPC call.
func (s *gaiaClientServer) GetSecret(ctx context.Context, req *proto.GetSecretRequest) (*proto.Secret, error) {
	value, err := s.daemon.GetSecret(req.Id)
	if err != nil {
		return nil, err
	}
	return &proto.Secret{Id: req.Id, Value: value}, nil
}
