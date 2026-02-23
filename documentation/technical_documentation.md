# Gaia Technical Documentation & Specification

## 1. Project Overview

Gaia is a secure, self-hosted, lightweight secret management daemon written in Go. It is designed to run on a single server, providing secure access to runtime secrets and credentials for co-located web applications. The architecture is built around a single Go binary that bundles a long-running gRPC daemon, a full CLI for scripting and administration, and an interactive terminal UI (TUI) for human-driven management tasks.

## 2. Architecture & Design Principles

### Single Executable
The entire Gaia application—daemon, CLI, and TUI—is compiled into a single binary. This simplifies distribution, installation, and upgrades. Version information (semantic version, git commit, build date) is injected at build time via linker flags.

### Client-Daemon Model
Gaia operates as a client-daemon system. The `gaiad` process (started with `gaia start`) is the long-running core service. The CLI and TUI both act as gRPC clients that connect to it over mTLS. This clear separation allows the daemon to run securely in the background as a service while clients interact with it on demand.

### Data Hierarchy
The logical data structure is a three-level hierarchy:

```
Client → Namespace → Record (key/value pair)
```

Secrets are stored in BoltDB. The storage key is a composite formed by joining the client name, namespace, and record ID with a **null byte (`\x00`) delimiter** — a binary separator that cannot appear in user input. Example:

```
client-app-a\x00production\x00database_url
```

**Naming rules:** All user-provided names (client, namespace, key) must consist only of lowercase letters, numbers, hyphens, and underscores, and must start and end with a letter or number.

**`common` namespace:** A special, reserved namespace. Any authenticated client can read secrets stored under the reserved `common` client name, regardless of their own identity.

### Asynchronous UI
The TUI is built with the `bubbletea` framework, which uses an asynchronous, message-passing model. This prevents the UI from blocking during gRPC calls to the daemon.

---

## 3. Security Model

### Master Passphrase & Encryption at Rest
The master passphrase is never stored on disk. It is provided interactively to derive an AES-256-GCM encryption key using `scrypt`. A hash of the derived key is stored to validate unlock attempts without exposing the key itself. All secrets are encrypted before being written to `gaia.db` (BoltDB).

### Mutual TLS (mTLS)
All gRPC communication between clients and the daemon is secured with mutual TLS:

- **Encrypted Traffic:** All data in transit is encrypted.
- **Mutual Authentication:** Both client and server must present valid certificates signed by Gaia's internal Certificate Authority (CA). Unauthorized processes cannot communicate with the daemon.
- **Client Identity:** A client's identity is derived directly from the **Common Name (CN)** in their mTLS certificate.

Supported key algorithms: `ECDSA` (P256, P384, P521) and `RSA` (2048, 3072, 4096 bits). Certificate expiry is configurable; default is 365 days.

### Locked / Unlocked State
The daemon operates in two states:

- **Locked (Default):** The daemon is running but the master decryption key is **not** in memory. Secret-serving RPCs are unavailable until the daemon is unlocked.
- **Unlocked:** The master decryption key has been loaded into memory following a successful `unlock` command. Administrative operations (add, delete, import, export) are available. The daemon can be returned to locked state at any time with `gaia lock`.

### Authorization Policies
Fine-grained access control is enforced via per-client policies. A policy consists of one or more rules, each specifying:

- **Path**: A resource pattern (e.g., `common/*`, `myapp/production/*`)
- **Capabilities**: One or more of `read`, `write`, `delete`, `list`
- **Description**: An optional human-readable description of the rule

Clients without a policy are denied access. Policies are managed via the `gaia policy` command group and stored in the daemon's BoltDB database.

### Audit Logging
An optional audit logging system records all gRPC operations. Three backend types are supported:

| Backend    | Description                                            |
|------------|--------------------------------------------------------|
| `file`     | Rotating log file (configurable size, backup, age)     |
| `internal` | Stored in BoltDB with configurable retention in days   |
| `webhook`  | HTTP POST to an external URL with rate limiting        |

An optional HMAC key can be configured to hash sensitive field values in audit records. Request and response logging are independently toggled.

---

## 4. Configuration

Configuration is loaded in the following precedence order (highest wins):

1. **Environment Variables** — override all file settings.
2. **Config File** — YAML file at an OS-specific default path, or a custom path passed via `--config`.
3. **Built-in Defaults** — sensible fallbacks for all settings.

### Default Config File Locations

| OS      | Path                                                              |
|---------|-------------------------------------------------------------------|
| Linux   | `/etc/gaia/gaia-config.yaml`                                     |
| macOS   | `~/Library/Application Support/Gaia/gaia-config.yaml`           |
| Windows | `%APPDATA%\Gaia\gaia-config.yaml`                                |

