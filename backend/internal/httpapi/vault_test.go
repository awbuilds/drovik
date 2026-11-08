package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

func TestVaultBackedServerHTTPFlow(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()

	vaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/secret/data/drovik/hosts/test" || r.Header.Get("X-Vault-Token") != "vault-token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"data": map[string]any{"password": sshServer.Password}},
		})
	}))
	defer vaultServer.Close()

	app := newTestAppWith(t, func(cfg *config.Config) {
		cfg.VaultEnabled = true
		cfg.VaultAddress = vaultServer.URL
		cfg.VaultToken = "vault-token"
		cfg.VaultMount = "secret"
		cfg.VaultTimeout = 2 * time.Second
	})
	setup := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin", Password: "correct horse battery staple",
	}, "")
	if setup.status != http.StatusCreated {
		t.Fatalf("setup: %d %s", setup.status, setup.body)
	}
	csrf := decodeAuth(t, setup.body).CSRFToken

	host, portText, err := net.SplitHostPort(sshServer.Addr)
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, _ := strconv.Atoi(portText)
	created := app.request(t, http.MethodPost, "/api/v1/servers", createServerRequest{
		DisplayName: "Vault host", Host: host, Port: port, Username: "test",
		AuthMethod: "password", SecretProvider: "vault", SecretRef: "drovik/hosts/test",
	}, csrf)
	if created.status != http.StatusCreated {
		t.Fatalf("create: %d %s", created.status, created.body)
	}
	var server serverResponse
	if err := json.Unmarshal(created.body, &server); err != nil {
		t.Fatalf("decode server: %v", err)
	}
	if server.SecretProvider != "vault" || server.SecretRef != "drovik/hosts/test" {
		t.Fatalf("unexpected provider: %+v", server)
	}
	var encrypted string
	if err := app.db.QueryRow(`SELECT encrypted_secret FROM servers WHERE id = ?`, server.ID).Scan(&encrypted); err != nil {
		t.Fatalf("read encrypted secret: %v", err)
	}
	if encrypted != "" {
		t.Fatal("Vault credential was written to the database")
	}

	tested := app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/test", nil, csrf)
	if tested.status != http.StatusConflict {
		t.Fatalf("expected host-key conflict, got %d %s", tested.status, tested.body)
	}
	details := decodeError(t, tested.body).Error.Details.(map[string]any)
	trusted := app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/trust-host-key", trustHostKeyRequest{
		Fingerprint: details["fingerprint"].(string),
		KeyType:     details["key_type"].(string),
		PublicKey:   details["public_key"].(string),
	}, csrf)
	if trusted.status != http.StatusNoContent {
		t.Fatalf("trust: %d %s", trusted.status, trusted.body)
	}
	result := app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/test", nil, csrf)
	if result.status != http.StatusOK {
		t.Fatalf("test connection: %d %s", result.status, result.body)
	}

	vaultServer.Close()
	failed := app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/test", nil, csrf)
	if failed.status != http.StatusBadGateway || decodeError(t, failed.body).Error.Code != CodeSecretUnavailable {
		t.Fatalf("expected SECRET_UNAVAILABLE, got %d %s", failed.status, failed.body)
	}
}
