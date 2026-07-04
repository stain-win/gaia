# Gaia — Security, Architecture & Deploy Review

> Review date: 2026-07-04. Scope: code quality, project structure/architecture, **security**, and
> **deploy (install/update)**. The master-password model is accepted as-is and intentionally
> excluded. Line references are to the state of `main` at review time.

---

## 0. Executive summary

Gaia is a well-structured, idiomatic Go codebase with a clean client-daemon split, solid crypto
primitives (AES-256-GCM + scrypt), and better-than-average operational tooling (Ansible role,
hardened systemd template, audit backends, self-update). The core cryptography and mTLS transport
are sound.

The single most important issue is an **authorization gap on the `GaiaAdmin` gRPC service**: its
handlers authenticate the TLS connection but never check *who* the caller is. Because every
application client certificate is issued by the same CA as the admin certificate, any registered
client can call admin RPCs (read every secret, rotate the password, stop the daemon), bypassing the
entire policy engine. This is P0 and should be fixed before anything else.

Severity legend: **P0** exploitable/trust-boundary break · **P1** meaningful weakness · **P2**
hardening/robustness · **P3** polish.

---

## 1. Architecture & code quality (overview)

**Strengths**
- Clear layering: `cmd/` (thin CLI) → `daemon/` (logic) → `encrypt/`, `certs/`, `policy/`,
  `audit/`. Matches the documented model.
- Consistent typed errors (`internal/errors`) and a clean gRPC error-mapping layer
  (`mapErrorToGRPCStatus`).
- Crypto is textbook-correct: random nonces, constant-time hash compare on unlock
  (`subtle.ConstantTimeCompare`), scrypt N=2^17, key wiped on lock.
- Rate-limited unlock with lockout; audit interceptor; graceful shutdown with timeout.

**Cross-cutting quality issues**
- ~~`daemon.go` is 1,486 lines and mixes lifecycle, crypto, storage, client/secret CRUD, and TLS
  loading.~~ ✅ **DONE** — split into `daemon.go` (types, constants, constructor, shared helpers),
  `daemon_lifecycle.go` (Start/stop/restart, TLS server creds, audit init, unsafe-mode check),
  `daemon_crypto.go` (init/lock/unlock, rate limiting, rotation, backup, CA creds),
  `daemon_clients.go` (client CRUD + lookup), and `daemon_secrets.go` (secret CRUD, streaming,
  import). Code moved verbatim (verified byte-identical); no behaviour change.
- `findClientByName` signals "found" by returning `fmt.Errorf("client found")` and then
  string-matches on `err.Error() == "client found"` (daemon.go:948–953). Fragile control-flow via
  error strings; use a sentinel or a boolean captured in the closure.
- Duplication: the "read salt/hash" + "read derivation version" + "select deriveFunc" block is
  copy-pasted across `UnlockDB` and `RotatePassword`. Extract a `loadKeyMaterial(tx)` helper.
- `ListSecretsStream` silently strips `\x00` from secret values (daemon.go:1226). This mangles any
  legitimately binary/embedded-null secret without warning — surprising data corruption. Prefer
  returning bytes untouched, or reject on write.

---

## 2. Security findings

### P0-1 — `GaiaAdmin` service has no caller authorization — ✅ FIXED
**Status:** Resolved. Admin certs are now stamped with an Organizational Unit marker
(`certs.AdminOU = "gaia-admin"`) by `gaia init` and `gaia certs create-client --admin`; a new
daemon interceptor (`daemon/admin_auth.go`) rejects any `/gaia.GaiaAdmin/` call whose peer cert
lacks the OU marker and whose CN is not in the configurable `tls.admin_common_names` allow-list
(default `["gaia_client"]`, for upgrade compatibility). RegisterClient-issued certs never carry the
OU, so application clients can no longer reach admin RPCs. Covered by `daemon/admin_auth_test.go`.
**Migration:** existing admin certs keep working via the CN allow-list; regenerate them with
`gaia certs create-client --admin gaia_client` (or empty the allow-list) to move to OU-only.

**Files:** `daemon/grpc_service.go` (all `gaiaAdminServer` methods), `daemon/daemon.go:213–215`,
`cmd/init.go:330`.

Both services are registered on the **same** gRPC listener with the **same** CA
(`RequireAndVerifyClientCert`, `ClientCAs = <single CA>`). The `gaiaClientServer` correctly derives
the caller from its certificate CN via `getClientIdentity` and runs the policy check. The
`gaiaAdminServer` methods (`AddSecret`, `DeleteSecret`, `ListSecrets`, `RegisterClient`,
`RevokeClient`, `RotatePassword`, `SetPolicy`, `Stop`, …) check only `s.d.isLocked` — they never
call `getClientIdentity` and never verify the caller is the admin.

