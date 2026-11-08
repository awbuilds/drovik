// Package vault resolves SSH credentials from a HashiCorp Vault KV v2 mount.
// Credentials are fetched at connect time and are never written to the Drovik
// database. Only the Vault path is stored.
package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/security"
)

const (
	maxResponseBytes = 1 << 20
	maxPasswordBytes = 1024
	maxPrivateKey    = 64 << 10
)

var (
	ErrDisabled    = errors.New("vault is disabled")
	ErrInvalidRef  = errors.New("invalid Vault secret reference")
	ErrNotFound    = errors.New("vault secret was not found")
	ErrUnavailable = errors.New("vault is unavailable")
)

var refPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_.-]{0,255}$`)

type Secret struct {
	Password   string
	PrivateKey string
	Passphrase string
}

type Client struct {
	address   string
	mount     string
	token     string
	namespace string
	http      *http.Client
}

func New(cfg *config.Config) (*Client, error) {
	if !cfg.VaultEnabled {
		return nil, ErrDisabled
	}
	token := strings.TrimSpace(cfg.VaultToken)
	if token == "" && cfg.VaultTokenFile != "" {
		raw, err := os.ReadFile(cfg.VaultTokenFile)
		if err != nil {
			return nil, fmt.Errorf("read Vault token file: %w", err)
		}
		token = strings.TrimSpace(string(raw))
	}
	if token == "" {
		return nil, errors.New("vault token is empty")
	}
	policy := security.NewEgressPolicy(cfg.EgressAllowPrivate, cfg.EgressAllowedPrefixes, cfg.EgressDeniedPrefixes)
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
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}
	timeout := cfg.VaultTimeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &Client{
		address:   strings.TrimRight(cfg.VaultAddress, "/"),
		mount:     cfg.VaultMount,
		token:     token,
		namespace: cfg.VaultNamespace,
		http: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// Get fetches one KV v2 secret. The reference is a path inside the configured
// mount, for example "drovik/hosts/prod-web-01".
func (c *Client) Get(ctx context.Context, ref string) (*Secret, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	if !ValidRef(ref) {
		return nil, ErrInvalidRef
	}
	endpoint := c.address + "/v1/" + url.PathEscape(c.mount) + "/data/" + ref
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Vault-Token", c.token)
	request.Header.Set("Accept", "application/json")
	if c.namespace != "" {
		request.Header.Set("X-Vault-Namespace", c.namespace)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed", ErrUnavailable)
	}
	defer func() { _ = response.Body.Close() }()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusForbidden, http.StatusUnauthorized:
		return nil, fmt.Errorf("%w: Vault denied the request", ErrUnavailable)
	default:
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, response.StatusCode)
	}
	var payload struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%w: malformed response", ErrUnavailable)
	}
	secret := &Secret{
		Password:   stringField(payload.Data.Data, "password"),
		PrivateKey: stringField(payload.Data.Data, "private_key"),
		Passphrase: stringField(payload.Data.Data, "passphrase"),
	}
	if secret.Password == "" && secret.PrivateKey == "" {
		return nil, fmt.Errorf("%w: neither password nor private_key is present", ErrNotFound)
	}
	if len(secret.Password) > maxPasswordBytes || len(secret.PrivateKey) > maxPrivateKey || len(secret.Passphrase) > maxPasswordBytes {
		return nil, fmt.Errorf("%w: secret exceeds the size limit", ErrInvalidRef)
	}
	return secret, nil
}

func ValidRef(ref string) bool {
	return refPattern.MatchString(ref) && !strings.Contains(ref, "..") && !strings.Contains(ref, "//")
}

func stringField(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}
