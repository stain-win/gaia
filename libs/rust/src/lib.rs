//! # Gaia Client Library
//!
//! A Rust client library for interacting with the Gaia secret management daemon.
//!
//! ## Features
//!
//! - Secure mTLS communication with the Gaia daemon
//! - Read secrets from your application's namespaces
//! - Access common secrets shared across clients
//! - Asynchronous API using Tokio
//!
//! ## Example
//!
//! ```no_run
//! use gaia_client::{GaiaClient, GaiaClientConfig};
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     let config = GaiaClientConfig::new(
//!         "localhost:50051",
//!         "/etc/gaia/certs/ca.crt",
//!         "/etc/gaia/certs/client.crt",
//!         "/etc/gaia/certs/client.key",
//!     );
//!
//!     let mut client = GaiaClient::connect(config).await?;
//!
//!     let secret = client.get_secret("production", "database_url").await?;
//!     println!("Secret value: {}", secret.value);
//!
//!     Ok(())
//! }
//! ```

// Use pre-generated protobuf code (checked into src/proto.rs)
// This eliminates the need for protoc at build time
#[allow(clippy::all)]
#[allow(warnings)]
pub mod proto {
    include!("proto.rs");
}

mod client;
mod config;
mod error;
mod tls;

pub use client::GaiaClient;
pub use config::GaiaClientConfig;
pub use error::{GaiaError, Result};
pub use proto::{
    gaia_client_client::GaiaClientClient, GetSecretRequest, Namespace, Secret,
};
