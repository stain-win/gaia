use thiserror::Error;

/// Result type alias for Gaia client operations.
pub type Result<T> = std::result::Result<T, GaiaError>;

/// Errors that can occur when using the Gaia client.
#[derive(Debug, Error)]
pub enum GaiaError {
    /// Error connecting to the Gaia daemon.
    #[error("failed to connect to Gaia daemon: {0}")]
    ConnectionError(String),

    /// Error loading TLS certificates.
    #[error("failed to load TLS certificates: {0}")]
    TlsError(String),

    /// Error reading certificate or key files.
    #[error("failed to read certificate file: {0}")]
    IoError(#[from] std::io::Error),

    /// gRPC transport error.
    #[error("gRPC transport error: {0}")]
    TransportError(#[from] tonic::transport::Error),

    /// gRPC status error (e.g., not found, permission denied).
    #[error("gRPC status error: {0}")]
    StatusError(Box<tonic::Status>),

    /// Secret not found.
    #[error("secret not found: namespace='{0}', id='{1}'")]
    SecretNotFound(String, String),

    /// Invalid configuration.
    #[error("invalid configuration: {0}")]
    ConfigError(String),

    /// The daemon is locked and cannot serve requests.
    #[error("daemon is locked, unlock it first")]
    DaemonLocked,

    /// The daemon is offline or unreachable.
    #[error("daemon is offline or unreachable")]
    DaemonOffline,
}

impl From<tonic::Status> for GaiaError {
    fn from(status: tonic::Status) -> Self {
        GaiaError::StatusError(Box::new(status))
    }
}

