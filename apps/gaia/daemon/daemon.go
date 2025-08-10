package daemon

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/stain-win/gaia/apps/gaia/encrypt"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Config holds non-sensitive daemon configuration.
type Config struct {
	GRPCPort   string
	DBFile     string
	CertFile   string
	KeyFile    string
	CACertFile string
}

// Daemon represents the state of the Gaia daemon.
type Daemon struct {
	config      Config
	server      *grpc.Server
	db          *bbolt.DB
	key         []byte
	dbLock      sync.RWMutex
	status      string
	isLocked    bool
	stopChannel chan struct{}
}

const (
	saltKey        = "__salt__"
	bucketName     = "secrets"
	StatusRunning  = "running"
	StatusStopped  = "stopped"
	StatusLocked   = "locked"
	StatusStarting = "starting"
)

// NewDaemon creates a new Daemon instance with default configuration.
func NewDaemon() *Daemon {
	return &Daemon{
		config: Config{
			GRPCPort:   ":50051",
			DBFile:     "gaia.db",
			CertFile:   "certs/server.crt",
			KeyFile:    "certs/server.key",
			CACertFile: "certs/ca.crt",
		},
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
func (s *gaiaAdminServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	log.Println("Received stop request via gRPC. Shutting down...")
	close(s.d.stopChannel)
	return &pb.StopResponse{Success: true}, nil
}

// Start launches the gRPC server and opens the database in a locked (read-only) state.
func (d *Daemon) Start() error {
	if d.status == StatusRunning {
		return errors.New("daemon already running")
	}
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

	listener, err := net.Listen("tcp", d.config.GRPCPort)
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
// This is now an internal method called by the gRPC Stop handler.
func (d *Daemon) stopDaemon(ctx context.Context) error {
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

// InitializeDB creates the encrypted BoltDB with a derived key. This is a CLI-only function.
func (d *Daemon) InitializeDB(passphrase string) error {
	if _, err := os.Stat(d.config.DBFile); err == nil {
		return errors.New("database already exists")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	if _, err := encrypt.DeriveKey([]byte(passphrase), salt); err != nil {
		return err
	}
	db, err := bbolt.Open(d.config.DBFile, 0600, nil)
	if err != nil {
		return err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte(bucketName))
		if b == nil {
			return errors.New("bucket not found")
		}
		return b.Put([]byte(saltKey), salt)
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
	log.Println("Daemon is now in a locked state.")
}

// UnlockDB opens the DB and loads the key into memory for an administrative session.
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

	var salt []byte
	err = d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return errors.New("bucket not found")
		}
		salt = b.Get([]byte(saltKey))
		if salt == nil {
			return errors.New("salt not found")
		}
		return nil
	})
	if err != nil {
		d.db.Close()
		return err
	}

	d.key, err = encrypt.DeriveKey([]byte(passphrase), salt)
	if err != nil {
		d.db.Close()
		d.db = nil
		return fmt.Errorf("invalid passphrase: %w", err)
	}

	d.isLocked = false
	log.Println("Daemon is now unlocked for an administrative session.")
	return nil
}

// AddSecret stores an encrypted secret in the DB with namespacing.
func (d *Daemon) AddSecret(namespace, id, value string) error {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.isLocked || d.db == nil {
		return errors.New("daemon is in a locked state, cannot write secrets")
	}

	namespacedKey := fmt.Sprintf("%s/%s", namespace, id)

	enc, err := encrypt.Encrypt(d.key, []byte(value))
	if err != nil {
		return err
	}
	return d.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return errors.New("bucket not found")
		}
		return b.Put([]byte(namespacedKey), []byte(enc))
	})
}

// GetSecret retrieves and decrypts a secret from the DB.
func (d *Daemon) GetSecret(id string) (string, error) {
	d.dbLock.RLock()
	defer d.dbLock.RUnlock()

	if d.db == nil {
		return "", errors.New("database not open")
	}
	var enc []byte
	err := d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return errors.New("bucket not found")
		}
		enc = b.Get([]byte(id))
		if enc == nil {
			return errors.New("secret not found")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	dec, err := encrypt.Decrypt(d.key, string(enc))
	if err != nil {
		return "", err
	}
	return string(dec), nil
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
	certPool := x509.NewCertPool()
	caCert, err := os.ReadFile(d.config.CACertFile)
	if err != nil {
		return nil, fmt.Errorf("could not read CA certificate: %w", err)
	}
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("could not append CA certificate to pool")
	}
	serverCert, err := tls.LoadX509KeyPair(d.config.CertFile, d.config.KeyFile)
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

	return d.Start()
}
