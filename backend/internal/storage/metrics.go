package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type MetricSample struct {
	ServerID      string
	CollectedAt   time.Time
	CPUPercent    *float64
	MemoryPercent *float64
	DiskPercent   *float64
	LoadOne       *float64
	RXBps         *float64
	TXBps         *float64
}

type MetricRepository struct {
	db *sql.DB
}

func NewMetricRepository(db *sql.DB) *MetricRepository {
	return &MetricRepository{db: db}
}

func (r *MetricRepository) Insert(ctx context.Context, sample MetricSample) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO metric_samples (
			server_id, collected_at, cpu_percent, memory_percent, disk_percent,
			load_one, rx_bps, tx_bps
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id, collected_at) DO NOTHING`,
		sample.ServerID, formatTime(sample.CollectedAt), sample.CPUPercent,
		sample.MemoryPercent, sample.DiskPercent, sample.LoadOne, sample.RXBps, sample.TXBps,
	)
	if err != nil {
		return fmt.Errorf("insert metric sample: %w", err)
	}
	return nil
}

func (r *MetricRepository) List(ctx context.Context, serverID string, since time.Time, limit int) ([]MetricSample, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT server_id, collected_at, cpu_percent, memory_percent, disk_percent,
		       load_one, rx_bps, tx_bps
		FROM metric_samples
		WHERE server_id = ? AND collected_at >= ?
		ORDER BY collected_at DESC LIMIT ?`,
		serverID, formatTime(since), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list metric samples: %w", err)
	}
	defer func() { _ = rows.Close() }()

	samples := make([]MetricSample, 0, limit)
	for rows.Next() {
		var (
			sample    MetricSample
			collected string
			cpu       sql.NullFloat64
			memory    sql.NullFloat64
			disk      sql.NullFloat64
			load      sql.NullFloat64
			rx        sql.NullFloat64
			tx        sql.NullFloat64
		)
		if err := rows.Scan(
			&sample.ServerID, &collected, &cpu, &memory, &disk, &load, &rx, &tx,
		); err != nil {
			return nil, fmt.Errorf("scan metric sample: %w", err)
		}
		sample.CollectedAt = parseTime(collected)
		sample.CPUPercent = nullableFloat(cpu)
		sample.MemoryPercent = nullableFloat(memory)
		sample.DiskPercent = nullableFloat(disk)
		sample.LoadOne = nullableFloat(load)
		sample.RXBps = nullableFloat(rx)
		sample.TXBps = nullableFloat(tx)
		samples = append(samples, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metric samples: %w", err)
	}
	for left, right := 0, len(samples)-1; left < right; left, right = left+1, right-1 {
		samples[left], samples[right] = samples[right], samples[left]
	}
	return samples, nil
}

func (r *MetricRepository) Prune(ctx context.Context, serverID string, before time.Time) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM metric_samples WHERE server_id = ? AND collected_at < ?`,
		serverID, formatTime(before),
	); err != nil {
		return fmt.Errorf("prune metric samples: %w", err)
	}
	return nil
}

func (r *MetricRepository) PruneAll(ctx context.Context, before time.Time) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM metric_samples WHERE collected_at < ?`, formatTime(before),
	); err != nil {
		return fmt.Errorf("prune all metric samples: %w", err)
	}
	return nil
}

func (r *MetricRepository) Count(ctx context.Context, serverID string) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM metric_samples WHERE server_id = ?`, serverID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count metric samples: %w", err)
	}
	return count, nil
}

type AlertRule struct {
	ID               string
	ServerID         string
	Metric           string
	Comparator       string
	Threshold        float64
	Enabled          bool
	WebhookEncrypted string
	LastNotifiedAt   *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type AlertRepository struct {
	db *sql.DB
}

func NewAlertRepository(db *sql.DB) *AlertRepository {
	return &AlertRepository{db: db}
}

func (r *AlertRepository) Upsert(ctx context.Context, rule AlertRule) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO alert_rules (
			id, server_id, metric, comparator, threshold, enabled,
			webhook_encrypted, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			metric = excluded.metric, comparator = excluded.comparator,
			threshold = excluded.threshold, enabled = excluded.enabled,
			webhook_encrypted = COALESCE(excluded.webhook_encrypted, alert_rules.webhook_encrypted),
			updated_at = excluded.updated_at
		WHERE alert_rules.server_id = excluded.server_id`,
		rule.ID, rule.ServerID, rule.Metric, rule.Comparator, rule.Threshold,
		boolToInt(rule.Enabled), nullableText(rule.WebhookEncrypted),
		formatTime(rule.CreatedAt), formatTime(rule.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert alert rule: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count alert rule changes: %w", err)
	}
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AlertRepository) List(ctx context.Context, serverID string) ([]AlertRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, server_id, metric, comparator, threshold, enabled,
		       webhook_encrypted, last_notified_at, created_at, updated_at
		FROM alert_rules WHERE server_id = ? ORDER BY created_at`, serverID)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	rules := make([]AlertRule, 0, 8)
	for rows.Next() {
		var (
			rule       AlertRule
			enabled    int
			webhook    sql.NullString
			notifiedAt sql.NullString
			createdAt  string
			updatedAt  string
		)
		if err := rows.Scan(
			&rule.ID, &rule.ServerID, &rule.Metric, &rule.Comparator, &rule.Threshold,
			&enabled, &webhook, &notifiedAt, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		rule.Enabled = enabled == 1
		rule.WebhookEncrypted = webhook.String
		if notifiedAt.Valid {
			value := parseTime(notifiedAt.String)
			rule.LastNotifiedAt = &value
		}
		rule.CreatedAt = parseTime(createdAt)
		rule.UpdatedAt = parseTime(updatedAt)
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert rules: %w", err)
	}
	return rules, nil
}

func (r *AlertRepository) Delete(ctx context.Context, serverID, id string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM alert_rules WHERE id = ? AND server_id = ?`, id, serverID)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted alert rules: %w", err)
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AlertRepository) MarkNotified(ctx context.Context, serverID, id string, at time.Time) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE alert_rules SET last_notified_at = ?
		WHERE id = ? AND server_id = ?`,
		formatTime(at), id, serverID,
	); err != nil {
		return fmt.Errorf("mark alert notified: %w", err)
	}
	return nil
}

func nullableFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

func (r *MetricRepository) PruneExcess(ctx context.Context, serverID string, maxSamples int) error {
	if maxSamples <= 0 {
		return nil
	}
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM metric_samples
		WHERE server_id = ? AND id NOT IN (
			SELECT id FROM metric_samples WHERE server_id = ?
			ORDER BY collected_at DESC LIMIT ?
		)`, serverID, serverID, maxSamples,
	); err != nil {
		return fmt.Errorf("prune excess metric samples: %w", err)
	}
	return nil
}
