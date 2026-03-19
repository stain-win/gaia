# CLAUDE.md — Gaia Codebase Guide

## Project Overview

**Gaia** is a self-hosted secrets management daemon. It exposes a gRPC API over mTLS, stores secrets encrypted at rest in an embedded BoltDB database, and ships a CLI, interactive TUI, and client libraries for Go, Rust, and JavaScript/TypeScript.

Repository: `github.com/stain-win/gaia`

---

## Repository Layout

```
gaia/
├── apps/gaia/          # Main Go application (daemon + CLI + TUI)
│   ├── audit/          # Audit logging backends (file, internal, webhook)
│   ├── certs/          # TLS certificate generation and management
│   ├── cmd/            # Cobra CLI commands
│   ├── config/         # Configuration loading/validation
│   ├── daemon/         # Core daemon logic and gRPC service implementations
│   ├── encrypt/        # AES-256-GCM encryption, scrypt key derivation
│   ├── internal/       # Private packages (grpcclient, validation, errors, secutil)
│   ├── log/            # Custom slog-based logging with rotation
│   ├── policy/         # Authorization and access control
│   ├── proto/          # Generated protobuf Go code (do not edit manually)
│   ├── tui/            # Bubbletea terminal UI
│   ├── go.mod
│   └── main.go
├── libs/
│   ├── go/             # Go client library
│   ├── js/             # TypeScript/JavaScript client library (npm)
│   └── rust/           # Rust async client library (crates.io)
├── proto/
│   ├── gaia.proto      # Admin + full service definitions
│   └── gaia-client.proto  # Client-only service definition (for lib consumers)
├── deploy/
│   ├── docker/         # Docker Compose setup and Dockerfiles
│   ├── ansible/        # Ansible playbooks for production deployment
│   └── systemd/        # systemd service unit files
├── documentation/      # Technical specifications
├── examples/           # Example configs and policy templates
├── scripts/            # Build/utility scripts
├── .github/workflows/  # GitHub Actions (ci.yml, release.yml)
├── Makefile
├── gaia-config.template.yaml
└── .goreleaser.yaml
```

---

## Architecture

### Component Model

Gaia follows a **client-daemon** architecture:

- **Daemon** listens on a gRPC address (default `:50051`) and handles all business logic.
- **CLI/TUI** connect to the daemon as privileged admin clients using a local Unix socket or TCP address.
- **External clients** (applications) authenticate via mTLS certificates issued by the daemon.

### Data Hierarchy

```
Client → Namespace → Record (key/value secret)
```

- Naming rules: lowercase letters, digits, hyphens, and underscores only.
- Composite BoltDB keys are null-byte delimited: `clientName\x00namespace\x00secretId`.
- The `common` namespace is always accessible to all registered clients.

### gRPC Services (proto/gaia.proto)

| Service | Caller | Purpose |
|---------|--------|---------|
| `GaiaAdmin` | CLI / TUI / admin tooling | Full CRUD: secrets, clients, policies, namespaces, password rotation |
| `GaiaClient` | Application clients | Read-only: `GetSecret`, `ListSecrets` |

### Security Model

- **At-rest encryption**: AES-256-GCM; master key derived from passphrase via scrypt.
- **Transport**: mTLS — both daemon and clients present certificates.
- **Certificates**: Ed25519, self-signed, issued by the daemon's built-in CA; stored in the configured `certs_dir`.
- **Unlock flow**: Daemon starts locked; `gaia unlock` (or TUI) supplies the passphrase to decrypt the master key into memory.
- **Memory security**: Sensitive byte slices are zeroed via `internal/secutil` after use.
- **Rate limiting**: Configurable limits on unlock attempts.
- **Audit logging**: Events written to file, internal BoltDB store, and/or a webhook.

---

## Development Workflows

### Prerequisites

- Go 1.24+
- `protoc` (Protocol Buffers compiler)
- `protoc-gen-go` and `protoc-gen-go-grpc` plugins
- Node.js / npm (for JS client only)
- Rust / Cargo (for Rust client only)

Install Go protoc plugins:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Common Make Targets