**Impact:** Any client that owns a CA-issued cert (i.e. every registered application client) can
connect to `GaiaAdmin` and read *all* secrets for *any* client, mutate policies, rotate the master
password, or stop the daemon. The policy/namespace isolation that `GaiaClient` enforces is fully
bypassable. This defeats the primary multi-tenant security guarantee.

Note `policy.CheckPermission` special-cases `clientName == "gaia"`, but the admin cert CN is
actually `"gaia_client"`, and admin handlers don't consult the policy engine anyway — so the check
is both mis-keyed and unreached.

**Fix options (pick one; A is recommended):**
- **A. Separate the trust domains.** Issue admin certs from a distinct admin CA (or tag admin certs
  with a dedicated OU/extended key usage). Add a gRPC unary+stream interceptor on the admin service
  that calls `getClientIdentity` and rejects any cert not in the admin set with
  `codes.PermissionDenied`. This is the clean, defensible boundary.
- **B. Identity allowlist.** Minimal stopgap: an admin interceptor that requires the caller CN to be
  in a configured admin-CN allowlist (default `{"gaia_client"}`). Cheap, but all certs still chain
  to one CA, so a forged/reissued cert with that CN would pass — weaker than A.
- Add regression tests: a non-admin client cert calling every `GaiaAdmin` RPC must get
  `PermissionDenied`.

### P1-2 — Revocation is not enforced at the transport layer (no CRL) — ✅ FIXED
**Status:** Resolved. `daemon/revocation.go` adds `checkPeerRevocation`, chained into the same
auth interceptor as the admin check: every `/gaia.GaiaClient/` call is rejected with
`PermissionDenied` if the peer certificate's CN matches a registered client whose status is
revoked — before any handler runs. Covered by `daemon/revocation_test.go`. A TLS-handshake-level
deny-list (by serial) and short cert lifetimes remain possible future hardening.

**Files:** `daemon/daemon.go:1058` (status check on `GetSecret`), `RevokeClient`.

`RevokeClient` flips a DB status flag. `GetSecret` checks `status == active`, but: (a) mTLS still
accepts the revoked certificate (no CRL / no deny-list at TLS handshake), and (b) admin RPCs and
`ListSecrets` paths don't all re-check status. Combined with P0-1, a revoked client's cert remains
fully usable. Recommend a revoked-cert deny-list checked in an interceptor (by CN and/or serial),
and consider short cert lifetimes to bound exposure.

### P1-3 — Software supply chain: release artifacts are checksummed but not signed — ✅ FIXED
**Status:** Fully resolved, both sides.
- **CI:** `.goreleaser.yaml` signs `checksums.txt` with cosign (keyless/OIDC), emitting a Sigstore
  bundle asset `checksums.txt.sigstore.json` (cert + signature + Rekor proof in one file);
  `release.yml` installs cosign (pinned v2.5.0) with job-scoped `id-token: write`. Release notes
  document `cosign verify-blob --bundle` usage.
- **Client:** `gaia update --install` now authenticates `checksums.txt` against the bundle via
  `sigstore-go` (`cmd/sigstore_verify.go`) **before** trusting any checksum: signature over the
  artifact, Fulcio chain + Rekor transparency-log inclusion (1 tlog entry, observer timestamp), and
  certificate identity pinned to `^https://github\.com/stain-win/gaia/` issued by GitHub Actions
  OIDC. Verification failure aborts the install; `--skip-signature` is an explicit, loudly-warned
  override. Releases published before signing exist get a warning + checksum-only fallback (an
  attacker able to strip the bundle asset controls the release anyway; the fallback can be removed
  once all supported releases are signed). Trust root is fetched via TUF and cached under
  `~/.sigstore`, so first verification needs network access to the Sigstore TUF CDN.

**Files:** `.goreleaser.yaml`, `.github/workflows/release.yml`, `cmd/version_check.go:159,522–582`.

`gaia update` downloads the release tarball and `checksums.txt` and verifies the tarball's SHA-256
against that file (good against corruption). But `checksums.txt` itself is **unsigned** — anyone who
can tamper with the release (or MITM a client that trusts a bad CA) can serve a matching
tarball+checksums pair. There is no cosign/GPG/minisign verification.

Recommend: sign checksums with cosign (keyless/OIDC) in GoReleaser (`signs:` block) and have
`gaia update` verify the signature + embedded public key before trusting `checksums.txt`. Pin the
expected repo/identity. Same gap applies to the Ansible installer (see P1-6).

