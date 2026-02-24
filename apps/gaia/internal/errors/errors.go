// Package errors provides custom error types and sentinel errors for the Gaia daemon.
// It defines domain-specific errors for validation, storage, authentication, and cryptography
// operations, enabling better error handling and debugging throughout the application.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common conditions that can be checked with errors.Is().
var (
	ErrDaemonLocked         = errors.New("daemon is locked")
	ErrDaemonNotRunning     = errors.New("daemon not running")
	ErrDaemonAlreadyRunning = errors.New("daemon already running")
	ErrInvalidPassphrase    = errors.New("invalid passphrase")
	ErrUnlockRateLimited    = errors.New("too many failed unlock attempts, please try again later")
	ErrDatabaseExists       = errors.New("database already exists")
	ErrClientNotFound       = errors.New("client not found")
	ErrClientExists         = errors.New("client already exists")
	ErrSecretNotFound       = errors.New("secret not found")
	ErrBucketNotFound       = errors.New("bucket not found")
	ErrSaltNotFound         = errors.New("salt not found")
	ErrKeyHashNotFound      = errors.New("key hash not found")
	ErrReservedName         = errors.New("reserved name")
	ErrNoPeerContext        = errors.New("could not get peer from context")
	ErrNotTLS               = errors.New("peer auth info is not TLS")
	ErrNoPeerCertificates   = errors.New("no peer certificates found")
	ErrPermissionDenied     = errors.New("permission denied")
)

// ValidationError represents a validation failure with detailed context.
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error.
func NewValidationError(field, value, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// StorageError represents database/storage operation failures with context.
type StorageError struct {
	Op      string // Operation being performed (e.g., "get", "put", "delete")
	Bucket  string // Bucket name
	Key     string // Key being accessed
	Message string // Human-readable message
	Err     error  // Underlying error
}

func (e *StorageError) Error() string {
	if e.Key != "" {
		if e.Err != nil {
			return fmt.Sprintf("storage error in %s (bucket=%s, key=%s): %s: %v", e.Op, e.Bucket, e.Key, e.Message, e.Err)
		}
		return fmt.Sprintf("storage error in %s (bucket=%s, key=%s): %s", e.Op, e.Bucket, e.Key, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("storage error in %s (bucket=%s): %s: %v", e.Op, e.Bucket, e.Message, e.Err)
	}
	return fmt.Sprintf("storage error in %s (bucket=%s): %s", e.Op, e.Bucket, e.Message)
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

// NewStorageError creates a new storage error.
func NewStorageError(op, bucket, key, message string, err error) *StorageError {
	return &StorageError{
		Op:      op,
		Bucket:  bucket,
		Key:     key,
		Message: message,
		Err:     err,
	}
}

// AuthError represents authentication/authorization failures.
type AuthError struct {
	Client  string
	Message string
	Err     error
}

func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("auth failed for client %s: %s: %v", e.Client, e.Message, e.Err)
	}
	return fmt.Sprintf("auth failed for client %s: %s", e.Client, e.Message)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// NewAuthError creates a new authentication error.
func NewAuthError(client, message string, err error) *AuthError {
	return &AuthError{
		Client:  client,
		Message: message,
		Err:     err,
	}
}

// CryptoError represents encryption/decryption failures.
type CryptoError struct {
	Op      string // Operation: "encrypt", "decrypt", "derive_key"
	Message string
	Err     error
}

func (e *CryptoError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("crypto error in %s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("crypto error in %s: %s", e.Op, e.Message)
}

func (e *CryptoError) Unwrap() error {
	return e.Err
}

// NewCryptoError creates a new cryptography error.
func NewCryptoError(op, message string, err error) *CryptoError {
	return &CryptoError{
		Op:      op,
		Message: message,
		Err:     err,
	}
}
