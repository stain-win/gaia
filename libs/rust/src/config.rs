use std::path::{Path, PathBuf};

/// Configuration for the Gaia client.
#[derive(Debug, Clone)]
pub struct GaiaClientConfig {
    /// The address of the Gaia daemon (e.g., "localhost:50051").
    pub server_address: String,

    /// Path to the CA certificate file.
    pub ca_cert_path: PathBuf,

    /// Path to the client certificate file.
    pub client_cert_path: PathBuf,

    /// Path to the client private key file.
    pub client_key_path: PathBuf,

    /// Optional domain name for TLS verification (defaults to the server host).
    pub domain_name: String,
}

impl GaiaClientConfig {
    /// Creates a new Gaia client configuration.
    ///
    /// # Arguments
    ///
    /// * `server_address` - The address of the Gaia daemon (e.g., "localhost:50051")
    /// * `ca_cert_path` - Path to the CA certificate file
    /// * `client_cert_path` - Path to the client certificate file
    /// * `client_key_path` - Path to the client private key file
    ///
    /// # Example
    ///
    /// ```
    /// use gaia_client::GaiaClientConfig;
    ///
    /// let config = GaiaClientConfig::new(
    ///     "localhost:50051",
    ///     "/etc/gaia/certs/ca.crt",
    ///     "/etc/gaia/certs/client.crt",
    ///     "/etc/gaia/certs/client.key",
    /// );
    /// ```
    pub fn new<S, P>(
        server_address: S,
        ca_cert_path: P,
        client_cert_path: P,
        client_key_path: P,
    ) -> Self
    where
        S: Into<String>,
        P: AsRef<Path>,
    {
        let server_address = server_address.into();
        let domain_name = server_address
            .split(':')
            .next()
            .unwrap_or("localhost")
            .to_string();
        Self {
            server_address,
            ca_cert_path: ca_cert_path.as_ref().to_path_buf(),
            client_cert_path: client_cert_path.as_ref().to_path_buf(),
            client_key_path: client_key_path.as_ref().to_path_buf(),
            domain_name,
        }
    }

    /// Sets a custom domain name for TLS verification.
    pub fn with_domain_name<S: Into<String>>(mut self, domain_name: S) -> Self {
        self.domain_name = domain_name.into();
        self
    }

    /// Creates a configuration from environment variables.
    ///
    /// Expects the following environment variables:
    /// - `GAIA_SERVER_ADDRESS` (defaults to "localhost:50051")
    /// - `GAIA_CA_CERT` (defaults to "/etc/gaia/certs/ca.crt")
    /// - `GAIA_CLIENT_CERT` (defaults to "/etc/gaia/certs/client.crt")
    /// - `GAIA_CLIENT_KEY` (defaults to "/etc/gaia/certs/client.key")
    ///
    /// # Example
    ///
    /// ```
    /// use gaia_client::GaiaClientConfig;
    ///
    /// let config = GaiaClientConfig::from_env();
    /// ```
    pub fn from_env() -> Self {
        let server_address =
            std::env::var("GAIA_SERVER_ADDRESS").unwrap_or_else(|_| "localhost:50051".to_string());
        let ca_cert_path =
            std::env::var("GAIA_CA_CERT").unwrap_or_else(|_| "/etc/gaia/certs/ca.crt".to_string());
        let client_cert_path = std::env::var("GAIA_CLIENT_CERT")
            .unwrap_or_else(|_| "/etc/gaia/certs/client.crt".to_string());
        let client_key_path = std::env::var("GAIA_CLIENT_KEY")
            .unwrap_or_else(|_| "/etc/gaia/certs/client.key".to_string());

        Self::new(
            server_address,
            ca_cert_path,
            client_cert_path,
            client_key_path,
        )
    }
}