| Target | Description |
|--------|-------------|
| `make all` | Compile proto + build binary (default) |
| `make build` | Build binary for current OS/arch → `bin/gaia` |
| `make test` | Run all Go tests (`cd apps/gaia && go test ./...`) |
| `make protoc` | Regenerate `apps/gaia/proto/*.pb.go` from `proto/gaia.proto` |
| `make cross-build` | Build for linux/darwin/windows × amd64/arm64 |
| `make clean` | Remove all build artifacts and generated proto files |
| `make build-js-client` | Build the TypeScript client library |
| `make protoc-client-go` | Generate Go client library proto bindings |
| `make protoc-client-js` | Copy proto file for JS dynamic loading |
| `make debug_build` | Build with debug flags |

### Running Tests

```bash
# All Go tests
make test

# Single package
cd apps/gaia && go test ./daemon/... -v

# With race detector
cd apps/gaia && go test -race ./...

# JavaScript client tests
cd libs/js && npm test
```

### Building

```bash
# Local build
make build        # outputs bin/gaia

# Cross-compile
make cross-build  # outputs bin/cross-build/gaia-<os>-<arch>
```

### Protobuf Regeneration

Always regenerate after editing any `.proto` file:

```bash
make protoc                # main app proto
make protoc-client-go      # Go client lib proto
make protoc-client-js      # copy proto for JS lib
```

**Never manually edit files in `apps/gaia/proto/`** — they are generated.

---

## Key Packages

### `apps/gaia/daemon`

The core of the application.

- `daemon.go` — daemon lifecycle, store management, encryption state, client/namespace/secret CRUD.
- `grpc_service.go` — implements `GaiaAdmin` and `GaiaClient` gRPC service interfaces.
- Business logic lives here; `cmd/` is thin wiring around it.

### `apps/gaia/cmd`

Cobra-based CLI. Each file corresponds to a command group:

| File | Commands |
|------|----------|
| `daemon.go` | `gaia daemon start/stop/status` |
| `secrets.go` | `gaia secrets add/get/delete/list/import/export` |
| `clients.go` | `gaia clients register/list/revoke` |
| `policy.go` | `gaia policy get/set/delete/list` |
| `certs.go` | `gaia certs generate/list/renew` |
| `init.go` | `gaia init` (first-run setup) |
| `setup.go` | `gaia setup` (interactive TUI setup) |
| `rotate.go` | `gaia rotate` (password rotation) |

### `apps/gaia/tui`

Bubbletea MVC terminal UI. Follow existing message-passing patterns:

- `model.go` — `Model` struct and `Update` function (message dispatch).
- `view.go` — `View` function (rendering).
- Feature-specific files (`policy.go`, `list_records.go`, etc.) add sub-models and messages.
- Use `tea.Cmd` for async operations; never block in `Update`.

### `apps/gaia/encrypt`

- `encrypt.go` — AES-256-GCM encrypt/decrypt helpers.
- `utils.go` — scrypt key derivation, passphrase utilities.
- Always call the wipe helpers from `internal/secutil` after handling derived keys.

### `apps/gaia/policy`

JSON-serialisable policy rules stored per-client. Capabilities: `read`, `write`, `delete`, `list`.
Path matching follows `client/namespace/secret` glob patterns.

### `apps/gaia/audit`

Three backends implement a common `AuditLogger` interface:

- `file` — append-only JSONL log file.
- `internal` — records stored in BoltDB audit bucket.
- `webhook` — HTTP POST to a configured endpoint.

### `apps/gaia/internal`

Private utilities that must not be imported outside this module:

- `grpcclient/` — shared gRPC dial helpers (mTLS config, connection pooling).
- `validation/` — `ValidateName`, `ValidateKey` with regex rules.
- `errors/` — typed error wrappers.
- `secutil/` — `Wipe([]byte)` memory zeroing.

---

## Client Libraries

### Go (`libs/go`)

```go
import "github.com/stain-win/gaia/libs/go/client"

c, err := client.New(client.Options{Addr: "localhost:50051", CertFile: ..., KeyFile: ..., CAFile: ...})
secret, err := c.GetSecret(ctx, "namespace", "key")
```

### JavaScript/TypeScript (`libs/js`)

Uses `@grpc/proto-loader` for dynamic proto loading (no pre-generated stubs needed at runtime).

```typescript
import { GaiaClient } from '@stain-win/gaia-client';
const client = new GaiaClient({ address: 'localhost:50051', certPath: ..., keyPath: ..., caPath: ... });
const secret = await client.getSecret({ namespace: 'ns', id: 'key' });
```

