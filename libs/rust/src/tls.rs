use crate::config::GaiaClientConfig;
use crate::error::{GaiaError, Result};
use tonic::transport::{Certificate, Channel, ClientTlsConfig, Identity};

/// Loads TLS configuration for mTLS communication with the Gaia daemon.
pub(crate) async fn create_tls_channel(config: &GaiaClientConfig) -> Result<Channel> {
    // Load CA certificate
    let ca_cert = load_ca_cert(&config.ca_cert_path)?;

    // Load client certificate and key
    let client_identity = load_client_identity(&config.client_cert_path, &config.client_key_path)?;

    // Create TLS configuration
    let tls_config = ClientTlsConfig::new()
        .ca_certificate(ca_cert)
        .identity(client_identity)
        .domain_name(&config.domain_name);

    // Create the channel with TLS
    let channel = Channel::from_shared(format!("https://{}", config.server_address))
        .map_err(|e| GaiaError::ConnectionError(e.to_string()))?
        .tls_config(tls_config)
        .map_err(|e| GaiaError::TlsError(e.to_string()))?
        .connect()
        .await?;

    Ok(channel)
}

/// Loads the CA certificate from the specified path.
fn load_ca_cert(path: &std::path::Path) -> Result<Certificate> {
    let cert_pem = std::fs::read(path)?;
    Ok(Certificate::from_pem(cert_pem))
}

/// Loads the client certificate and private key from the specified paths.
fn load_client_identity(
    cert_path: &std::path::Path,
    key_path: &std::path::Path,
) -> Result<Identity> {
    let cert_pem = std::fs::read(cert_path)?;
    let key_pem = std::fs::read(key_path)?;
    Ok(Identity::from_pem(cert_pem, key_pem))
}
