use gaia_client::{GaiaClient, GaiaClientConfig};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Create configuration from environment variables or use defaults
    let config = GaiaClientConfig::new(
        "localhost:50051",
        "/etc/gaia/certs/ca.crt",
        "/etc/gaia/certs/client.crt",
        "/etc/gaia/certs/client.key",
    );

    // Connect to the Gaia daemon
    let mut client = GaiaClient::connect(config).await?;

    // List available namespaces
    let namespaces = client.list_secrets(None).await?;
    println!("\nAvailable namespaces:");
    for ns in namespaces {
        println!("  - {}", ns.name);
    }

    Ok(())
}
