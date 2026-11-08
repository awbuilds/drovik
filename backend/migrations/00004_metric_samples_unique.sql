-- +goose Up
-- Collapse duplicate samples produced by coalesced overview requests before
-- enforcing one row per server and collection time.
DELETE FROM metric_samples
WHERE id NOT IN (
    SELECT MIN(id) FROM metric_samples GROUP BY server_id, collected_at
);

CREATE UNIQUE INDEX idx_metric_samples_server_time_unique
    ON metric_samples(server_id, collected_at);

-- +goose Down
DROP INDEX idx_metric_samples_server_time_unique;
