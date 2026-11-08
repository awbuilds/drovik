// Package oidc implements the authorization-code flow with PKCE for a
// pre-configured OpenID Connect provider. Accounts are never auto-provisioned:
// an operator must create and link the local user first.
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/awbuilds/drovik/backend/internal/config"
)

var (
	ErrDisabled       = errors.New("OIDC is disabled")
	ErrInvalidState   = errors.New("OIDC state is invalid or expired")
	ErrExchange       = errors.New("OIDC code exchange failed")
	ErrNoIdentity     = errors.New("OIDC provider returned no usable identity")
	ErrIssuerMismatch = errors.New("OIDC discovery issuer mismatch")
)

type discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

type Identity struct {
	Subject           string
	Username          string
	Email             string
	EmailVerified     bool
	PreferredUsername string
}

type pendingState struct {
	verifier  string
	mode      string
	userID    string
	expiresAt time.Time
}

// Result is the outcome of a completed authorization. Mode is "login" or
// "link"; UserID is set for a link flow.
type Result struct {
	Identity *Identity
	Mode     string
	UserID   string
}

type Provider struct {
	cfg     *config.Config
	logger  *slog.Logger
	client  *http.Client
	meta    discovery
	mu      sync.Mutex
	pending map[string]pendingState
}

func New(cfg *config.Config, logger *slog.Logger) (*Provider, error) {
	if !cfg.OIDCEnabled {
		return nil, ErrDisabled
	}
	provider := &Provider{
		cfg:    cfg,
		logger: logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
			// An identity provider never needs to redirect discovery or token
			// requests. Refusing redirects avoids an open redirect turning into
			// an SSRF pivot.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		pending: make(map[string]pendingState),
	}
	discoveryURL := strings.TrimRight(cfg.OIDCIssuer, "/") + "/.well-known/openid-configuration"
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := provider.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery returned status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read OIDC discovery: %w", err)
	}
	if err := json.Unmarshal(body, &provider.meta); err != nil {
		return nil, fmt.Errorf("decode OIDC discovery: %w", err)
	}
	if strings.TrimRight(provider.meta.Issuer, "/") != strings.TrimRight(cfg.OIDCIssuer, "/") {
		return nil, ErrIssuerMismatch
	}
	if provider.meta.AuthorizationEndpoint == "" || provider.meta.TokenEndpoint == "" || provider.meta.UserinfoEndpoint == "" {
		return nil, errors.New("OIDC discovery is missing required endpoints")
	}
	return provider, nil
}

func (p *Provider) Enabled() bool { return p != nil && p.cfg.OIDCEnabled }

func (p *Provider) Begin(mode, userID string) (string, error) {
	if !p.Enabled() {
		return "", ErrDisabled
	}
	state, err := randomToken(32)
	if err != nil {
		return "", err
	}
	verifier, err := randomToken(48)
	if err != nil {
		return "", err
	}
	challenge := pkceChallenge(verifier)
	p.mu.Lock()
	p.pruneLocked()
	if len(p.pending) >= 1000 {
		p.mu.Unlock()
		return "", errors.New("too many pending OIDC authorizations")
	}
	p.pending[state] = pendingState{
		verifier: verifier, mode: mode, userID: userID,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	p.mu.Unlock()

	parsed, err := url.Parse(p.meta.AuthorizationEndpoint)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", p.cfg.OIDCClientID)
	query.Set("redirect_uri", p.cfg.OIDCRedirectURL)
	query.Set("scope", "openid profile email")
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (p *Provider) Complete(ctx context.Context, state, code string) (*Result, error) {
	if !p.Enabled() {
		return nil, ErrDisabled
	}
	p.mu.Lock()
	pending, ok := p.pending[state]
	delete(p.pending, state)
	p.mu.Unlock()
	if !ok || time.Now().After(pending.expiresAt) {
		return nil, ErrInvalidState
	}
	if pending.mode != "login" && pending.mode != "link" {
		return nil, ErrInvalidState
	}
	if pending.mode == "link" && pending.userID == "" {
		return nil, ErrInvalidState
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", p.cfg.OIDCRedirectURL)
	form.Set("client_id", p.cfg.OIDCClientID)
	form.Set("code_verifier", pending.verifier)
	if p.cfg.OIDCClientSecret != "" {
		form.Set("client_secret", p.cfg.OIDCClientSecret)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.meta.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExchange, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrExchange, response.StatusCode)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&token); err != nil || token.AccessToken == "" {
		return nil, fmt.Errorf("%w: no access token", ErrExchange)
	}
	infoRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, p.meta.UserinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	infoRequest.Header.Set("Authorization", "Bearer "+token.AccessToken)
	infoRequest.Header.Set("Accept", "application/json")
	infoResponse, err := p.client.Do(infoRequest)
	if err != nil {
		return nil, fmt.Errorf("OIDC userinfo: %w", err)
	}
	defer func() { _ = infoResponse.Body.Close() }()
	if infoResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC userinfo returned status %d", infoResponse.StatusCode)
	}
	var claims struct {
		Subject           string `json:"sub"`
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
	}
	if err := json.NewDecoder(io.LimitReader(infoResponse.Body, 1<<20)).Decode(&claims); err != nil {
		return nil, err
	}
	username := strings.TrimSpace(claims.PreferredUsername)
	if username == "" {
		username = strings.TrimSpace(claims.Email)
	}
	if claims.Subject == "" || username == "" {
		return nil, ErrNoIdentity
	}
	return &Result{
		Identity: &Identity{
			Subject: claims.Subject, Username: username, Email: claims.Email,
			EmailVerified: claims.EmailVerified, PreferredUsername: claims.PreferredUsername,
		},
		Mode: pending.mode, UserID: pending.userID,
	}, nil
}

func (p *Provider) pruneLocked() {
	now := time.Now()
	for key, value := range p.pending {
		if now.After(value.expiresAt) {
			delete(p.pending, key)
		}
	}
}

func randomToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
