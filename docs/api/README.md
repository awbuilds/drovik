# Drovik API

The versioned contract lives in [`openapi.yaml`](openapi.yaml) (OpenAPI 3.1).

Phase 1 endpoints:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/setup/status` | Whether first-run setup is required |
| `POST` | `/api/v1/setup` | Create the initial administrator |
| `POST` | `/api/v1/auth/login` | Authenticate |
| `GET` | `/api/v1/auth/me` | Current user and CSRF token |
| `POST` | `/api/v1/auth/logout` | Revoke the current session |
| `GET` | `/api/v1/version` | Version and schema metadata |
| `GET` | `/healthz` | Liveness |
| `GET` | `/readyz` | Readiness |

Cookie-authenticated mutations require the session-bound `X-CSRF-Token` returned by login, setup
or `/auth/me`. It is not stored in a readable cookie. Errors use the stable envelope described in
the specification.

The document is the intended source of truth for generated Go and TypeScript types. Generation
and contract tests land with the Phase 2 API expansion.
