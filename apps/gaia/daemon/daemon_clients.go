package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"

	"time"

	"github.com/google/uuid"
	gaiaerrors "github.com/stain-win/gaia/apps/gaia/internal/errors"
	gaialog "github.com/stain-win/gaia/apps/gaia/log"
	"github.com/stain-win/gaia/apps/gaia/policy"
	"go.etcd.io/bbolt"
)

// RegisterClient creates a new client, assigns it a UUID, and stores it in the database.
func (d *Daemon) RegisterClient(clientName string) error {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.isLocked || d.db == nil {
		return gaiaerrors.ErrDaemonLocked
	}

	if clientName == commonNamespace {
		return fmt.Errorf("%w: 'common' is a reserved client name", gaiaerrors.ErrReservedName)
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		// First, check if a client with this name already exists to prevent duplicates.
		_, _, err := d.findClientByName(tx, clientName)
		if err == nil {
			return fmt.Errorf("%w: '%s'", gaiaerrors.ErrClientExists, clientName)
		}

		newClient := Client{
			ID:          uuid.New().String(),
			Name:        clientName,
			Status:      ClientStatusActive,
			TimeCreated: time.Now().UTC().Format(time.RFC3339),
		}

		clientData, err := json.Marshal(newClient)
		if err != nil {
			return fmt.Errorf("failed to marshal new client: %w", err)
		}

		b := tx.Bucket([]byte(clientsBucket))
		if b == nil {
			return gaiaerrors.NewStorageError("registerClient", clientsBucket, "", "bucket not found", gaiaerrors.ErrBucketNotFound)
		}
		if err := b.Put([]byte(newClient.ID), clientData); err != nil {
			return fmt.Errorf("failed to store client: %w", err)
		}

		gaialog.Get().Info("client registered", slog.String("client_name", clientName), slog.String("client_id", newClient.ID))
		return nil
	})

	if err != nil {
		return err
	}

	// Create default policy for the new client outside the db transaction
	defaultPolicy := policy.CreateDefaultPolicy(clientName)
	if err := d.policyStore.SetPolicy(defaultPolicy); err != nil {
		gaialog.Get().Warn("failed to create default policy", slog.String("client", clientName), slog.String("error", err.Error()))
		// Don't fail the registration if policy creation fails since the client was already created
	} else {
		gaialog.Get().Info("default policy created", slog.String("client", clientName))
	}

	return nil
}

// ListClients returns a list of all registered clients.
func (d *Daemon) ListClients() ([]Client, error) {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.isLocked || d.db == nil {
		return nil, gaiaerrors.ErrDaemonLocked
	}

	var clients []Client
	err := d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clientsBucket))
		if b == nil {
			return nil // No client's bucket means no clients.
		}

		return b.ForEach(func(k, v []byte) error {
			var client Client
			if err := json.Unmarshal(v, &client); err != nil {
				// Log the error but continue, so one bad record doesn't fail the whole list.
				gaialog.Get().Warn("failed to unmarshal client data, skipping", "key", string(k), "error", err)
				return nil
			}
			clients = append(clients, client)
			return nil
		})
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list clients from database: %w", err)
	}

	return clients, nil
}

// RevokeClient finds a client by name and sets its status to "revoked".
func (d *Daemon) RevokeClient(clientName string) error {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.isLocked || d.db == nil {
		return gaiaerrors.ErrDaemonLocked
	}

	return d.db.Update(func(tx *bbolt.Tx) error {
		client, key, err := d.findClientByName(tx, clientName)
		if err != nil {
			return err // findClientByName will return a clear error if not found.
		}

		if client.Status == ClientStatusRevoked {
			return fmt.Errorf("client '%s' is already revoked", clientName)
		}

		client.Status = ClientStatusRevoked
		updatedData, err := json.Marshal(client)
		if err != nil {
			return fmt.Errorf("failed to marshal updated client data: %w", err)
		}

		b := tx.Bucket([]byte(clientsBucket))
		if err := b.Put(key, updatedData); err != nil {
			return err
		}

		gaialog.Get().Info("client revoked", slog.String("client_name", clientName), slog.String("client_id", client.ID))
		return nil
	})
}

// findClientByName is an internal helper to locate a client by their name.
// It must be called within a database transaction.
func (d *Daemon) findClientByName(tx *bbolt.Tx, name string) (*Client, []byte, error) {
	b := tx.Bucket([]byte(clientsBucket))
	if b == nil {
		return nil, nil, gaiaerrors.NewStorageError("findClientByName", clientsBucket, "", "bucket not found", gaiaerrors.ErrBucketNotFound)
	}

	// First, try a direct lookup. This is a fast path for the 'common' client.
	val := b.Get([]byte(name))
	if val != nil {
		var c Client
		if err := json.Unmarshal(val, &c); err == nil && c.Name == name {
			return &c, []byte(name), nil
		}
	}

	// If direct lookup fails, iterate to find by name.
	var foundClient *Client
	var foundKey []byte
	err := b.ForEach(func(k, v []byte) error {
		var c Client
		if err := json.Unmarshal(v, &c); err != nil {
			return nil // Skip malformed records.
		}
		if c.Name == name {
			foundClient = &c
			foundKey = k
			return fmt.Errorf("client found") // Stop iteration.
		}
		return nil
	})

	if err != nil && err.Error() != "client found" {
		return nil, nil, err
	}

	if foundClient == nil {
		return nil, nil, fmt.Errorf("%w: '%s'", gaiaerrors.ErrClientNotFound, name)
	}

	return foundClient, foundKey, nil
}

// ListNamespaces retrieves all unique namespaces associated with a given client.
func (d *Daemon) ListNamespaces(clientName string) ([]string, error) {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.isLocked || d.db == nil {
		return nil, gaiaerrors.ErrDaemonLocked
	}

	var namespaces []string
	err := d.db.View(func(tx *bbolt.Tx) error {
		client, _, err := d.findClientByName(tx, clientName)
		if err != nil {
			return err
		}

		namespaceSet := make(map[string]struct{})
		prefix := []byte(client.ID + "\x00")

		c := tx.Bucket([]byte(secretsBucket)).Cursor()
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			trimmedKey := bytes.TrimPrefix(k, prefix)
			parts := bytes.SplitN(trimmedKey, []byte("\x00"), 2)
			if len(parts) > 0 {
				namespaceSet[string(parts[0])] = struct{}{}
			}
		}

		for ns := range namespaceSet {
			namespaces = append(namespaces, ns)
		}
		return nil
	})

	return namespaces, err
}
