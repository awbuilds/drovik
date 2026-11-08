package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/storage"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

type testApp struct {
	server *httptest.Server
	client *http.Client
	api    *Server
	db     *sql.DB
}

type testResponse struct {
	status int
	body   []byte
	header http.Header
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.Migrate(context.Background(), db); err != nil {
		t.Fatalf("storage.Migrate: %v", err)
	}

	staticDir := filepath.Join(dir, "static")
	if err := os.MkdirAll(filepath.Join(staticDir, "assets"), 0o700); err != nil {
		t.Fatalf("create static dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<!doctype html><title>Drovik test</title>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "assets", "app.js"), []byte("console.log('drovik')"), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	publicURL, _ := url.Parse("http://example.com")
	cfg := &config.Config{
		Env:                     config.EnvTest,
		HTTPAddr:                "127.0.0.1:0",
		PublicURL:               publicURL,
		TrustedOrigins:          []string{"http://example.com"},
		LogLevel:                "error",
		LogFormat:               "text",
		DataDir:                 dir,
		DBPath:                  filepath.Join(dir, "test.db"),
		StaticDir:               staticDir,
		EncryptionKey:           []byte("0123456789abcdef0123456789abcdef"),
		SessionIdleTTL:          time.Hour,
		SessionAbsoluteTTL:      24 * time.Hour,
		CookieSecure:            false,
		LoginRateLimit:          5,
		LoginRateWindow:         time.Minute,
		ShutdownTimeout:         time.Second,
		ReadinessTimeout:        time.Second,
		SSHDialTimeout:          5 * time.Second,
		SSHHandshakeTimeout:     5 * time.Second,
		SSHOperationTimeout:     5 * time.Second,
		EgressAllowPrivate:      true,
		EgressAllowedPrefixes:   []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
		SSHMaxSessionsPerUser:   2,
		SSHMaxSessionsGlobal:    10,
		SFTPMaxUploadBytes:      1 << 20,
		SFTPMaxEditorBytes:      64 << 10,
		SFTPMaxListEntries:      100,
		SFTPMaxRecursiveEntries: 100,
		SFTPOperationTimeout:    10 * time.Second,
		MonitorRefreshInterval:  10 * time.Millisecond,
		MonitorStaleAfter:       time.Second,
		MonitorTimeout:          5 * time.Second,
		MonitorMaxOutputBytes:   256 * 1024,
		MonitorMaxFilesystems:   64,
		MonitorMaxInterfaces:    64,
		OpsCommandTimeout:       5 * time.Second,
		ProcessMaxEntries:       1000,
		ServiceMaxUnits:         500,
		LogMaxLines:             500,
		LogMaxOutputBytes:       1 << 20,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	auditService := audit.New(storage.NewAuditRepository(db), logger)
	api, err := New(cfg, logger, db, auditService)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	server := httptest.NewServer(api.Handler())
	t.Cleanup(server.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	return &testApp{server: server, client: client, api: api, db: db}
}

func (a *testApp) request(t *testing.T, method, path string, body any, csrf string) testResponse {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, a.server.URL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Origin", "http://example.com")
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response, err := a.client.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return testResponse{status: response.StatusCode, body: data, header: response.Header.Clone()}
}

func doRaw(client *http.Client, request *http.Request) (testResponse, error) {
	response, err := client.Do(request)
	if err != nil {
		return testResponse{}, err
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return testResponse{}, err
	}
	return testResponse{status: response.StatusCode, body: data, header: response.Header.Clone()}, nil
}

func decodeAuth(t *testing.T, data []byte) authResponse {
	t.Helper()
	var response authResponse
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("decode auth response: %v (%s)", err, string(data))
	}
	return response
}

func decodeError(t *testing.T, data []byte) errorEnvelope {
	t.Helper()
	var response errorEnvelope
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("decode error response: %v (%s)", err, string(data))
	}
	return response
}

func TestSetupLoginLogoutFlow(t *testing.T) {
	app := newTestApp(t)

	result := app.request(t, http.MethodGet, "/api/v1/setup/status", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("setup status: %d %s", result.status, result.body)
	}
	var status setupStatusResponse
	if err := json.Unmarshal(result.body, &status); err != nil || !status.Required {
		t.Fatalf("expected setup required, got %s (%v)", result.body, err)
	}

	result = app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin",
		Password: "correct horse battery staple",
	}, "")
	if result.status != http.StatusCreated {
		t.Fatalf("setup: %d %s", result.status, result.body)
	}
	setup := decodeAuth(t, result.body)
	if setup.User.Username != "admin" || setup.CSRFToken == "" {
		t.Fatalf("unexpected setup response: %+v", setup)
	}

	result = app.request(t, http.MethodGet, "/api/v1/setup/status", nil, "")
	_ = json.Unmarshal(result.body, &status)
	if status.Required {
		t.Fatal("setup should no longer be required")
	}

	result = app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "second",
		Password: "correct horse battery staple",
	}, "")
	if result.status != http.StatusForbidden {
		t.Fatalf("second setup: %d %s", result.status, result.body)
	}
	if decodeError(t, result.body).Error.Code != CodeSetupAlreadyComplete {
		t.Fatalf("unexpected second setup code: %s", result.body)
	}

	result = app.request(t, http.MethodGet, "/api/v1/auth/me", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("me: %d %s", result.status, result.body)
	}
	me := decodeAuth(t, result.body)
	if me.User.Username != "admin" || me.CSRFToken == "" {
		t.Fatalf("unexpected me response: %+v", me)
	}

	result = app.request(t, http.MethodPost, "/api/v1/auth/logout", nil, "")
	if result.status != http.StatusForbidden {
		t.Fatalf("logout without csrf: %d %s", result.status, result.body)
	}
	if decodeError(t, result.body).Error.Code != CodeCSRFInvalid {
		t.Fatalf("unexpected csrf code: %s", result.body)
	}

	result = app.request(t, http.MethodPost, "/api/v1/auth/logout", nil, me.CSRFToken)
	if result.status != http.StatusNoContent {
		t.Fatalf("logout: %d %s", result.status, result.body)
	}

	result = app.request(t, http.MethodGet, "/api/v1/auth/me", nil, "")
	if result.status != http.StatusUnauthorized {
		t.Fatalf("me after logout: %d %s", result.status, result.body)
	}
}

