# Gaia

A self-hosted secrets management daemon. Gaia exposes a gRPC API over mTLS, stores secrets encrypted at rest in an embedded BoltDB database, and ships a CLI, interactive TUI, and client libraries for Go, Rust, and JavaScript/TypeScript.

## Quick start

```bash
# Build the binary
make build          # → bin/gaia

# First-run setup (certificates + encrypted database)
gaia init

# Start the daemon
gaia start

# Unlock with your master passphrase
gaia unlock

# Add a secret
gaia secrets add my-app production database-url

# Read it back
gaia secrets get my-app production database-url
```

---

## Installation

### Prerequisites

- Go 1.24+
- `protoc` + `protoc-gen-go` / `protoc-gen-go-grpc` (only needed to regenerate proto files)

### Build from source

```bash
make build          # current OS/arch → bin/gaia
make cross-build    # all platforms   → bin/cross-build/
```

---

## CLI reference

### Daemon lifecycle

| Command | Description |
|---------|-------------|
| `gaia init` | First-run setup: create encrypted database and generate mTLS certificates |
| `gaia start` | Start the daemon in the foreground (locked state) |
| `gaia unlock` | Supply the master passphrase to unlock a running daemon |
| `gaia stop` | Gracefully stop the running daemon |
| `gaia restart` | Send stop signal (re-run `gaia start` to start again) |
| `gaia status` | Print the current daemon status (`locked` / `unlocked` / `stopped`) |
| `gaia rotate` | Re-encrypt all secrets under a new master passphrase |

### Secrets

```bash
gaia secrets add    <client> <namespace> <key>        # add / update a secret
gaia secrets get    <client> <namespace> <key>        # read a secret
gaia secrets delete <client> <namespace> <key>        # delete a secret
gaia secrets list   [client]                          # list all secrets
gaia secrets import <file>                            # bulk import (YAML/JSON)
gaia secrets export [client]                          # bulk export
```

### Clients

```bash
gaia clients register <name>    # register a client and issue mTLS certificates
gaia clients list               # list all registered clients
gaia clients revoke  <name>     # revoke a client's access
```

### Policy

```bash
gaia policy get    <client>                     # show access policy
gaia policy set    <client> <rule>              # set an access policy
gaia policy delete <client>                     # remove a policy
gaia policy list                               # list all policies
```

### Certificates

```bash
gaia certs generate    # generate CA + server + admin-client certificates
gaia certs list        # list existing certificates
gaia certs renew       # renew expiring certificates
```

### TUI

Running `gaia` with no arguments launches the interactive terminal UI.

---

## Local development mode

Gaia ships two convenience modes that eliminate setup friction for local development and CI.

### Unsafe mode — `gaia start --unsafe`

Creates a local directory (default: `./gaia-dev/`) containing the database, config, certs, and logs. Passphrase strength is **not enforced**.

```bash
gaia start --unsafe [--dir ./my-dev-dir]
```

> **Warning printed to stderr on every start:**
> ```
> ╔══════════════════════════════════════════════════════════════╗
> ║  WARNING: UNSAFE LOCAL DEV MODE — NOT FOR PRODUCTION USE    ║
> ║  ...                                                         ║
> ╚══════════════════════════════════════════════════════════════╝
> ```

The unsafe flag is **sticky**: a database created with `--unsafe` must always be started with `--unsafe`, and vice versa.

`gaia init --unsafe` accepts the same flag to skip passphrase strength validation during first-run setup.

### Ephemeral mode — `gaia start --unsafe --ephemeral`

Everything lives in memory. Nothing survives process exit. Ideal for CI/CD pipelines and automated tests.

```bash
gaia start --unsafe --ephemeral
```

- mTLS certificates are auto-generated in a temp directory and removed on exit.
- The BoltDB file is created as an OS temp file and unlinked immediately after opening (POSIX). The process holds an open file descriptor; the OS reclaims disk space when the daemon stops.
- The daemon auto-initializes and starts **fully unlocked** — no `gaia unlock` step needed.
- `--ephemeral` without `--unsafe` is rejected with a clear error.
- AES-256-GCM encryption, mTLS, memory wiping, and audit logging remain active.

### `gaia dev` — one-command dev environment

The `dev` command combines init + start + auto-unlock + optional secret seeding into a single command:

```bash
gaia dev [--dir ./gaia-dev] [--passphrase "mypass"] [--seed ./seed.yaml]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `./gaia-dev` | Working directory for the dev daemon |
| `--passphrase` | *(prompted)* | Master passphrase (or set `GAIA_PASSPHRASE` env var) |
| `--seed` | *(none)* | YAML file with secrets to import on startup |

**Seed file format:**

```yaml
secrets:
  - namespace: app
    key: DATABASE_URL
    value: postgres://localhost/devdb
  - namespace: app
    key: API_KEY
    value: dev-api-key-123
```

---

## Security model

| Concern | Safe mode | Unsafe / Dev mode | Ephemeral mode |
|---------|-----------|-------------------|----------------|
| AES-256-GCM encryption at rest | Yes | Yes | Yes |
| mTLS transport | Yes | Yes | Yes |
| Memory wiping (`secutil.Wipe`) | Yes | Yes | Yes |
| Audit logging | Yes | Yes | Yes |
| scrypt key derivation | N=2^17 (strong) | N=2^15 (~4× faster) | N=2^15 |
| Passphrase strength check | Enforced | Bypassed | Bypassed |
| DB persistence | Yes | Yes | No |
| DB flag sticky | N/A | Yes | No |

---

## Docker (dev image)

A zero-install dev image is available for demos and quick integration tests:

```bash
# Build and run
make docker-dev

# Or with Docker Compose
docker compose -f deploy/docker/docker-compose.dev.yml up
```

The container auto-inits on first start and exposes gRPC on port `50051`.
Set `GAIA_DEV_PASSPHRASE` to override the default passphrase (`devpassphrase`).

---

## Go SDK test helper

The `gaiatest` package (`libs/go/gaiatest`) starts an ephemeral Gaia daemon as a subprocess in Go tests:

```go
import "github.com/stain-win/gaia/libs/go/gaiatest"

func TestMyFeature(t *testing.T) {
    srv := gaiatest.NewServer(t) // stops automatically via t.Cleanup
    // srv.Addr    — gRPC address, e.g. "127.0.0.1:50123"
    // srv.CertDir — directory with ca.crt, gaia_client.crt, gaia_client.key
}
```

Requires the `gaia` binary in `PATH` or pointed to by the `GAIA_BIN` environment variable.

JS and Rust equivalents are documented stubs; full implementations are planned.

---

## Configuration

Default config path: `~/.config/Gaia/gaia-config.yaml` (macOS: `~/Library/Application Support/Gaia/gaia-config.yaml`).

See `gaia-config.template.yaml` at the repo root for all available options.
Environment variables (prefix `GAIA_`) override config file values.

---

## Make targets

```
make all              — compile proto + build binary (default)
make build            — build binary → bin/gaia
make test             — run all Go tests
make protoc           — regenerate proto Go bindings
make cross-build      — build for linux/darwin/windows × amd64/arm64
make clean            — remove build artifacts
make build-js-client  — build the TypeScript client library
make docker-dev       — build and run the dev Docker image
```

---

## Interactive TUI

```bash
gaia          # launch the interactive terminal UI (no args)
gaia setup    # guided setup wizard
```
