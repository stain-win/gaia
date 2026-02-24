use crate::config::GaiaClientConfig;
use crate::error::Result;
use crate::proto::gaia_client_client::GaiaClientClient;
use crate::proto::{
    GetSecretRequest, Namespace, Secret,
};
use crate::tls::create_tls_channel;
use tonic::transport::Channel;

/// Options for loading secrets into the environment.
#[derive(Debug, Clone, Default)]
pub struct LoadEnvOptions {
    /// Prefix to prepend to environment variable names.
    /// If strictly empty or None, no prefix is added.
    pub prefix: Option<String>,
    
    /// Whether to include the namespace in the environment variable name.
    pub use_namespace: bool,
}

impl LoadEnvOptions {
    /// Create new options with default values (key only).
    pub fn new() -> Self {
        Self::default()
    }

    /// Set the prefix.
    pub fn with_prefix(mut self, prefix: impl Into<String>) -> Self {
        self.prefix = Some(prefix.into());
        self
    }

    /// Set whether to use the namespace.
    pub fn with_namespace(mut self, use_namespace: bool) -> Self {
        self.use_namespace = use_namespace;
        self
    }
}

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

    /// Fetches all accessible secrets and loads them into the current process's environment.
    ///
    /// By default, environment variables are named after the secret key, converted to uppercase
    /// with hyphens replaced by underscores. Optional prefix and namespace inclusion can be
    /// configured via `LoadEnvOptions`.
    ///
    /// # Arguments
    ///
    /// * `options` - Optional configuration for env var naming
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use gaia_client::{GaiaClient, GaiaClientConfig, LoadEnvOptions};
    /// # #[tokio::main]
    /// # async fn main() -> Result<(), Box<dyn std::error::Error>> {
    /// # let config = GaiaClientConfig::new("localhost:50051", "ca.crt", "client.crt", "client.key");
    /// # let mut client = GaiaClient::connect(config).await?;
    /// // Load with default behavior (key only)
    /// client.load_env(None).await?;
    ///
    /// // Load with prefix and namespace: GAIA_PRODUCTION_DATABASE_URL
    /// client.load_env(Some(LoadEnvOptions::new().with_prefix("GAIA").with_namespace(true))).await?;
    /// # Ok(())
    /// # }
    /// ```
    pub async fn load_env(&mut self, options: Option<LoadEnvOptions>) -> Result<()> {
        let opts = options.unwrap_or_default();
        let namespaces = self.list_secrets(None).await?;

        for ns in namespaces {
            for secret in ns.secrets {
                let mut parts = Vec::new();

                if let Some(prefix) = &opts.prefix {
                    if !prefix.is_empty() {
                        parts.push(prefix.to_uppercase().replace("-", "_"));
                    }
                }

                if opts.use_namespace {
                    parts.push(ns.name.to_uppercase().replace("-", "_"));
                }

                parts.push(secret.id.to_uppercase().replace("-", "_"));

                let env_var_name = parts.join("_");
                std::env::set_var(env_var_name, secret.value);
            }
        }

        Ok(())
    }
}