### Key Configuration Sections

| Section  | Key Fields                                                             |
|----------|------------------------------------------------------------------------|
| `daemon` | `listen_addr` (default `0.0.0.0:50051`), `db_file`, `timeout`        |
| `log`    | `file_path`, `max_size_mb`, `max_backups`, `max_age_days`, `level`   |
| `tls`    | `certs_directory`, `ca_cert`, `server_cert`, `server_key`, `key_algorithm`, `key_size`, `cert_expiry_days` |
| `audit`  | `enabled`, `hmac_key`, `log_request`, `log_response`, `backends`      |

### Key Environment Variables

| Variable              | Purpose                                      |
|-----------------------|----------------------------------------------|
| `GAIA_LISTEN_ADDR`    | Override daemon listen address               |
| `GAIA_DB_FILE`        | Override database file path                  |
| `GAIA_LOG_FILE`       | Override log file path                       |
| `GAIA_LOG_LEVEL`      | Override log level (`debug/info/warn/error`) |
| `GAIA_CERTS_DIR`      | Override certificates directory              |
| `GAIA_CLIENT_TIMEOUT` | Override gRPC client timeout                 |

---

## 5. gRPC Services

There are two distinct gRPC services defined in separate proto files to enforce the principle of least privilege.

### `GaiaAdmin` Service (`proto/gaia.proto`)

Used exclusively by the Gaia CLI and TUI. Most operations require the daemon to be in an unlocked state.

| RPC                        | Description                                                        |
|----------------------------|--------------------------------------------------------------------|
| `AddSecret`                | Adds a new secret to a client's namespace                          |
| `DeleteSecret`             | Deletes an existing secret                                         |
| `ListSecrets`              | Returns all secrets for a given client (unary)                     |
| `ListSecretsStream`        | Streams secrets for a given client one-by-one (server-streaming)   |
| `ImportSecrets`            | Bulk-imports secrets from a structured stream (client-streaming)   |
| `GetStatus`                | Returns the daemon's current operational status                    |
| `Unlock`                   | Unlocks the daemon by providing the master passphrase              |
| `Lock`                     | Immediately locks the daemon and wipes the key from memory         |
| `Stop`                     | Gracefully shuts down the daemon                                   |
| `RegisterClient`           | Registers a new client and issues a signed mTLS certificate        |
| `ListClients`              | Lists all registered clients                                       |
| `RevokeClient`             | Revokes a client's access                                          |
| `ListNamespaces`           | Lists all namespaces for a given client                            |
| `ListPolicies`             | Lists all configured authorization policies                        |
| `GetPolicy`                | Retrieves the policy for a specific client                         |
| `SetPolicy`                | Creates or replaces a client's authorization policy                |
| `DeletePolicy`             | Deletes a client's authorization policy                            |

### `GaiaClient` Service (`proto/gaia-client.proto`)

A minimal, read-only service intended for application use. Available even when the daemon is **locked**. Client identity is derived from the mTLS certificate CN.

| RPC           | Description                                                                                          |
|---------------|------------------------------------------------------------------------------------------------------|
| `GetSecret`   | Retrieves a single secret by namespace and key                                                       |
| `ListSecrets` | Returns all secrets for the authenticated client; optionally filtered by namespace. Always includes the `common` namespace when not filtered |

---

## 6. CLI Commands