### Rust (`libs/rust`)

Async Tokio + Tonic client.

```rust
use gaia_client::GaiaClient;
let mut client = GaiaClient::connect("https://localhost:50051", tls_config).await?;
let secret = client.get_secret("namespace", "key").await?;
```

---

## Configuration

Template: `gaia-config.template.yaml`

Key sections:

```yaml
daemon:
  listen_addr: "0.0.0.0:50051"
  db_file: "/var/lib/gaia/gaia.db"
  timeout: 30s

logging:
  file: "/var/log/gaia/gaia.log"
  level: "info"         # debug | info | warn | error
  max_size_mb: 100
  max_backups: 3

tls:
  certs_dir: "/etc/gaia/certs"
  algorithm: "ed25519"
  expiry_days: 365

audit:
  backends:
    - type: file
      path: "/var/log/gaia/audit.log"
    - type: internal
    - type: webhook
      url: "https://example.com/audit"
```

Environment variables override config file values (prefix: `GAIA_`).

---

## CI/CD

### GitHub Actions

| Workflow | Trigger | Jobs |
|----------|---------|------|
| `ci.yml` | Push/PR to `main` or `develop` | `test` → `build` → `lint` |
| `release.yml` | Tag push (`v*`) | GoReleaser, npm publish, crates.io publish |

CI pipeline steps:
1. Install `protoc` and Go proto plugins.
2. `make protoc` — regenerate proto code.
3. `make test` — run Go tests.
4. `go vet ./...` — static analysis.
5. `golangci-lint` — linting (working-directory: `apps/gaia`).
6. `make build` — verify binary compiles.

### Versioning

Version info is injected at build time via `ldflags`:

```
-X 'github.com/stain-win/gaia/apps/gaia/cmd.version=$(GIT_VERSION)'
-X 'github.com/stain-win/gaia/apps/gaia/cmd.gitCommit=$(GIT_COMMIT)'
-X 'github.com/stain-win/gaia/apps/gaia/cmd.buildDate=$(BUILD_DATE)'
```

`GIT_VERSION` comes from `git describe --tags --always --dirty`.

---

## Conventions and Patterns

### Go Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Error wrapping: use `fmt.Errorf("context: %w", err)` — do not discard errors.
- Typed errors are in `internal/errors/`; prefer them over raw `errors.New` for domain errors.
- Name validation is enforced at the gRPC handler level via `internal/validation`.

### Sensitive Data Handling

- **Always zero secrets from memory** using `secutil.Wipe(buf)` in a `defer` immediately after allocating a sensitive byte slice.
- Never log secret values — the audit system has a redaction layer; check `audit/redact.go` before adding new log fields.
- Passphrases must pass strength validation (`wagslane/go-password-validator`) before acceptance.

### Protobuf Changes

1. Edit `proto/gaia.proto` (or `proto/gaia-client.proto` for client-facing changes).
2. Run `make protoc` (and `make protoc-client-*` as appropriate).
3. Implement any new RPC methods in `daemon/grpc_service.go`.
4. Add corresponding CLI commands in `cmd/`.
5. Update client libraries if the client proto changed.

### Adding a New CLI Command

1. Create or extend a file in `apps/gaia/cmd/`.
2. Register the command in the parent command's `init()` or `AddCommand()` call.
3. Use `grpc_client.go` helpers for daemon communication — do not create ad-hoc connections.
4. Keep command files thin; business logic belongs in `daemon/`.

### Adding a New TUI Screen

1. Define a new message type and sub-model in a dedicated file under `tui/`.
2. Handle the message in `model.go`'s `Update` switch.
3. Render via `view.go` or a dedicated `view` method.
4. Use `lipgloss` for styling; follow the existing style constants in `tui/styles.go` (or equivalent).

### Test Conventions

- Test files live alongside source files (`*_test.go`).
- Use table-driven tests for functions with multiple input variants.
- Daemon tests in `daemon/daemon_test.go` spin up an in-memory BoltDB instance — avoid touching the filesystem.
- Security-sensitive tests (encryption, cert generation) must cover both happy paths and known-bad inputs.

---

## Deployment

### Docker

```bash
cd deploy/docker
docker compose up -d
```

See `deploy/docker/README.md` for certificate volume configuration.

### Ansible

```bash
cd deploy/ansible
ansible-playbook -i inventory.yml site.yml
```

