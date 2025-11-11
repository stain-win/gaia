// Package daemon implements the core Gaia daemon server.
// It manages the gRPC server, BoltDB database, encryption state, and provides
// the main API for secret management, client registration, and daemon lifecycle.
package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/encrypt"
	gaiaerrors "github.com/stain-win/gaia/apps/gaia/internal/errors"
	gaialog "github.com/stain-win/gaia/apps/gaia/log"
	"github.com/stain-win/gaia/apps/gaia/policy"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// nullByte is the delimiter used for constructing composite keys in the database.
var nullByte = []byte{0x00}

const (
	metaPrefix      = "gaia:internal:cmfk1rbd000000m74bic9evy3"
	saltKey         = metaPrefix + "__salt__"
	keyHashKey      = metaPrefix + "__key_hash__"
	secretsBucket   = "secrets"
	clientsBucket   = "clients"
	StatusRunning   = "running"
	StatusStopped   = "stopped"
	StatusStarting  = "starting"
	commonNamespace = "common"

	// ClientStatusActive Client statuses
	ClientStatusActive  = "active"
	ClientStatusRevoked = "revoked"
)

// Client represents a registered client in the Gaia system.
type Client struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	TimeCreated string `json:"time_created"`
}

// Daemon represents the state of the Gaia daemon.
type Daemon struct {
	config      *config.Config
	server      *grpc.Server
	db          *bbolt.DB
	policyStore *policy.Store
	key         []byte
	caCert      *x509.Certificate
	caKey       *rsa.PrivateKey
	dbLock      sync.RWMutex
	status      string
	isLocked    bool
	stopChannel chan struct{}
	stopOnce    sync.Once // Ensures shutdown runs only once
	createdAt   time.Time
}

// NewDaemon creates a new Daemon instance with the default configuration.
func NewDaemon(cfg *config.Config) *Daemon {
	return &Daemon{
		config:      cfg,
		status:      StatusStopped,
		isLocked:    true,
		stopChannel: make(chan struct{}),
		createdAt:   time.Now().UTC(),
	}
}

// gaiaClientServer implements the GaiaClientServer interface from the protobuf.
type gaiaClientServer struct {
	pb.UnimplementedGaiaClientServer
	daemon *Daemon
}

// gaiaAdminServer implements the GaiaAdminServer interface from the protobuf.
type gaiaAdminServer struct {
	pb.UnimplementedGaiaAdminServer
	d *Daemon
}

// Start launches the gRPC server and opens the database in a locked (read-only) state.
func (d *Daemon) Start(cfg *config.Config) error {
	if d.status == StatusRunning {
		return gaiaerrors.ErrDaemonAlreadyRunning
	}

	d.config = cfg

	if _, err := os.Stat(d.config.Daemon.DBFile); os.IsNotExist(err) {
		return fmt.Errorf("initial setup not complete, run 'gaia init' first")
	}

	d.status = StatusStarting

	creds, err := d.loadTLSCredentials()
	if err != nil {
		d.status = StatusStopped
		return fmt.Errorf("failed to load TLS credentials: %w", err)
	}

	d.dbLock.Lock()
	if err := d.openDB(); err != nil {
		d.dbLock.Unlock()
		d.status = StatusStopped
		return fmt.Errorf("failed to open database: %w", err)
	}
	d.dbLock.Unlock()

	serverOpts := []grpc.ServerOption{
		grpc.Creds(creds),
		grpc.MaxConcurrentStreams(100),
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10 MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10 MB
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Minute,
			PermitWithoutStream: true,
		}),
	}

	d.server = grpc.NewServer(serverOpts...)
	pb.RegisterGaiaAdminServer(d.server, &gaiaAdminServer{d: d})
	pb.RegisterGaiaClientServer(d.server, &gaiaClientServer{daemon: d})

	listener, err := net.Listen("tcp", d.config.Daemon.ListenAddr)
	if err != nil {
		if closeErr := d.db.Close(); closeErr != nil {
			gaialog.Get().Error("failed to close database after listen error", "error", closeErr)
		}
		d.status = StatusStopped
		return fmt.Errorf("failed to listen: %w", err)
	}

	d.status = StatusRunning
	d.isLocked = true

	gaialog.Get().Info("Gaia daemon started successfully", "address", d.config.Daemon.ListenAddr)

	errChan := make(chan error, 1)
	go func() {
		if err := d.server.Serve(listener); err != nil {
			errChan <- fmt.Errorf("gRPC server stopped with error: %w", err)
		}
	}()

	// Block until a stop signal is received via the channel
	select {
	case <-d.stopChannel:
		gaialog.Get().Info("Shutdown signal received, stopping gracefully...")

		// Graceful shutdown with timeout
		stopped := make(chan struct{})
		go func() {
			d.server.GracefulStop()
			close(stopped)
		}()

		// Wait for graceful shutdown with 30-second timeout
		select {
		case <-stopped:
			gaialog.Get().Info("Server stopped gracefully")
		case <-time.After(30 * time.Second):
			gaialog.Get().Warn("Graceful shutdown timeout, forcing stop")
			d.server.Stop()
		}

	case err := <-errChan:
		d.status = StatusStopped
		if closeErr := d.db.Close(); closeErr != nil {
			gaialog.Get().Error("failed to close database", "error", closeErr)
		}
		return err
	}

	d.status = StatusStopped
	if err := d.db.Close(); err != nil {
		gaialog.Get().Error("failed to close database during shutdown", "error", err)
	}
	return nil
}

