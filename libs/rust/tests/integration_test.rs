//! Integration tests for the Gaia client library.
//!
//! Note: These tests require a running Gaia daemon with proper certificates.
//! They are marked with #[ignore] by default to allow CI builds to pass.
//! Run them locally with: cargo test -- --ignored

use gaia_client::{GaiaClient, GaiaClientConfig};

#[tokio::test]
#[ignore] // Requires running Gaia daemon
async fn test_connect_to_daemon() {
    let config = GaiaClientConfig::new(
        "localhost:50051",
        "../../certs/ca.crt",
        "../../certs/gaia_client.crt",
        "../../certs/gaia_client.key",
    );

    let result = GaiaClient::connect(config).await;
    assert!(result.is_ok(), "Failed to connect to daemon");
}

#[tokio::test]
#[ignore] // Requires running Gaia daemon
async fn test_get_status() {
    let config = GaiaClientConfig::new(
        "localhost:50051",
        "../../certs/ca.crt",
        "../../certs/gaia_client.crt",
        "../../certs/gaia_client.key",
    );

    let mut client = GaiaClient::connect(config).await.unwrap();
    let status = client.get_status().await;

    assert!(status.is_ok(), "Failed to get status");
    let status = status.unwrap();
    assert!(
        status.status == "locked" || status.status == "unlocked",
        "Invalid status: {}",
        status.status
    );
}

#[tokio::test]
#[ignore] // Requires running Gaia daemon
async fn test_is_unlocked() {
    let config = GaiaClientConfig::new(
        "localhost:50051",
        "../../certs/ca.crt",
        "../../certs/gaia_client.crt",
        "../../certs/gaia_client.key",
    );

    let mut client = GaiaClient::connect(config).await.unwrap();
    let is_unlocked = client.is_unlocked().await;

    assert!(is_unlocked.is_ok(), "Failed to check unlock status");
}

#[tokio::test]
#[ignore] // Requires running Gaia daemon with unlocked state
async fn test_get_namespaces() {
    let config = GaiaClientConfig::new(
        "localhost:50051",
        "../../certs/ca.crt",
        "../../certs/gaia_client.crt",
        "../../certs/gaia_client.key",
    );

    let mut client = GaiaClient::connect(config).await.unwrap();

    if !client.is_unlocked().await.unwrap() {
        eprintln!("Skipping test: daemon is locked");
        return;
    }

    let namespaces = client.get_namespaces().await;
    assert!(namespaces.is_ok(), "Failed to get namespaces");
}

#[test]
fn test_config_creation() {
    let config = GaiaClientConfig::new(
        "localhost:50051",
        "/etc/gaia/certs/ca.crt",
        "/etc/gaia/certs/client.crt",
        "/etc/gaia/certs/client.key",
    );

    assert_eq!(config.server_address, "localhost:50051");
    assert_eq!(config.domain_name, "gaia");
}

#[test]
fn test_config_with_custom_domain() {
    let config = GaiaClientConfig::new(
        "gaia.example.com:50051",
        "/etc/gaia/certs/ca.crt",
        "/etc/gaia/certs/client.crt",
        "/etc/gaia/certs/client.key",
    )
    .with_domain_name("gaia.example.com");

    assert_eq!(config.domain_name, "gaia.example.com");
}
