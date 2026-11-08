# Stable error codes

Every API error uses the envelope below. Codes are stable; messages may be reworded.

```json
{
  "error": {
    "code": "SSH_HOST_KEY_UNKNOWN",
    "message": "The SSH host key is not trusted yet.",
    "correlation_id": "…",
    "details": {}
  }
}
```

`details` is optional and safe to display. Stack traces, credentials, file contents, terminal
output and log contents are never included.

## Authentication and request handling

| Code | HTTP | Meaning |
|---|---|---|
| `AUTH_REQUIRED` | 401 | No valid session cookie |
| `AUTH_INVALID_CREDENTIALS` | 401 | Username or password rejected |
| `AUTH_SESSION_EXPIRED` | 401 | Session expired or revoked |
| `AUTH_FORBIDDEN` | 403 | Authenticated but not allowed for this resource |
| `CSRF_INVALID` | 403 | Missing or invalid `X-CSRF-Token` |
| `VALIDATION_ERROR` | 422 | Request failed validation; `details.field` may be present |
| `SETUP_ALREADY_COMPLETE` | 409 | First-run setup already performed |
| `RATE_LIMITED` | 429 | Authentication or input rate limit exceeded |
| `NOT_FOUND` | 404 | Generic resource not found |
| `METHOD_NOT_ALLOWED` | 405 | Method not supported for this route |
| `PAYLOAD_TOO_LARGE` | 413 | Request body exceeded the route limit |
| `UNSUPPORTED_MEDIA_TYPE` | 415 | Expected JSON or multipart content |
| `CONFLICT` | 409 | Generic state conflict |
| `SERVICE_UNAVAILABLE` | 503 | Process is shutting down |
| `INTERNAL_ERROR` | 500 | Unexpected server error; correlation ID is logged |
| `CAPABILITY_UNAVAILABLE` | 422 | The host lacks the required capability |

## SSH and servers

| Code | HTTP | Meaning |
|---|---|---|
| `SERVER_NOT_FOUND` | 404 | Saved server does not exist or is inactive |
| `SSH_CONNECTION_FAILED` | 502 | Dial, handshake or command failed |
| `SSH_HOST_KEY_UNKNOWN` | 409 | Key not approved; `details` carries the fingerprint and public key |
| `SSH_HOST_KEY_CHANGED` | 409 | Presented key differs; `details` carries previous and presented keys |
| `SSH_AUTH_FAILED` | 422 | Password/private key rejected |
| `SSH_OPERATION_TIMEOUT` | 504 | Dial, handshake or remote operation timed out |

Host-key errors are deliberate: the operator must verify and submit the exact fingerprint.

## Files (SFTP)

| Code | HTTP | Meaning |
|---|---|---|
| `SFTP_UNAVAILABLE` | 422 | Host does not expose the SFTP subsystem |
| `SFTP_PERMISSION_DENIED` | 403 | Remote account cannot perform the operation |
| `FILE_TOO_LARGE` | 413 | Upload or editor content exceeds the configured cap |
| `FILE_BINARY` | 422 | Editor refused a binary or non-UTF-8 file |
| `FILE_STALE` | 409 | Remote mtime/size changed since the editor loaded it |
| `FILE_CONFLICT` | 409 | Target exists and overwrite was not requested |
| `SYMLINK_NOT_ALLOWED` | 422 | The action would follow a symlink |
| `IS_DIRECTORY` | 422 | A file operation targeted a directory |

## Monitoring

| Code | HTTP | Meaning |
|---|---|---|
| `CAPABILITY_UNAVAILABLE` | 422 | `/proc` metrics are unreadable |
| `SSH_CONNECTION_FAILED` | 502 | Metrics collection failed |
| `SSH_OPERATION_TIMEOUT` | 504 | Metrics collection timed out |

A successful response may still contain `stale: true` and a warning when the last snapshot is
shown. That is a degraded success, not an API error.

## Processes

| Code | HTTP | Meaning |
|---|---|---|
| `PROCESS_UNAVAILABLE` | 422 | `ps` is missing or lacks the required format |
| `PROCESS_NOT_FOUND` | 404 | Process disappeared |
| `PROCESS_PERMISSION_DENIED` | 403 | Remote account cannot signal the process |
| `PROCESS_PID_REUSED` | 409 | Start time changed; no signal was sent |
| `PROCESS_ACTION_FAILED` | 502 | Enumerated signal command failed |

## Services and logs

| Code | HTTP | Meaning |
|---|---|---|
| `SERVICE_MANAGER_UNAVAILABLE` | 422 | Host is not running systemd or the bus is unreachable |
| `SERVICE_NOT_FOUND` | 404 | Unit not found or not loaded |
| `SERVICE_PERMISSION_DENIED` | 403 | PolicyKit/systemd denied the action |
| `SERVICE_ACTION_FAILED` | 502 | Enumerated action failed |
| `JOURNAL_PERMISSION_DENIED` | 403 | Account cannot read the journal |
| `CAPABILITY_UNAVAILABLE` | 422 | `journalctl` is unavailable |

Service action success responses include `state_known`; if false, the action completed but Drovik
could not re-read the unit state and the UI must not claim a specific result.
