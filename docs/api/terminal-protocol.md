# Terminal WebSocket Protocol

**Endpoint:** `GET /api/v1/servers/{id}/terminal` (HTTP Upgrade to WebSocket)
**Authentication:** the same `drovik_session` cookie as the HTTP API.
**Origin:** required and validated against `DROVIK_TRUSTED_ORIGINS`.
**Transport:** text frames containing JSON envelopes.

## Client → server

```json
{ "type": "input", "data": "ls -la\n" }
{ "type": "resize", "cols": 120, "rows": 40 }
{ "type": "close" }
```

- `input.data` is written only to the SSH PTY stdin. Drovik does not rewrite commands.
- `resize` validates `cols`/`rows` to 1–1000 and calls the SSH window change API.
- `close` ends the terminal and releases the SSH channel.

## Server → client

```json
{ "type": "status", "state": "connected", "data": "deploy@server.example.com" }
{ "type": "output", "data": "$ " }
{ "type": "status", "state": "closed" }
{ "type": "error", "code": "SSH_HOST_KEY_UNKNOWN", "message": "Verify this server's host key..." }
```

- Output is chunked to keep individual messages bounded.
- `error` is followed by a normal close. Stable codes include `SSH_HOST_KEY_UNKNOWN`,
  `SSH_HOST_KEY_CHANGED`, `SSH_AUTH_FAILED`, `SSH_CONNECTION_FAILED`,
  `SSH_CHANNEL_FAILURE`, `RESOURCE_LIMIT_REACHED`, `RATE_LIMITED` and `INVALID_MESSAGE`.

## Limits and lifecycle

- Read limit: 64 KiB per WebSocket message.
- Input rate: 200 messages per second per session; excess input closes the session.
- Sessions are bounded per user and globally by `DROVIK_SSH_MAX_SESSIONS_PER_USER` and
  `DROVIK_SSH_MAX_SESSIONS_GLOBAL`.
- The SSH client, PTY channel, stdin/stdout/stderr pipes and WebSocket are closed on explicit
  close, browser disconnect, logout/session expiry, idle timeout or backend shutdown.
- Terminal input/output is **not recorded** by default. Nothing is written to the database.
- The remote shell runs with the permissions of the configured SSH user. Drovik does not use sudo.
