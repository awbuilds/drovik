-- +goose Up
ALTER TABLE alert_rules ADD COLUMN webhook_encrypted TEXT;
ALTER TABLE alert_rules ADD COLUMN last_notified_at TEXT;

-- +goose Down
ALTER TABLE alert_rules DROP COLUMN last_notified_at;
ALTER TABLE alert_rules DROP COLUMN webhook_encrypted;