### P1-4 — Daemon binds `0.0.0.0:50051` by default — ✅ FIXED
**Status:** Resolved. Application default is now `127.0.0.1:50051` (config.go + template); network
exposure is an explicit opt-in. The Ansible role intentionally keeps `0.0.0.0` (it deploys a
network-facing server) with a comment pointing to the firewall variables.

**Files:** `config/config.go:102`, `deploy/ansible/roles/gaia/defaults/main.yml:67`.

Default listen address is all interfaces. For a secrets daemon the safer default is `127.0.0.1` (or
a Unix socket for local admin), with network exposure an explicit opt-in. mTLS mitigates but does
not eliminate the exposure (see P0-1, P1-2). At minimum, document and default-narrow; the Ansible
firewall role is off by default (`gaia_configure_firewall: false`), so an out-of-the-box deploy is
internet-reachable if the host is.

### P1-5 — Hardcoded default audit HMAC key in Ansible — ✅ FIXED
**Status:** Resolved. `roles/gaia/tasks/main.yml` now asserts (tag `always`) that
`gaia_audit_hmac_key` differs from the shipped default whenever audit logging is enabled; the play
fails with instructions (`openssl rand -hex 32`) otherwise.

**File:** `deploy/ansible/roles/gaia/defaults/main.yml:94`
(`gaia_audit_hmac_key: "change-me-in-production-..."`).

Ships a predictable default HMAC key. If unchanged, audit-log hashing of sensitive values is
forgeable/deanonymizable. Generate per-host at provision time (e.g. `lookup('password', ...)` or a
`openssl rand -hex 32` fact stored in the vault) and fail the play if left at the default.

### P1-6 — Ansible installs binary without integrity verification — ✅ FIXED
**Status:** Resolved. `install.yml` now downloads the release `checksums.txt`, extracts the
SHA-256 entry for the platform archive (failing the play if none is found), and passes it to
`get_url` as `checksum: sha256:...` so a tampered or corrupted download aborts the install.

**File:** `deploy/ansible/roles/gaia/tasks/install.yml:66–77`.

`get_url` downloads the tarball with no `checksum:` and unarchives it directly. A compromised/MITM'd
download is installed and run as a privileged service. Fetch `checksums.txt` (and, once P1-3 lands,
its signature), verify, then install. Also pin `gaia_version` in production rather than `latest`.

### P2-7 — TLS min version not pinned — ✅ FIXED
**Status:** Resolved. `MinVersion: tls.VersionTLS13` pinned in both the daemon server credentials
(`daemon/daemon.go`) and the CLI/TUI dial helper (`internal/grpcclient/client.go`).

**File:** `daemon/daemon.go:1431–1435`.

`tls.Config` sets `ClientAuth`, `Certificates`, `ClientCAs` but not `MinVersion`. Pin
`MinVersion: tls.VersionTLS13` (all components are Go and you control both ends) to drop legacy
downgrade surface.

### P2-8 — Policy wildcard matching lacks a path boundary — ✅ FIXED
**Status:** Resolved. A bare trailing `*` now completes a single path segment only ("app*" matches
"app2" but no longer "app-admin/prod/key"); "prefix/*" subtree grants unchanged. New table cases in
`policy/policy_test.go`. Note: any policy that relied on "app*" as a subtree grant now fails closed
and must be rewritten as "app/*".

**File:** `policy/policy.go:167–170`.

The single-trailing-`*` branch does a bare `strings.HasPrefix(requestedPath, prefix)`. A rule for
`app*` matches `app-admin/prod/key`, over-granting to sibling namespaces that share a prefix.
Restrict single-`*` to a segment boundary, or document that only `.../ *` (slash-star) is supported
and validate rule paths accordingly.

### P2-9 — CA validity is 10× the configured cert validity — ✅ FIXED
**Status:** Resolved. New explicit `tls.ca_expiry_days` config (default 3650); `generateCA` no
longer multiplies. Configs without the field fall back to 10× `cert_expiry_days` for compatibility.

**File:** `certs/generate.go:45` (`validityDays*10`).

With the 365-day default the CA lives ~10 years, silently. Make CA validity an explicit config
value rather than a hidden multiplier, so operators can reason about root-of-trust lifetime.

### P2-10 — Client RSA server/leaf keys are 2048-bit; CA is 4096 — ✅ DOCS RECONCILED
**Status:** Documentation fixed — CLAUDE.md no longer claims Ed25519 and now documents the actual
algorithms (RSA-4096 CA, RSA-2048 server, ECDSA P-256 clients) and real config keys. Migrating the
CA/server to ECDSA end-to-end remains a deliberate future decision (breaking: existing CA loading
is RSA-typed and re-issuing the root invalidates all issued certs).

