package vault

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/config"
)

func TestGetKVv2Secret(t *testing.T) {
	var token, namespace, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token = r.Header.Get("X-Vault-Token")
		namespace = r.Header.Get("X-Vault-Namespace")
		path = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"data": map[string]any{
				"password": "secret-password", "passphrase": "secret-passphrase",
			}},
		})
	}))
	defer server.Close()
	client, err := New(&config.Config{
		VaultEnabled: true, VaultAddress: server.URL, VaultToken: "token-1",
		VaultNamespace: "team-a", VaultMount: "secret", VaultTimeout: 2 * time.Second,
		EgressAllowPrivate: true, EgressAllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	secret, err := client.Get(context.Background(), "drovik/hosts/prod-web-01")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if secret.Password != "secret-password" || secret.Passphrase != "secret-passphrase" {
		t.Fatalf("unexpected secret: %+v", secret)
	}
	if token != "token-1" || namespace != "team-a" || path != "/v1/secret/data/drovik/hosts/prod-web-01" {
		t.Fatalf("unexpected request: token=%q namespace=%q path=%q", token, namespace, path)
	}
}

func TestGetRejectsInvalidRefAndMissingSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/secret/data/missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"data": map[string]any{"unused": "value"}},
		})
	}))
	defer server.Close()
	client, err := New(&config.Config{
		VaultEnabled: true, VaultAddress: server.URL, VaultToken: "token",
		VaultMount: "secret", VaultTimeout: 2 * time.Second,
		EgressAllowPrivate: true, EgressAllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, ref := range []string{"", "../etc", "/leading", "a//b", "a b"} {
		if _, err := client.Get(context.Background(), ref); err != ErrInvalidRef {
			t.Fatalf("expected %q to be rejected, got %v", ref, err)
		}
	}
	if _, err := client.Get(context.Background(), "missing"); err != ErrNotFound {
		t.Fatalf("expected not-found, got %v", err)
	}
	if _, err := client.Get(context.Background(), "empty"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing-field rejection, got %v", err)
	}
}

func TestNewRequiresToken(t *testing.T) {
	if _, err := New(&config.Config{VaultEnabled: true, VaultAddress: "https://vault.example.com"}); err == nil {
		t.Fatal("expected a missing-token error")
	}
	if _, err := New(&config.Config{}); err != ErrDisabled {
		t.Fatalf("expected disabled, got %v", err)
	}
}
