//! gaiatest — Test helpers for starting an ephemeral Gaia daemon in Rust tests.
//!
//! **STATUS: Stub** — full implementation coming in a future release.
//! Use the Go `gaiatest` package (`libs/go/gaiatest`) for integration tests today.
//!
//! # Planned API
//!
//! ```rust,ignore
//! use gaia_client::gaiatest::GaiaTestServer;
//!
//! #[tokio::test]
//! async fn test_my_feature() {
//!     let server = GaiaTestServer::start(Default::default()).await.unwrap();
//!     // server.addr  — gRPC address
//!     // server.cert_dir — directory with TLS certs
//!     server.stop().await;
//! }
//! ```
//!
//! # Requirements (when implemented)
//! - `gaia` binary in `PATH` or `GAIA_BIN` env var
//! - Tokio async runtime

/// Options for configuring the test server.
#[derive(Debug, Default)]
pub struct GaiaTestServerOptions {
    /// Path to the gaia binary. Defaults to `GAIA_BIN` env var or PATH lookup.
    pub bin: Option<String>,
    /// Master passphrase for the ephemeral daemon. Defaults to `"gaiatest-dev"`.
    pub passphrase: Option<String>,
}

/// An ephemeral Gaia daemon process for use in tests.
///
/// **Stub** — not yet functional.
pub struct GaiaTestServer {
    /// gRPC address of the running daemon, e.g. `"127.0.0.1:50123"`.
    pub addr: String,
    /// Directory containing `ca.crt`, `gaia_client.crt`, `gaia_client.key`.
    pub cert_dir: String,
}

impl GaiaTestServer {
    /// Start an ephemeral Gaia daemon for testing.
    ///
    /// # Errors
    /// Always returns `Err` until this stub is fully implemented.
    pub async fn start(_opts: GaiaTestServerOptions) -> Result<Self, String> {
        Err(
            "GaiaTestServer::start() is not yet implemented. \
             Use the Go gaiatest package (libs/go/gaiatest) for integration tests."
                .to_string(),
        )
    }

    /// Stop the daemon and clean up temporary files.
    ///
    /// No-op stub.
    pub async fn stop(self) {}
}
