-- +goose Up
CREATE TABLE servers (
    id                   TEXT PRIMARY KEY,
    owner_user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    display_name         TEXT NOT NULL,
    host                 TEXT NOT NULL,
    port                 INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    username             TEXT NOT NULL,
    auth_method          TEXT NOT NULL CHECK (auth_method IN ('password', 'private_key')),
    encrypted_secret     TEXT NOT NULL,
    secret_nonce         TEXT NOT NULL,
    secret_version       INTEGER NOT NULL DEFAULT 1,
    encrypted_passphrase TEXT,
    passphrase_nonce     TEXT,
    passphrase_version   INTEGER,
    tags                 TEXT NOT NULL DEFAULT '',
    notes                TEXT NOT NULL DEFAULT '',
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL,
    last_connected_at    TEXT,
    is_active            INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1))
);

CREATE INDEX idx_servers_owner ON servers(owner_user_id);

CREATE TABLE known_hosts (
    id          TEXT PRIMARY KEY,
    server_id   TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    host        TEXT NOT NULL,
    port        INTEGER NOT NULL,
    key_type    TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    public_key  TEXT NOT NULL,
    approved_at TEXT NOT NULL,
    approved_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    revoked_at  TEXT
);

CREATE UNIQUE INDEX idx_known_hosts_active ON known_hosts(server_id) WHERE revoked_at IS NULL;

CREATE TABLE server_capabilities (
    server_id   TEXT PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
    data        TEXT NOT NULL,
    detected_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE server_capabilities;
DROP TABLE known_hosts;
DROP TABLE servers;