See `deploy/ansible/README.md` for variable definitions and role structure.

### systemd

Unit files in `deploy/systemd/`. After binary installation:

```bash
sudo cp deploy/systemd/gaia.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now gaia
```

---

## Release Process

Releases are automated via GoReleaser (`.goreleaser.yaml`) and triggered by pushing a version tag:

```bash
git tag v1.2.3
git push origin v1.2.3
```

This triggers:
- Cross-platform binary builds for linux/darwin/windows × amd64/arm64.
- GitHub Release creation with binaries and checksums.
- npm package publish for the JS client library.
- Crates.io publish for the Rust client library.

---

## Unsafe Local Dev Mode — Implementation Plan

> **Status:** Implemented (commit 512226c). Ephemeral mode, SDK test helper, `gaia dev` command, and Docker devtools image added in the next layer.

### Overview

The unsafe local dev mode is a convenience feature for local development and
testing **only**. It co-locates all Gaia artefacts (database, config, certs,
binary) in a single directory, and relaxes the passphrase-strength requirement.
A permanent flag is written into the database at initialisation so the daemon
always knows it was created in unsafe mode — and can never be silently promoted
to a production-grade database.

**What it must NOT do:**
- Change the underlying cryptographic primitives (AES-256-GCM and scrypt remain).
- Affect any codepath that does not have `--unsafe` explicitly set.
- Be reachable in any non-interactive / CI build without a visible opt-in.

---

### User-Facing Interface

#### Starting in unsafe mode

```bash
gaia start --unsafe [--dir ./my-dev-dir]
```

- `--unsafe` is **required** to enter this mode. No config file or environment
  variable can enable it silently.
- `--dir` (optional) overrides the working directory; defaults to `./gaia-dev`
  in the current working directory.
- When `--unsafe` is set and the directory does not yet contain a database,
  `gaia start` automatically calls `InitializeDB` with the provided (or
  prompted) passphrase **without** strength-checking it.
- A bold terminal warning is printed to **stderr** before anything else starts:

```
⚠  WARNING: Gaia is running in UNSAFE LOCAL DEV MODE.
   All files are stored in: ./gaia-dev
   Passphrase strength is NOT enforced.
   This database CANNOT be used in production.
   Do not store real secrets here.
```

#### First-run inside unsafe mode

If `--unsafe` is set and no database exists yet, the daemon calls its own
`InitializeDB` path, but:
1. Skips `encrypt.ValidatePassword` (passphrase strength check is bypassed).
2. Writes the `unsafeModeKey` metadata entry (see below) before closing the
   bootstrap transaction.

If the database already exists and **does not** contain `unsafeModeKey`, the
daemon refuses to start and exits with a clear error:

```
error: database was created without --unsafe and cannot be started in unsafe mode
```

If the database already contains `unsafeModeKey` but `--unsafe` was **not**
passed, the daemon refuses to start:

```
error: database was created in unsafe mode; you must pass --unsafe to start it
```

This makes the unsafe flag **sticky** to the database, in both directions.

---

### Database-Level Unsafe Flag

A single metadata key is stored in the `secrets` BoltDB bucket alongside
`saltKey` and `keyHashKey` (all share the `metaPrefix` namespace so they are
skipped by secret enumeration):

```go
const unsafeModeKey = metaPrefix + "__unsafe_mode__"
```

Value stored: the RFC 3339 UTC timestamp at which the database was initialised
in unsafe mode, encoded as a UTF-8 string. Non-empty presence is the signal;
the timestamp helps with debugging.

Rules enforced at startup in `daemon.go → Start()`:

| DB has `unsafeModeKey` | `--unsafe` flag passed | Action |
|------------------------|------------------------|--------|
| No                     | No                     | Normal safe start — proceed |
| No                     | Yes                    | Error: db is safe, cannot start unsafe |
| Yes                    | No                     | Error: db is unsafe, must pass `--unsafe` |
| Yes                    | Yes                    | Unsafe start — proceed with warning |

**There is no migration path from unsafe to safe.** If a developer needs a
production database they must run `gaia init` fresh (without `--unsafe`).

---

### Directory Layout (unsafe mode)

When `--unsafe --dir ./gaia-dev` is used, all paths are rewritten:

