-- +goose Up
ALTER TABLE users ADD COLUMN mfa_secret TEXT;
ALTER TABLE users ADD COLUMN mfa_enabled INTEGER NOT NULL DEFAULT 0 CHECK (mfa_enabled IN (0, 1));

CREATE TABLE mfa_recovery_codes (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT NOT NULL,
    used_at    TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_mfa_recovery_user ON mfa_recovery_codes(user_id);

-- +goose Down
DROP TABLE mfa_recovery_codes;
ALTER TABLE users DROP COLUMN mfa_enabled;
ALTER TABLE users DROP COLUMN mfa_secret;
