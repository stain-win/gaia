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
