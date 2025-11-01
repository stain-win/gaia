package tui

import (
	"context"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stain-win/gaia/apps/gaia/config"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
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

// registerClientCmd makes the gRPC call to register a client and save the certificates.
func registerClientCmd(cfg *config.Config, clientName string) tea.Cmd {
	return func() tea.Msg {
		conn, err := getAdminClientConn(cfg)
		if err != nil {
			return clientRegisteredMsg{clientName: clientName, err: err}
		}
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		res, err := client.RegisterClient(ctx, &pb.RegisterClientRequest{ClientName: clientName})
		if err != nil {
			return clientRegisteredMsg{clientName: clientName, err: err}
		}

		// Save certificate and key to disk
		certPath := cfg.CertsDirectory + "/" + clientName + ".crt"
		keyPath := cfg.CertsDirectory + "/" + clientName + ".key"

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
		defer conn.Close()

		client := pb.NewGaiaAdminClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		res, err := client.Unlock(ctx, &pb.UnlockRequest{Passphrase: passphrase})
		if err != nil {
			return unlockResultMsg{success: false, err: err}
		}

		return unlockResultMsg{success: res.Success}
	}
}
