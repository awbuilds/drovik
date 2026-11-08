# 2026-11-08 Phase 9 alert webhooks

## Task / Area
Add external alert notifications through per-rule webhooks.

## Current State
- Migration `00006_notifications.sql`: encrypted webhook URL and `last_notified_at` on alert rules.
- `internal/advanced/notifications.go`: URL validation (http/https, no credentials), AES-GCM storage
  with AAD `alert_rule:<id>:webhook`, egress-policy DNS resolution before dialing, no redirects,
  8 s timeout, 4 KiB response drain and a 15-minute per-rule cooldown claimed before delivery.
- `RecordSnapshot` triggers delivery asynchronously through `safe.Go` with a detached 30 s context.
- UI: alert-rule form accepts a webhook URL and shows a `webhook` badge; the URL is never returned.
- Tests: encrypted storage, invalid-scheme rejection, payload delivery and cooldown suppression.

## Decisions
- Webhook URLs are secrets and are never returned by the API; only `webhook_configured` is exposed.
- Delivery uses the same egress policy as SSH, so loopback/link-local/metadata are blocked by
  default and private networks remain usable in self-hosted deployments.
- A failed delivery waits for the cooldown rather than retrying in a storm.

## Build / Test Status
- `go test -race ./internal/advanced -run TestAlertWebhookDelivery` passes.
- `make verify` is run before merge.

## Next Steps
- Email/paging integrations, OIDC/SSO, destructive storage/network actions and non-Linux support.