// stopDaemon gracefully stops the gRPC server and closes the database.
func (d *Daemon) stopDaemon(_ context.Context) error {
	if d.status != StatusRunning {
		return gaiaerrors.ErrDaemonNotRunning
	}
	d.server.GracefulStop()
	d.db.Close()
	d.status = StatusStopped
	d.isLocked = true
	log.Println("Gaia daemon stopped")
	return nil
}

// Restart stops and then starts the daemon.
func (d *Daemon) Restart(ctx context.Context) error {
	log.Println("Restarting daemon...")
	if d.status == StatusRunning {
		err := d.stopDaemon(ctx)
		if err != nil {
			log.Printf("Failed to stop daemon for restart: %v", err)
		}
	} else {
		log.Println("Daemon not running, attempting to start directly.")
	}

	return d.Start(d.config)
}

// Status returns the current operational status of the daemon.
// Returns "locked", "unlocked", "starting", or "stopped"
func (d *Daemon) Status() string {
	// If the daemon is running, return lock state
	if d.status == StatusRunning {
		if d.isLocked {
			return "locked"
		}
		return "unlocked"
	}
	// Otherwise return the daemon status (starting, stopped, etc.)
	return d.status
}

func (d *Daemon) GetConfig() *config.Config {
	if d.config == nil {
		return config.NewDefaultConfig()
	}
	return d.config
}

// Stop is the gRPC method for stopping the daemon.
func (s *gaiaAdminServer) Stop(_ context.Context, _ *pb.StopRequest) (*pb.StopResponse, error) {
	gaialog.Get().Info("Received stop request via gRPC")

	// Use sync.Once to ensure shutdown happens only once
	s.d.stopOnce.Do(func() {
		close(s.d.stopChannel)
	})

	return &pb.StopResponse{Success: true}, nil
}

