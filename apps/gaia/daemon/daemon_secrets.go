package daemon

import (
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"strings"

	"github.com/stain-win/gaia/apps/gaia/encrypt"
	gaiaerrors "github.com/stain-win/gaia/apps/gaia/internal/errors"
	gaialog "github.com/stain-win/gaia/apps/gaia/log"
	"github.com/stain-win/gaia/apps/gaia/policy"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"go.etcd.io/bbolt"
)

// AddSecret stores an encrypted secret for a specific client and namespace.
func (d *Daemon) AddSecret(clientName, namespace, id, value string) error {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.isLocked || d.db == nil {
		return gaiaerrors.ErrDaemonLocked
	}

	return d.db.Update(func(tx *bbolt.Tx) error {
		client, _, err := d.findClientByName(tx, clientName)
		if err != nil {
			return err
		}

		key := constructDBKey(client.ID, namespace, id)

		encValue, err := encrypt.Encrypt(d.key, []byte(value))
		if err != nil {
			return fmt.Errorf("failed to encrypt secret: %w", err)
		}

		b := tx.Bucket([]byte(secretsBucket))
		if err := b.Put(key, []byte(encValue)); err != nil {
			return err
		}

		gaialog.Get().Info("secret added/updated",
			slog.String("client_name", clientName),
			slog.String("namespace", namespace),
			slog.String("id", id),
		)
		return nil
	})
}

// GetSecret retrieves and decrypts a secret, enforcing policy-based authorization.
func (d *Daemon) GetSecret(clientName, namespace, id string) (string, error) {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.isLocked {
		return "", gaiaerrors.ErrDaemonLocked
	}

	if d.db == nil {
		return "", fmt.Errorf("database not open")
	}

	var decryptedValue string
	err := d.db.View(func(tx *bbolt.Tx) error {
		// Verify client exists and is active
		requestingClient, _, err := d.findClientByName(tx, clientName)
		if err != nil {
			return fmt.Errorf("unauthorized: client '%s' not registered", clientName)
		}

		if requestingClient.Status != ClientStatusActive {
			return fmt.Errorf("unauthorized: client '%s' is not active", clientName)
		}

		// Determine which client owns the secret based on namespace
		var ownerClientID string
		var ownerClientName string
		if namespace == commonNamespace {
			// Secret is in the common namespace
			commonClient, _, err := d.findClientByName(tx, commonNamespace)
			if err != nil {
				return fmt.Errorf("internal error: common client not found")
			}
			ownerClientID = commonClient.ID
			ownerClientName = commonNamespace
		} else {
			// Secret belongs to a specific client - find that client by namespace name
			ownerClient, _, err := d.findClientByName(tx, namespace)
			if err != nil {
				return fmt.Errorf("namespace '%s' does not exist", namespace)
			}
			ownerClientID = ownerClient.ID
			ownerClientName = ownerClient.Name
		}

		// Check policy-based authorization
		// Build path: owner/namespace/key
		path := fmt.Sprintf("%s/%s/%s", ownerClientName, namespace, id)
		if err := d.policyStore.CheckPermission(clientName, path, policy.CapabilityRead); err != nil {
			return fmt.Errorf("permission denied: %w", err)
		}

		// Retrieve and decrypt the secret
		key := constructDBKey(ownerClientID, namespace, id)

		b := tx.Bucket([]byte(secretsBucket))
		if b == nil {
			return gaiaerrors.ErrBucketNotFound
		}
		encValue := b.Get(key)
		if encValue == nil {
			return fmt.Errorf("%w: %s/%s/%s", gaiaerrors.ErrSecretNotFound, ownerClientName, namespace, id)
		}

		decVal, err := encrypt.Decrypt(d.key, string(encValue))
		if err != nil {
			gaialog.Get().Error("secret failed to decrypt", "client", clientName, "namespace", namespace, "id", id)
			return fmt.Errorf("failed to decrypt secret: %w", err)
		}
		decryptedValue = string(decVal)

		gaialog.Get().Info("secret accessed",
			slog.String("client_name", clientName),
			slog.String("namespace", namespace),
			slog.String("id", id),
			slog.String("policy_path", path))
		return nil
	})

	return decryptedValue, err
}

// DeleteSecret removes a specific secret from the database.
func (d *Daemon) DeleteSecret(clientName, namespace, id string) error {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.isLocked || d.db == nil {
		return gaiaerrors.ErrDaemonLocked
	}

	return d.db.Update(func(tx *bbolt.Tx) error {
		client, _, err := d.findClientByName(tx, clientName)
		if err != nil {
			return err
		}

		key := constructDBKey(client.ID, namespace, id)

		b := tx.Bucket([]byte(secretsBucket))
		if b == nil {
			return nil
		}
		return b.Delete(key)
	})
}