| Artefact | Default (safe) | Unsafe mode |
|----------|---------------|-------------|
| Database | platform default (`/var/lib/gaia/gaia.db`) | `<dir>/gaia.db` |
| Config | platform default (`/etc/gaia/gaia-config.yaml`) | `<dir>/gaia-config.yaml` (auto-generated if absent) |
| Certs dir | `/etc/gaia/certs` | `<dir>/certs/` |
| Log file | `/var/log/gaia/gaia.log` | `<dir>/gaia.log` |
| Audit log | `/var/log/gaia/audit.log` | `<dir>/audit.log` |

The daemon creates `<dir>` and `<dir>/certs/` if they do not exist (mode 0700).

---

### Scrypt Parameters in Unsafe Mode

The current production parameters are `N=2^17, r=8, p=1`. These are
intentionally slow.

For unsafe mode, use the **existing** `encrypt.DeriveKeyLegacy` function
(already in `encrypt/utils.go`, `N=2^15`). This is faster for local iteration
while still using scrypt. **Do not invent new crypto parameters** — reuse what
is already audited.

The choice of derivation function is recorded in the database alongside the
unsafe flag so the `UnlockDB` path can select the correct one at runtime.

```go
const derivationVersionKey = metaPrefix + "__derive_version__"
// Values: "v1" (N=2^17, production), "v1-legacy" (N=2^15, unsafe/dev)
```

The unlock path already has a `DeriveKeyLegacy` fallback (line ~534 in
daemon.go). Extend it to read `derivationVersionKey` instead of trying both.

---

### Code Changes Required

#### 1. `apps/gaia/cmd/daemon.go`

- Add `--unsafe` (`bool`) and `--dir` (`string`) flags to `startCmd`.
- When `--unsafe` is set:
  - Compute absolute path of `--dir` (default `./gaia-dev`).
  - Override `cfg.Daemon.DBFile`, `cfg.TLS.CertsDirectory`, `cfg.Log.FilePath`,
    and audit file path to point inside `<dir>`.
  - Set `cfg.UnsafeMode = true` (new field on `Config`).
  - Print the warning banner to `os.Stderr` before calling `gaiaDaemon.Start`.

#### 2. `apps/gaia/config/config.go`

- Add field to `Config`:
  ```go
  UnsafeMode    bool   `yaml:"-"` // Never persisted; runtime-only flag
  UnsafeDir     string `yaml:"-"` // Resolved absolute path of --dir
  ```
  Both are tagged `yaml:"-"` so they cannot appear in a config file.

#### 3. `apps/gaia/daemon/daemon.go`

- Add constants:
  ```go
  const (
      unsafeModeKey       = metaPrefix + "__unsafe_mode__"
      derivationVersionKey = metaPrefix + "__derive_version__"
  )
  ```
- Add field to `Daemon`:
  ```go
  unsafeMode bool
  ```
- `NewDaemon`: populate `d.unsafeMode` from `cfg.UnsafeMode`.
- `InitializeDB`: accept a new `unsafeMode bool` parameter (or read from
  `d.config.UnsafeMode`):
  - If unsafe: skip `encrypt.ValidatePassword`; use `DeriveKeyLegacy`; store
    `unsafeModeKey` and `derivationVersionKey = "v1-legacy"` in the transaction.
  - If safe: existing behaviour unchanged; store `derivationVersionKey = "v1"`.
- `Start` (the function that calls `bbolt.Open` and then `UnlockDB`): after
  opening the DB but before unlocking, read `unsafeModeKey` and
  `derivationVersionKey`, then apply the four-row matrix above and error-out
  where needed.
- `UnlockDB` / passphrase verification: read `derivationVersionKey` to choose
  `DeriveKey` vs `DeriveKeyLegacy` instead of silently trying both.

#### 4. `apps/gaia/encrypt/utils.go`

- No new functions needed — `DeriveKeyLegacy` already exists.
- Add a `ValidatePasswordUnsafe` no-op helper that always returns `(true, nil)`
  to make call sites explicit:
  ```go
  // ValidatePasswordUnsafe skips entropy checking. Only call from unsafe dev mode paths.
  func ValidatePasswordUnsafe(_ string) (bool, error) { return true, nil }
  ```

#### 5. `apps/gaia/cmd/init.go` and `apps/gaia/cmd/setup.go`

- Both call `encrypt.ValidatePassword`. Add an `--unsafe` flag to `gaia init`
  as well so that first-run initialisation can also be done without a strong
  passphrase. The same flag must be forwarded to `daemon.InitializeDB`.