func TestLoginAndRateLimit(t *testing.T) {
	app := newTestApp(t)
	app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin",
		Password: "correct horse battery staple",
	}, "")

	for i := 0; i < 5; i++ {
		result := app.request(t, http.MethodPost, "/api/v1/auth/login", loginRequest{
			Username: "admin",
			Password: "wrong password",
		}, "")
		if result.status != http.StatusUnauthorized {
			t.Fatalf("wrong login %d: %d %s", i, result.status, result.body)
		}
	}

	result := app.request(t, http.MethodPost, "/api/v1/auth/login", loginRequest{
		Username: "admin",
		Password: "wrong password",
	}, "")
	if result.status != http.StatusTooManyRequests {
		t.Fatalf("expected rate limit, got %d %s", result.status, result.body)
	}
	if decodeError(t, result.body).Error.Code != CodeRateLimited {
		t.Fatalf("unexpected rate limit code: %s", result.body)
	}
}

func TestLoginSuccess(t *testing.T) {
	app := newTestApp(t)
	app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin",
		Password: "correct horse battery staple",
	}, "")

	result := app.request(t, http.MethodPost, "/api/v1/auth/login", loginRequest{
		Username: "admin",
		Password: "correct horse battery staple",
	}, "")
	if result.status != http.StatusOK {
		t.Fatalf("login: %d %s", result.status, result.body)
	}
	login := decodeAuth(t, result.body)
	if login.User.Username != "admin" || login.CSRFToken == "" {
		t.Fatalf("unexpected login response: %+v", login)
	}
}

