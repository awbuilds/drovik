-- +goose Up
ALTER TABLE servers ADD COLUMN secret_provider TEXT NOT NULL DEFAULT 'stored';
ALTER TABLE servers ADD COLUMN secret_ref TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE servers DROP COLUMN secret_ref;
ALTER TABLE servers DROP COLUMN secret_provider;