---

### Security Boundaries — What Must NOT Change

| Concern | Safe mode | Unsafe mode |
|---------|-----------|-------------|
| AES-256-GCM encryption at rest | Yes | Yes (unchanged) |
| mTLS transport | Yes | Yes (unchanged) |
| Memory wiping (`secutil.Wipe`) | Yes | Yes (unchanged) |
| Audit logging | Yes | Yes (unchanged) |
| scrypt (key derivation) | `N=2^17` (strong) | `N=2^15` (faster, via `DeriveKeyLegacy`) |
| Passphrase entropy check | Enforced | Bypassed |
| Rate-limiting on unlock | Yes | Yes (unchanged) |
| DB flag sticky | N/A | Written at init; checked at every start |

The unsafe flag must **never** disable mTLS, skip certificate generation, or
remove memory wiping. Those features must remain fully active.

---

### Warning Message Implementation

The warning must be hard to miss. Print it to `os.Stderr` (not the log file)
with a visible border, immediately before the daemon blocks on the gRPC
listener. Suggested format (use `fmt.Fprintf(os.Stderr, ...)` — no lipgloss
dependency in `cmd/`):

```
╔══════════════════════════════════════════════════════════════╗
║  ⚠  UNSAFE LOCAL DEV MODE — NOT FOR PRODUCTION USE  ⚠      ║
║                                                              ║
║  All data is stored in: /absolute/path/to/gaia-dev          ║
║  Passphrase strength is NOT enforced.                        ║
║  This database CANNOT be migrated to safe mode.             ║
║  Do NOT store real secrets here.                             ║
╚══════════════════════════════════════════════════════════════╝
```

This is printed once at startup and once more on every subsequent start of the
same unsafe database.

---

### Tests to Add

| Test location | What to cover |
|---------------|---------------|
| `daemon/daemon_test.go` | `InitializeDB` with unsafe flag writes `unsafeModeKey`; safe DB refuses unsafe start; unsafe DB refuses safe start; `UnlockDB` selects correct derivation function |
| `encrypt/utils_test.go` | `ValidatePasswordUnsafe` always succeeds |
| `cmd/daemon_test.go` (new) | `--unsafe` flag wires correct `Config` fields; `--dir` resolves to absolute path |

---

### Out of Scope for This Feature

- Auto-unlock via environment variable passphrase (separate feature).
- A TUI toggle for unsafe mode (TUI is admin-only, not a first-run tool here).
- Upgrading an unsafe DB to safe (intentionally unsupported).
- Docker or systemd support for unsafe mode (dev-only, local binary usage).

---

## Quick Reference

```bash
# Build everything
make all

# Run tests
make test

# Regenerate proto (after editing .proto files)
make protoc

# Build JS client
make build-js-client

# Clean all artifacts
make clean

# Cross-compile for all platforms
make cross-build
```


<!-- nx configuration start-->
<!-- Leave the start & end comments to automatically receive updates. -->

## General Guidelines for working with Nx

- For navigating/exploring the workspace, invoke the `nx-workspace` skill first - it has patterns for querying projects, targets, and dependencies
- When running tasks (for example build, lint, test, e2e, etc.), always prefer running the task through `nx` (i.e. `nx run`, `nx run-many`, `nx affected`) instead of using the underlying tooling directly
- Prefix nx commands with the workspace's package manager (e.g., `pnpm nx build`, `npm exec nx test`) - avoids using globally installed CLI
- You have access to the Nx MCP server and its tools, use them to help the user
- For Nx plugin best practices, check `node_modules/@nx/<plugin>/PLUGIN.md`. Not all plugins have this file - proceed without it if unavailable.
- NEVER guess CLI flags - always check nx_docs or `--help` first when unsure

## Scaffolding & Generators

- For scaffolding tasks (creating apps, libs, project structure, setup), ALWAYS invoke the `nx-generate` skill FIRST before exploring or calling MCP tools

## When to use nx_docs

- USE for: advanced config options, unfamiliar flags, migration guides, plugin configuration, edge cases
- DON'T USE for: basic generator syntax (`nx g @nx/react:app`), standard commands, things you already know
- The `nx-generate` skill handles generator discovery internally - don't call nx_docs just to look up generator syntax


<!-- nx configuration end-->