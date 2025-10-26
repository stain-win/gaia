# PR 2: Error Handling Improvements

## Overview

This PR implements comprehensive error handling improvements across the Gaia codebase, following Go best practices and the guidelines from Effective Go, Go Code Review Comments, and Google's Go Style Guide.

## Changes Made

### 1. New Error Package (`apps/gaia/errors/errors.go`)

Created a dedicated error package with:

#### Sentinel Errors
Exported sentinel errors for common conditions that can be checked with `errors.Is()`:
- `ErrDaemonLocked` - Daemon is in locked state
- `ErrDaemonNotRunning` - Daemon is not running
- `ErrDaemonAlreadyRunning` - Daemon is already running
- `ErrInvalidPassphrase` - Invalid passphrase provided
- `ErrDatabaseExists` - Database file already exists
- `ErrClientNotFound` - Client not found in database
- `ErrClientExists` - Client already exists
- `ErrNamespaceNotFound` - Namespace not found
- `ErrSecretNotFound` - Secret not found
- `ErrBucketNotFound` - Database bucket not found
- `ErrSaltNotFound` - Salt not found in database
- `ErrKeyHashNotFound` - Key hash not found in database
- `ErrReservedName` - Reserved name used
- `ErrNoPeerContext` - No peer context available
- `ErrNotTLS` - Peer auth is not TLS
- `ErrNoPeerCertificates` - No peer certificates found

#### Custom Error Types
Domain-specific error types with context:

**ValidationError**
- Captures field name, value, and detailed message
- Used for input validation failures
- Provides clear feedback about what failed validation

**StorageError**
- Includes operation, bucket, key, and message
- Wraps underlying errors with `Unwrap()`
- Provides context for database operations

**AuthError**
- Captures client name and error context
- Used for authentication/authorization failures
- Supports error wrapping

**CryptoError**
- Includes operation type (encrypt/decrypt/derive_key)
- Provides context for cryptographic failures
- Wraps underlying errors

### 2. Enhanced Validation Package (`apps/gaia/validation/validation.go`)

**Improvements:**
- Uses custom `ValidationError` type instead of generic errors
- Added specific validation functions:
  - `ValidateClient()` - Validates client names
  - `ValidateNamespace()` - Validates namespace names (checks for reserved names)
  - `ValidateKey()` - Validates secret key names
- Error messages follow Go conventions (lowercase, no punctuation)
- Provides detailed context about validation failures

### 3. Enhanced Encrypt Package (`apps/gaia/encrypt/encrypt.go`)

**Improvements:**
- All errors now use `CryptoError` type
- Each operation (encrypt, decrypt, derive_key) is clearly identified
- Better error messages with context:
  - "failed to create AES cipher"
  - "failed to generate nonce"
  - "ciphertext too short"
  - "failed to decrypt data"
- Error wrapping preserves underlying errors

### 4. Daemon Package Updates (`apps/gaia/daemon/daemon.go`)

**Improvements:**
- Replaced all `errors.New()` calls with sentinel errors or wrapped errors
- Consistent error checking for daemon state (locked/unlocked)
- Storage operations use `StorageError` for better context
- Error messages provide operation context:
  - "failed to open database: %w"
  - "failed to read database metadata: %w"
  - "failed to derive key: %w"
  - "failed to load CA credentials: %w"

**Key changes:**
- `Start()`: Uses `ErrDaemonAlreadyRunning`
- `stopDaemon()`: Uses `ErrDaemonNotRunning`
- `InitializeDB()`: Uses `ErrDatabaseExists`
- `UnlockDB()`: Uses `ErrBucketNotFound`, `ErrSaltNotFound`, `ErrKeyHashNotFound`, `ErrInvalidPassphrase`
- `RegisterClient()`: Uses `ErrDaemonLocked`, `ErrReservedName`, `ErrClientExists`
- All CRUD operations: Use `ErrDaemonLocked` consistently
- `findClientByName()`: Uses `ErrClientNotFound` with context
- `GetSecret()`: Uses `ErrSecretNotFound` with path context

### 5. gRPC Service Updates (`apps/gaia/daemon/grpc_service.go`)

**Improvements:**
- Uses gRPC status codes appropriately:
  - `codes.FailedPrecondition` for locked daemon state
  - `codes.InvalidArgument` for validation errors
- Consistent error handling across all RPC methods
- Better client identity errors using sentinel errors:
  - `ErrNoPeerContext`
  - `ErrNotTLS`
  - `ErrNoPeerCertificates`
- All validation now uses the enhanced validation functions
- Error messages follow Go conventions (lowercase)

## Benefits

### 1. **Better Error Checking**
Consumers can now use `errors.Is()` to check for specific error conditions:
```go
if errors.Is(err, gaiaerrors.ErrDaemonLocked) {
    // Handle locked state
}
```

### 2. **Better Error Context**
Error chains preserve context at each level:
```go
failed to unlock database: failed to read database metadata: bucket not found
```

### 3. **Type Safety**
Custom error types can be inspected with `errors.As()`:
```go
var valErr *gaiaerrors.ValidationError
if errors.As(err, &valErr) {
    fmt.Printf("Field %s failed: %s\n", valErr.Field, valErr.Message)
}
```

### 4. **Consistent Error Messages**
- All lowercase (Go convention)
- No trailing punctuation (Go convention)
- Context provided at each level
- Clear operation identification

### 5. **Production Ready**
- Errors provide enough context for debugging
- Security-sensitive errors don't leak information
- Structured error types enable better logging
- Error wrapping preserves stack context

## Testing

- All existing tests pass ✅
- Build completes successfully ✅
- No breaking changes to public APIs ✅

## Code Examples

### Before:
```go
if d.isLocked {
    return errors.New("daemon is in a locked state, cannot add secrets")
}
```

### After:
```go
if d.isLocked {
    return gaiaerrors.ErrDaemonLocked
}
```

### Before:
```go
if err := validation.ValidateName(req.ClientName); err != nil {
    return nil, status.Errorf(codes.InvalidArgument, "invalid client name: %v", err)
}
```

### After:
```go
if err := validation.ValidateClient(req.ClientName); err != nil {
    return nil, status.Errorf(codes.InvalidArgument, "%v", err)
}
```

## Migration Guide

For code that checks errors, update to use `errors.Is()`:

```go
// Old
if err != nil && err.Error() == "daemon is locked" {
    // ...
}

// New
if errors.Is(err, gaiaerrors.ErrDaemonLocked) {
    // ...
}
```

## Future Enhancements

1. Add error tests to verify error types and messages
2. Add metrics/logging for different error types
3. Create client-facing error documentation
4. Consider adding error codes for API responses

## Compliance

This PR follows:
- ✅ [Effective Go](https://go.dev/doc/effective_go) error handling guidelines
- ✅ [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) error conventions
- ✅ [Google's Go Style Guide](https://google.github.io/styleguide/go/) error practices
- ✅ Gaia project coding standards

## Files Changed

1. **New Files:**
   - `apps/gaia/errors/errors.go` - Custom error types and sentinel errors

2. **Modified Files:**
   - `apps/gaia/validation/validation.go` - Enhanced validation with custom errors
   - `apps/gaia/encrypt/encrypt.go` - Crypto errors with context
   - `apps/gaia/daemon/daemon.go` - Comprehensive error handling improvements
   - `apps/gaia/daemon/grpc_service.go` - gRPC error handling with status codes

## Summary

This PR significantly improves error handling throughout the Gaia codebase, making it more maintainable, debuggable, and production-ready. All errors now provide clear context, follow Go conventions, and can be properly inspected by consumers of the API.

