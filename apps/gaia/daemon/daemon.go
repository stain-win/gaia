// Package daemon implements the core Gaia daemon server.
// It manages the gRPC server, BoltDB database, encryption state, and provides
// the main API for secret management, client registration, and daemon lifecycle.
package daemon

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"os"
	"sync"

	"time"

	"github.com/stain-win/gaia/apps/gaia/audit"
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/policy"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"go.etcd.io/bbolt"
	"google.golang.org/grpc"
)

// nullByte is the delimiter used for constructing composite keys in the database.
var nullByte = []byte{0x00}

const (
	metaPrefix           = "gaia:internal:cmfk1rbd000000m74bic9evy3"
	saltKey              = metaPrefix + "__salt__"
	keyHashKey           = metaPrefix + "__key_hash__"
	unsafeModeKey        = metaPrefix + "__unsafe_mode__"
	derivationVersionKey = metaPrefix + "__derive_version__"

	// Derivation version values
	derivationV1       = "v1"        // Production: scrypt N=2^17
	derivationV1Legacy = "v1-legacy" // Unsafe/dev: scrypt N=2^15
	secretsBucket      = "secrets"
	clientsBucket      = "clients"
	StatusRunning      = "running"
	StatusStopped      = "stopped"
	StatusStarting     = "starting"
	commonNamespace    = "common"

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

// Rate limiting constants for unlock attempts
const (
	maxUnlockAttempts  = 5                // Maximum failed attempts before lockout
	unlockLockoutTime  = 5 * time.Minute  // Lockout duration after max attempts
	unlockAttemptReset = 15 * time.Minute // Reset attempt counter after this duration
)

// Daemon represents the state of the Gaia daemon.
type Daemon struct {
	config        *config.Config
	server        *grpc.Server
	db            *bbolt.DB
	policyStore   *policy.Store
	auditLogger   *audit.Logger
	key           []byte
	caCert        *x509.Certificate
	caKey         *rsa.PrivateKey
	dbLock        sync.RWMutex
	status        string
	isLocked      bool
	unsafeMode    bool
	ephemeralMode bool
	stopChannel   chan struct{}
	stopOnce      sync.Once // Ensures shutdown runs only once
	createdAt     time.Time

	// Rate limiting for unlock attempts
	unlockAttempts    int       // Current number of failed attempts
	unlockLastAttempt time.Time // Time of last unlock attempt
	unlockLockedUntil time.Time // If set, unlock is locked out until this time
	unlockLock        sync.Mutex
}

// NewDaemon creates a new Daemon instance with the default configuration.
func NewDaemon(cfg *config.Config) *Daemon {
	return &Daemon{
		config:        cfg,
		status:        StatusStopped,
		isLocked:      true,
		unsafeMode:    cfg.UnsafeMode,
		ephemeralMode: cfg.EphemeralMode,
		stopChannel:   make(chan struct{}),
		createdAt:     time.Now().UTC(),
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

	// For ephemeral mode, unlink the file immediately after opening.
	// The file descriptor stays valid (POSIX); the OS reclaims disk space when closed.
	if d.ephemeralMode {
		_ = os.Remove(d.config.Daemon.DBFile) // best-effort unlink
	}

	// Initialize policy store
	d.policyStore, err = policy.NewStore(d.db)
	if err != nil {
		_ = d.db.Close()
		return fmt.Errorf("failed to initialize policy store: %w", err)
	}

	return nil
}
