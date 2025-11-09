package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/stain-win/gaia/apps/gaia/certs"
	gaiaerrors "github.com/stain-win/gaia/apps/gaia/internal/errors"
	"github.com/stain-win/gaia/apps/gaia/internal/validation"
	"github.com/stain-win/gaia/apps/gaia/policy"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// mapErrorToGRPCStatus converts domain errors to appropriate gRPC status codes.
func mapErrorToGRPCStatus(err error) error {
	if err == nil {
		return nil
	}

	// Check for sentinel errors
	switch {
	case errors.Is(err, gaiaerrors.ErrDaemonLocked):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, gaiaerrors.ErrDaemonNotRunning):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, gaiaerrors.ErrInvalidPassphrase):
		return status.Error(codes.Unauthenticated, "invalid passphrase")
	case errors.Is(err, gaiaerrors.ErrClientNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, gaiaerrors.ErrClientExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, gaiaerrors.ErrSecretNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, gaiaerrors.ErrReservedName):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, gaiaerrors.ErrNoPeerContext),
		errors.Is(err, gaiaerrors.ErrNotTLS),
		errors.Is(err, gaiaerrors.ErrNoPeerCertificates):
		return status.Error(codes.Unauthenticated, err.Error())
	}

	// Check for permission denied errors
	if errors.Is(err, gaiaerrors.ErrPermissionDenied) {
		return status.Error(codes.PermissionDenied, err.Error())
	}

	// Check for typed errors
	var valErr *gaiaerrors.ValidationError
	if errors.As(err, &valErr) {
		return status.Error(codes.InvalidArgument, valErr.Error())
	}

	var authErr *gaiaerrors.AuthError
	if errors.As(err, &authErr) {
		return status.Error(codes.PermissionDenied, authErr.Error())
	}

	var cryptoErr *gaiaerrors.CryptoError
	if errors.As(err, &cryptoErr) {
		return status.Error(codes.Internal, "cryptographic operation failed")
	}

	var storageErr *gaiaerrors.StorageError
	if errors.As(err, &storageErr) {
		return status.Error(codes.Internal, "storage operation failed")
	}

	// Default to Internal for unknown errors
	return status.Error(codes.Internal, "internal server error")
}

func getClientIdentity(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", gaiaerrors.ErrNoPeerContext
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return "", gaiaerrors.ErrNotTLS
	}
	if len(tlsInfo.State.PeerCertificates) == 0 {
		return "", gaiaerrors.ErrNoPeerCertificates
	}
	// The client's certificate is the first in the chain.
	clientCert := tlsInfo.State.PeerCertificates[0]
	return clientCert.Subject.CommonName, nil
}

// checkPermission verifies if the authenticated client has permission to perform an action.
func (s *gaiaAdminServer) checkPermission(ctx context.Context, clientName, namespace, secretKey string, capability string) error {
	// Get the authenticated client's identity from the mTLS certificate
	authenticatedClient, err := getClientIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get client identity: %w", err)
	}

	// Build the path for policy check: client/namespace/key
	path := fmt.Sprintf("%s/%s", clientName, namespace)
	if secretKey != "" {
		path = fmt.Sprintf("%s/%s", path, secretKey)
	}

	// Check permission using the policy store
	if err := s.d.policyStore.CheckPermission(authenticatedClient, path, policy.Capability(capability)); err != nil {
		return fmt.Errorf("%w: %v", gaiaerrors.ErrPermissionDenied, err)
	}

	return nil
}

