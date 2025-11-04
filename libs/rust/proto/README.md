# Protocol Buffer Definitions

This directory contains the Protocol Buffer definitions used by the Gaia Rust client library.

## Build Process

The Rust code for these proto files is **automatically generated at build time** by the `build.rs` script in the parent directory.

### How It Works

1. When you run `cargo build`, the `build.rs` script executes
2. It uses `tonic-build` to compile the proto files
   - **Development mode**: Uses `../../proto/gaia-client.proto` (root directory)
   - **Published crate**: Uses `proto/gaia-client.proto` (this directory)
3. Generated Rust code is placed in `target/` directory
4. The library imports the generated code via `tonic::include_proto!("gaia")`

### Why Store Proto Files Here?

- **Reference**: Developers can see what proto definitions the library uses
- **Documentation**: Makes it clear what the gRPC API looks like
- **Versioning**: Proto files are versioned with the library code
- **Publishing**: These files are included in the published crate so it can build standalone

### Development vs Published

- **Development**: The build script prefers `../../proto/` (source of truth in repository)
- **Published**: The build script uses `proto/` (included in the crate package)
- **Automatic**: The `build.rs` script automatically detects which location to use

## Proto File

- `gaia-client.proto` - Defines the GaiaClient service for read-only operations

## Generated Code

The generated code includes:

- `Secret` - Message type for secrets
- `Namespace` - Message type for namespaces
- `GetSecretRequest` - Request for fetching a secret
- `GetCommonSecretsRequest` - Request for common secrets
- `StatusResponse` - Response for daemon status
- `NamespaceResponse` - Response for listing namespaces
- `GaiaClientClient` - gRPC client for communicating with the daemon

## Updating Proto Files

If the proto definitions change:

1. Update the proto file in `../../proto/gaia-client.proto`
2. Copy the updated file here: `cp ../../proto/gaia-client.proto proto/`
3. Run `cargo build` to regenerate the Rust code
4. Update client code if the API changed

## Dependencies

The build process requires:

- `protoc` - Protocol Buffer compiler (install via package manager)
- `tonic-build` - Rust code generator (included in build-dependencies)

