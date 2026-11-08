package advanced

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/awbuilds/drovik/backend/internal/monitoring"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

type HistoryPoint struct {
	CollectedAt   time.Time `json:"collected_at"`
	CPUPercent    *float64  `json:"cpu_percent"`
	MemoryPercent *float64  `json:"memory_percent"`
	DiskPercent   *float64  `json:"disk_percent"`
	LoadOne       *float64  `json:"load_one"`
	RXBps         *float64  `json:"rx_bps"`
	TXBps         *float64  `json:"tx_bps"`
}

type AlertState struct {
	RuleID     string  `json:"rule_id"`
	Metric     string  `json:"metric"`
	Comparator string  `json:"comparator"`
	Threshold  float64 `json:"threshold"`
	Value      float64 `json:"value"`
	Active     bool    `json:"active"`
}

type HistoryResult struct {
	ServerID  string         `json:"server_id"`
	Since     string         `json:"since"`
	Samples   []HistoryPoint `json:"samples"`
	Alerts    []AlertState   `json:"alerts"`
	Retention string         `json:"retention"`
}

type AlertRuleInput struct {
	Metric     string  `json:"metric"`
	Comparator string  `json:"comparator"`
	Threshold  float64 `json:"threshold"`
	Enabled    bool    `json:"enabled"`
}

var historyRanges = map[string]time.Duration{
	"1h": time.Hour, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour,
}

func (s *Service) RecordSnapshot(ctx context.Context, snapshot *monitoring.Snapshot) {
	if snapshot == nil || snapshot.Stale {
		return
	}
	sample := storage.MetricSample{ServerID: snapshot.ServerID, CollectedAt: snapshot.CollectedAt}
	sample.CPUPercent = snapshot.CPU.UsagePercent
	sample.MemoryPercent = snapshot.Memory.UsedPercent
	if snapshot.Load.Available {
		value := snapshot.Load.One
		sample.LoadOne = &value
	}
	if snapshot.FilesystemsAvailable && len(snapshot.Filesystems) > 0 {
		maxUsed := 0.0
		for _, filesystem := range snapshot.Filesystems {
			if filesystem.UsedPercent != nil && *filesystem.UsedPercent > maxUsed {
				maxUsed = *filesystem.UsedPercent
			}
		}
		sample.DiskPercent = &maxUsed
	}
	if snapshot.NetworkAvailable {
		var rx, tx float64
		for _, iface := range snapshot.Network {
			if iface.ReceiveRateBps != nil {
				rx += *iface.ReceiveRateBps
			}
			if iface.TransmitRateBps != nil {
				tx += *iface.TransmitRateBps
			}
		}
		sample.RXBps, sample.TXBps = &rx, &tx
	}
	if err := s.metrics.Insert(ctx, sample); err != nil {
		s.logger.Warn("metric sample insert failed", "error", err, "server_id", snapshot.ServerID)
		return
	}
	cutoff := time.Now().UTC().Add(-s.cfg.MetricsRetention)
	if err := s.metrics.Prune(ctx, snapshot.ServerID, cutoff); err != nil {
		s.logger.Warn("metric sample prune failed", "error", err, "server_id", snapshot.ServerID)
	}
	if err := s.metrics.PruneExcess(ctx, snapshot.ServerID, s.cfg.MetricsMaxSamples); err != nil {
		s.logger.Warn("metric sample cap failed", "error", err, "server_id", snapshot.ServerID)
	}
}

