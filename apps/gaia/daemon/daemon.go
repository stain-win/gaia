package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/encrypt"
	"github.com/stain-win/gaia/apps/gaia/gaialog"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Daemon represents the state of the Gaia daemon.
type Daemon struct {
	config      *config.Config
	server      *grpc.Server
	db          *bbolt.DB
	key         []byte
	caCert      *x509.Certificate
	caKey       *rsa.PrivateKey
	dbLock      sync.RWMutex
	status      string
	isLocked    bool
	stopChannel chan struct{}
}

const (
	saltKey        = "__salt__"
	keyHashKey     = "__key_hash__"
	bucketName     = "secrets"
	clientsBucket  = "clients"
	StatusRunning  = "running"
	StatusStopped  = "stopped"
	StatusStarting = "starting"
)

// NewDaemon creates a new Daemon instance with default configuration.
func NewDaemon(cfg *config.Config) *Daemon {
	return &Daemon{
		config:      cfg,
		status:      StatusStopped,
		isLocked:    true,
		stopChannel: make(chan struct{}),
	}
}

// gaiaClientServer implements the GaiaClientServer interface from the protobuf.
type gaiaClientServer struct {
	pb.UnimplementedGaiaClientServer
	daemon *Daemon
}

// Stop is the gRPC method for stopping the daemon.
func (s *gaiaAdminServer) Stop(_ context.Context, _ *pb.StopRequest) (*pb.StopResponse, error) {
	log.Println("Received stop request via gRPC. Shutting down...")
	close(s.d.stopChannel)
	return &pb.StopResponse{Success: true}, nil
}

// Start launches the gRPC server and opens the database in a locked (read-only) state.
func (d *Daemon) Start(cfg *config.Config) error {
	if d.status == StatusRunning {
		return errors.New("daemon already running")
	}

	d.config = cfg

	if _, err := os.Stat(d.config.DBFile); os.IsNotExist(err) {
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

	d.server = grpc.NewServer(grpc.Creds(creds))
	pb.RegisterGaiaAdminServer(d.server, &gaiaAdminServer{d: d})
	pb.RegisterGaiaClientServer(d.server, &gaiaClientServer{daemon: d})

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", d.config.GRPCPort))
	if err != nil {
		d.db.Close()
		d.status = StatusStopped
		return fmt.Errorf("failed to listen: %w", err)
	}

	d.status = StatusRunning
	d.isLocked = true

	log.Println("Gaia daemon started successfully and is running in the foreground.")
	errChan := make(chan error, 1)
	go func() {
		if err := d.server.Serve(listener); err != nil {
			errChan <- fmt.Errorf("gRPC server stopped with error: %w", err)
		}
	}()

	// Block until a stop signal is received via the channel
	select {
	case <-d.stopChannel:
		d.server.GracefulStop()
	case err := <-errChan:
		return err
	}
	d.status = StatusStopped
	d.db.Close()
	return nil
}

// stopDaemon gracefully stops the gRPC server and closes the database.
func (d *Daemon) stopDaemon(_ context.Context) error {
	if d.status != StatusRunning {
		return errors.New("daemon not running")
	}
	d.server.GracefulStop()
	d.db.Close()
	d.status = StatusStopped
	d.isLocked = true
	log.Println("Gaia daemon stopped")
	return nil
}

// Status returns the current operational status of the daemon.
func (d *Daemon) Status() string {
	return d.status
}

// InitializeDB creates the encrypted BoltDB, derives the key, and stores a hash of the key for validation.
func (d *Daemon) InitializeDB(passphrase string) error {
	if _, err := os.Stat(d.config.DBFile); err == nil {
		return errors.New("database already exists")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}

	// Derive the key from the passphrase.
	key, err := encrypt.DeriveKey([]byte(passphrase), salt)
	if err != nil {
		return err
	}

	// Create a hash of the key for future validation.
	keyHash := sha256.Sum256(key)

	db, err := bbolt.Open(d.config.DBFile, 0600, nil)
	if err != nil {
		return err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte(bucketName))
		if b == nil {
			return errors.New("bucket not found")
		}
		if err := b.Put([]byte(saltKey), salt); err != nil {
			return err
		}
		return b.Put([]byte(keyHashKey), keyHash[:])
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
		return err
	}

	var salt, storedHash []byte
	err = d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return errors.New("bucket not found")
		}
		salt = b.Get([]byte(saltKey))
		if salt == nil {
			return errors.New("salt not found")
		}
		storedHash = b.Get([]byte(keyHashKey))
		if storedHash == nil {
			return errors.New("key hash not found for validation")
		}
		return nil
	})
	if err != nil {
		d.db.Close()
		return err
	}

	// Derive a key from the provided passphrase.
	derivedKey, err := encrypt.DeriveKey([]byte(passphrase), salt)
	if err != nil {
		d.db.Close()
		return err
	}

	// **VALIDATION STEP**
	// Hash the derived key and compare it to the stored hash.
	derivedKeyHash := sha256.Sum256(derivedKey)
	if !bytes.Equal(derivedKeyHash[:], storedHash) {
		d.db.Close()
		return errors.New("invalid passphrase")
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
	gaialog.Get().Info("Daemon is now in a locked state.")
	return nil
}

