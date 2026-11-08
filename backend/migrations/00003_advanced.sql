-- +goose Up
CREATE TABLE server_assignments (
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    server_id    TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    access_level TEXT NOT NULL DEFAULT 'write' CHECK (access_level IN ('read', 'write')),
    created_at   TEXT NOT NULL,
    PRIMARY KEY (user_id, server_id)
);

CREATE INDEX idx_server_assignments_server ON server_assignments(server_id);

CREATE TABLE snippets (
    id            TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    command       TEXT NOT NULL,
    variables     TEXT NOT NULL DEFAULT '[]',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE INDEX idx_snippets_owner ON snippets(owner_user_id);

CREATE TABLE metric_samples (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id       TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    collected_at    TEXT NOT NULL,
    cpu_percent     REAL,
    memory_percent  REAL,
    disk_percent    REAL,
    load_one        REAL,
    rx_bps          REAL,
    tx_bps          REAL
);

CREATE INDEX idx_metric_samples_server_time ON metric_samples(server_id, collected_at);

CREATE TABLE alert_rules (
    id             TEXT PRIMARY KEY,
    server_id      TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    metric         TEXT NOT NULL CHECK (metric IN ('cpu_percent', 'memory_percent', 'disk_percent', 'load_one')),
    comparator     TEXT NOT NULL CHECK (comparator IN ('gt', 'gte')),
    threshold      REAL NOT NULL,
    enabled        INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);

CREATE INDEX idx_alert_rules_server ON alert_rules(server_id);

-- +goose Down
DROP TABLE alert_rules;
DROP TABLE metric_samples;
DROP TABLE snippets;
DROP TABLE server_assignments;
