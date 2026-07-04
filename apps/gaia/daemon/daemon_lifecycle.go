package daemon

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"time"

	"github.com/stain-win/gaia/apps/gaia/audit"
	"github.com/stain-win/gaia/apps/gaia/config"
	gaiaerrors "github.com/stain-win/gaia/apps/gaia/internal/errors"
	gaialog "github.com/stain-win/gaia/apps/gaia/log"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
	"go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Start launches the gRPC server and opens the database in a locked (read-only) state.
func (d *Daemon) Start(cfg *config.Config) error {
	if d.status == StatusRunning {
		return gaiaerrors.ErrDaemonAlreadyRunning
	}

	d.config = cfg

	if !d.ephemeralMode {
		if _, err := os.Stat(d.config.Daemon.DBFile); os.IsNotExist(err) {
			return fmt.Errorf("initial setup not complete, run 'gaia init' first")
		}
	}

	d.status = StatusStarting

	creds, err := d.loadTLSCredentials()
	if err != nil {
		d.status = StatusStopped
		return fmt.Errorf("failed to load TLS credentials: %w", err)
	}

	if d.ephemeralMode {
		// Auto-initialize and unlock the ephemeral database with a random session key.
		randBytes := make([]byte, 32)
		if _, err := rand.Read(randBytes); err != nil {
			d.status = StatusStopped
			return fmt.Errorf("ephemeral: failed to generate session key: %w", err)
		}
		ephemeralPass := base64.StdEncoding.EncodeToString(randBytes)
		if err := d.InitializeDB(ephemeralPass); err != nil {
			d.status = StatusStopped
			return fmt.Errorf("ephemeral: failed to initialize DB: %w", err)
		}
		if err := d.UnlockDB(ephemeralPass); err != nil {
			d.status = StatusStopped
			return fmt.Errorf("ephemeral: failed to unlock DB: %w", err)
		}
	} else {
		d.dbLock.Lock()
		if err := d.openDB(); err != nil {
			d.dbLock.Unlock()
			d.status = StatusStopped
			return fmt.Errorf("failed to open database: %w", err)
		}

		// Check unsafe mode flag consistency between DB and runtime flag
		if err := d.checkUnsafeModeConsistency(); err != nil {
			_ = d.db.Close()
			d.db = nil
			d.dbLock.Unlock()
			d.status = StatusStopped
			return err
		}
		d.dbLock.Unlock()
	}

	// Initialize audit logger
	if err := d.initAuditLogger(); err != nil {
		gaialog.Get().Warn("failed to initialize audit logger, continuing without audit", "error", err)
		d.auditLogger = audit.NoopLogger()
	}

	// Create audit interceptor
	auditInterceptor := audit.NewInterceptor(d.auditLogger)

	serverOpts := []grpc.ServerOption{
		grpc.Creds(creds),
		grpc.MaxConcurrentStreams(100),
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10 MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10 MB
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Minute,
			PermitWithoutStream: true,
		}),
		// Audit interceptor is outermost so that rejected calls are still logged;
		// the auth interceptor runs next and blocks unauthorized admin callers and
		// revoked clients before any handler executes.
		grpc.ChainUnaryInterceptor(auditInterceptor.UnaryServerInterceptor(), d.authUnaryInterceptor()),
		grpc.ChainStreamInterceptor(auditInterceptor.StreamServerInterceptor(), d.authStreamInterceptor()),
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
	// Ephemeral mode starts fully unlocked; safe/unsafe modes start locked.
	d.isLocked = !d.ephemeralMode

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

	// Close audit logger first to flush any pending entries
	if d.auditLogger != nil {
		if err := d.auditLogger.Close(); err != nil {
			gaialog.Get().Error("failed to close audit logger", "error", err)
		}
	}

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
	_ = d.db.Close()
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

