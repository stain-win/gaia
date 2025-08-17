# Gaia Technical Documentation & Specification
## 1. Project Overview
   Gaia is a secure, cross-platform daemon and command-line interface (CLI) for managing runtime secrets and credentials.
   It is designed to run on a single server, providing secure access to sensitive data for co-located web applications.
   Gaia's architecture is built on a single Go executable, combining a long-running gRPC daemon with a user-friendly terminal UI (TUI) for administrative tasks.

## 2. Architecture & Design Principles
   ### Single Executable
   The entire Gaia application—daemon, CLI, and TUI—is compiled into a single binary.
   This simplifies distribution, installation, and upgrades.

   ### Client-Daemon Model
   Gaia operates as a client-daemon system. The daemon (`gaiad`) is the long-running core service, 
   while the CLI (`gaia`) acts as a gRPC client to manage it.
   This clear separation of concerns ensures that the daemon can run independently and securely in the background.

   ### Asynchronous Operations
   The application's UI is built with the `bubbletea` framework, which uses an asynchronous, message-passing model.
   This prevents the UI from freezing during long-running operations, such as making gRPC calls to the daemon.

## 3. Security Model
   Gaia's security is its highest priority and is implemented in several layers:

   ### Master Passphrase
   The master passphrase is the central key to all administrative access. It is never stored on disk.
   It is provided by a user during an administrative session to derive the decryption key, which is held in memory for a limited time and then wiped.

   ### Encrypted Persistence
   All sensitive data is encrypted at rest using AES-256-GCM before being stored in the BoltDB file (`gaia.db`).
   The encryption key is derived from the master passphrase using a strong key derivation function like `scrypt`.

   ### Mutual TLS (mTLS)
   All gRPC communication between clients and the daemon is secured with mTLS. This ensures:

   - Encrypted Traffic: All data in transit is encrypted, preventing eavesdropping.

   - Mutual Authentication: Both the client and the server must present valid certificates signed by a trusted Root CA. This prevents unauthorized applications from communicating with the daemon.

   ### Locked/Unlocked State
   The daemon operates in two states:

   - **Locked (Default)**: The daemon is running and serving read-only requests. Its decryption key is not in memory, and it cannot perform administrative actions.

   - **Unlocked**: The daemon is in an administrative session. The decryption key is in memory, allowing for read, write, and edit operations. This state is triggered by a successful unlock command from an authorized user.

## 4. Configuration
   Gaia's configuration is flexible and is loaded in a clear hierarchy:

   1. **Configuration File**: The daemon first reads from a YAML configuration file. The file's location is OS-specific (`/etc/gaia/` on Linux, `~/Library/Application Support/Gaia/` on macOS).
   2. **Command-Line Flags**: Values provided via CLI flags (e.g., `--port 50052`) override any values set in the configuration file.

   The `gaia init` command, when run for the first time, automatically generates a default configuration file.

## 5. gRPC Services
   The application uses two distinct gRPC services to enforce the principle of least privilege:

   ### ```GaiaAdmin``` Service
   This service is used exclusively by the gaia CLI and requires the daemon to be in an unlocked state.

   - ```AddSecret(AddSecretRequest)```: Adds a new secret to a specified namespace.

   - ```ListSecrets(ListSecretsRequest)```: Returns a list of all secrets in a given namespace.

   - ```UpdateSecret(UpdateSecretRequest)```: Modifies an existing secret.

   - ```DeleteSecret(DeleteSecretRequest)```: Deletes a secret.

   - ```GetStatus(GetStatusRequest)```: Returns the daemon's current operational status.

   - ```Stop(StopRequest)```: Gracefully shuts down the daemon.

   - ```Unlock(UnlockRequest)```: Unlocks the daemon for administrative tasks.

   ### ```GaiaClient``` Service
   This service is for client applications and is available even when the daemon is in a locked state.

   - ```GetSecret(GetSecretRequest)```: Retrieves a secret by its namespaced key.

   - ```ListNamespaces(ListNamespacesRequest)```: Returns a list of all available namespaces.

## 6. CLI Commands & TUI
   ### Command-Line Interface (```gaia```)
   The CLI is built with cobra and handles daemon lifecycle management and configuration.

   - `gaia init`: Initializes Gaia's encrypted database and configuration file.

   - `gaia start`: Starts the daemon as a foreground process.

   - `gaia stop`: Sends a gRPC request to stop the running daemon.

   - `gaia status`: Sends a gRPC request to get the daemon's status.

   - `gaia certs generate`: Generates new mTLS certificates for clients.

   - `gaia`: Runs the interactive TUI for administrative tasks.

   ### Terminal UI (TUI)
   The TUI, built with `bubbletea`, provides an interactive, full-screen menu for managing data and certificates. It makes gRPC calls to the daemon to perform all actions.

   ### Data Management
   This screen provides options to:

   - **Add New Record**: A form for adding a new secret to a selected namespace.

   - **List All Records**: A list of all secrets in a given namespace.

   - **Back**: Returns to the main menu.

   ### Certificate Management
   This screen provides options to:

   - **Register Client**: Registers a new client name, creating a new top-level namespace.

   - **Create New Certificates**: A form for generating a client's mTLS certificate, which is a key part of the secure key exchange process.

   - **List Existing Certificates**: A list of all clients and their certificate status.

   - **Back**: Returns to the main menu.

## 7. Next Steps
   This document provides a complete and detailed specification for Gaia.
   The next steps will involve implementing the remaining features outlined in this document, such as the `ListSecrets` and `UpdateSecret` gRPC methods, 
   the client registration process, and the logic to handle the `gaia unlock `command
