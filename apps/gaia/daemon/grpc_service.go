package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/stain-win/gaia/apps/gaia/proto"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// gaiaAdminServer implements the GaiaAdmin gRPC service.
type gaiaAdminServer struct {
	proto.UnimplementedGaiaAdminServer
	d *Daemon
}

func getClientIdentity(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", errors.New("could not get peer from context")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return "", errors.New("peer auth info is not TLS")
	}
	if len(tlsInfo.State.PeerCertificates) == 0 {
		return "", errors.New("no peer certificates found")
	}
	// The client's certificate is the first in the chain.
	clientCert := tlsInfo.State.PeerCertificates[0]
	return clientCert.Subject.CommonName, nil
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
	// The client name is provided in the request for admin operations.
	err := s.d.AddSecret(req.ClientName, req.Namespace, req.Id, req.Value)
	if err != nil {
		return &proto.AddSecretResponse{Success: false, Message: err.Error()}, nil
	}
	return &proto.AddSecretResponse{Success: true, Message: "Secret added successfully"}, nil
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
	clientName, err := getClientIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not identify client: %w", err)
	}

	value, err := s.daemon.GetSecret(clientName, req.Namespace, req.Id)
	if err != nil {
		return nil, err
	}
	return &proto.Secret{Id: req.Id, Value: value}, nil
}

// Lock handles the Lock RPC call.
func (s *gaiaAdminServer) Lock(ctx context.Context, req *proto.LockRequest) (*proto.LockResponse, error) {
	s.d.LockDB()
	return &proto.LockResponse{Success: true}, nil
}

// Unlock handles the Unlock RPC call.
func (s *gaiaAdminServer) Unlock(ctx context.Context, req *proto.UnlockRequest) (*proto.UnlockResponse, error) {
	err := s.d.UnlockDB(req.Passphrase)
	if err != nil {
		return &proto.UnlockResponse{Success: false}, err
	}
	return &proto.UnlockResponse{Success: true}, nil
}
