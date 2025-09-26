package tui

import (
	"context"
	"time"

	"github.com/stain-win/gaia/apps/gaia/config"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)
import tea "github.com/charmbracelet/bubbletea"

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

type recordAddResultMsg struct {
	err error
}

// clientsLoadedMsg is sent when the list of clients has been fetched.
type clientsLoadedMsg struct {
	clients []*pb.Client
	err     error
}

// recordAddedMsg is sent when the AddSecret RPC is complete.
type recordAddedMsg struct {
	err error
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

// A mock function to simulate fetching namespaces from the daemon.
func mockListNamespaces() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(50 * time.Millisecond) // Simulate network latency
		namespaces := []string{"common", "client-a", "client-b"}
		return NamespacesReadyMsg(namespaces)
	}
}

func addRecordToDaemon(namespace, key, value string) tea.Cmd {
	return func() tea.Msg {
		// This is a placeholder; in a real app, you'd make a gRPC call.
		// Assume success for now
		return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return nil // Using Tick to simulate an async operation that completes
		})
	}
}

func FetchSecretsForNamespace(namespace string) tea.Cmd {
	return func() tea.Msg {
		// Mock gRPC call
		time.Sleep(100 * time.Millisecond)
		mockSecrets := make(map[string]string)
		if namespace == "common" {
			mockSecrets["api_key_service_x"] = "****************"
			mockSecrets["database_url"] = "postgres://..."
		} else if namespace == "client-app-a" {
			mockSecrets["app_specific_key"] = "****************"
		}
		return SecretsFetchedMsg{Namespace: namespace, Secrets: mockSecrets}
	}
}

// fetchClientsCmd is a command that fetches the list of registered clients.
func fetchClientsCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return clientsLoadedMsg{err: err}
		}
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

// addRecordToDaemonCmd makes the gRPC call to add a new secret.
func addRecordToDaemonCmd(cfg *config.Config, clientName, namespace, key, value string) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return recordAddResultMsg{err: err}
		}
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), cfg.GRPCClientTimeout)
		defer cancel()

		_, err = client.AddSecret(ctx, &pb.AddSecretRequest{
			ClientName: clientName,
			Namespace:  namespace,
			Id:         key,
			Value:      value,
		})
		return recordAddResultMsg{err: err}
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
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := client.ListSecrets(ctx, &pb.ListSecretsRequest{ClientName: clientName})
		if err != nil {
			return secretsForClientLoadedMsg{clientName: clientName, err: err}
		}
		return secretsForClientLoadedMsg{clientName: clientName, namespaces: res.Namespaces}
	}
}