// InitializeDB creates the encrypted BoltDB, derives the key, and stores a hash of the key for validation.
func (d *Daemon) InitializeDB(passphrase string) error {
	if _, err := os.Stat(d.config.Daemon.DBFile); err == nil {
		return gaiaerrors.ErrDatabaseExists
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive the key from the passphrase.
	key, err := encrypt.DeriveKey([]byte(passphrase), salt)
	if err != nil {
		return err
	}

	// Create a hash of the key for future validation.
	keyHash := sha256.Sum256(key)

	db, err := bbolt.Open(d.config.Daemon.DBFile, 0600, nil)
	if err != nil {
		return err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		secretsB, err := tx.CreateBucketIfNotExists([]byte(secretsBucket))
		if err != nil {
			return fmt.Errorf("failed to create secrets bucket: %w", err)
		}
		if err := secretsB.Put([]byte(saltKey), salt); err != nil {
			return err
		}
		if err := secretsB.Put([]byte(keyHashKey), keyHash[:]); err != nil {
			return fmt.Errorf("failed to store key hash: %w", err)
		}
		clientsB, err := tx.CreateBucketIfNotExists([]byte(clientsBucket))
		if err != nil {
			return fmt.Errorf("failed to create clients bucket: %w", err)
		}

		// Create the special 'common' client, keyed by its name for easy lookup.
		commonClient := Client{
			ID:          commonNamespace, // Use the name as the ID for this special client.
			Name:        commonNamespace,
			Status:      ClientStatusActive,
			TimeCreated: time.Now().UTC().Format(time.RFC3339),
		}
		clientData, err := json.Marshal(commonClient)
		if err != nil {
			return fmt.Errorf("failed to marshal common client: %w", err)
		}
		if err := clientsB.Put([]byte(commonNamespace), clientData); err != nil {
			return fmt.Errorf("failed to register common client: %w", err)
		}

		return nil
	})
	if err != nil {
		db.Close()
		return err
	}
	return db.Close()
}

// LockDB closes the DB and wipes the in-memory key, returning to a locked state.
func (d *Daemon) LockDB() {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.db != nil {
		d.db.Close()
		d.db = nil
	}
	// Wipe the key from memory
	for i := range d.key {
		d.key[i] = 0
	}
	d.key = nil
	d.isLocked = true
	gaialog.Get().Info("Daemon is now in a locked state.")
}

// UnlockDB validates the passphrase, loads the decryption key, and loads the CA credentials.
func (d *Daemon) UnlockDB(passphrase string) error {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.db != nil {
		d.db.Close()
	}

	err := d.openDB()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	var salt, storedHash []byte
	err = d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(secretsBucket))
		if b == nil {
			return gaiaerrors.ErrBucketNotFound
		}
		salt = b.Get([]byte(saltKey))
		if salt == nil {
			return gaiaerrors.ErrSaltNotFound
		}
		storedHash = b.Get([]byte(keyHashKey))
		if storedHash == nil {
			return gaiaerrors.ErrKeyHashNotFound
		}
		return nil
	})
	if err != nil {
		d.db.Close()
		return fmt.Errorf("failed to read database metadata: %w", err)
	}

	// Derive a key from the provided passphrase.
	derivedKey, err := encrypt.DeriveKey([]byte(passphrase), salt)
	if err != nil {
		d.db.Close()
		return fmt.Errorf("failed to derive key: %w", err)
	}

	// **VALIDATION STEP**
	// Hash the derived key and compare it to the stored hash.
	derivedKeyHash := sha256.Sum256(derivedKey)
	if subtle.ConstantTimeCompare(derivedKeyHash[:], storedHash) != 1 {
		d.db.Close()
		return gaiaerrors.ErrInvalidPassphrase
	}

	// If validation passes, store the key and proceed.
	d.key = derivedKey

	if err := d.loadCACredentials(); err != nil {
		d.db.Close()
		d.db = nil
		d.key = nil
		return fmt.Errorf("failed to load CA credentials: %w", err)
	}

	d.isLocked = false
	d.status = StatusRunning
	gaialog.Get().Info("Daemon is now unlocked.")
	return nil
}

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

	return d.db.Update(func(tx *bbolt.Tx) error {
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

		// Create default policy for the new client
		defaultPolicy := policy.CreateDefaultPolicy(clientName)
		if err := d.policyStore.SetPolicy(defaultPolicy); err != nil {
			gaialog.Get().Warn("failed to create default policy", slog.String("client", clientName), slog.String("error", err.Error()))
			// Don't fail the registration if policy creation fails
		} else {
			gaialog.Get().Info("default policy created", slog.String("client", clientName))
		}

		return nil
	})
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

