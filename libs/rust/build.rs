fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Compile the proto files
    tonic_build::configure()
        .build_server(false)
        .compile(&["../../proto/gaia-client.proto"], &["../../proto"])?;
    Ok(())
}
