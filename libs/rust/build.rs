fn main() -> Result<(), Box<dyn std::error::Error>> {
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
    Ok(())
}
