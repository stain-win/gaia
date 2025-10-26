# Error Handling Quick Reference Guide

## When to Use Which Error Type

### Sentinel Errors (use with `errors.Is()`)

Use sentinel errors for **known, expected error conditions** that calling code might need to check:

```go
import gaiaerrors "github.com/stain-win/gaia/apps/gaia/errors"

// Check for specific error
if errors.Is(err, gaiaerrors.ErrDaemonLocked) {
    return status.Error(codes.FailedPrecondition, err.Error())
}

// Return sentinel error
if d.isLocked {
    return gaiaerrors.ErrDaemonLocked
}
```

**Available Sentinel Errors:**
- `ErrDaemonLocked` - Operations requiring unlocked daemon
- `ErrDaemonNotRunning` - Operations requiring running daemon
- `ErrDaemonAlreadyRunning` - Starting already running daemon
- `ErrInvalidPassphrase` - Wrong passphrase provided
- `ErrDatabaseExists` - Database already initialized
- `ErrClientNotFound` - Client lookup failed
- `ErrClientExists` - Duplicate client registration
- `ErrSecretNotFound` - Secret lookup failed
- `ErrBucketNotFound` - Database bucket missing
- `ErrSaltNotFound` - Salt missing from DB
- `ErrKeyHashNotFound` - Key hash missing from DB
- `ErrReservedName` - Reserved name used
- `ErrNoPeerContext` - No peer in context
- `ErrNotTLS` - Auth is not TLS
- `ErrNoPeerCertificates` - No certs found

### Custom Error Types (use with `errors.As()`)

Use custom error types when you need to **inspect error details**:

#### ValidationError

```go
// Creating validation error
return gaiaerrors.NewValidationError(
    "client_name",
    name,
    "must be 1-63 characters, start/end with letter/number",
)

// Checking validation error
var valErr *gaiaerrors.ValidationError
if errors.As(err, &valErr) {
    log.Printf("Field %s with value '%s' failed: %s", 
        valErr.Field, valErr.Value, valErr.Message)
}
```

#### StorageError

```go
// Creating storage error
return gaiaerrors.NewStorageError(
    "getSecret",        // operation
    "secrets",          // bucket
    "client/ns/key",    // key
    "not found",        // message
    nil,                // underlying error
)

// Checking storage error
var storageErr *gaiaerrors.StorageError
if errors.As(err, &storageErr) {
    log.Printf("Storage op %s failed on bucket %s", 
        storageErr.Op, storageErr.Bucket)
}
```

#### CryptoError

```go
// Creating crypto error
return gaiaerrors.NewCryptoError(
    "encrypt",                    // operation
    "failed to create AES cipher", // message
    err,                          // underlying error
)

// Checking crypto error
var cryptoErr *gaiaerrors.CryptoError
if errors.As(err, &cryptoErr) {
    log.Printf("Crypto op %s failed: %s", 
        cryptoErr.Op, cryptoErr.Message)
}
```

#### AuthError

```go
// Creating auth error
return gaiaerrors.NewAuthError(
    clientName,              // client
    "certificate revoked",   // message
    err,                     // underlying error
)

// Checking auth error
var authErr *gaiaerrors.AuthError
if errors.As(err, &authErr) {
    log.Printf("Auth failed for client %s: %s", 
        authErr.Client, authErr.Message)
}
```

### Error Wrapping (use with `fmt.Errorf` and `%w`)

Always wrap errors to provide context as they bubble up:

```go
// Good - provides context at each level
if err := d.openDB(); err != nil {
    return fmt.Errorf("failed to open database: %w", err)
}

// Bad - loses context
if err := d.openDB(); err != nil {
    return err
}

// Bad - breaks error chain
if err := d.openDB(); err != nil {
    return fmt.Errorf("failed to open database: %v", err)
}
```

### Validation Pattern

```go
// Use specific validation functions
if err := validation.ValidateClient(name); err != nil {
    return fmt.Errorf("invalid client: %w", err)
}

if err := validation.ValidateNamespace(ns); err != nil {
    return fmt.Errorf("invalid namespace: %w", err)
}

if err := validation.ValidateKey(key); err != nil {
    return fmt.Errorf("invalid key: %w", err)
}
```

### gRPC Error Handling

Map errors to appropriate gRPC status codes:

```go
// Precondition failed (e.g., daemon locked)
if errors.Is(err, gaiaerrors.ErrDaemonLocked) {
    return nil, status.Error(codes.FailedPrecondition, err.Error())
}

// Invalid argument (e.g., validation error)
if err := validation.ValidateClient(req.ClientName); err != nil {
    return nil, status.Errorf(codes.InvalidArgument, "%v", err)
}

// Not found
if errors.Is(err, gaiaerrors.ErrClientNotFound) {
    return nil, status.Error(codes.NotFound, err.Error())
}

// Already exists
if errors.Is(err, gaiaerrors.ErrClientExists) {
    return nil, status.Error(codes.AlreadyExists, err.Error())
}

// Generic error
if err != nil {
    return nil, status.Errorf(codes.Internal, "internal error: %v", err)
}
```

## Error Message Conventions

Follow Go conventions for error messages:

✅ **Good:**
```go
errors.New("daemon is locked")
fmt.Errorf("failed to open database: %w", err)
fmt.Errorf("client not found: %s", name)
```

❌ **Bad:**
```go
errors.New("Daemon Is Locked")  // Don't capitalize
errors.New("daemon is locked.") // Don't add punctuation
errors.New("Error: daemon is locked") // Don't use "Error:" prefix
```

## Common Patterns

### Check and Return Sentinel Error

```go
if d.isLocked {
    return gaiaerrors.ErrDaemonLocked
}
```

### Wrap with Context

```go
if err := someOperation(); err != nil {
    return fmt.Errorf("failed to perform operation: %w", err)
}
```

### Sentinel + Context

```go
if clientNotFound {
    return fmt.Errorf("%w: '%s'", gaiaerrors.ErrClientNotFound, clientName)
}
```

### Custom Error with Wrapped Error

```go
return gaiaerrors.NewStorageError(
    "getSecret",
    "secrets",
    key,
    "bucket not found",
    gaiaerrors.ErrBucketNotFound,
)
```

## Testing Error Conditions

```go
func TestSomeFunction(t *testing.T) {
    err := someFunction()
    
    // Check for sentinel error
    if !errors.Is(err, gaiaerrors.ErrDaemonLocked) {
        t.Errorf("expected ErrDaemonLocked, got: %v", err)
    }
    
    // Check for custom error type
    var valErr *gaiaerrors.ValidationError
    if !errors.As(err, &valErr) {
        t.Errorf("expected ValidationError, got: %v", err)
    }
    
    // Check error message
    if !strings.Contains(err.Error(), "expected text") {
        t.Errorf("error message doesn't contain expected text: %v", err)
    }
}
```

## Summary

1. **Sentinel Errors**: Known conditions, use `errors.Is()`
2. **Custom Types**: Need details, use `errors.As()`
3. **Error Wrapping**: Always add context with `%w`
4. **Lowercase**: Error messages are lowercase
5. **No Punctuation**: Don't end with period
6. **gRPC Codes**: Map to appropriate status codes
7. **Context**: Each level adds context to the error

## References

- [Effective Go - Errors](https://go.dev/doc/effective_go#errors)
- [Go Blog - Error Handling](https://go.dev/blog/error-handling-and-go)
- [Go Blog - Working with Errors](https://go.dev/blog/go1.13-errors)

