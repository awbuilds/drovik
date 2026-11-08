package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"github.com/awbuilds/drovik/backend/internal/testssh"
)

func loginAs(t *testing.T, app *testApp, username, password string) (*http.Client, string) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client := &http.Client{Jar: jar}
	body, _ := json.Marshal(loginRequest{Username: username, Password: password})
	request, _ := http.NewRequest(http.MethodPost, app.server.URL+"/api/v1/auth/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://example.com")
	result, err := doRaw(client, request)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.status != http.StatusOK {
		t.Fatalf("login status: %d %s", result.status, result.body)
	}
	var auth authResponse
	if err := json.Unmarshal(result.body, &auth); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	return client, auth.CSRFToken
}

func requestAs(t *testing.T, app *testApp, client *http.Client, method, path string, body any, csrf string) testResponse {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	request, err := http.NewRequest(method, app.server.URL+path, reader)
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
	result, err := doRaw(client, request)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return result
}

func TestReadOnlyUserIsBlockedFromMutations(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	app := newTestApp(t)
	server, adminCSRF := setupFilesServer(t, app, sshServer)

	created := app.request(t, http.MethodPost, "/api/v1/users", createUserRequest{
		Username: "viewer", Password: "correct horse battery staple", Role: "read_only",
	}, adminCSRF)
	if created.status != http.StatusCreated {
		t.Fatalf("create user: %d %s", created.status, created.body)
	}
	var viewer userAdminResponse
	if err := json.Unmarshal(created.body, &viewer); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	assigned := app.request(t, http.MethodPut, "/api/v1/servers/"+server.ID+"/access/"+viewer.ID, setAssignmentRequest{
		AccessLevel: "read",
	}, adminCSRF)
	if assigned.status != http.StatusNoContent {
		t.Fatalf("assign: %d %s", assigned.status, assigned.body)
	}

	viewerClient, viewerCSRF := loginAs(t, app, "viewer", "correct horse battery staple")
	servers := requestAs(t, app, viewerClient, http.MethodGet, "/api/v1/servers", nil, "")
	if servers.status != http.StatusOK {
		t.Fatalf("viewer servers: %d %s", servers.status, servers.body)
	}
	var listing struct {
		Servers []serverResponse `json:"servers"`
	}
	if err := json.Unmarshal(servers.body, &listing); err != nil {
		t.Fatalf("decode servers: %v", err)
	}
	if len(listing.Servers) != 1 || listing.Servers[0].ID != server.ID || listing.Servers[0].AccessLevel != "read" {
		t.Fatalf("unexpected viewer listing: %+v", listing.Servers)
	}

	signal := requestAs(t, app, viewerClient, http.MethodPost,
		"/api/v1/servers/"+server.ID+"/processes/4242/signal",
		signalProcessRequest{Signal: "SIGTERM"}, viewerCSRF)
	if signal.status != http.StatusForbidden {
		t.Fatalf("expected read-only signal rejection, got %d %s", signal.status, signal.body)
	}
	snippet := requestAs(t, app, viewerClient, http.MethodPost, "/api/v1/snippets", snippetRequest{
		Name: "blocked", Command: "true",
	}, viewerCSRF)
	if snippet.status != http.StatusForbidden {
		t.Fatalf("expected read-only snippet rejection, got %d %s", snippet.status, snippet.body)
	}
	users := requestAs(t, app, viewerClient, http.MethodGet, "/api/v1/users", nil, "")
	if users.status != http.StatusForbidden {
		t.Fatalf("expected non-admin users rejection, got %d %s", users.status, users.body)
	}

}

func TestReadAssignmentBlocksOperatorWrites(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	app := newTestApp(t)
	server, adminCSRF := setupFilesServer(t, app, sshServer)

	created := app.request(t, http.MethodPost, "/api/v1/users", createUserRequest{
		Username: "assigned-operator", Password: "correct horse battery staple", Role: "operator",
	}, adminCSRF)
	if created.status != http.StatusCreated {
		t.Fatalf("create operator: %d %s", created.status, created.body)
	}
	var operator userAdminResponse
	if err := json.Unmarshal(created.body, &operator); err != nil {
		t.Fatalf("decode operator: %v", err)
	}
	assigned := app.request(t, http.MethodPut, "/api/v1/servers/"+server.ID+"/access/"+operator.ID, setAssignmentRequest{
		AccessLevel: "read",
	}, adminCSRF)
	if assigned.status != http.StatusNoContent {
		t.Fatalf("assign read: %d %s", assigned.status, assigned.body)
	}

	client, csrf := loginAs(t, app, "assigned-operator", "correct horse battery staple")
	signal := requestAs(t, app, client, http.MethodPost,
		"/api/v1/servers/"+server.ID+"/processes/4242/signal",
		signalProcessRequest{Signal: "SIGTERM"}, csrf)
	if signal.status != http.StatusForbidden {
		t.Fatalf("expected read assignment rejection, got %d %s", signal.status, signal.body)
	}

	write := app.request(t, http.MethodPut, "/api/v1/servers/"+server.ID+"/access/"+operator.ID, setAssignmentRequest{
		AccessLevel: "write",
	}, adminCSRF)
	if write.status != http.StatusNoContent {
		t.Fatalf("assign write: %d %s", write.status, write.body)
	}
	signal = requestAs(t, app, client, http.MethodPost,
		"/api/v1/servers/"+server.ID+"/processes/4242/signal",
		signalProcessRequest{Signal: "SIGTERM"}, csrf)
	if signal.status == http.StatusForbidden {
		t.Fatalf("write assignment was unexpectedly blocked: %s", signal.body)
	}
}
