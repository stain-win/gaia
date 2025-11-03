# Protocol Buffer Definitions

This directory contains the Protocol Buffer definitions used by the Gaia Rust client library.

## Build Process

The Rust code for these proto files is **automatically generated at build time** by the `build.rs` script in the parent directory.

### How It Works

1. When you run `cargo build`, the `build.rs` script executes
2. It uses `tonic-build` to compile the proto files from `../../proto/gaia-client.proto`
3. Generated Rust code is placed in `target/` directory
4. The library imports the generated code via `tonic::include_proto!("gaia")`

### Why Store Proto Files Here?

- **Reference**: Developers can see what proto definitions the library uses
- **Documentation**: Makes it clear what the gRPC API looks like
- **Versioning**: Proto files are versioned with the library code

### Note

The **source of truth** for proto files is in the root `../../proto/` directory. The files here are copies for reference. The build process always uses the files from the root proto directory.

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