func TestOriginAndContentTypeValidation(t *testing.T) {
	app := newTestApp(t)

	request, err := http.NewRequest(http.MethodPost, app.server.URL+"/api/v1/setup",
		strings.NewReader(`{"username":"admin","password":"correct horse battery staple"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://evil.example")
	result, err := doRaw(app.client, request)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if result.status != http.StatusForbidden {
		t.Fatalf("expected origin rejection, got %d", result.status)
	}

	request, err = http.NewRequest(http.MethodPost, app.server.URL+"/api/v1/setup",
		strings.NewReader(`{"username":"admin","password":"correct horse battery staple"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "text/plain")
	request.Header.Set("Origin", "http://example.com")
	result, err = doRaw(app.client, request)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if result.status != http.StatusUnsupportedMediaType {
		t.Fatalf("expected content type rejection, got %d", result.status)
	}
}

func TestPanicRecovery(t *testing.T) {
	app := newTestApp(t)
	router := chi.NewRouter()
	router.Use(app.api.requestID)
	router.Use(app.api.accessLog)
	router.Use(app.api.recoverer)
	router.Get("/panic", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	router.Get("/ok", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	request, err := http.NewRequest(http.MethodGet, server.URL+"/panic", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	result, err := doRaw(server.Client(), request)
	if err != nil {
		t.Fatalf("panic request: %v", err)
	}
	if result.status != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d %s", result.status, result.body)
	}
	if !strings.Contains(string(result.body), CodeInternal) {
		t.Fatalf("expected internal error code, got %s", result.body)
	}

	request, err = http.NewRequest(http.MethodGet, server.URL+"/ok", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	result, err = doRaw(server.Client(), request)
	if err != nil {
		t.Fatalf("post-panic request: %v", err)
	}
	if result.status != http.StatusOK {
		t.Fatalf("server did not survive panic: %d", result.status)
	}
}

func TestReadiness(t *testing.T) {
	app := newTestApp(t)
	result := app.request(t, http.MethodGet, "/readyz", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("readyz: %d %s", result.status, result.body)
	}
	if err := app.db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	result = app.request(t, http.MethodGet, "/readyz", nil, "")
	if result.status != http.StatusServiceUnavailable {
		t.Fatalf("readyz after db close: %d %s", result.status, result.body)
	}
}

func TestVersionAndSecurityHeaders(t *testing.T) {
	app := newTestApp(t)

	result := app.request(t, http.MethodGet, "/api/v1/version", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("version: %d %s", result.status, result.body)
	}
	var version versionResponse
	if err := json.Unmarshal(result.body, &version); err != nil || version.SchemaVersion < 1 {
		t.Fatalf("unexpected version response: %s (%v)", result.body, err)
	}
	if got := result.header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("missing nosniff header: %q", got)
	}
	if got := result.header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("missing frame denial header: %q", got)
	}
	if got := result.header.Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") {
		t.Fatalf("unexpected CSP: %q", got)
	}
}

func TestStaticSPA(t *testing.T) {
	app := newTestApp(t)

	result := app.request(t, http.MethodGet, "/", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("index: %d %s", result.status, result.body)
	}
	if !strings.Contains(string(result.body), "Drovik test") {
		t.Fatalf("unexpected index body: %s", result.body)
	}
	if got := result.header.Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("unexpected index content type: %q", got)
	}

	result = app.request(t, http.MethodGet, "/settings", nil, "")
	if result.status != http.StatusOK || !strings.Contains(string(result.body), "Drovik test") {
		t.Fatalf("SPA route: %d %s", result.status, result.body)
	}

	result = app.request(t, http.MethodGet, "/assets/app.js", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("asset: %d %s", result.status, result.body)
	}
	if got := result.header.Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("unexpected asset cache policy: %q", got)
	}

	result = app.request(t, http.MethodGet, "/api/v1/does-not-exist", nil, "")
	if result.status != http.StatusNotFound {
		t.Fatalf("api 404: %d %s", result.status, result.body)
	}
	if decodeError(t, result.body).Error.Code != CodeNotFound {
		t.Fatalf("unexpected api 404 body: %s", result.body)
	}
}

func TestSetupRateLimit(t *testing.T) {
	app := newTestApp(t)
	app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin",
		Password: "correct horse battery staple",
	}, "")

	for i := 0; i < 4; i++ {
		result := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
			Username: "second",
			Password: "correct horse battery staple",
		}, "")
		if result.status != http.StatusForbidden {
			t.Fatalf("setup attempt %d: %d %s", i, result.status, result.body)
		}
	}
	result := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "second",
		Password: "correct horse battery staple",
	}, "")
	if result.status != http.StatusTooManyRequests {
		t.Fatalf("expected setup rate limit, got %d %s", result.status, result.body)
	}
}