### P3-11 — `gaia exec` injects secrets into subprocess environment
**File:** `cmd/exec.go:126,142`. Expected for the feature, but env vars are readable via
`/proc/<pid>/environ` by same-user processes and may leak into child process dumps. Document the
trade-off; consider a file/`--env-file` or stdin delivery option.

---

## 3. Deploy: install & update

### Current state
- **systemd (Ansible template)** `roles/gaia/templates/gaia.service.j2` is genuinely well hardened:
  `ProtectSystem=strict`, `NoNewPrivileges`, empty capability set, `SystemCallFilter=@system-service`,
  `MemoryDenyWriteExecute`, `RestrictAddressFamilies`. Good.
- **Self-update** (`gaia update`) is thoughtful: refuses to downgrade/replace dev builds without
  `--force`, atomic install via temp file + `os.Rename`, checksum verification.
- **Ansible** role covers install/upgrade, certs, firewall, backups, logrotate, docker ACLs.

### Findings
- **D1 (P1) — The checked-in `deploy/systemd/gaia.service` is NOT hardened. — ✅ FIXED**
  The unit now carries the full hardening set from the Ansible template (ProtectSystem=strict,
  NoNewPrivileges, empty capability set, syscall filter, etc.), with a comment noting the two files
  must stay in sync.
- **D2 (P2) — Config path mismatch. — ✅ FIXED** The unit now uses
  `--config /etc/gaia/gaia-config.yaml`, matching the application default and the Ansible role.
- **D3 (P2) — Update playbook: no rollback on failed upgrade. — ✅ FIXED** `install.yml` now backs
  up the current binary to `<path>.bak` before upgrading, wraps stop→install→`gaia version` verify
  in a `block`, and on failure the `rescue` restores the previous binary, restarts the service, and
  fails the play with a clear message.
- **D4 (P2) — Docker image hygiene. — ✅ FIXED** Both Dockerfiles pin `alpine:3.22`;
  `Dockerfile.daemon` adds a TCP `HEALTHCHECK` on 50051 and uses the canonical
  `/etc/gaia/gaia-config.yaml` path; compose now binds `127.0.0.1:50051`, mounts config read-only,
  drops all capabilities, and sets `no-new-privileges`. Bonus: both Dockerfiles' build stages were
  actually **broken** (they copied the root `go.mod`, a different module, and built `./apps/gaia`
  across a nested-module boundary) — they now build inside the `apps/gaia` module. Distroless/
  `scratch` was deliberately skipped to keep the busybox healthcheck probe and debug shell.
- **D5 (P3) — No dependency vulnerability scanning in CI. — ✅ PARTLY FIXED** `ci.yml` now runs
  `govulncheck ./...` after `go vet`. SBOM generation and `go mod verify` in the release path remain
  optional follow-ups.
- **D6 (P3) — Backups (`gaia-backup.sh.j2`, `createBackup`) are unencrypted `.bak` copies of the
  encrypted DB.** That's fine (data is encrypted at rest) but the backup dir permissions and
  retention should be asserted (0700, owned by gaia) and documented, since the file still contains
  the salt + key-hash + ciphertext.

---

## 4. Suggested implementation order

1. **P0-1** admin-service authorization interceptor + separate admin trust (with tests). *Blocker.*
2. **P1-3 / P1-6** sign release checksums (cosign) and verify in both `gaia update` and Ansible.
3. **P1-2** revoked-cert deny-list interceptor; short cert lifetimes.
4. **P1-4 / P1-5** default listen to loopback; generate per-host audit HMAC, fail on default.
5. **D1 / D2** ship the hardened checked-in systemd unit and fix the config path.
6. **P2 batch**: pin `MinVersion: TLS1.3`, policy wildcard boundary, explicit CA validity, Docker
   hardening, `govulncheck` in CI.
7. **Quality**: split `daemon.go`, remove the `"client found"` error-string control flow, extract
   `loadKeyMaterial`, stop silently stripping `\x00`.

---

## 5. Quick wins (low effort, high value)
- Pin `tls.MinVersion` (one line).
- Fix `deploy/systemd/gaia.service` hardening + config path (copy the Ansible template).
- Fail Ansible if `gaia_audit_hmac_key` is unchanged (one `assert`).
- Add `checksum:` to the Ansible `get_url`.
- Pin Docker base image and bind compose port to `127.0.0.1`.
