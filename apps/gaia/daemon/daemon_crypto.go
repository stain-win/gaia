package daemon

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"time"

	"github.com/stain-win/gaia/apps/gaia/encrypt"
	gaiaerrors "github.com/stain-win/gaia/apps/gaia/internal/errors"
	gaialog "github.com/stain-win/gaia/apps/gaia/log"
	"go.etcd.io/bbolt"
)

// InitializeDB creates the encrypted BoltDB, derives the key, and stores a hash of the key for validation.
func (d *Daemon) InitializeDB(passphrase string) error {
	if d.ephemeralMode {
		// Create a temporary file for the ephemeral database.
		// The path is stored in d.config.Daemon.DBFile so openDB can find it.
		tmpFile, err := os.CreateTemp("", "gaia-ephemeral-*.db")
		if err != nil {
			return fmt.Errorf("failed to create ephemeral database: %w", err)
		}
		d.config.Daemon.DBFile = tmpFile.Name()
		_ = tmpFile.Close()
	} else {
		if _, err := os.Stat(d.config.Daemon.DBFile); err == nil {
			return gaiaerrors.ErrDatabaseExists
		}
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Choose derivation function based on unsafe mode.
	deriveFunc := encrypt.DeriveKey
	deriveVersion := derivationV1
	if d.unsafeMode {
		deriveFunc = encrypt.DeriveKeyLegacy
		deriveVersion = derivationV1Legacy
	}

	key, err := deriveFunc([]byte(passphrase), salt)
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

		// Store derivation version for unlock-time selection
		if err := secretsB.Put([]byte(derivationVersionKey), []byte(deriveVersion)); err != nil {
			return fmt.Errorf("failed to store derivation version: %w", err)
		}

		// Store unsafe mode flag if applicable
		if d.unsafeMode {
			timestamp := time.Now().UTC().Format(time.RFC3339)
			if err := secretsB.Put([]byte(unsafeModeKey), []byte(timestamp)); err != nil {
				return fmt.Errorf("failed to store unsafe mode flag: %w", err)
			}
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
		_ = db.Close()
		return err
	}
	return db.Close()
}

// LockDB closes the DB and wipes the in-memory key, returning to a locked state.
func (d *Daemon) LockDB() {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.db != nil {
		_ = d.db.Close()
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

// checkUnlockRateLimit checks if unlock attempts are rate limited.
// Returns an error if too many failed attempts have occurred.
func (d *Daemon) checkUnlockRateLimit() error {
	d.unlockLock.Lock()
	defer d.unlockLock.Unlock()

	now := time.Now()

	// Check if currently locked out
	if !d.unlockLockedUntil.IsZero() && now.Before(d.unlockLockedUntil) {
		remaining := d.unlockLockedUntil.Sub(now).Round(time.Second)
		gaialog.Get().Warn("unlock attempt rejected due to rate limiting",
			"remaining_lockout", remaining.String(),
			"failed_attempts", d.unlockAttempts)
		return fmt.Errorf("%w: try again in %v", gaiaerrors.ErrUnlockRateLimited, remaining)
	}

	// Reset lockout if it has expired
	if !d.unlockLockedUntil.IsZero() && now.After(d.unlockLockedUntil) {
		d.unlockLockedUntil = time.Time{}
		d.unlockAttempts = 0
	}

	// Reset attempt counter if enough time has passed since last attempt
	if !d.unlockLastAttempt.IsZero() && now.Sub(d.unlockLastAttempt) > unlockAttemptReset {
		d.unlockAttempts = 0
	}

	return nil
}

// recordFailedUnlockAttempt records a failed unlock attempt and triggers lockout if needed.
func (d *Daemon) recordFailedUnlockAttempt() {
	d.unlockLock.Lock()
	defer d.unlockLock.Unlock()

	d.unlockAttempts++
	d.unlockLastAttempt = time.Now()

	gaialog.Get().Warn("failed unlock attempt",
		"attempt_number", d.unlockAttempts,
		"max_attempts", maxUnlockAttempts)

	// Trigger lockout if max attempts exceeded
	if d.unlockAttempts >= maxUnlockAttempts {
		d.unlockLockedUntil = time.Now().Add(unlockLockoutTime)
		gaialog.Get().Error("unlock rate limit triggered",
			"lockout_until", d.unlockLockedUntil.Format(time.RFC3339),
			"lockout_duration", unlockLockoutTime.String())
	}
}

// resetUnlockAttempts resets the failed unlock attempt counter after successful unlock.
func (d *Daemon) resetUnlockAttempts() {
	d.unlockLock.Lock()
	defer d.unlockLock.Unlock()

	d.unlockAttempts = 0
	d.unlockLastAttempt = time.Time{}
	d.unlockLockedUntil = time.Time{}
}

// UnlockDB validates the passphrase, loads the decryption key, and loads the CA credentials.
func (d *Daemon) UnlockDB(passphrase string) error {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.db != nil {
		_ = d.db.Close()
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
		_ = d.db.Close()
		return fmt.Errorf("failed to read database metadata: %w", err)
	}

	// Read derivation version to select the correct key derivation function
	var deriveVersion []byte
	err = d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(secretsBucket))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(derivationVersionKey))
		if v != nil {
			deriveVersion = make([]byte, len(v))
			copy(deriveVersion, v)
		}
		return nil
	})
	if err != nil {
		_ = d.db.Close()
		return fmt.Errorf("failed to read derivation version: %w", err)
	}

	deriveFunc := encrypt.DeriveKey // default: production params
	if string(deriveVersion) == derivationV1Legacy {
		deriveFunc = encrypt.DeriveKeyLegacy
	}

	derivedKey, err := deriveFunc([]byte(passphrase), salt)
	if err != nil {
		_ = d.db.Close()
		return fmt.Errorf("failed to derive key: %w", err)
	}

	// **VALIDATION STEP**
	// Hash the derived key and compare it to the stored hash.
	derivedKeyHash := sha256.Sum256(derivedKey)
	if subtle.ConstantTimeCompare(derivedKeyHash[:], storedHash) != 1 {
		_ = d.db.Close()
		return gaiaerrors.ErrInvalidPassphrase
	}

	// If validation passes, store the key and proceed.
	d.key = derivedKey

	if err := d.loadCACredentials(); err != nil {
		_ = d.db.Close()
		d.db = nil
		d.key = nil
		return fmt.Errorf("failed to load CA credentials: %w", err)
	}

	d.isLocked = false
	d.status = StatusRunning
	gaialog.Get().Info("Daemon is now unlocked.")
	return nil
}

