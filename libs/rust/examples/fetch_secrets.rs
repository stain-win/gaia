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

    // Ensure daemon is unlocked
    if !client.is_unlocked().await? {
        eprintln!("Error: Daemon is locked. Please unlock it first.");
        return Ok(());
    }

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

    // Get all common secrets
    println!("\nFetching common secrets...");
    let common_secrets = client.get_common_secrets(None).await?;
    for namespace in common_secrets {
        println!("\nNamespace: {}", namespace.name);
        for secret in namespace.secrets {
            println!("  {}: {}", secret.id, secret.value);
        }
    }

    // Get secrets from a specific common namespace
    println!("\nFetching secrets from 'production' common namespace...");
    match client.get_common_namespace_secrets("production").await {
        Ok(secrets) => {
            for secret in secrets {
                println!("  {}: {}", secret.id, secret.value);
            }
        }
        Err(e) => {
            eprintln!("Failed to fetch common namespace secrets: {}", e);
        }
    }

    Ok(())
}