// GetSecret retrieves and decrypts a secret, enforcing authorization.
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
		requestingClient, _, err := d.findClientByName(tx, clientName)
		if err != nil {
			return fmt.Errorf("unauthorized: client '%s' not registered", clientName)
		}

		if requestingClient.Status != ClientStatusActive {
			return fmt.Errorf("unauthorized: client '%s' is not active", clientName)
		}

		var lookupClientID string
		if namespace == commonNamespace {
			commonClient, _, err := d.findClientByName(tx, commonNamespace)
			if err != nil {
				return fmt.Errorf("internal error: common client not found")
			}
			lookupClientID = commonClient.ID
		} else if namespace == clientName {
			lookupClientID = requestingClient.ID
		} else {
			return fmt.Errorf("permission denied: client '%s' is not authorized for namespace '%s'", clientName, namespace)
		}

		key := constructDBKey(lookupClientID, namespace, id)

		var encValue []byte
		b := tx.Bucket([]byte(secretsBucket))
		if b == nil {
			return gaiaerrors.ErrBucketNotFound
		}
		encValue = b.Get(key)
		if encValue == nil {
			return fmt.Errorf("%w: %s/%s/%s", gaiaerrors.ErrSecretNotFound, clientName, namespace, id)
		}

		decVal, err := encrypt.Decrypt(d.key, string(encValue))
		if err != nil {
			gaialog.Get().Error("secret failed to decrypt", "client", clientName, "namespace", namespace, "id", id)
			return fmt.Errorf("failed to decrypt secret: %w", err)
		}
		decryptedValue = string(decVal)

		gaialog.Get().Info("secret accessed", slog.String("client_name", clientName), slog.String("namespace", namespace), slog.String("id", id))
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

// constructDBKey safely joins the parts of a secret's key using a null byte delimiter.
func constructDBKey(clientID, namespace, key string) []byte {
	return bytes.Join([][]byte{[]byte(clientID), []byte(namespace), []byte(key)}, nullByte)
}

// openDB is an internal helper to open the BoltDB file.
func (d *Daemon) openDB() error {
	var err error
	d.db, err = bbolt.Open(d.config.Daemon.DBFile, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}

	// Initialize policy store
	d.policyStore, err = policy.NewStore(d.db)
	if err != nil {
		d.db.Close()
		return fmt.Errorf("failed to initialize policy store: %w", err)
	}

	return nil
}

// loadTLSCredentials is an internal helper to set up mTLS.
func (d *Daemon) loadTLSCredentials() (credentials.TransportCredentials, error) {
	caCertPath := filepath.Join(d.config.TLS.CertsDirectory, d.config.TLS.CACert)
	serverCertPath := filepath.Join(d.config.TLS.CertsDirectory, d.config.TLS.ServerCert)
	serverKeyPath := filepath.Join(d.config.TLS.CertsDirectory, d.config.TLS.ServerKey)

	certPool := x509.NewCertPool()
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("could not read CA certificate: %w", err)
	}
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("could not append CA certificate to pool")
	}

	serverCert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load server key pair: %w", err)
	}
	creds := credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    certPool,
	})
	return creds, nil
}

// loadCACredentials loads the CA certificate and private key from disk.
// Supports both PKCS#1 and PKCS#8 private key formats.
func (d *Daemon) loadCACredentials() error {
	caKeyPath := filepath.Join(d.config.TLS.CertsDirectory, "ca.key")
	caCertPath := filepath.Join(d.config.TLS.CertsDirectory, "ca.crt")

	keyBytes, err := os.ReadFile(caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read CA key file: %w", err)
	}
	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode CA private key PEM")
	}

	// Try PKCS#1 first (traditional RSA format)
	rsaKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		// Fallback to PKCS#8 (modern format that supports multiple key types)
		pkcs8Key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse CA private key (tried both PKCS#1 and PKCS#8): %w", err)
		}

		// Type assert to RSA key
		var ok bool
		rsaKey, ok = pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("CA private key is not an RSA key (got %T)", pkcs8Key)
		}
	}
	d.caKey = rsaKey

	certBytes, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate file: %w", err)
	}
	certBlock, _ := pem.Decode(certBytes)
	if certBlock == nil {
		return fmt.Errorf("failed to decode CA certificate PEM")
	}
	d.caCert, err = x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	return nil
}
