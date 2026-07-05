---
name: consuming-gaia-secrets
description: Use when adding Gaia secret consumption to a Go, Node/TypeScript, or Rust application; registering a new Gaia client and distributing its mTLS certs; or debugging "permission denied", "no such file", or gRPC auth errors when a service tries to read its Gaia-issued certificate/key or fetch secrets.
---

# Consuming Gaia Secrets

## Overview

Gaia exposes a read-only `GaiaClient` gRPC service over mTLS. Every consuming
app needs: (1) a registered client identity with its own CA-signed cert/key,
(2) one of the three official client libraries, (3) the daemon address + three
cert paths (CA, client cert, client key).

## Step 1: Register the client and get certs

An admin (someone with an admin-marked cert, e.g. via the CLI) registers the
new client and receives a fresh cert/key pair signed by Gaia's CA:

```bash
gaia clients register my-app --output-dir ./certs
# writes ./certs/my-app.crt and ./certs/my-app.key
```

You also need the CA certificate (`ca.crt`) from wherever the daemon's certs
live (`tls.certs_directory` in the daemon's config) — the client verifies the
server with it, and the server verifies the client with it.

**Permission gotcha:** client keys are written owner-only (mode `0600`). If
the consuming app runs as a different user/container than whatever registered
it, that user needs read access — either run under the same user, or have an
admin grant access via a named ACL entry (`setfacl -m u:appuser:r my-app.key`
or `g:appgroup:r`) on the specific key file. Don't just chmod the key wide
open. See `apps/gaia/certs/disk.go`'s `RelaxKeyACLMask` for how Gaia itself
handles this when a directory has an operator-configured default ACL.

## Step 2: Install the client library and connect

| Language | Package | Install |
|---|---|---|
| Go | `github.com/stain-win/gaia/libs/go/client` | `go get github.com/stain-win/gaia/libs/go/client` |
| Node/TS | `@stain-win/gaia-client` | `npm install @stain-win/gaia-client` |
| Rust | `gaia-client` | `cargo add gaia-client` |

All three need the same four things: daemon address, CA cert path, client
cert path, client key path.

**Go** (`libs/go/client`):
```go
c, err := client.NewClient(client.Config{
    Address:        "localhost:50051",
    CACertFile:     "./certs/ca.crt",
    ClientCertFile: "./certs/my-app.crt",
    ClientKeyFile:  "./certs/my-app.key",
})
if err != nil { /* connection + first ListSecrets call happen inside NewClient */ }
defer c.Close()

secret, err := c.GetSecret(ctx, "production", "database_url")
all, err := c.ListSecrets(ctx)                 // map[namespace]map[key]value
err = c.LoadEnv(ctx, client.LoadEnvOptions{Prefix: "MYAPP", UseNamespace: true})
```

**Node/TypeScript** (`@stain-win/gaia-client`) — pattern taken from the real,
verified `playground/web/src/app/gaia-api.ts` (Next.js SSR, module-level
singleton so the client is created once per process):
```ts
import { createClient } from "@stain-win/gaia-client";

let client: Awaited<ReturnType<typeof createClient>> | null = null;

async function getGaiaClient() {
  if (client) return client;
  client = await createClient({
    address: process.env.GAIA_ADDRESS || "localhost:50051",
    caCertFile: process.env.GAIA_CA_CERT,
    clientCertFile: process.env.GAIA_CLIENT_CERT,
    clientKeyFile: process.env.GAIA_CLIENT_KEY,
  });
  return client;
}

const secret = await (await getGaiaClient()).getSecret("production", "database_url");
const all = await (await getGaiaClient()).listSecrets();
await (await getGaiaClient()).loadEnv({ prefix: "MYAPP", useNamespace: true });
```
Fetch secrets only in server-side code (Server Components, API routes, backend
services) — never in client-bundled code, or the secret ships to the browser.

**Rust** (`gaia-client` crate):
```rust
use gaia_client::{GaiaClient, GaiaClientConfig};

let config = GaiaClientConfig::new(
    "localhost:50051", "./certs/ca.crt", "./certs/my-app.crt", "./certs/my-app.key",
);
let mut client = GaiaClient::connect(config).await?;

let secret = client.get_secret("production", "database_url").await?;
let namespaces = client.list_secrets(None).await?;
```

## Reference implementation

`playground/` in this repo is a full Docker Compose example (Gaia daemon +
Next.js SSR frontend sharing a certs volume over mTLS) and is kept working
against the current codebase — `playground/README.md` has the exact
`docker compose up` / `gaia clients register` / `gaia secrets import` flow.
Treat it as the canonical end-to-end reference; if you change the daemon's
gRPC auth, cert generation, or client-registration code paths, re-verify the
playground still builds and serves secrets (`docker compose up -d --build`
from `playground/`, then check the page renders the imported test secrets).

## Common mistakes

- Forgetting the client is **read-only** (`GaiaClient` service) — it can't
  create/delete secrets or namespaces; that's `GaiaAdmin`, gated by an
  admin-OU-marked cert (see `apps/gaia/daemon/admin_auth.go`).
- Pointing `clientCertFile`/`ClientCertFile` at an *admin* cert instead of a
  client-registered one — works today (admin certs aren't blocked from
  `GaiaClient` calls) but conflates two identities; register a dedicated
  client per app.
- Hardcoding cert paths instead of reading them from env vars — makes the
  same code portable across dev/staging/prod without rebuilding.