// AddSecret handles the AddSecret RPC call.
func (s *gaiaAdminServer) AddSecret(ctx context.Context, req *pb.AddSecretRequest) (*pb.AddSecretResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	if err := validation.ValidateClient(req.ClientName); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := validation.ValidateNamespace(req.Namespace); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := validation.ValidateKey(req.Id); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	// Check permission to write to this path
	if err := s.checkPermission(ctx, req.ClientName, req.Namespace, req.Id, string(policy.CapabilityWrite)); err != nil {
		return nil, mapErrorToGRPCStatus(err)
	}

	err := s.d.AddSecret(req.ClientName, req.Namespace, req.Id, req.Value)
	if err != nil {
		return &pb.AddSecretResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.AddSecretResponse{Success: true, Message: "secret added successfully"}, nil
}

// DeleteSecret handles the gRPC request to delete a secret.
func (s *gaiaAdminServer) DeleteSecret(ctx context.Context, req *pb.DeleteSecretRequest) (*pb.DeleteSecretResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	// Check permission to delete from this path
	if err := s.checkPermission(ctx, req.ClientName, req.Namespace, req.Id, string(policy.CapabilityDelete)); err != nil {
		return nil, mapErrorToGRPCStatus(err)
	}

	if err := s.d.DeleteSecret(req.ClientName, req.Namespace, req.Id); err != nil {
		return nil, fmt.Errorf("failed to delete secret for client '%s': %w", req.ClientName, err)
	}

	return &pb.DeleteSecretResponse{Success: true}, nil
}

// GetStatus handles the GetStatus RPC call.
func (s *gaiaAdminServer) GetStatus(_ context.Context, _ *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	return &pb.GetStatusResponse{Status: s.d.Status()}, nil
}

// GetSecret handles the GetSecret RPC call.
func (s *gaiaClientServer) GetSecret(ctx context.Context, req *pb.GetSecretRequest) (*pb.Secret, error) {
	clientName, err := getClientIdentity(ctx)
	if err != nil {
		return nil, mapErrorToGRPCStatus(err)
	}

	value, err := s.daemon.GetSecret(clientName, req.Namespace, req.Id)
	if err != nil {
		return nil, mapErrorToGRPCStatus(err)
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
		return &pb.UnlockResponse{Success: false}, mapErrorToGRPCStatus(err)
	}
	return &pb.UnlockResponse{Success: true}, nil
}

func (s *gaiaAdminServer) RegisterClient(_ context.Context, req *pb.RegisterClientRequest) (*pb.RegisterClientResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	if err := validation.ValidateClient(req.ClientName); err != nil {
		return nil, mapErrorToGRPCStatus(err)
	}

	certPEM, keyPEM, err := certs.GenerateClientCertificateData(req.ClientName, s.d.caCert, s.d.caKey, s.d.config.TLS.CertExpiryDays)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate client certificate: %v", err)
	}

	if err := s.d.RegisterClient(req.ClientName); err != nil {
		return nil, mapErrorToGRPCStatus(err)
	}

	return &pb.RegisterClientResponse{
		Certificate: string(certPEM),
		PrivateKey:  string(keyPEM),
	}, nil
}

func (s *gaiaAdminServer) ListClients(_ context.Context, _ *pb.ListClientsRequest) (*pb.ListClientsResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	clients, err := s.d.ListClients()
	if err != nil {
		return nil, mapErrorToGRPCStatus(err)
	}

	pbClients := make([]*pb.Client, len(clients))
	for i, c := range clients {
		pbClients[i] = &pb.Client{
			Name:        c.Name,
			TimeCreated: c.TimeCreated,
		}
	}

	return &pb.ListClientsResponse{Clients: pbClients}, nil
}

// RevokeClient handles the gRPC request to revoke a client.
func (s *gaiaAdminServer) RevokeClient(_ context.Context, req *pb.RevokeClientRequest) (*pb.RevokeClientResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	if err := validation.ValidateClient(req.ClientName); err != nil {
		return nil, mapErrorToGRPCStatus(err)
	}

	if err := s.d.RevokeClient(req.ClientName); err != nil {
		return nil, mapErrorToGRPCStatus(err)
	}

	return &pb.RevokeClientResponse{Success: true}, nil
}

func (s *gaiaAdminServer) ListNamespaces(_ context.Context, req *pb.ListNamespacesRequest) (*pb.ListNamespacesResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	namespaces, err := s.d.ListNamespaces(req.ClientName)
	if err != nil {
		return nil, mapErrorToGRPCStatus(err)
	}

	return &pb.ListNamespacesResponse{Namespaces: namespaces}, nil
}

// ImportSecrets handles the client-streaming RPC for bulk secret import.
func (s *gaiaAdminServer) ImportSecrets(stream pb.GaiaAdmin_ImportSecretsServer) error {
	if s.d.isLocked {
		return status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	initialReq, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("error receiving initial import request: %w", err)
	}

	configPayload, ok := initialReq.GetPayload().(*pb.ImportSecretsRequest_Config)
	if !ok {
		return status.Error(codes.InvalidArgument, "expected the first message to be import configuration")
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
			return status.Error(codes.InvalidArgument, "expected subsequent messages to be secret items")
		}
		receivedSecrets = append(receivedSecrets, itemPayload.Item)
	}

	count, err := s.d.ImportSecrets(receivedSecrets, overwrite)
	if err != nil {
		return err
	}

	return stream.SendAndClose(&pb.ImportSecretsResponse{
		SecretsImported: int32(count),
		Message:         "secrets imported successfully",
	})
}