// Stop is the gRPC method for stopping the daemon.
func (s *gaiaAdminServer) Stop(_ context.Context, _ *pb.StopRequest) (*pb.StopResponse, error) {
	gaialog.Get().Info("Received stop request via gRPC")

	// Use sync.Once to ensure shutdown happens only once
	s.d.stopOnce.Do(func() {
		close(s.d.stopChannel)
	})
	return &pb.StopResponse{Success: true}, nil
}

// ListSecretsStream implements the streaming RPC for listing secrets.
func (s *gaiaAdminServer) ListSecretsStream(req *pb.ListSecretsRequest, stream pb.GaiaAdmin_ListSecretsStreamServer) error {
	return s.d.ListSecretsStream(req.ClientName, stream)
}

// checkUnsafeModeConsistency verifies that the --unsafe runtime flag matches the database's
// stored unsafe mode state. Must be called with d.db open and d.dbLock held.
// Ephemeral databases are always created fresh and skip this check.
func (d *Daemon) checkUnsafeModeConsistency() error {
	if d.ephemeralMode {
		return nil
	}
	var dbIsUnsafe bool
	err := d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(secretsBucket))
		if b == nil {
			return nil
		}
		if v := b.Get([]byte(unsafeModeKey)); v != nil {
			dbIsUnsafe = true
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to read unsafe mode flag: %w", err)
	}

	switch {
	case !dbIsUnsafe && d.unsafeMode:
		return fmt.Errorf("database was created without --unsafe and cannot be started in unsafe mode")
	case dbIsUnsafe && !d.unsafeMode:
		return fmt.Errorf("database was created in unsafe mode; you must pass --unsafe to start it")
	}
	return nil
}

// initAuditLogger initializes the audit logger with configured backends.
func (d *Daemon) initAuditLogger() error {
	auditCfg := d.config.Audit

	if !auditCfg.Enabled {
		gaialog.Get().Info("audit logging is disabled")
		d.auditLogger = audit.NoopLogger()
		return nil
	}

	if len(auditCfg.Backends) == 0 {
		gaialog.Get().Warn("audit logging enabled but no backends configured, using default file backend")
		// Default to file backend writing to stdout
		auditCfg.Backends = []config.AuditBackendConfig{
			{Type: "file", Path: "-"},
		}
	}

	// Convert config backend configs to audit backend configs
	var backendConfigs []audit.BackendConfig
	for _, bc := range auditCfg.Backends {
		backendConfigs = append(backendConfigs, audit.BackendConfig{
			Type: bc.Type,
			Path: bc.Path,
			Options: audit.BackendOptions{
				MaxSizeMB:       bc.Options.MaxSizeMB,
				MaxBackups:      bc.Options.MaxBackups,
				MaxAgeDays:      bc.Options.MaxAgeDays,
				RetentionDays:   bc.Options.RetentionDays,
				RateLimitPerSec: bc.Options.RateLimitPerSec,
				TimeoutSeconds:  bc.Options.TimeoutSeconds,
				Headers:         bc.Options.Headers,
			},
		})
	}

	// Create backends
	backends, err := audit.NewBackendsFromConfig(backendConfigs, d.db)
	if err != nil {
		return fmt.Errorf("failed to create audit backends: %w", err)
	}

	// Create the logger
	loggerCfg := audit.LoggerConfig{
		Enabled:     true,
		HMACKey:     auditCfg.HMACKey,
		LogRequest:  auditCfg.LogRequest,
		LogResponse: auditCfg.LogResponse,
	}

	d.auditLogger = audit.NewLogger(loggerCfg, backends...)

	gaialog.Get().Info("audit logging initialized",
		slog.Int("backends", len(backends)),
		slog.Bool("log_request", auditCfg.LogRequest),
		slog.Bool("log_response", auditCfg.LogResponse))

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
		// All Gaia components are Go and support TLS 1.3; refusing older
		// protocol versions removes the legacy downgrade surface.
		MinVersion: tls.VersionTLS13,
	})
	return creds, nil
}
