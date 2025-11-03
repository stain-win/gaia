use crate::config::GaiaClientConfig;
use crate::error::{GaiaError, Result};
use crate::proto::gaia_client_client::GaiaClientClient;
use crate::proto::{
    GetCommonSecretsRequest, GetSecretRequest, Namespace, NamespaceResponse, Secret, StatusResponse,
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

    /// Retrieves the daemon status.
    ///
    /// Returns "locked" or "unlocked" depending on the daemon state.
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use gaia_client::{GaiaClient, GaiaClientConfig};
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// # let config = GaiaClientConfig::new("localhost:50051", "ca.crt", "client.crt", "client.key");
    /// # let mut client = GaiaClient::connect(config).await?;
    /// let status = client.get_status().await?;
    /// println!("Daemon status: {}", status.status);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn get_status(&mut self) -> Result<StatusResponse> {
        let request = tonic::Request::new(());
        let response = self.inner.get_status(request).await?;
        Ok(response.into_inner())
    }

    /// Checks if the daemon is unlocked and ready to serve secrets.
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use gaia_client::{GaiaClient, GaiaClientConfig};
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// # let config = GaiaClientConfig::new("localhost:50051", "ca.crt", "client.crt", "client.key");
    /// # let mut client = GaiaClient::connect(config).await?;
    /// if client.is_unlocked().await? {
    ///     println!("Daemon is ready!");
    /// }
    /// # Ok(())
    /// # }
    /// ```
    pub async fn is_unlocked(&mut self) -> Result<bool> {
        match self.get_status().await {
            Ok(status) => Ok(status.status == "unlocked"),
            Err(GaiaError::DaemonOffline) => Ok(false),
            Err(e) => Err(e),
        }
    }

    /// Lists all namespaces accessible to this client.
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use gaia_client::{GaiaClient, GaiaClientConfig};
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// # let config = GaiaClientConfig::new("localhost:50051", "ca.crt", "client.crt", "client.key");
    /// # let mut client = GaiaClient::connect(config).await?;
    /// let namespaces = client.get_namespaces().await?;
    /// for ns in namespaces.namespaces {
    ///     println!("Namespace: {}", ns);
    /// }
    /// # Ok(())
    /// # }
    /// ```
    pub async fn get_namespaces(&mut self) -> Result<NamespaceResponse> {
        let request = tonic::Request::new(());
        let response = self.inner.get_namespaces(request).await?;
        Ok(response.into_inner())
    }

    /// Retrieves all secrets from the common namespace.
    ///
    /// The common namespace contains secrets that are accessible to all clients.
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use gaia_client::{GaiaClient, GaiaClientConfig};
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// # let config = GaiaClientConfig::new("localhost:50051", "ca.crt", "client.crt", "client.key");
    /// # let mut client = GaiaClient::connect(config).await?;
    /// let common_secrets = client.get_common_secrets(None).await?;
    /// for namespace in common_secrets {
    ///     println!("Namespace: {}", namespace.name);
    ///     for secret in namespace.secrets {
    ///         println!("  - {}: {}", secret.id, secret.value);
    ///     }
    /// }
    /// # Ok(())
    /// # }
    /// ```
    pub async fn get_common_secrets(
        &mut self,
        namespace: Option<String>,
    ) -> Result<Vec<Namespace>> {
        let request = tonic::Request::new(GetCommonSecretsRequest { namespace });
        let response = self.inner.get_common_secrets(request).await?;
        Ok(response.into_inner().namespaces)
    }

    /// Retrieves secrets from a specific namespace in the common area.
    ///
    /// # Arguments
    ///
    /// * `namespace` - The namespace to retrieve secrets from
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use gaia_client::{GaiaClient, GaiaClientConfig};
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// # let config = GaiaClientConfig::new("localhost:50051", "ca.crt", "client.crt", "client.key");
    /// # let mut client = GaiaClient::connect(config).await?;
    /// let secrets = client.get_common_namespace_secrets("production").await?;
    /// for secret in secrets {
    ///     println!("{}: {}", secret.id, secret.value);
    /// }
    /// # Ok(())
    /// # }
    /// ```
    pub async fn get_common_namespace_secrets(&mut self, namespace: &str) -> Result<Vec<Secret>> {
        let namespaces = self.get_common_secrets(Some(namespace.to_string())).await?;

        namespaces
            .into_iter()
            .find(|ns| ns.name == namespace)
            .map(|ns| ns.secrets)
            .ok_or_else(|| GaiaError::SecretNotFound("common".to_string(), namespace.to_string()))
    }
}
