package advanced

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/secrets"
	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

const (
	webhookCooldown    = 15 * time.Minute
	webhookMaxResponse = 4 << 10
)

type webhookPayload struct {
	ServerID   string    `json:"server_id"`
	RuleID     string    `json:"rule_id"`
	Metric     string    `json:"metric"`
	Comparator string    `json:"comparator"`
	Threshold  float64   `json:"threshold"`
	Value      float64   `json:"value"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (s *Service) validateWebhook(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return fmt.Errorf("%w: invalid webhook URL", ErrInvalidTarget)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: webhook URL must use http or https", ErrInvalidTarget)
	}
	if parsed.User != nil {
		return fmt.Errorf("%w: webhook URL must not contain credentials", ErrInvalidTarget)
	}
	return nil
}

func (s *Service) encryptWebhook(ruleID, raw string) (string, error) {
	encrypted, err := s.cipher.Encrypt([]byte(raw), []byte("alert_rule:"+ruleID+":webhook"))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s:%s", encrypted.Version,
		base64.StdEncoding.EncodeToString(encrypted.Nonce),
		base64.StdEncoding.EncodeToString(encrypted.Ciphertext)), nil
}

func (s *Service) decryptWebhook(ruleID, stored string) (string, error) {
	parts := strings.SplitN(stored, ":", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed webhook secret")
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("malformed webhook version")
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("malformed webhook nonce")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("malformed webhook ciphertext")
	}
	plain, err := s.cipher.Decrypt(secrets.Encrypted{
		Version: version, Nonce: nonce, Ciphertext: ciphertext,
	}, []byte("alert_rule:"+ruleID+":webhook"))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *Service) webhookClient() *http.Client {
	policy := security.NewEgressPolicy(s.cfg.EgressAllowPrivate, s.cfg.EgressAllowedPrefixes, s.cfg.EgressDeniedPrefixes)
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			resolved, err := policy.Resolve(ctx, host)
			if err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
		},
		ResponseHeaderTimeout: 5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
	}
	return &http.Client{
		Timeout:   8 * time.Second,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (s *Service) notifyAlerts(ctx context.Context, serverID string, sample storage.MetricSample) {
	rules, err := s.alerts.List(ctx, serverID)
	if err != nil {
		s.logger.Warn("alert notification list failed", "error", err, "server_id", serverID)
		return
	}
	client := s.webhookClient()
	now := time.Now().UTC()
	for _, rule := range rules {
		if !rule.Enabled || rule.WebhookEncrypted == "" {
			continue
		}
		if rule.LastNotifiedAt != nil && now.Sub(*rule.LastNotifiedAt) < webhookCooldown {
			continue
		}
		value := metricValue(rule.Metric, &sample)
		if value == nil || !comparisonActive(rule.Comparator, *value, rule.Threshold) {
			continue
		}
		endpoint, err := s.decryptWebhook(rule.ID, rule.WebhookEncrypted)
		if err != nil {
			s.logger.Warn("alert webhook decrypt failed", "error", err, "rule_id", rule.ID)
			continue
		}
		payload := webhookPayload{
			ServerID: serverID, RuleID: rule.ID, Metric: rule.Metric,
			Comparator: rule.Comparator, Threshold: rule.Threshold,
			Value: *value, OccurredAt: sample.CollectedAt,
		}
		// Claim the cooldown before delivery so overlapping snapshots cannot
		// double-notify the same rule. A failed delivery is logged and retried
		// after the cooldown instead of storming the endpoint.
		if err := s.alerts.MarkNotified(ctx, serverID, rule.ID, now); err != nil {
			s.logger.Warn("alert notification mark failed", "error", err, "rule_id", rule.ID)
			continue
		}
		if err := s.postWebhook(ctx, client, endpoint, payload); err != nil {
			s.logger.Warn("alert webhook delivery failed", "error", err, "rule_id", rule.ID)
		}
	}
}

func (s *Service) postWebhook(ctx context.Context, client *http.Client, endpoint string, payload webhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Drovik-Alert/1")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, webhookMaxResponse))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", response.StatusCode)
	}
	return nil
}

func comparisonActive(comparator string, value, threshold float64) bool {
	if comparator == "gte" {
		return value >= threshold
	}
	return value > threshold
}
