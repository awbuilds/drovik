package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

func TestNonLinuxHostsRejectWriteRoutes(t *testing.T) {
	app := newTestApp(t)
	setup := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin", Password: "correct horse battery staple",
	}, "")
	if setup.status != http.StatusCreated {
		t.Fatalf("setup: %d %s", setup.status, setup.body)
	}
	csrf := decodeAuth(t, setup.body).CSRFToken
	created := app.request(t, http.MethodPost, "/api/v1/servers", createServerRequest{
		DisplayName: "Mac host", Host: "192.0.2.30", Port: 22, Username: "deploy",
		AuthMethod: "password", Password: "test-only",
	}, csrf)
	if created.status != http.StatusCreated {
		t.Fatalf("create server: %d %s", created.status, created.body)
	}
	var server serverResponse
	if err := json.Unmarshal(created.body, &server); err != nil {
		t.Fatalf("decode server: %v", err)
	}
	capabilities, _ := json.Marshal(sshclient.Capabilities{OS: "Darwin", Architecture: "arm64", SFTP: true})
	if err := storage.NewCapabilitiesRepository(app.db).Upsert(
		t.Context(), server.ID, string(capabilities), time.Now().UTC(),
	); err != nil {
		t.Fatalf("store capabilities: %v", err)
	}

	blocked := app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/packages/action", packageActionRequest{
		Package: "curl", Action: "install", Confirm: true,
	}, csrf)
	if blocked.status != http.StatusForbidden {
		t.Fatalf("expected non-Linux write rejection, got %d %s", blocked.status, blocked.body)
	}
	envelope := decodeError(t, blocked.body)
	if envelope.Error.Code != CodeCapabilityUnavailable {
		t.Fatalf("expected CAPABILITY_UNAVAILABLE, got %s", envelope.Error.Code)
	}

	// A Linux capability record must not be rejected by the read-only gate.
	linux, _ := json.Marshal(sshclient.Capabilities{OS: "Linux", Architecture: "amd64", SFTP: true})
	if err := storage.NewCapabilitiesRepository(app.db).Upsert(
		t.Context(), server.ID, string(linux), time.Now().UTC(),
	); err != nil {
		t.Fatalf("store Linux capabilities: %v", err)
	}
	allowed := app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/packages/action", packageActionRequest{
		Package: "curl", Action: "install", Confirm: true,
	}, csrf)
	if allowed.status == http.StatusForbidden && decodeError(t, allowed.body).Error.Code == CodeCapabilityUnavailable {
		t.Fatalf("Linux host was wrongly blocked: %d %s", allowed.status, allowed.body)
	}
}