// RegisterClient adds a new client name to the database.
func (d *Daemon) RegisterClient(clientName string) error {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.isLocked || d.db == nil {
		return errors.New("daemon is in a locked state, cannot register clients")
	}

	err := d.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(clientsBucket))
		if err != nil {
			return fmt.Errorf("failed to create or get clients bucket: %w", err)
		}
		return b.Put([]byte(clientName), []byte(clientName))
	})

	if err == nil {
		gaialog.Get().Info("client registered", slog.String("client_name", clientName))
	}
	return err
}

// AddSecret stores an encrypted secret for a specific client and namespace.
func (d *Daemon) AddSecret(clientName, namespace, id, value string) error {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.isLocked || d.db == nil {
		return errors.New("daemon is in a locked state, cannot write secrets")
	}

	key := fmt.Sprintf("%s/%s/%s", clientName, namespace, id)

	encValue, err := encrypt.Encrypt(d.key, []byte(value))
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}

	err = d.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return fmt.Errorf("failed to create or get bucket: %w", err)
		}
		return b.Put([]byte(key), []byte(encValue))
	})

	if err == nil {
		gaialog.Get().Info("secret added/updated",
			slog.String("client_name", clientName),
			slog.String("namespace", namespace),
			slog.String("id", id),
		)
	}
	return err
}

// DeleteSecret removes a specific secret from the database.
func (d *Daemon) DeleteSecret(clientName, namespace, id string) error {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.isLocked || d.db == nil {
		return errors.New("daemon is in a locked state, cannot delete secrets")
	}

	key := fmt.Sprintf("%s/%s/%s", clientName, namespace, id)

	err := d.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			// If the bucket doesn't exist, the secret can't exist either.
			return nil
		}
		// b.Delete does not return an error if the key does not exist.
		return b.Delete([]byte(key))
	})

	if err == nil {
		gaialog.Get().Info("secret deleted",
			slog.String("client_name", clientName),
			slog.String("namespace", namespace),
			slog.String("id", id),
		)
	}
	return err
}

// GetSecret retrieves and decrypts a secret, enforcing authorization.
func (d *Daemon) GetSecret(clientName, namespace, id string) (string, error) {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.isLocked {
		return "", errors.New("daemon is locked")
	}

	if d.db == nil {
		return "", errors.New("database not open")
	}

	// Authorization Logic: A client can access its own namespace or the common one.
	if namespace != "common" && clientName != namespace {
		return "", fmt.Errorf("permission denied: client '%s' is not authorized for namespace '%s'", clientName, namespace)
	}

	// For the 'common' namespace, the key is stored under a literal 'common' client name.
	lookupClient := clientName
	if namespace == "common" {
		lookupClient = "common"
	}
	key := fmt.Sprintf("%s/%s/%s", lookupClient, namespace, id)

	var encValue []byte
	err := d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return errors.New("bucket not found")
		}
		encValue = b.Get([]byte(key))
		if encValue == nil {
			return errors.New("secret not found")
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	decValue, err := encrypt.Decrypt(d.key, string(encValue))
	if err != nil {
		gaialog.Get().Error("secret failed to decrypt",
			"client", clientName,
			"namespace", namespace,
			"id", id,
		)
		return "", fmt.Errorf("failed to decrypt secret: %w", err)
	}

	gaialog.Get().Info("secret accessed",
		slog.String("client_name", clientName),
		slog.String("namespace", namespace),
		slog.String("id", id),
	)
	return string(decValue), nil
}

