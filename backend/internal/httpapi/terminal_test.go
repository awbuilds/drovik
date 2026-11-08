package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/awbuilds/drovik/backend/internal/testssh"
)

type terminalMessage struct {
	Type    string `json:"type"`
	Data    string `json:"data"`
	State   string `json:"state"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func TestTerminalWebSocket(t *testing.T) {
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
		DisplayName: "Terminal host",
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

	tested := app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/test", nil, csrf)
	if tested.status != http.StatusConflict {
		t.Fatalf("expected unknown host key, got %d %s", tested.status, tested.body)
	}
	hostKeyError := decodeError(t, tested.body)
	details := hostKeyError.Error.Details.(map[string]any)
	trusted := app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/trust-host-key", trustHostKeyRequest{
		Fingerprint: details["fingerprint"].(string),
		KeyType:     details["key_type"].(string),
		PublicKey:   details["public_key"].(string),
	}, csrf)
	if trusted.status != http.StatusNoContent {
		t.Fatalf("trust: %d %s", trusted.status, trusted.body)
	}

	serverURL, _ := url.Parse(app.server.URL)
	cookies := app.client.Jar.Cookies(serverURL)
	cookieParts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		cookieParts = append(cookieParts, cookie.Name+"="+cookie.Value)
	}
	wsURL := "ws" + strings.TrimPrefix(app.server.URL, "http") + "/api/v1/servers/" + server.ID + "/terminal"

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	//nolint:bodyclose // on success the response body is owned by the WebSocket connection
	conn, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"http://example.com"},
			"Cookie": []string{strings.Join(cookieParts, "; ")},
		},
	})
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
			_ = response.Body.Close()
		}
		t.Fatalf("websocket dial: %v (status %d)", err, status)
	}
	defer func() { _ = conn.CloseNow() }()

	var output strings.Builder
	waitFor := func(needle string) {
		t.Helper()
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("read waiting for %q: %v (output %q)", needle, err, output.String())
			}
			if typ != websocket.MessageText {
				continue
			}
			var message terminalMessage
			if err := json.Unmarshal(data, &message); err != nil {
				t.Fatalf("decode message: %v", err)
			}
			if message.Type == "output" {
				output.WriteString(message.Data)
			}
			if message.Type == "error" {
				t.Fatalf("terminal error %s: %s", message.Code, message.Message)
			}
			if strings.Contains(output.String(), needle) {
				return
			}
		}
	}

	waitFor("drovik-test$")

	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","cols":100,"rows":30}`)); err != nil {
		t.Fatalf("resize: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"input","data":"hello\n"}`)); err != nil {
		t.Fatalf("input: %v", err)
	}
	waitFor("hello")

	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"close"}`)); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestTerminalRejectsUntrustedOrigin(t *testing.T) {
	app := newTestApp(t)
	setup := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin",
		Password: "correct horse battery staple",
	}, "")
	csrf := decodeAuth(t, setup.body).CSRFToken
	created := app.request(t, http.MethodPost, "/api/v1/servers", createServerRequest{
		DisplayName: "Origin test",
		Host:        "192.0.2.10",
		Port:        22,
		Username:    "test",
		AuthMethod:  "password",
		Password:    "test-password",
	}, csrf)
	if created.status != http.StatusCreated {
		t.Fatalf("create: %d %s", created.status, created.body)
	}
	var server serverResponse
	if err := json.Unmarshal(created.body, &server); err != nil {
		t.Fatalf("decode server: %v", err)
	}

	serverURL, _ := url.Parse(app.server.URL)
	cookies := app.client.Jar.Cookies(serverURL)
	cookieParts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		cookieParts = append(cookieParts, cookie.Name+"="+cookie.Value)
	}
	wsURL := "ws" + strings.TrimPrefix(app.server.URL, "http") + "/api/v1/servers/" + server.ID + "/terminal"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//nolint:bodyclose // on success the response body is owned by the WebSocket connection
	conn, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"http://evil.example"},
			"Cookie": []string{strings.Join(cookieParts, "; ")},
		},
	})
	if conn != nil {
		_ = conn.CloseNow()
	}
	if err == nil {
		t.Fatal("expected origin rejection")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		status := 0
		if response != nil {
			status = response.StatusCode
			_ = response.Body.Close()
		}
		t.Fatalf("expected 403, got %d (%v)", status, err)
	}
	_ = response.Body.Close()
}

func TestTerminalRequiresAuthentication(t *testing.T) {
	app := newTestApp(t)
	wsURL := "ws" + strings.TrimPrefix(app.server.URL, "http") + "/api/v1/servers/unknown/terminal"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//nolint:bodyclose // the response body is only inspected on failure
	conn, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://example.com"}},
	})
	if conn != nil {
		_ = conn.CloseNow()
	}
	if err == nil {
		t.Fatal("expected authentication rejection")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		status := 0
		if response != nil {
			status = response.StatusCode
			_ = response.Body.Close()
		}
		t.Fatalf("expected 401, got %d (%v)", status, err)
	}
	_ = response.Body.Close()
}

func TestTerminalClosesAfterLogout(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()

	app := newTestApp(t)
	server, csrf := setupFilesServer(t, app, sshServer)
	serverURL, _ := url.Parse(app.server.URL)
	cookies := app.client.Jar.Cookies(serverURL)
	cookieParts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		cookieParts = append(cookieParts, cookie.Name+"="+cookie.Value)
	}
	wsURL := "ws" + strings.TrimPrefix(app.server.URL, "http") + "/api/v1/servers/" + server.ID + "/terminal"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	//nolint:bodyclose // the successful response body is owned by the WebSocket connection
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"http://example.com"},
			"Cookie": []string{strings.Join(cookieParts, "; ")},
		},
	})
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	waitForPrompt := func() {
		t.Helper()
		var output strings.Builder
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("read prompt: %v (output %q)", err, output.String())
			}
			if typ != websocket.MessageText {
				continue
			}
			var message terminalMessage
			if err := json.Unmarshal(data, &message); err != nil {
				t.Fatalf("decode message: %v", err)
			}
			if message.Type == "output" {
				output.WriteString(message.Data)
			}
			if strings.Contains(output.String(), "drovik-test$") {
				return
			}
		}
	}
	waitForPrompt()

	logout := app.request(t, http.MethodPost, "/api/v1/auth/logout", nil, csrf)
	if logout.status != http.StatusNoContent {
		t.Fatalf("logout: %d %s", logout.status, logout.body)
	}

	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"input","data":"post_logout_probe\n"}`)); err == nil {
		readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer readCancel()
		if _, _, err := conn.Read(readCtx); err == nil {
			t.Fatal("terminal remained readable after logout")
		}
	}
}
