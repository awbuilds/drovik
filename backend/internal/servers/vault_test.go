package servers

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
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/testssh"
	"github.com/awbuilds/drovik/backend/internal/vault"
)

func TestVaultBackedConnection(t *testing.T) {
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
	client, err := vault.New(&config.Config{
		VaultEnabled: true, VaultAddress: vaultServer.URL, VaultToken: "vault-token",
		VaultMount: "secret", VaultTimeout: 2 * time.Second,
		EgressAllowPrivate: true, EgressAllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
	})
	if err != nil {
		t.Fatalf("vault.New: %v", err)
	}
	env := newServiceTestEnvWith(t, sshServer, client)

	input := testCreateInput(t, sshServer)
	input.Password = ""
	input.SecretProvider = SecretProviderVault
	input.SecretRef = "drovik/hosts/test"
	server, err := env.service.Create(context.Background(), env.users.ID, input)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if server.SecretProvider != SecretProviderVault || server.SecretRef != "drovik/hosts/test" {
		t.Fatalf("unexpected provider: %+v", server)
	}
	if server.EncryptedSecret != "" || server.SecretNonce != "" {
		t.Fatal("a Vault-backed server must not store a database credential")
	}
	stored, err := env.servers.GetByID(context.Background(), server.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.EncryptedSecret != "" {
		t.Fatal("Vault credential leaked into the database")
	}

	_, err = env.service.TestConnection(context.Background(), env.users.ID, server.ID)
	var unknown *sshclient.HostKeyUnknownError
	if !errors.As(err, &unknown) {
		t.Fatalf("expected unknown host key, got %v", err)
	}
	if err := env.service.ApproveHostKey(
		context.Background(), env.users.ID, server.ID,
		unknown.Info.Fingerprint, unknown.Info.KeyType, unknown.Info.PublicKey, false,
	); err != nil {
		t.Fatalf("ApproveHostKey: %v", err)
	}
	result, err := env.service.TestConnection(context.Background(), env.users.ID, server.ID)
	if err != nil {
		t.Fatalf("TestConnection with Vault credential: %v", err)
	}
	if result.Capabilities.OS == "" {
		t.Fatalf("expected capabilities, got %+v", result.Capabilities)
	}

	// A Vault failure must fail closed and map to the stable sentinel.
	vaultServer.Close()
	if _, err := env.service.TestConnection(context.Background(), env.users.ID, server.ID); !errors.Is(err, ErrSecretUnavailable) {
		t.Fatalf("expected secret resolution failure, got %v", err)
	}
}

func TestVaultValidation(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env := newServiceTestEnv(t, sshServer)

	input := testCreateInput(t, sshServer)
	input.Password = ""
	input.SecretProvider = SecretProviderVault
	input.SecretRef = "../etc/passwd"
	if _, err := env.service.Create(context.Background(), env.users.ID, input); err == nil {
		t.Fatal("expected an invalid Vault reference rejection")
	}
	input.SecretRef = "drovik/hosts/test"
	if _, err := env.service.Create(context.Background(), env.users.ID, input); err == nil {
		t.Fatal("expected Vault to be rejected when it is not configured")
	}
}