// openDB is an internal helper to open the BoltDB file.
func (d *Daemon) openDB() error {
	var err error
	d.db, err = bbolt.Open(d.config.DBFile, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	return nil
}

// loadTLSCredentials is an internal helper to set up mTLS.
func (d *Daemon) loadTLSCredentials() (credentials.TransportCredentials, error) {
	caCertPath := filepath.Join(d.config.CertsDirectory, d.config.CACertFile)
	serverCertPath := filepath.Join(d.config.CertsDirectory, d.config.ServerCertFile)
	serverKeyPath := filepath.Join(d.config.CertsDirectory, d.config.ServerKeyFile)

	certPool := x509.NewCertPool()
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("could not read CA certificate: %w", err)
	}
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("could not append CA certificate to pool")
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
func (d *Daemon) loadCACredentials() error {
	caKeyPath := d.config.CertsDirectory + "/ca.key"
	caCertPath := d.config.CertsDirectory + "/ca.crt"

	keyBytes, err := os.ReadFile(caKeyPath)
	if err != nil {
		return err
	}
	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		return errors.New("failed to decode CA private key PEM")
	}
	d.caKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}

	certBytes, err := os.ReadFile(caCertPath)
	if err != nil {
		return err
	}
	certBlock, _ := pem.Decode(certBytes)
	if certBlock == nil {
		return errors.New("failed to decode CA certificate PEM")
	}
	d.caCert, err = x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return err
	}

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

func (d *Daemon) GetConfig() *config.Config {
	if d.config == nil {
		return config.NewDefaultConfig()
	}
	return d.config
}

func (d *Daemon) ListClients() ([]string, error) {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.isLocked || d.db == nil {
		return nil, errors.New("daemon is in a locked state, cannot list clients")
	}

	var clients []string
	err := d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clientsBucket))
		if b == nil {
			// If the bucket doesn't exist for some reason, return an empty list.
			return nil
		}

		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			clients = append(clients, string(k))
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list clients from database: %w", err)
	}

	return clients, nil
}

// RevokeClient removes a client's registration and all of its associated secrets.
func (d *Daemon) RevokeClient(clientName string) error {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.isLocked || d.db == nil {
		return errors.New("daemon is in a locked state, cannot revoke clients")
	}

	return d.db.Update(func(tx *bbolt.Tx) error {
		clientsB := tx.Bucket([]byte(clientsBucket))
		if clientsB != nil {
			if err := clientsB.Delete([]byte(clientName)); err != nil {
				return fmt.Errorf("failed to delete client from registry: %w", err)
			}
		}
		secretsB := tx.Bucket([]byte(bucketName))
		if secretsB == nil {
			return nil // No secrets bucket, so nothing to delete.
		}

		prefix := []byte(clientName + "/")
		c := secretsB.Cursor()
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			if err := c.Delete(); err != nil {
				log.Printf("error deleting secret %s for revoked client %s: %v", string(k), clientName, err)
			}
		}

		return nil
	})
}

// ListNamespaces retrieves all unique namespaces associated with a given client.
func (d *Daemon) ListNamespaces(clientName string) ([]string, error) {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.isLocked || d.db == nil {
		return nil, errors.New("daemon is in a locked state, cannot list namespaces")
	}

	namespaceSet := make(map[string]struct{})
	prefix := []byte(clientName + "/")

	err := d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil // No secrets, so no namespaces.
		}

		c := b.Cursor()
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			// The key format is clientName/namespace/id
			// We need to extract the 'namespace' part.
			trimmedKey := bytes.TrimPrefix(k, prefix)
			parts := bytes.SplitN(trimmedKey, []byte("/"), 2)
			if len(parts) > 0 {
				namespaceSet[string(parts[0])] = struct{}{}
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces for client '%s': %w", clientName, err)
	}

	// Convert the set of namespaces to a slice.
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	return namespaces, nil
}

// ImportSecrets performs a bulk, transactional import of secrets.
func (d *Daemon) ImportSecrets(secrets []*pb.ImportSecretItem, overwrite bool) (int, error) {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.isLocked || d.db == nil {
		return 0, errors.New("daemon is in a locked state, cannot import secrets")
	}

	var importedCount int
	err := d.db.Update(func(tx *bbolt.Tx) error {
		secretsB, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return fmt.Errorf("failed to get secrets bucket: %w", err)
		}

		for _, secret := range secrets {
			key := fmt.Sprintf("%s/%s/%s", secret.ClientName, secret.Namespace, secret.Id)
			keyBytes := []byte(key)

			// If not overwriting, check if the secret already exists.
			if !overwrite && secretsB.Get(keyBytes) != nil {
				return fmt.Errorf("secret '%s' already exists. Use --overwrite to replace it", key)
			}

			encValue, err := encrypt.Encrypt(d.key, []byte(secret.Value))
			if err != nil {
				// Failing here will roll back the entire transaction.
				return fmt.Errorf("failed to encrypt secret %s: %w", key, err)
			}

			if err := secretsB.Put(keyBytes, []byte(encValue)); err != nil {
				return fmt.Errorf("failed to write secret %s to db: %w", key, err)
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
