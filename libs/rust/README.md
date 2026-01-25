# Gaia Client Library (Rust)

A Rust client library for interacting with the [Gaia](https://github.com/stain-win/gaia) secret management daemon.

## Features

- 🔒 **Secure mTLS Communication** - All communication is encrypted and authenticated using mutual TLS
- 🚀 **Async/Await Support** - Built on Tokio for high-performance async operations
- 📦 **Easy to Use** - Simple, idiomatic Rust API
- 🔑 **Secret Management** - Read secrets from your application's namespaces
- 🌐 **Common Secrets** - Access shared secrets available to all clients
- ⚡ **gRPC Based** - Fast, efficient communication protocol
- 🛠️ **No Build Tools Required** - Ships with pre-generated protobuf code, no need for `protoc`

## Installation

Add this to your `Cargo.toml`:

```toml
[dependencies]
gaia-client = "0.1"
tokio = { version = "1.0", features = ["full"] }
```

**No additional build tools required!** The library ships with pre-generated protobuf code, so you don't need to install `protoc` or any other external dependencies.

## Quick Start

```rust
use gaia_client::{GaiaClient, GaiaClientConfig};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Configure the client
    let config = GaiaClientConfig::new(
        "localhost:50051",
        "/etc/gaia/certs/ca.crt",
        "/etc/gaia/certs/client.crt",
        "/etc/gaia/certs/client.key",
    );

    // Connect to the Gaia daemon
    let mut client = GaiaClient::connect(config).await?;

    // Check if daemon is ready
    if !client.is_unlocked().await? {
        eprintln!("Daemon is locked. Please unlock it first.");
        return Ok(());
    }

    // Fetch a secret
    let secret = client.get_secret("production", "database_url").await?;
    println!("Database URL: {}", secret.value);

    Ok(())
}
```

## Configuration

### From Code

```rust
use gaia_client::GaiaClientConfig;

let config = GaiaClientConfig::new(
    "localhost:50051",              // Server address
    "/etc/gaia/certs/ca.crt",       // CA certificate
    "/etc/gaia/certs/client.crt",   // Client certificate
    "/etc/gaia/certs/client.key",   // Client private key
);
```

### From Environment Variables

```rust
use gaia_client::GaiaClientConfig;

// Reads from:
// - GAIA_SERVER_ADDRESS (default: "localhost:50051")
// - GAIA_CA_CERT (default: "/etc/gaia/certs/ca.crt")
// - GAIA_CLIENT_CERT (default: "/etc/gaia/certs/client.crt")
// - GAIA_CLIENT_KEY (default: "/etc/gaia/certs/client.key")
let config = GaiaClientConfig::from_env();
```

### Custom Domain Name

```rust
let config = GaiaClientConfig::new(
    "gaia.example.com:50051",
    "/etc/gaia/certs/ca.crt",
    "/etc/gaia/certs/client.crt",
    "/etc/gaia/certs/client.key",
).with_domain_name("gaia.example.com");
```

## API Reference

### Client Methods

#### `connect(config: GaiaClientConfig) -> Result<GaiaClient>`

Connects to the Gaia daemon using the provided configuration.

```rust
let mut client = GaiaClient::connect(config).await?;
```

#### `get_status() -> Result<StatusResponse>`

Retrieves the daemon status ("locked" or "unlocked").

```rust
let status = client.get_status().await?;
println!("Status: {}", status.status);
```

#### `is_unlocked() -> Result<bool>`

Checks if the daemon is unlocked and ready to serve secrets.

```rust
if client.is_unlocked().await? {
    println!("Daemon is ready!");
}
```

#### `get_secret(namespace: &str, id: &str) -> Result<Secret>`

Retrieves a specific secret from a namespace.

```rust
let secret = client.get_secret("production", "database_url").await?;
println!("Secret: {}", secret.value);
```

#### `get_namespaces() -> Result<NamespaceResponse>`

Lists all namespaces accessible to this client.

```rust
let namespaces = client.get_namespaces().await?;
for ns in namespaces.namespaces {
    println!("Namespace: {}", ns);
}
```

#### `get_common_secrets(namespace: Option<String>) -> Result<Vec<Namespace>>`

Retrieves all secrets from the common area, optionally filtered by namespace.

```rust
// Get all common secrets
let all_common = client.get_common_secrets(None).await?;

// Get common secrets from specific namespace
let production_common = client.get_common_secrets(Some("production".to_string())).await?;
```

#### `get_common_namespace_secrets(namespace: &str) -> Result<Vec<Secret>>`

Retrieves secrets from a specific namespace in the common area.

```rust
let secrets = client.get_common_namespace_secrets("production").await?;
for secret in secrets {
    println!("{}: {}", secret.id, secret.value);
}
```

#### `list_secrets(namespace: Option<String>) -> Result<Vec<Namespace>>` ⭐ NEW

Lists all secrets for the authenticated client, including both the client's own namespaces and the common namespace.

```rust
// Get all secrets (client's own + common)
let all_secrets = client.list_secrets(None).await?;
for namespace in &all_secrets {
    println!("Namespace: {}", namespace.name);
    for secret in &namespace.secrets {
        println!("  - {}: {}", secret.id, secret.value);
    }
}

// Get secrets from specific namespace only
let prod_secrets = client.list_secrets(Some("production".to_string())).await?;
```

**Note:** This is the recommended method for fetching all secrets at once, as it's more efficient than making multiple individual requests.

## Examples

The library includes several examples in the `examples/` directory:

### Simple Client

Demonstrates basic connection and status checking:

```bash
cargo run --example simple_client
```

### Fetch Secrets

Shows how to retrieve secrets from different namespaces:

```bash
cargo run --example fetch_secrets
```

## Error Handling

The library provides a comprehensive error type:

```rust
use gaia_client::GaiaError;

match client.get_secret("production", "api_key").await {
    Ok(secret) => println!("API Key: {}", secret.value),
    Err(GaiaError::DaemonLocked) => eprintln!("Daemon is locked"),
    Err(GaiaError::SecretNotFound(ns, id)) => eprintln!("Secret {}/{} not found", ns, id),
    Err(GaiaError::DaemonOffline) => eprintln!("Daemon is offline"),
    Err(e) => eprintln!("Error: {}", e),
}
```

## Certificate Setup

Before using the client, you need to:

1. **Generate or obtain certificates** from your Gaia administrator
2. **Place certificates** in the appropriate directory (e.g., `/etc/gaia/certs/`)
3. **Ensure permissions** are correct (certificates should be readable by your application)

Example certificate structure:
```
/etc/gaia/certs/
├── ca.crt           # CA certificate (verifies server)
├── client.crt       # Client certificate (authenticates your app)
└── client.key       # Client private key
```

## Platform Support

The library works on:
- ✅ Linux
- ✅ macOS
- ✅ Windows

## Requirements

- Rust 1.70 or later
- Tokio runtime
- Valid mTLS certificates from Gaia

**No external build tools required!** The library ships with pre-generated protobuf code.

## Building from Source

The library uses Protocol Buffers for gRPC communication. The proto files are compiled automatically at build time.

### Quick Build

```bash
cd libs/rust
cargo build --release
```

### Using Makefile

A Makefile is provided for common tasks:

```bash
# See all available commands
make help

# Build the library
make build

# Run tests
make test

# Run all quality checks (format, clippy, test)
make all

# Format code
make fmt

# Run clippy lints
make clippy

# Generate documentation
make doc

# Clean build artifacts
make clean
```

### Proto Files

The library uses gRPC with Protocol Buffers. The proto definitions are located in `../../proto/gaia-client.proto` (root of the repository).

At build time, the `build.rs` script automatically:
1. Reads the proto file from `../../proto/gaia-client.proto`
2. Compiles it using `tonic-build`
## Development

### Protocol Buffers

This library uses **pre-generated protobuf code** checked into `src/proto.rs`, eliminating the need for `protoc` at build time. This makes the library easy to use as a dependency - users don't need to install any external tools.

#### For Library Users

Just add the dependency to your `Cargo.toml` and start using it. No `protoc` installation required!

#### For Library Developers

If you need to regenerate the protobuf code (e.g., after updating the proto files):

1. Install `protoc` (Protocol Buffers compiler):
   ```bash
   # macOS
   brew install protobuf
   
   # Ubuntu/Debian
   sudo apt install protobuf-compiler
   
   # Or download from: https://github.com/protocolbuffers/protobuf/releases
   ```

2. Regenerate the proto code:
   ```bash
   REGENERATE_PROTO=1 cargo build --features regenerate-proto
   ```

3. Copy the generated code to source:
   ```bash
   cp target/debug/build/gaia-client-*/out/gaia.rs src/proto.rs
   ```

4. Commit the updated `src/proto.rs` file

**Note**: The proto files are kept in sync with the main Gaia repository at `../../proto/gaia-client.proto` during development, and copied to `proto/` when publishing.

### Development Workflow

```bash
# Check if everything is set up correctly
make setup

# Development cycle
make dev        # Quick format and check

# Before committing
make all        # Full quality check

# Run examples (requires running Gaia daemon)
make example-simple
make example-fetch
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](../../LICENSE) file for details.

## Related Projects

- [Gaia](https://github.com/stain-win/gaia) - The main Gaia secret management daemon
- [Gaia Go Client](../go) - Go client library
- [Gaia JS Client](../js) - JavaScript/TypeScript client library

## Support

For issues, questions, or contributions, please visit the [GitHub repository](https://github.com/stain-win/gaia).

