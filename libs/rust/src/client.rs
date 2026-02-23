use crate::config::GaiaClientConfig;
use crate::error::Result;
use crate::proto::gaia_client_client::GaiaClientClient;
use crate::proto::{
    GetSecretRequest, Namespace, Secret,
};
use crate::tls::create_tls_channel;
use tonic::transport::Channel;

/// A client for interacting with the Gaia secret management daemon.
///
/// The client uses mutual TLS (mTLS) for secure communication and provides
/// methods to retrieve secrets and check daemon status.
pub struct GaiaClient {
    inner: GaiaClientClient<Channel>,
}

impl GaiaClient {
    /// Connects to the Gaia daemon using the provided configuration.
    ///
    /// # Arguments
    ///
    /// * `config` - Configuration containing server address and TLS certificates
    ///
    /// # Errors
    ///
    /// Returns an error if:
    /// - TLS certificates cannot be loaded
    /// - Connection to the daemon fails
    ///
    /// # Example
    ///
    /// ```no_run
    /// use gaia_client::{GaiaClient, GaiaClientConfig};
    ///
    /// #[tokio::main]
    /// async fn main() -> Result<(), Box<dyn std::error::Error>> {
    ///     let config = GaiaClientConfig::new(
    ///         "localhost:50051",
    ///         "/etc/gaia/certs/ca.crt",
    ///         "/etc/gaia/certs/client.crt",
    ///         "/etc/gaia/certs/client.key",
    ///     );
    ///
    ///     let client = GaiaClient::connect(config).await?;
    ///     Ok(())
    /// }
    /// ```
    pub async fn connect(config: GaiaClientConfig) -> Result<Self> {
        let channel = create_tls_channel(&config).await?;
        let inner = GaiaClientClient::new(channel);
        Ok(Self { inner })
    }

    /// Retrieves a secret from the specified namespace.
    ///
    /// # Arguments
    ///
    /// * `namespace` - The namespace containing the secret
    /// * `id` - The secret identifier
    ///
    /// # Errors
    ///
    /// Returns an error if:
    /// - The daemon is locked
    /// - The secret does not exist
    /// - A network error occurs
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use gaia_client::{GaiaClient, GaiaClientConfig};
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// # let config = GaiaClientConfig::new("localhost:50051", "ca.crt", "client.crt", "client.key");
    /// # let mut client = GaiaClient::connect(config).await?;
    /// let secret = client.get_secret("production", "database_url").await?;
    /// println!("Secret: {}", secret.value);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn get_secret(&mut self, namespace: &str, id: &str) -> Result<Secret> {
        let request = tonic::Request::new(GetSecretRequest {
            namespace: namespace.to_string(),
            id: id.to_string(),
        });

        let response = self.inner.get_secret(request).await?;
        Ok(response.into_inner())
    }









    /// Lists all secrets for the authenticated client.
    ///
    /// Returns secrets from the client's own namespaces plus the common namespace.
    /// If a namespace filter is provided, only secrets from that namespace are returned.
    ///
    /// # Arguments
    ///
    /// * `namespace` - Optional namespace filter
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use gaia_client::{GaiaClient, GaiaClientConfig};
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// # let config = GaiaClientConfig::new("localhost:50051", "ca.crt", "client.crt", "client.key");
    /// # let mut client = GaiaClient::connect(config).await?;
    /// // Get all secrets (client's own + common)
    /// let all_secrets = client.list_secrets(None).await?;
    ///
    /// // Get only secrets from a specific namespace
    /// let prod_secrets = client.list_secrets(Some("production".to_string())).await?;
    ///
    /// for namespace in all_secrets {
    ///     println!("Namespace: {}", namespace.name);
    ///     for secret in namespace.secrets {
    ///         println!("  - {}: {}", secret.id, secret.value);
    ///     }
    /// }
    /// # Ok(())
    /// # }
    /// ```
    pub async fn list_secrets(&mut self, namespace: Option<String>) -> Result<Vec<Namespace>> {
        use crate::proto::ClientListSecretsRequest;

        let request = tonic::Request::new(ClientListSecretsRequest {
            namespace: namespace.unwrap_or_default(),
        });

        let response = self.inner.list_secrets(request).await?;
        Ok(response.into_inner().namespaces)
    }
}