func TestServerRoutesWithSSHServer(t *testing.T) {
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

	host, portText, err := net.SplitHostPort(sshServer.Addr)
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}
	port, _ := strconv.Atoi(portText)

	result := app.request(t, http.MethodPost, "/api/v1/servers", createServerRequest{
		DisplayName: "Test host",
		Host:        host,
		Port:        port,
		Username:    "test",
		AuthMethod:  "password",
		Password:    sshServer.Password,
	}, "")
	if result.status != http.StatusForbidden {
		t.Fatalf("expected CSRF rejection, got %d %s", result.status, result.body)
	}

	result = app.request(t, http.MethodPost, "/api/v1/servers", createServerRequest{
		DisplayName: "Test host",
		Host:        host,
		Port:        port,
		Username:    "test",
		AuthMethod:  "password",
		Password:    sshServer.Password,
	}, csrf)
	if result.status != http.StatusCreated {
		t.Fatalf("create server: %d %s", result.status, result.body)
	}
	if strings.Contains(string(result.body), sshServer.Password) {
		t.Fatal("server response contains the SSH password")
	}
	var created serverResponse
	if err := json.Unmarshal(result.body, &created); err != nil {
		t.Fatalf("decode server: %v", err)
	}
	if created.DisplayName != "Test host" || created.AuthMethod != "password" {
		t.Fatalf("unexpected server: %+v", created)
	}

	result = app.request(t, http.MethodPost, "/api/v1/servers/"+created.ID+"/test", nil, csrf)
	if result.status != http.StatusConflict {
		t.Fatalf("expected unknown host key, got %d %s", result.status, result.body)
	}
	hostKeyError := decodeError(t, result.body)
	if hostKeyError.Error.Code != CodeSSHHostKeyUnknown {
		t.Fatalf("unexpected host key error: %s", result.body)
	}
	details, ok := hostKeyError.Error.Details.(map[string]any)
	if !ok {
		t.Fatalf("unexpected host key details: %s", result.body)
	}
	fingerprint, _ := details["fingerprint"].(string)
	keyType, _ := details["key_type"].(string)
	publicKey, _ := details["public_key"].(string)
	if fingerprint == "" || keyType == "" || publicKey == "" {
		t.Fatalf("incomplete host key details: %s", result.body)
	}

	result = app.request(t, http.MethodPost, "/api/v1/servers/"+created.ID+"/trust-host-key", trustHostKeyRequest{
		Fingerprint: fingerprint,
		KeyType:     keyType,
		PublicKey:   publicKey,
	}, csrf)
	if result.status != http.StatusNoContent {
		t.Fatalf("trust host key: %d %s", result.status, result.body)
	}

	result = app.request(t, http.MethodPost, "/api/v1/servers/"+created.ID+"/test", nil, csrf)
	if result.status != http.StatusOK {
		t.Fatalf("test connection: %d %s", result.status, result.body)
	}
	var tested connectionTestResponse
	if err := json.Unmarshal(result.body, &tested); err != nil {
		t.Fatalf("decode connection test: %v", err)
	}
	if tested.Capabilities.OS != "Linux" || !tested.Capabilities.SFTP {
		t.Fatalf("unexpected capabilities: %+v", tested.Capabilities)
	}
	if tested.HostKey.Fingerprint != fingerprint {
		t.Fatalf("unexpected tested host key: %+v", tested.HostKey)
	}

	result = app.request(t, http.MethodGet, "/api/v1/servers", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("list servers: %d %s", result.status, result.body)
	}

	result = app.request(t, http.MethodGet, "/api/v1/servers/"+created.ID+"/capabilities", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("capabilities: %d %s", result.status, result.body)
	}

	result = app.request(t, http.MethodDelete, "/api/v1/servers/"+created.ID, nil, csrf)
	if result.status != http.StatusNoContent {
		t.Fatalf("delete server: %d %s", result.status, result.body)
	}
}
