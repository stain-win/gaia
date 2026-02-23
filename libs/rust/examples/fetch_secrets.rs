use gaia_client::{GaiaClient, GaiaClientConfig};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Create configuration
    let config = GaiaClientConfig::new(
        "localhost:50051",
        "/etc/gaia/certs/ca.crt",
        "/etc/gaia/certs/client.crt",
        "/etc/gaia/certs/client.key",
    );

    // Connect to the Gaia daemon
    let mut client = GaiaClient::connect(config).await?;

    // Note: Implicitly check if daemon is reachable by making a request

    // Get a specific secret
    println!("Fetching secret from production namespace...");
    match client.get_secret("production", "database_url").await {
        Ok(secret) => {
            println!("Secret ID: {}", secret.id);
            println!("Secret Value: {}", secret.value);
        }
        Err(e) => {
            eprintln!("Failed to fetch secret: {}", e);
        }
    }

    Ok(())
}