func (s *Service) History(ctx context.Context, ownerID, serverID, rangeName string) (*HistoryResult, error) {
	window, ok := historyRanges[rangeName]
	if !ok {
		return nil, fmt.Errorf("%w: unsupported history range", ErrInvalidTarget)
	}
	if _, err := s.servers.Get(ctx, ownerID, serverID); err != nil {
		return nil, err
	}
	since := time.Now().UTC().Add(-window)
	samples, err := s.metrics.List(ctx, serverID, since, 2000)
	if err != nil {
		return nil, err
	}
	result := &HistoryResult{
		ServerID: serverID, Since: rangeName, Retention: s.cfg.MetricsRetention.String(),
		Samples: make([]HistoryPoint, 0, len(samples)),
	}
	for _, sample := range samples {
		result.Samples = append(result.Samples, HistoryPoint{
			CollectedAt: sample.CollectedAt, CPUPercent: sample.CPUPercent,
			MemoryPercent: sample.MemoryPercent, DiskPercent: sample.DiskPercent,
			LoadOne: sample.LoadOne, RXBps: sample.RXBps, TXBps: sample.TXBps,
		})
	}
	rules, err := s.alerts.List(ctx, serverID)
	if err != nil {
		return nil, err
	}
	latest := latestSample(samples)
	result.Alerts = evaluateAlerts(rules, latest)
	return result, nil
}

func (s *Service) ListAlertRules(ctx context.Context, ownerID, serverID string) ([]storage.AlertRule, error) {
	if _, err := s.servers.Get(ctx, ownerID, serverID); err != nil {
		return nil, err
	}
	return s.alerts.List(ctx, serverID)
}

func (s *Service) SaveAlertRule(ctx context.Context, ownerID, serverID, ruleID string, input AlertRuleInput) (*storage.AlertRule, error) {
	if _, err := s.servers.Get(ctx, ownerID, serverID); err != nil {
		return nil, err
	}
	if input.Metric != "cpu_percent" && input.Metric != "memory_percent" && input.Metric != "disk_percent" && input.Metric != "load_one" {
		return nil, fmt.Errorf("%w: unsupported alert metric", ErrInvalidTarget)
	}
	if input.Comparator != "gt" && input.Comparator != "gte" {
		return nil, fmt.Errorf("%w: unsupported comparator", ErrInvalidTarget)
	}
	if input.Threshold < 0 || input.Threshold > 100000 {
		return nil, fmt.Errorf("%w: threshold is out of range", ErrInvalidTarget)
	}
	now := time.Now().UTC()
	if ruleID == "" {
		ruleID = storage.NewID()
	}
	rule := storage.AlertRule{
		ID: ruleID, ServerID: serverID, Metric: input.Metric, Comparator: input.Comparator,
		Threshold: input.Threshold, Enabled: input.Enabled, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.alerts.Upsert(ctx, rule); err != nil {
		return nil, err
	}
	s.record(ctx, ownerID, serverID, "alert.rule_saved", input.Metric, "success", "")
	return &rule, nil
}

func (s *Service) DeleteAlertRule(ctx context.Context, ownerID, serverID, ruleID string) error {
	if _, err := s.servers.Get(ctx, ownerID, serverID); err != nil {
		return err
	}
	if err := s.alerts.Delete(ctx, serverID, ruleID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrSnippetNotFound
		}
		return err
	}
	s.record(ctx, ownerID, serverID, "alert.rule_deleted", ruleID, "success", "")
	return nil
}

func latestSample(samples []storage.MetricSample) *storage.MetricSample {
	if len(samples) == 0 {
		return nil
	}
	return &samples[len(samples)-1]
}

func evaluateAlerts(rules []storage.AlertRule, sample *storage.MetricSample) []AlertState {
	result := make([]AlertState, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		state := AlertState{
			RuleID: rule.ID, Metric: rule.Metric, Comparator: rule.Comparator, Threshold: rule.Threshold,
		}
		if sample != nil {
			value := metricValue(rule.Metric, sample)
			if value != nil {
				state.Value = *value
				if rule.Comparator == "gt" {
					state.Active = *value > rule.Threshold
				} else {
					state.Active = *value >= rule.Threshold
				}
			}
		}
		result = append(result, state)
	}
	return result
}

func metricValue(metric string, sample *storage.MetricSample) *float64 {
	switch metric {
	case "cpu_percent":
		return sample.CPUPercent
	case "memory_percent":
		return sample.MemoryPercent
	case "disk_percent":
		return sample.DiskPercent
	case "load_one":
		return sample.LoadOne
	default:
		return nil
	}
}
