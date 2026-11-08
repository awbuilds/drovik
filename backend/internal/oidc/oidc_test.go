package oidc

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/config"
)

func TestAuthorizationCodeFlow(t *testing.T) {
	var server *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize",
			"token_endpoint": server.URL + "/token", "userinfo_endpoint": server.URL + "/userinfo",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "good-code" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "access-token", "token_type": "Bearer"})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub": "subject-1", "preferred_username": "admin", "email": "admin@example.com", "email_verified": true,
		})
	})
	server = httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{
		OIDCEnabled: true, OIDCIssuer: server.URL, OIDCClientID: "client",
		OIDCClientSecret: "secret", OIDCRedirectURL: "http://app.example/callback",
	}
	provider, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	authURL, err := provider.Begin("login", "")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	state := parsed.Query().Get("state")
	if state == "" || parsed.Query().Get("code_challenge") == "" || parsed.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("unexpected authorization URL: %s", authURL)
	}
	result, err := provider.Complete(context.Background(), state, "good-code")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if result.Mode != "login" || result.UserID != "" {
		t.Fatalf("unexpected result mode: %+v", result)
	}
	if result.Identity.Subject != "subject-1" || result.Identity.Username != "admin" || !result.Identity.EmailVerified {
		t.Fatalf("unexpected identity: %+v", result.Identity)
	}
	if _, err := provider.Complete(context.Background(), state, "good-code"); err != ErrInvalidState {
		t.Fatalf("state must be single-use, got %v", err)
	}
	if _, err := provider.Complete(context.Background(), "missing", "good-code"); err != ErrInvalidState {
		t.Fatalf("expected invalid state, got %v", err)
	}
}

func TestLinkModeRequiresUser(t *testing.T) {
	provider := &Provider{pending: make(map[string]pendingState), cfg: &config.Config{OIDCEnabled: true}}
	provider.pending["state"] = pendingState{verifier: "v", mode: "link", expiresAt: time.Now().Add(time.Minute)}
	if _, err := provider.Complete(context.Background(), "state", "code"); err != ErrInvalidState {
		t.Fatalf("link mode without a user must be rejected, got %v", err)
	}
}