func (s *gaiaAdminServer) ListSecrets(_ context.Context, req *pb.ListSecretsRequest) (*pb.ListSecretsResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	if err := validation.ValidateClient(req.ClientName); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	allData, err := s.d.ListSecrets(req.ClientName)
	if err != nil {
		return nil, err
	}

	var namespaces []*pb.Namespace
	for nsName, secretsMap := range allData {
		ns := &pb.Namespace{Name: nsName}
		for key, value := range secretsMap {
			ns.Secrets = append(ns.Secrets, &pb.Secret{Id: key, Value: value})
		}
		namespaces = append(namespaces, ns)
	}

	return &pb.ListSecretsResponse{Namespaces: namespaces}, nil
}

// Policy management handlers

func (s *gaiaAdminServer) ListPolicies(_ context.Context, _ *pb.ListPoliciesRequest) (*pb.ListPoliciesResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	policies, err := s.d.policyStore.ListPolicies()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list policies: %v", err)
	}

	pbPolicies := make([]*pb.Policy, len(policies))
	for i, pol := range policies {
		pbPolicies[i] = policyToProto(&pol)
	}

	return &pb.ListPoliciesResponse{Policies: pbPolicies}, nil
}

func (s *gaiaAdminServer) GetPolicy(_ context.Context, req *pb.GetPolicyRequest) (*pb.GetPolicyResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	if err := validation.ValidateClient(req.ClientName); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	pol, err := s.d.policyStore.GetPolicy(req.ClientName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "policy not found for client '%s': %v", req.ClientName, err)
	}

	return &pb.GetPolicyResponse{Policy: policyToProto(pol)}, nil
}

func (s *gaiaAdminServer) SetPolicy(_ context.Context, req *pb.SetPolicyRequest) (*pb.SetPolicyResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	if req.Policy == nil {
		return &pb.SetPolicyResponse{Success: false, Message: "policy cannot be nil"}, nil
	}

	// Convert protobuf policy to domain policy
	pol := protoToPolicy(req.Policy)

	// Validate policy
	if err := policy.ValidatePolicy(pol); err != nil {
		return &pb.SetPolicyResponse{Success: false, Message: fmt.Sprintf("invalid policy: %v", err)}, nil
	}

	// Set policy
	if err := s.d.policyStore.SetPolicy(pol); err != nil {
		return &pb.SetPolicyResponse{Success: false, Message: fmt.Sprintf("failed to set policy: %v", err)}, nil
	}

	return &pb.SetPolicyResponse{Success: true, Message: "policy set successfully"}, nil
}

func (s *gaiaAdminServer) DeletePolicy(_ context.Context, req *pb.DeletePolicyRequest) (*pb.DeletePolicyResponse, error) {
	if s.d.isLocked {
		return nil, status.Error(codes.FailedPrecondition, gaiaerrors.ErrDaemonLocked.Error())
	}

	if err := validation.ValidateClient(req.ClientName); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	if err := s.d.policyStore.DeletePolicy(req.ClientName); err != nil {
		return &pb.DeletePolicyResponse{Success: false, Message: fmt.Sprintf("failed to delete policy: %v", err)}, nil
	}

	return &pb.DeletePolicyResponse{Success: true, Message: "policy deleted successfully"}, nil
}

// Helper functions for policy conversion

func policyToProto(p *policy.Policy) *pb.Policy {
	pbRules := make([]*pb.PolicyRule, len(p.Rules))
	for i, rule := range p.Rules {
		pbRules[i] = &pb.PolicyRule{
			Path:         rule.Path,
			Capabilities: capabilitiesToStrings(rule.Capabilities),
			Description:  rule.Description,
		}
	}

	return &pb.Policy{
		ClientName: p.ClientName,
		Rules:      pbRules,
	}
}

func protoToPolicy(p *pb.Policy) policy.Policy {
	rules := make([]policy.PolicyRule, len(p.Rules))
	for i, pbRule := range p.Rules {
		rules[i] = policy.PolicyRule{
			Path:         pbRule.Path,
			Capabilities: stringsToCapabilities(pbRule.Capabilities),
			Description:  pbRule.Description,
		}
	}

	return policy.Policy{
		ClientName: p.ClientName,
		Rules:      rules,
	}
}

func capabilitiesToStrings(caps []policy.Capability) []string {
	strs := make([]string, len(caps))
	for i, cap := range caps {
		strs[i] = string(cap)
	}
	return strs
}

func stringsToCapabilities(strs []string) []policy.Capability {
	caps := make([]policy.Capability, len(strs))
	for i, str := range strs {
		caps[i] = policy.Capability(str)
	}
	return caps
}