// RotatePassword re-encrypts all secrets with a new key derived from a new passphrase.
// It creates a backup of the database before performing the rotation.
// The entire operation is atomic: if any step fails, the database is unchanged.
func (d *Daemon) RotatePassword(currentPassphrase, newPassphrase string) (int, string, error) {
	d.dbLock.Lock()
	defer d.dbLock.Unlock()

	if d.isLocked || d.db == nil {
		return 0, "", gaiaerrors.ErrDaemonLocked
	}

	// Step 1: Validate current passphrase (defense-in-depth)
	var salt, storedHash []byte
	err := d.db.View(func(tx *bbolt.Tx) error {
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
		return 0, "", fmt.Errorf("failed to read database metadata: %w", err)
	}

	// Read derivation version to select the correct key derivation function
	var deriveVersion []byte
	err = d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(secretsBucket))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(derivationVersionKey))
		if v != nil {
			deriveVersion = make([]byte, len(v))
			copy(deriveVersion, v)
		}
		return nil
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to read derivation version: %w", err)
	}

	deriveFunc := encrypt.DeriveKey
	if string(deriveVersion) == derivationV1Legacy {
		deriveFunc = encrypt.DeriveKeyLegacy
	}

	derivedKey, err := deriveFunc([]byte(currentPassphrase), salt)
	if err != nil {
		return 0, "", fmt.Errorf("failed to derive key: %w", err)
	}
	derivedKeyHash := sha256.Sum256(derivedKey)
	if subtle.ConstantTimeCompare(derivedKeyHash[:], storedHash) != 1 {
		return 0, "", gaiaerrors.ErrInvalidPassphrase
	}

	// Step 2: Validate new passphrase
	if currentPassphrase == newPassphrase {
		return 0, "", gaiaerrors.ErrPassphraseSameAsCurrent
	}
	if !d.unsafeMode {
		if _, err := encrypt.ValidatePassword(newPassphrase); err != nil {
			return 0, "", fmt.Errorf("new passphrase too weak: %w", err)
		}
	}

	// Step 3: Create backup before any mutation
	backupPath, err := d.createBackup()
	if err != nil {
		return 0, "", fmt.Errorf("failed to create backup: %w", err)
	}
	gaialog.Get().Info("database backup created", slog.String("path", backupPath))

	// Step 4: Derive new key from new passphrase + new salt
	newSalt := make([]byte, 16)
	if _, err := rand.Read(newSalt); err != nil {
		return 0, backupPath, fmt.Errorf("failed to generate new salt: %w", err)
	}

	newKey, err := deriveFunc([]byte(newPassphrase), newSalt)
	if err != nil {
		return 0, backupPath, fmt.Errorf("failed to derive new key: %w", err)
	}
	newKeyHash := sha256.Sum256(newKey)

	// Step 5: Atomic re-encryption in a single transaction
	var secretCount int
	err = d.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(secretsBucket))
		if b == nil {
			return gaiaerrors.ErrBucketNotFound
		}

		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			keyStr := string(k)

			// Skip metadata keys
			if strings.HasPrefix(keyStr, metaPrefix) {
				continue
			}

			// Decrypt with old key
			plaintext, err := encrypt.Decrypt(d.key, string(v))
			if err != nil {
				return fmt.Errorf("failed to decrypt secret during rotation (key=%s): %w", keyStr, err)
			}

			// Re-encrypt with new key
			newEncValue, err := encrypt.Encrypt(newKey, plaintext)
			if err != nil {
				return fmt.Errorf("failed to re-encrypt secret during rotation (key=%s): %w", keyStr, err)
			}

			if err := b.Put(k, []byte(newEncValue)); err != nil {
				return fmt.Errorf("failed to write re-encrypted secret: %w", err)
			}
			secretCount++
		}

		// Update salt and key hash
		if err := b.Put([]byte(saltKey), newSalt); err != nil {
			return fmt.Errorf("failed to update salt: %w", err)
		}
		if err := b.Put([]byte(keyHashKey), newKeyHash[:]); err != nil {
			return fmt.Errorf("failed to update key hash: %w", err)
		}

		return nil
	})
	if err != nil {
		return 0, backupPath, fmt.Errorf("rotation failed (database unchanged, backup at %s): %w", backupPath, err)
	}

	// Step 6: Swap in-memory key
	for i := range d.key {
		d.key[i] = 0
	}
	d.key = newKey

	gaialog.Get().Info("password rotation completed",
		slog.Int("secrets_rotated", secretCount),
		slog.String("backup_path", backupPath))

	return secretCount, backupPath, nil
}

// createBackup creates a consistent snapshot of the database file.
func (d *Daemon) createBackup() (string, error) {
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupPath := d.config.Daemon.DBFile + ".bak." + timestamp

	backupFile, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer func() { _ = backupFile.Close() }()

	err = d.db.View(func(tx *bbolt.Tx) error {
		_, err := tx.WriteTo(backupFile)
		return err
	})
	if err != nil {
		_ = os.Remove(backupPath)
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	return backupPath, nil
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
