package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/stain-win/gaia/apps/gaia/certs"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// gaiaAdminServer implements the GaiaAdmin gRPC service.
type gaiaAdminServer struct {
	pb.UnimplementedGaiaAdminServer
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
func NewAdminServer(d *Daemon) pb.GaiaAdminServer {
	return &gaiaAdminServer{d: d}
}

// NewClientServer creates a new server for the GaiaClient service.
func NewClientServer(d *Daemon) pb.GaiaClientServer {
	return &gaiaClientServer{daemon: d}
}

// AddSecret handles the AddSecret RPC call.
func (s *gaiaAdminServer) AddSecret(_ context.Context, req *pb.AddSecretRequest) (*pb.AddSecretResponse, error) {
	if s.d.isLocked {
		return nil, errors.New("daemon is in a locked state, cannot add secrets")
	}
	// The client name is provided in the request for admin operations.
	err := s.d.AddSecret(req.ClientName, req.Namespace, req.Id, req.Value)
	if err != nil {
		return &pb.AddSecretResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.AddSecretResponse{Success: true, Message: "Secret added successfully"}, nil
}

// DeleteSecret handles the gRPC request to delete a secret.
func (s *gaiaAdminServer) DeleteSecret(_ context.Context, req *pb.DeleteSecretRequest) (*pb.DeleteSecretResponse, error) {
	if s.d.isLocked {
		return nil, errors.New("daemon is in a locked state, cannot delete secrets")
	}

	if err := s.d.DeleteSecret(req.ClientName, req.Namespace, req.Id); err != nil {
		return nil, fmt.Errorf("failed to delete secret for client '%s': %w", req.ClientName, err)
	}

	return &pb.DeleteSecretResponse{Success: true}, nil
}

// RevokeCert handles the RevokeCert RPC call.
func (s *gaiaAdminServer) RevokeCert(_ context.Context, _ *pb.RevokeCertRequest) (*pb.RevokeCertResponse, error) {
	// TODO: Implement certificate revocation logic here.
	return &pb.RevokeCertResponse{Success: false}, errors.New("not implemented")
}

// GetStatus handles the GetStatus RPC call.
func (s *gaiaAdminServer) GetStatus(_ context.Context, _ *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	return &pb.GetStatusResponse{Status: s.d.Status()}, nil
}

// GetSecret handles the GetSecret RPC call.
func (s *gaiaClientServer) GetSecret(ctx context.Context, req *pb.GetSecretRequest) (*pb.Secret, error) {
	clientName, err := getClientIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not identify client: %w", err)
	}

	value, err := s.daemon.GetSecret(clientName, req.Namespace, req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.Secret{Id: req.Id, Value: value}, nil
}

// Lock handles the Lock RPC call.
func (s *gaiaAdminServer) Lock(_ context.Context, _ *pb.LockRequest) (*pb.LockResponse, error) {
	s.d.LockDB()
	return &pb.LockResponse{Success: true}, nil
}

// Unlock handles the Unlock RPC call.
func (s *gaiaAdminServer) Unlock(_ context.Context, req *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	err := s.d.UnlockDB(req.Passphrase)
	if err != nil {
		return &pb.UnlockResponse{Success: false}, err
	}
	return &pb.UnlockResponse{Success: true}, nil
}

func (s *gaiaAdminServer) RegisterClient(_ context.Context, req *pb.RegisterClientRequest) (*pb.RegisterClientResponse, error) {
	if s.d.isLocked {
		return nil, errors.New("daemon is in a locked state, cannot register new clients")
	}

	certPEM, keyPEM, err := certs.GenerateClientCertificateData(req.ClientName, s.d.caCert, s.d.caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client certificate: %w", err)
	}

	if err := s.d.RegisterClient(req.ClientName); err != nil {
		return nil, fmt.Errorf("failed to register client in database: %w", err)
	}

	return &pb.RegisterClientResponse{
		Certificate: string(certPEM),
		PrivateKey:  string(keyPEM),
	}, nil
}

func (s *gaiaAdminServer) ListClients(_ context.Context, _ *pb.ListClientsRequest) (*pb.ListClientsResponse, error) {
	if s.d.isLocked {
		return nil, errors.New("daemon is in a locked state, cannot list clients")
	}

	clientNames, err := s.d.ListClients()
	if err != nil {
		return nil, fmt.Errorf("failed to get client list: %w", err)
	}

	return &pb.ListClientsResponse{ClientNames: clientNames}, nil
}

// RevokeClient handles the gRPC request to revoke a client.
func (s *gaiaAdminServer) RevokeClient(_ context.Context, req *pb.RevokeClientRequest) (*pb.RevokeClientResponse, error) {
	if s.d.isLocked {
		return nil, errors.New("daemon is in a locked state, cannot revoke clients")
	}

	if err := s.d.RevokeClient(req.ClientName); err != nil {
		return nil, fmt.Errorf("failed to revoke client '%s': %w", req.ClientName, err)
	}

	return &pb.RevokeClientResponse{Success: true}, nil
}

func (s *gaiaAdminServer) ListNamespaces(_ context.Context, req *pb.ListNamespacesRequest) (*pb.ListNamespacesResponse, error) {
	if s.d.isLocked {
		return nil, errors.New("daemon is in a locked state, cannot list namespaces")
	}

	namespaces, err := s.d.ListNamespaces(req.ClientName)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace list for client '%s': %w", req.ClientName, err)
	}

	return &pb.ListNamespacesResponse{Namespaces: namespaces}, nil
}

// ImportSecrets handles the client-streaming RPC for bulk secret import.
func (s *gaiaAdminServer) ImportSecrets(stream pb.GaiaAdmin_ImportSecretsServer) error {
	if s.d.isLocked {
		return errors.New("daemon is in a locked state, cannot import secrets")
	}

	initialReq, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("error receiving initial import request: %w", err)
	}

	configPayload, ok := initialReq.GetPayload().(*pb.ImportSecretsRequest_Config)
	if !ok {
		return errors.New("expected the first message to be import configuration")
	}
	overwrite := configPayload.Config.GetOverwrite()

	var receivedSecrets []*pb.ImportSecretItem
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// The client has finished sending.
			break
		}
		if err != nil {
			return fmt.Errorf("error receiving stream: %w", err)
		}

		itemPayload, ok := req.GetPayload().(*pb.ImportSecretsRequest_Item)
		if !ok {
			return errors.New("expected subsequent messages to be secret items")
		}
		receivedSecrets = append(receivedSecrets, itemPayload.Item)
	}

	count, err := s.d.ImportSecrets(receivedSecrets, overwrite)
	if err != nil {
		return err
	}

	return stream.SendAndClose(&pb.ImportSecretsResponse{
		SecretsImported: int32(count),
		Message:         "Secrets imported successfully.",
	})
}
