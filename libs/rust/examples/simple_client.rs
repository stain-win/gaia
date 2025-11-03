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

    // Check daemon status
    let status = client.get_status().await?;
    println!("Daemon status: {}", status.status);

    if !client.is_unlocked().await? {
        eprintln!("Error: Daemon is locked. Please unlock it first.");
        return Ok(());
    }

    // List available namespaces
    let namespaces = client.get_namespaces().await?;
    println!("\nAvailable namespaces:");
    for ns in namespaces.namespaces {
        println!("  - {}", ns);
    }

    Ok(())
}
