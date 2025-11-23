// Build script for gaia-client
//
// By default, this does nothing - the library uses pre-generated protobuf code
// checked into src/proto.rs, eliminating the need for protoc at build time.
//
// To regenerate the proto code (for development), set REGENERATE_PROTO=1:
//   REGENERATE_PROTO=1 cargo build --features regenerate-proto
//
// This requires protoc to be installed.

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Only regenerate proto if explicitly requested
    if std::env::var("REGENERATE_PROTO").is_ok() {
        #[cfg(feature = "regenerate-proto")]
        {
            println!("cargo:warning=Regenerating protobuf code (REGENERATE_PROTO is set)");

            // Compile the proto files from the local proto directory
            // When building locally, we prefer ../../proto if it exists (development)
            // When building from published crate, we use ./proto (included in package)
            let proto_path = if std::path::Path::new("../../proto/gaia-client.proto").exists() {
                "../../proto/gaia-client.proto"
            } else {
                "proto/gaia-client.proto"
            };

            let proto_dir = if std::path::Path::new("../../proto").exists() {
                "../../proto"
            } else {
                "proto"
            };

            tonic_build::configure()
                .build_server(false)
                .compile(&[proto_path], &[proto_dir])?;

            println!("cargo:warning=Protobuf code regenerated in target/*/build/gaia-client-*/out/gaia.rs");
            println!("cargo:warning=Copy it to src/proto.rs: cp target/debug/build/gaia-client-*/out/gaia.rs src/proto.rs");
        }

        #[cfg(not(feature = "regenerate-proto"))]
        {
            println!("cargo:warning=REGENERATE_PROTO is set but 'regenerate-proto' feature is not enabled");
            println!("cargo:warning=Use: REGENERATE_PROTO=1 cargo build --features regenerate-proto");
        }
    } else {
        // Normal build - use pre-generated code in src/proto.rs
        println!("cargo:rerun-if-changed=src/proto.rs");
    }

    Ok(())
}
