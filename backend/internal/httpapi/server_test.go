package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/storage"
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

	publicURL, _ := url.Parse("http://example.com")
	cfg := &config.Config{
		Env:                config.EnvTest,
		HTTPAddr:           "127.0.0.1:0",
		PublicURL:          publicURL,
		TrustedOrigins:     []string{"http://example.com"},
		LogLevel:           "error",
		LogFormat:          "text",
		DataDir:            dir,
		DBPath:             filepath.Join(dir, "test.db"),
		StaticDir:          filepath.Join(dir, "static"),
		EncryptionKey:      []byte("0123456789abcdef0123456789abcdef"),
		SessionIdleTTL:     time.Hour,
		SessionAbsoluteTTL: 24 * time.Hour,
		CookieSecure:       false,
		LoginRateLimit:     5,
		LoginRateWindow:    time.Minute,
		ShutdownTimeout:    time.Second,
		ReadinessTimeout:   time.Second,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	auditService := audit.New(storage.NewAuditRepository(db), logger)
	api := New(cfg, logger, db, auditService)

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
	if !strings.Contains(string(result.body), `"schema_version":1`) {
		t.Fatalf("unexpected version response: %s", result.body)
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
