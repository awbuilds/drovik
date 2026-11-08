-- +goose Up
ALTER TABLE users ADD COLUMN oidc_issuer TEXT;
ALTER TABLE users ADD COLUMN oidc_subject TEXT;

CREATE UNIQUE INDEX idx_users_oidc_identity
    ON users(oidc_issuer, oidc_subject)
    WHERE oidc_subject IS NOT NULL;

-- +goose Down
DROP INDEX idx_users_oidc_identity;
ALTER TABLE users DROP COLUMN oidc_subject;
ALTER TABLE users DROP COLUMN oidc_issuer;
