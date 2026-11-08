-- +goose Up
CREATE TABLE teams (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL COLLATE NOCASE UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE team_members (
    team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    PRIMARY KEY (team_id, user_id)
);

CREATE TABLE team_servers (
    team_id      TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    server_id    TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    access_level TEXT NOT NULL CHECK (access_level IN ('read', 'write')),
    created_at   TEXT NOT NULL,
    PRIMARY KEY (team_id, server_id)
);

CREATE INDEX idx_team_members_user ON team_members(user_id);
CREATE INDEX idx_team_servers_server ON team_servers(server_id);

-- +goose Down
DROP TABLE team_servers;
DROP TABLE team_members;
DROP TABLE teams;
