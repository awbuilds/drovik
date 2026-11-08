# 2026-11-08 Phase 9 Vault KV v2 credentials

## Task / Area
Add optional external secret-manager credentials for saved SSH servers.

## Current State
- Config: `DROVIK_VAULT_ENABLED`, `DROVIK_VAULT_ADDR`, `DROVIK_VAULT_TOKEN_FILE` (or
  `DROVIK_VAULT_TOKEN`), `DROVIK_VAULT_NAMESPACE`, `DROVIK_VAULT_MOUNT` (default `secret`) and
  `DROVIK_VAULT_TIMEOUT`.
- `internal/vault`: KV v2 `GET /v1/<mount>/data/<path>` with a bounded timeout, no redirects, the
  shared egress policy and a 1 MiB response cap. Returns `password`, `private_key` and optional
  `passphrase`; refuses oversized or malformed secrets.
- Migration `00008_vault.sql`: `servers.secret_provider` and `servers.secret_ref`. Stored
  credentials keep the existing AES-GCM columns; Vault-backed servers store an empty ciphertext and
  only the path.
- Create/update accepts `secret_provider=stored|vault` and `secret_ref`; paths are validated.
  Connect and connection tests fetch the secret at connect time. Vault failures map to
  `SECRET_UNAVAILABLE` and fail closed.
- UI: credential-source selector and Vault path field in the server dialog, plus a `vault` badge.
- Tests: Vault client (path, headers, KV v2 shape, invalid refs, 404), service-level create/connect
  with a mock Vault and SSH host, and config validation.

## Decisions
- Only HashiCorp Vault KV v2 is supported. No new dependency.
- Secrets are never cached or written to SQLite; each connection resolves the current value, so a
  rotated or revoked Vault secret takes effect immediately.
- The Vault address is operator configuration and is resolved through the same egress policy as
  SSH and webhooks.
- Additional secret-manager backends remain out of scope.

## Build / Test Status
- `go test -race ./internal/vault ./internal/servers ./internal/config` passes.
- `make verify` is run before merge.
- Not run: a real Vault server or a real Vault-backed SSH host.

## Next Steps
- Site/release refresh, then the final full security audit of the expanded tree.
