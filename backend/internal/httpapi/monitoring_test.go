package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/awbuilds/drovik/backend/internal/monitoring"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

func TestServerOverviewHTTPFlow(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()

	app := newTestApp(t)
	setup := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin",
		Password: "correct horse battery staple",
	}, "")
	if setup.status != http.StatusCreated {
		t.Fatalf("setup: %d %s", setup.status, setup.body)
	}
	csrf := decodeAuth(t, setup.body).CSRFToken

	host, portText, _ := net.SplitHostPort(sshServer.Addr)
	port, _ := strconv.Atoi(portText)
	created := app.request(t, http.MethodPost, "/api/v1/servers", createServerRequest{
		DisplayName: "Metrics host",
		Host:        host,
		Port:        port,
		Username:    "test",
		AuthMethod:  "password",
		Password:    sshServer.Password,
	}, csrf)
	if created.status != http.StatusCreated {
		t.Fatalf("create: %d %s", created.status, created.body)
	}
	var server serverResponse
	if err := json.Unmarshal(created.body, &server); err != nil {
		t.Fatalf("decode server: %v", err)
	}

	overviewURL := "/api/v1/servers/" + server.ID + "/overview"
	untrusted := app.request(t, http.MethodGet, overviewURL, nil, "")
	if untrusted.status != http.StatusConflict {
		t.Fatalf("expected host-key conflict, got %d %s", untrusted.status, untrusted.body)
	}
	hostKeyError := decodeError(t, untrusted.body)
	if hostKeyError.Error.Code != CodeSSHHostKeyUnknown {
		t.Fatalf("unexpected error: %s", untrusted.body)
	}
	details := hostKeyError.Error.Details.(map[string]any)
	trusted := app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/trust-host-key", trustHostKeyRequest{
		Fingerprint: details["fingerprint"].(string),
		KeyType:     details["key_type"].(string),
		PublicKey:   details["public_key"].(string),
	}, csrf)
	if trusted.status != http.StatusNoContent {
		t.Fatalf("trust: %d %s", trusted.status, trusted.body)
	}

	result := app.request(t, http.MethodGet, overviewURL, nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("overview: %d %s", result.status, result.body)
	}
	var snapshot monitoring.Snapshot
	if err := json.Unmarshal(result.body, &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.Host.Hostname != "drovik-test" || snapshot.Host.Kernel != "6.8.0-drovik-test" {
		t.Fatalf("unexpected host: %+v", snapshot.Host)
	}
	if snapshot.CPU.UsagePercent != nil {
		t.Fatalf("first sample must not invent CPU usage: %+v", snapshot.CPU.UsagePercent)
	}
	if snapshot.Memory.TotalBytes == 0 || len(snapshot.Filesystems) == 0 || len(snapshot.Network) == 0 {
		t.Fatalf("incomplete snapshot: %+v", snapshot)
	}

	cached := app.request(t, http.MethodGet, overviewURL, nil, "")
	if cached.status != http.StatusOK {
		t.Fatalf("cached overview: %d %s", cached.status, cached.body)
	}
	var cachedSnapshot monitoring.Snapshot
	if err := json.Unmarshal(cached.body, &cachedSnapshot); err != nil {
		t.Fatalf("decode cached snapshot: %v", err)
	}
	if !cachedSnapshot.Cached {
		t.Fatal("expected the second overview inside the refresh interval to be cached")
	}
}