// ListSecrets retrieves all namespaces and their secrets for a given client.
func (d *Daemon) ListSecrets(clientName string) (map[string]map[string]string, error) {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.isLocked {
		return nil, gaiaerrors.ErrDaemonLocked
	}

	allSecrets := make(map[string]map[string]string)
	err := d.db.View(func(tx *bbolt.Tx) error {
		client, _, err := d.findClientByName(tx, clientName)
		if err != nil {
			return err
		}

		prefix := []byte(client.ID + "\x00")
		c := tx.Bucket([]byte(secretsBucket)).Cursor()

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			// Use bytes.SplitN instead of converting to string for better performance
			parts := bytes.SplitN(k, nullByte, 3)
			if len(parts) != 3 {
				continue
			}

			namespace := string(parts[1])
			secretKey := string(parts[2])

			decryptedValue, err := encrypt.Decrypt(d.key, string(v))
			if err != nil {
				gaialog.Get().Warn("failed to decrypt secret, skipping", "key", string(k), "error", err)
				continue
			}

			if _, ok := allSecrets[namespace]; !ok {
				allSecrets[namespace] = make(map[string]string)
			}
			allSecrets[namespace][secretKey] = string(decryptedValue)
		}
		return nil
	})

	return allSecrets, err
}

// ListSecretsStream streams all secrets for a given client to the client.
func (d *Daemon) ListSecretsStream(clientName string, stream pb.GaiaAdmin_ListSecretsStreamServer) error {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.isLocked {
		return gaiaerrors.ErrDaemonLocked
	}

	return d.db.View(func(tx *bbolt.Tx) error {
		client, _, err := d.findClientByName(tx, clientName)
		if err != nil {
			return err
		}

		prefix := []byte(client.ID + "\x00")
		c := tx.Bucket([]byte(secretsBucket)).Cursor()

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			// Use bytes.SplitN instead of converting to string for better performance
			parts := bytes.SplitN(k, nullByte, 3)
			if len(parts) != 3 {
				continue
			}

			namespace := string(parts[1])
			secretKey := string(parts[2])

			decryptedValue, err := encrypt.Decrypt(d.key, string(v))
			if err != nil {
				gaialog.Get().Warn("failed to decrypt secret, skipping", "key", string(k), "error", err)
				continue
			}

			// Sanitize value to remove null bytes if present
			val := strings.ReplaceAll(string(decryptedValue), "\x00", "")

			if err := stream.Send(&pb.ListSecretsStreamResponse{
				Namespace: namespace,
				Secret: &pb.Secret{
					Id:    secretKey,
					Value: val,
				},
			}); err != nil {
				return fmt.Errorf("failed to send secret to stream: %w", err)
			}
		}
		return nil
	})
}

// ImportSecrets performs a bulk, transactional import of secrets.
func (d *Daemon) ImportSecrets(secrets []*pb.ImportSecretItem, overwrite bool) (int, error) {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.isLocked || d.db == nil {
		return 0, gaiaerrors.ErrDaemonLocked
	}

	var importedCount int
	err := d.db.Update(func(tx *bbolt.Tx) error {
		secretsB, err := tx.CreateBucketIfNotExists([]byte(secretsBucket))
		if err != nil {
			return fmt.Errorf("failed to get secrets bucket: %w", err)
		}

		for _, secret := range secrets {
			client, _, err := d.findClientByName(tx, secret.ClientName)
			if err != nil {
				return fmt.Errorf("client '%s' not found for import: %w", secret.ClientName, err)
			}

			key := constructDBKey(client.ID, secret.Namespace, secret.Id)

			if !overwrite && secretsB.Get(key) != nil {
				return fmt.Errorf("secret '%s' for client '%s' already exists. Use --overwrite to replace it", secret.Id, secret.ClientName)
			}

			encValue, err := encrypt.Encrypt(d.key, []byte(secret.Value))
			if err != nil {
				return fmt.Errorf("failed to encrypt secret %s: %w", secret.Id, err)
			}

			if err := secretsB.Put(key, []byte(encValue)); err != nil {
				return fmt.Errorf("failed to write secret %s to db: %w", secret.Id, err)
			}
			importedCount++
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	gaialog.Get().Info("bulk secrets imported", slog.Int("count", importedCount))
	log.Printf("Bulk secrets imported successfully, imported %d secrets", importedCount)
	return importedCount, nil
}
