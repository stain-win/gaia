package tui

import (
	"context"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/policy"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

// Centralized timeout constants for TUI operations
const (
	// defaultTimeout is the standard timeout for most gRPC operations
	defaultTimeout = 10 * time.Second

	// quickTimeout is for fast operations like status checks
	quickTimeout = 5 * time.Second
)

// BackMsg is a custom message to signal returning to the previous menu.
type BackMsg struct{}

// ListNamespacesMsg is a message to trigger fetching namespaces from the daemon.
type ListNamespacesMsg struct{}

// NamespacesReadyMsg is a message for when namespaces are fetched.
type NamespacesReadyMsg []string

type SecretsFetchedMsg struct {
	Namespace string
	Secrets   map[string]string
}

type backToDataManagementMsg struct{}

// clientsLoadedMsg is sent when the list of clients has been fetched.
type clientsLoadedMsg struct {
	clients []*pb.Client
	err     error
}

// recordAddedMsg is sent when the AddSecret RPC is complete.
type recordAddedMsg struct {
	err error
}

// recordDeletedMsg is sent when the DeleteSecret RPC is complete.
type recordDeletedMsg struct {
	client    string
	namespace string
	id        string
	err       error
}

type statusUpdatedMsg struct {
	status string
	err    error
}

// allClientsLoadedMsg is sent when ListClients RPC is complete.
type allClientsLoadedMsg struct {
	clients []*pb.Client
	err     error
}

// secretsForClientLoadedMsg is sent when ListSecrets RPC is complete.
type secretsForClientLoadedMsg struct {
	clientName string
	namespaces []*pb.Namespace
	err        error
}

// clientRegisteredMsg is sent when RegisterClient RPC is complete.
type clientRegisteredMsg struct {
	clientName string
	certPath   string
	keyPath    string
	err        error
}

// unlockResultMsg is sent when Unlock RPC is complete.
type unlockResultMsg struct {
	success bool
	err     error
}

// Policy-related message types

type policiesLoadedMsg struct {
	policies []policyListItem
	err      error
}

type policyLoadedMsg struct {
	policy policy.Policy
	err    error
}

type policyDeletedMsg struct {
	clientName string
	err        error
}

type policySavedMsg struct {
	clientName string
	err        error
}

type clientsForPolicyMsg struct {
	clients   []string
	operation string // "view", "edit", "create", "delete", "export"
	err       error
}

// fetchPoliciesCmd fetches all policies from the daemon.
func fetchPoliciesCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return policiesLoadedMsg{err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		res, err := client.ListPolicies(ctx, &pb.ListPoliciesRequest{})
		if err != nil {
			return policiesLoadedMsg{err: err}
		}

		// Convert protobuf policies to TUI policy list items
		policies := make([]policyListItem, len(res.Policies))
		for i, p := range res.Policies {
			pol := protoToPolicy(p)
			policies[i] = policyListItem{
				clientName: pol.ClientName,
				ruleCount:  len(pol.Rules),
				summary:    buildAccessSummary(pol),
			}
		}

		return policiesLoadedMsg{policies: policies}
	}
}

// protoToPolicy converts a protobuf policy to a domain policy
func protoToPolicy(p *pb.Policy) policy.Policy {
	rules := make([]policy.PolicyRule, len(p.Rules))
	for i, r := range p.Rules {
		caps := make([]policy.Capability, len(r.Capabilities))
		for j, c := range r.Capabilities {
			caps[j] = policy.Capability(c)
		}
		rules[i] = policy.PolicyRule{
			Path:         r.Path,
			Capabilities: caps,
			Description:  r.Description,
		}
	}
	return policy.Policy{
		ClientName: p.ClientName,
		Rules:      rules,
	}
}

// fetchClientsCmd is a command that fetches the list of registered clients.
func fetchClientsCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return clientsLoadedMsg{err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), quickTimeout)
		defer cancel()

		res, err := client.ListClients(ctx, &pb.ListClientsRequest{})
		if err != nil {
			return clientsLoadedMsg{err: err}
		}
		// "common" is a special client, always add it to the list for selection.
		//clients := append(res.Clients, "common")
		return clientsLoadedMsg{clients: res.Clients}
	}
}

// deleteRecordFromDaemonCmd makes the gRPC call to delete a secret.
func deleteRecordFromDaemonCmd(cfg *config.Config, clientName, namespace, id string) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return recordDeletedMsg{client: clientName, namespace: namespace, id: id, err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), cfg.GRPCClientTimeout)
		defer cancel()

		_, err = client.DeleteSecret(ctx, &pb.DeleteSecretRequest{
			ClientName: clientName,
			Namespace:  namespace,
			Id:         id,
		})
		return recordDeletedMsg{client: clientName, namespace: namespace, id: id, err: err}
	}
}

// addRecordToDaemonCmd makes the gRPC call to add or update a secret.
func addRecordToDaemonCmd(cfg *config.Config, clientName, namespace, key, value string) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return recordAddedMsg{err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), cfg.GRPCClientTimeout)
		defer cancel()

		_, err = client.AddSecret(ctx, &pb.AddSecretRequest{
			ClientName: clientName,
			Namespace:  namespace,
			Id:         key,
			Value:      value,
		})
		return recordAddedMsg{err: err}
	}
}

func checkStatusCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		status, err := GetDaemonStatus(cfg)
		if err != nil {
			return statusUpdatedMsg{
				status: "offline",
				err:    err,
			}
		}

		return statusUpdatedMsg{
			status: status,
			err:    nil,
		}
	}
}

// fetchAllClientsCmd makes the gRPC call to get all client names.
func fetchAllClientsCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return allClientsLoadedMsg{err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), quickTimeout)
		defer cancel()

		res, err := client.ListClients(ctx, &pb.ListClientsRequest{})
		if err != nil {
			return allClientsLoadedMsg{err: err}
		}
		return allClientsLoadedMsg{clients: res.Clients}
	}
}

// fetchSecretsForClientCmd makes the gRPC call to get all secrets for a client.
func fetchSecretsForClientCmd(cfg *config.Config, clientName string) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return secretsForClientLoadedMsg{clientName: clientName, err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), quickTimeout)
		defer cancel()

		res, err := client.ListSecrets(ctx, &pb.ListSecretsRequest{ClientName: clientName})
		if err != nil {
			return secretsForClientLoadedMsg{clientName: clientName, err: err}
		}
		return secretsForClientLoadedMsg{clientName: clientName, namespaces: res.Namespaces}
	}
}

// registerClientCmd makes the gRPC call to register a client and save the certificates.
func registerClientCmd(cfg *config.Config, clientName string) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return clientRegisteredMsg{clientName: clientName, err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		res, err := client.RegisterClient(ctx, &pb.RegisterClientRequest{ClientName: clientName})
		if err != nil {
			return clientRegisteredMsg{clientName: clientName, err: err}
		}

		// Save certificate and key to disk
		certPath := cfg.TLS.CertsDirectory + "/" + clientName + ".crt"
		keyPath := cfg.TLS.CertsDirectory + "/" + clientName + ".key"

		if err := os.WriteFile(certPath, []byte(res.Certificate), 0644); err != nil {
			return clientRegisteredMsg{clientName: clientName, err: err}
		}
		if err := os.WriteFile(keyPath, []byte(res.PrivateKey), 0600); err != nil {
			return clientRegisteredMsg{clientName: clientName, err: err}
		}

		return clientRegisteredMsg{
			clientName: clientName,
			certPath:   certPath,
			keyPath:    keyPath,
		}
	}
}

// unlockDaemonCmd makes the gRPC call to unlock the daemon.
func unlockDaemonCmd(cfg *config.Config, passphrase string) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return unlockResultMsg{success: false, err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		res, err := client.Unlock(ctx, &pb.UnlockRequest{Passphrase: passphrase})
		if err != nil {
			return unlockResultMsg{success: false, err: err}
		}

		return unlockResultMsg{success: res.Success}
	}
}

// Policy operation commands

// fetchClientsForPolicyCmd fetches clients for policy operations
func fetchClientsForPolicyCmd(cfg *config.Config, operation string) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return clientsForPolicyMsg{err: err, operation: operation}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), quickTimeout)
		defer cancel()

		res, err := client.ListClients(ctx, &pb.ListClientsRequest{})
		if err != nil {
			return clientsForPolicyMsg{err: err, operation: operation}
		}

		clients := make([]string, len(res.Clients))
		for i, c := range res.Clients {
			clients[i] = c.Name
		}

		return clientsForPolicyMsg{clients: clients, operation: operation}
	}
}

// fetchPolicyCmd fetches a specific policy
func fetchPolicyCmd(cfg *config.Config, clientName string) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return policyLoadedMsg{err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		res, err := client.GetPolicy(ctx, &pb.GetPolicyRequest{ClientName: clientName})
		if err != nil {
			return policyLoadedMsg{err: err}
		}

		pol := protoToPolicy(res.Policy)
		return policyLoadedMsg{policy: pol}
	}
}

// savePolicyCmd saves a policy to the daemon
func savePolicyCmd(cfg *config.Config, pol policy.Policy) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return policySavedMsg{clientName: pol.ClientName, err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		// Convert to protobuf
		pbPolicy := policyToProto(&pol)

		_, err = client.SetPolicy(ctx, &pb.SetPolicyRequest{Policy: pbPolicy})
		if err != nil {
			return policySavedMsg{clientName: pol.ClientName, err: err}
		}

		return policySavedMsg{clientName: pol.ClientName}
	}
}

// deletePolicyCmd deletes a policy from the daemon
func deletePolicyCmd(cfg *config.Config, clientName string) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return policyDeletedMsg{clientName: clientName, err: err}
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		_, err = client.DeletePolicy(ctx, &pb.DeletePolicyRequest{ClientName: clientName})
		if err != nil {
			return policyDeletedMsg{clientName: clientName, err: err}
		}

		return policyDeletedMsg{clientName: clientName}
	}
}

// policyToProto converts domain policy to protobuf
func policyToProto(p *policy.Policy) *pb.Policy {
	pbRules := make([]*pb.PolicyRule, len(p.Rules))
	for i, rule := range p.Rules {
		caps := make([]string, len(rule.Capabilities))
		for j, cap := range rule.Capabilities {
			caps[j] = string(cap)
		}
		pbRules[i] = &pb.PolicyRule{
			Path:         rule.Path,
			Capabilities: caps,
			Description:  rule.Description,
		}
	}

	return &pb.Policy{
		ClientName: p.ClientName,
		Rules:      pbRules,
	}
}