The CLI is built with [Cobra](https://github.com/spf13/cobra). Running `gaia` with no arguments launches the interactive TUI.

### Global Flags

| Flag              | Description                                        |
|-------------------|----------------------------------------------------|
| `--config, -c`    | Custom path to the config file                     |
| `--version, -v`   | Print the installed Gaia version                   |

### Daemon Lifecycle

| Command          | Description                                              |
|------------------|----------------------------------------------------------|
| `gaia start`     | Starts the daemon as a foreground process                |
| `gaia stop`      | Gracefully stops the running daemon via gRPC             |
| `gaia restart`   | Restarts the daemon                                      |
| `gaia status`    | Queries and prints the daemon's current status           |
| `gaia unlock`    | Prompts for the master passphrase and unlocks the daemon |
| `gaia lock`      | Immediately locks the daemon                             |

### Initialization

| Command          | Description                                                                         |
|------------------|-------------------------------------------------------------------------------------|
| `gaia init`      | Bootstraps Gaia: creates the CA, generates server certs, initializes the database, and writes the default config file |

### Client Management

| Command                                     | Description                                                          |
|---------------------------------------------|----------------------------------------------------------------------|
| `gaia clients register <name> [--output-dir]` | Registers a new client and saves its certificate and private key   |

### Secret Management

| Command                                     | Description                                                                      |
|---------------------------------------------|----------------------------------------------------------------------------------|
| `gaia secrets import <file> [--overwrite]`  | Bulk imports secrets from a structured JSON file using a client-streaming RPC    |
| `gaia secrets export [--client] [--namespace] [--format json\|yaml]` | Exports secrets via server-streaming RPC to JSON or YAML |

**Import JSON format:**
```json
{
  "client-app-a": {
    "production": {
      "database_url": "postgres://...",
      "api_key": "secret_prod_key"
    }
  },
  "common": {
    "shared": {
      "global_key": "common_value"
    }
  }
}
```

### Policy Management

| Command                                    | Description                                              |
|--------------------------------------------|----------------------------------------------------------|
| `gaia policy list`                         | Lists all configured client policies                     |
| `gaia policy get <client>`                 | Displays the full policy for a given client              |
| `gaia policy set <client> <policy.json>`   | Creates or replaces a client's policy from a JSON file   |
| `gaia policy delete <client>`              | Removes a client's policy (denies all access afterward)  |
| `gaia policy validate <policy.json>`       | Validates a policy file locally without applying it      |

**Policy JSON format:**
```json
{
  "client_name": "myapp",
  "rules": [
    {
      "path": "common/*",
      "capabilities": ["read"],
      "description": "Read common secrets"
    },
    {
      "path": "myapp/*",
      "capabilities": ["read", "write", "delete", "list"],
      "description": "Full access to own namespace"
    }
  ]
}
```

### Secret Injection

| Command                                    | Description                                                                          |
|--------------------------------------------|--------------------------------------------------------------------------------------|
| `gaia exec -- <command> [args...]`         | Fetches secrets and injects them as environment variables before running a command   |

Secrets are exposed as `GAIA_<NAMESPACE>_<KEY>` (uppercased, non-alphanumeric characters replaced with `_`). This uses the `GaiaClient.ListSecrets` RPC and does not require the daemon to be unlocked.

### Version & Updates

| Command          | Description                                              |
|------------------|----------------------------------------------------------|
| `gaia version`   | Prints version, commit hash, build date, and Go runtime  |
| `gaia update`    | Checks GitHub releases for a newer version               |

---

## 7. Terminal UI (TUI)

The TUI is built with `bubbletea` and provides a full-screen interactive interface for managing secrets and clients. It makes gRPC calls to the daemon as a `GaiaAdmin` client and requires the daemon to be running and unlocked.

### Data Management Screen
- **Add New Record:** A form to add a new secret to a selected client/namespace.
- **List All Records:** Browsable list of all secrets in a given namespace.

### Certificate & Client Management Screen
- **Register Client:** Registers a new client name in the daemon's database.
- **Create New Certificates:** Generates a signed client mTLS certificate pair.
- **List Existing Certificates:** Lists all clients and their certificate status.

---

## 8. Client Libraries

Gaia ships official client libraries for consuming the `GaiaClient` service from application code. All libraries connect via mTLS and the `gaia-client.proto` definition.

| Language   | Location         | Key Features                                                     |
|------------|------------------|------------------------------------------------------------------|
| Go         | `libs/go/`       | `GetSecret`, `ListSecrets`, `LoadEnv` (populates `os.Getenv`)   |
| JavaScript | `libs/js/`       | `getSecret`, `listSecrets`, `loadEnv`                            |
| Rust       | `libs/rust/`     | `get_secret`, `list_secrets`, `load_env`                         |

`LoadEnv`/`load_env` is the primary integration entry point: it fetches all secrets for the authenticated client via `GaiaClient.ListSecrets` and injects them as environment variables into the running process, following the same `GAIA_<NAMESPACE>_<KEY>` naming convention used by `gaia exec`.

---

## 9. Package Overview

| Package                  | Description                                                                        |
|--------------------------|------------------------------------------------------------------------------------|
| `cmd`                    | CLI commands built with Cobra; parses flags and calls daemon gRPC endpoints        |
| `daemon`                 | Core service: gRPC server, BoltDB operations, lock/unlock state management         |
| `tui`                    | Interactive TUI built with Bubble Tea; acts as a gRPC client to the daemon         |
| `config`                 | Loads configuration from YAML file, environment variables, and built-in defaults   |
| `certs`                  | CA, server, and client certificate creation and management                         |
| `audit`                  | Structured audit logging with pluggable backends (file, internal BoltDB, webhook)  |
| `policy`                 | Policy model, validation, and capability types                                     |
| `log`                    | Structured, rotating application logger backed by `slog`                           |
| `encrypt`                | AES-256-GCM encryption/decryption helpers                                          |
| `internal/validation`    | Private package for validating user-provided names (client, namespace, key)        |
| `internal/secutil`       | Security utilities (e.g., wiping sensitive byte slices from memory)                |
