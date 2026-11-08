package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

func TestOIDCLoginFlow(t *testing.T) {
	var provider *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 provider.URL,
			"authorization_endpoint": provider.URL + "/authorize",
			"token_endpoint":         provider.URL + "/token",
			"userinfo_endpoint":      provider.URL + "/userinfo",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		code := r.Form.Get("code")
		if r.Form.Get("grant_type") != "authorization_code" || (code != "good-code" && code != "stranger-code") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Form.Get("code_verifier") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "token-" + code, "token_type": "Bearer",
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer token-good-code":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub": "subject-1", "preferred_username": "admin",
				"email": "admin@example.com", "email_verified": true,
			})
		case "Bearer token-stranger-code":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub": "subject-2", "preferred_username": "stranger",
			})
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
	provider = httptest.NewServer(mux)
	defer provider.Close()

	app := newTestAppWith(t, func(cfg *config.Config) {
		cfg.OIDCEnabled = true
		cfg.OIDCIssuer = provider.URL
		cfg.OIDCClientID = "client"
		cfg.OIDCClientSecret = "secret"
		cfg.OIDCRedirectURL = "http://example.com/api/v1/auth/oidc/callback"
	})

	status := app.request(t, http.MethodGet, "/api/v1/auth/oidc/status", nil, "")
	if status.status != http.StatusOK || !strings.Contains(string(status.body), `"enabled":true`) {
		t.Fatalf("status: %d %s", status.status, status.body)
	}
	setup := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin", Password: "correct horse battery staple",
	}, "")
	if setup.status != http.StatusCreated {
		t.Fatalf("setup: %d %s", setup.status, setup.body)
	}
	csrf := decodeAuth(t, setup.body).CSRFToken

	linkStatus := app.request(t, http.MethodGet, "/api/v1/auth/oidc/link", nil, "")
	if linkStatus.status != http.StatusOK || !strings.Contains(string(linkStatus.body), `"linked":false`) {
		t.Fatalf("link status: %d %s", linkStatus.status, linkStatus.body)
	}
	linkStart := app.request(t, http.MethodPost, "/api/v1/auth/oidc/link", nil, csrf)
	if linkStart.status != http.StatusOK {
		t.Fatalf("link start: %d %s", linkStart.status, linkStart.body)
	}
	var linkBody struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(linkStart.body, &linkBody); err != nil {
		t.Fatalf("decode link URL: %v", err)
	}
	linkURL, err := url.Parse(linkBody.URL)
	if err != nil {
		t.Fatalf("parse link URL: %v", err)
	}
	linkState := linkURL.Query().Get("state")
	if linkState == "" || linkURL.Query().Get("code_challenge") == "" || linkURL.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("unexpected link URL: %s", linkBody.URL)
	}

	noRedirect := &http.Client{
		Jar: app.client.Jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	linkCallback := requestAs(t, app, noRedirect, http.MethodGet,
		"/api/v1/auth/oidc/callback?state="+url.QueryEscape(linkState)+"&code=good-code", nil, "")
	if linkCallback.status != http.StatusFound || linkCallback.header.Get("Location") != "/settings?oidc_linked=1" {
		t.Fatalf("link callback: %d %s", linkCallback.status, linkCallback.header.Get("Location"))
	}
	linkStatus = app.request(t, http.MethodGet, "/api/v1/auth/oidc/link", nil, "")
	if linkStatus.status != http.StatusOK || !strings.Contains(string(linkStatus.body), `"linked":true`) {
		t.Fatalf("expected a linked identity, got %d %s", linkStatus.status, linkStatus.body)
	}

	logout := app.request(t, http.MethodPost, "/api/v1/auth/logout", nil, csrf)
	if logout.status != http.StatusNoContent {
		t.Fatalf("logout: %d %s", logout.status, logout.body)
	}

	start := requestAs(t, app, noRedirect, http.MethodGet, "/api/v1/auth/oidc/start", nil, "")
	if start.status != http.StatusFound {
		t.Fatalf("start: %d %s", start.status, start.body)
	}
	location, err := url.Parse(start.header.Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize location: %v", err)
	}
	state := location.Query().Get("state")
	callback := requestAs(t, app, noRedirect, http.MethodGet,
		"/api/v1/auth/oidc/callback?state="+url.QueryEscape(state)+"&code=good-code", nil, "")
	if callback.status != http.StatusFound || callback.header.Get("Location") != "/" {
		t.Fatalf("callback: %d %s", callback.status, callback.header.Get("Location"))
	}
	me := app.request(t, http.MethodGet, "/api/v1/auth/me", nil, "")
	if me.status != http.StatusOK || !strings.Contains(string(me.body), `"username":"admin"`) {
		t.Fatalf("expected an authenticated session, got %d %s", me.status, me.body)
	}

	// The state is single-use.
	replay := requestAs(t, app, noRedirect, http.MethodGet,
		"/api/v1/auth/oidc/callback?state="+url.QueryEscape(state)+"&code=good-code", nil, "")
	if replay.status != http.StatusFound || !strings.Contains(replay.header.Get("Location"), "oidc_error=invalid") {
		t.Fatalf("expected replay rejection, got %d %s", replay.status, replay.header.Get("Location"))
	}

	// A valid provider identity without a linked local account is denied and
	// never auto-provisioned.
	strangerStart := requestAs(t, app, noRedirect, http.MethodGet, "/api/v1/auth/oidc/start", nil, "")
	strangerLocation, err := url.Parse(strangerStart.header.Get("Location"))
	if err != nil {
		t.Fatalf("parse stranger authorize location: %v", err)
	}
	strangerState := strangerLocation.Query().Get("state")
	denied := requestAs(t, app, noRedirect, http.MethodGet,
		"/api/v1/auth/oidc/callback?state="+url.QueryEscape(strangerState)+"&code=stranger-code", nil, "")
	if denied.status != http.StatusFound || !strings.Contains(denied.header.Get("Location"), "oidc_error=no_account") {
		t.Fatalf("expected unlinked account denial, got %d %s", denied.status, denied.header.Get("Location"))
	}
	users, err := storage.NewUserRepository(app.db).List(t.Context())
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("OIDC must not auto-provision accounts, got %d users", len(users))
	}

	// Unlinking requires the local password.
	session := decodeAuth(t, func() []byte {
		login := requestAs(t, app, noRedirect, http.MethodPost, "/api/v1/auth/login", loginRequest{
			Username: "admin", Password: "correct horse battery staple",
		}, "")
		if login.status != http.StatusOK {
			t.Fatalf("password login: %d %s", login.status, login.body)
		}
		return login.body
	}())
	wrong := app.request(t, http.MethodPost, "/api/v1/auth/oidc/unlink", oidcUnlinkRequest{Password: "wrong"}, session.CSRFToken)
	if wrong.status != http.StatusUnauthorized {
		t.Fatalf("expected password rejection, got %d %s", wrong.status, wrong.body)
	}
	unlinked := app.request(t, http.MethodPost, "/api/v1/auth/oidc/unlink", oidcUnlinkRequest{Password: "correct horse battery staple"}, session.CSRFToken)
	if unlinked.status != http.StatusNoContent {
		t.Fatalf("unlink: %d %s", unlinked.status, unlinked.body)
	}
	linkStatus = app.request(t, http.MethodGet, "/api/v1/auth/oidc/link", nil, "")
	if !strings.Contains(string(linkStatus.body), `"linked":false`) {
		t.Fatalf("expected an unlinked identity, got %s", linkStatus.body)
	}
}
