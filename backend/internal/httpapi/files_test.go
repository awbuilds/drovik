package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/awbuilds/drovik/backend/internal/files"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

func setupFilesServer(t *testing.T, app *testApp, sshServer *testssh.Server) (serverResponse, string) {
	t.Helper()
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
		DisplayName: "Files host",
		Host:        host,
		Port:        port,
		Username:    "test",
		AuthMethod:  "password",
		Password:    sshServer.Password,
	}, csrf)
	if created.status != http.StatusCreated {
		t.Fatalf("create server: %d %s", created.status, created.body)
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
		t.Fatalf("trust host key: %d %s", trusted.status, trusted.body)
	}
	return server, csrf
}

func TestFilesHTTPFlow(t *testing.T) {
	workDir := t.TempDir()
	sshServer, err := testssh.Start(workDir)
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	app := newTestApp(t)
	server, csrf := setupFilesServer(t, app, sshServer)

	filePath := filepath.Join(workDir, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	listURL := "/api/v1/servers/" + server.ID + "/files?path=" + url.QueryEscape(workDir)
	result := app.request(t, http.MethodGet, listURL, nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("list: %d %s", result.status, result.body)
	}
	var listing files.ListResult
	if err := json.Unmarshal(result.body, &listing); err != nil {
		t.Fatalf("decode listing: %v", err)
	}
	if len(listing.Entries) == 0 {
		t.Fatalf("empty listing: %s", result.body)
	}

	contentURL := "/api/v1/servers/" + server.ID + "/files/content?path=" + url.QueryEscape(filePath)
	result = app.request(t, http.MethodGet, contentURL, nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("read content: %d %s", result.status, result.body)
	}
	var content files.Content
	if err := json.Unmarshal(result.body, &content); err != nil {
		t.Fatalf("decode content: %v", err)
	}
	if content.Content != "hello world\n" {
		t.Fatalf("unexpected content: %+v", content)
	}

	writeBody := writeFileRequest{
		Path:               filePath,
		Content:            "updated\n",
		ExpectedModifiedAt: &content.ModifiedAt,
		ExpectedSize:       &content.Size,
	}
	result = app.request(t, http.MethodPut, "/api/v1/servers/"+server.ID+"/files/content", writeBody, csrf)
	if result.status != http.StatusNoContent {
		t.Fatalf("write content: %d %s", result.status, result.body)
	}
	result = app.request(t, http.MethodPut, "/api/v1/servers/"+server.ID+"/files/content", writeBody, csrf)
	if result.status != http.StatusConflict || decodeError(t, result.body).Error.Code != CodeFileStale {
		t.Fatalf("expected stale conflict, got %d %s", result.status, result.body)
	}
	result = app.request(t, http.MethodPut, "/api/v1/servers/"+server.ID+"/files/content", writeBody, "")
	if result.status != http.StatusForbidden {
		t.Fatalf("expected CSRF rejection, got %d %s", result.status, result.body)
	}

	directory := filepath.Join(workDir, "created")
	result = app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/files/mkdir", pathRequest{Path: directory}, csrf)
	if result.status != http.StatusCreated {
		t.Fatalf("mkdir: %d %s", result.status, result.body)
	}
	newFile := filepath.Join(directory, "new.txt")
	result = app.request(t, http.MethodPost, "/api/v1/servers/"+server.ID+"/files/touch", pathRequest{Path: newFile}, csrf)
	if result.status != http.StatusCreated {
		t.Fatalf("touch: %d %s", result.status, result.body)
	}
	renamed := filepath.Join(directory, "renamed.txt")
	result = app.request(t, http.MethodPatch, "/api/v1/servers/"+server.ID+"/files/rename", renameRequest{
		From: newFile, To: renamed,
	}, csrf)
	if result.status != http.StatusNoContent {
		t.Fatalf("rename: %d %s", result.status, result.body)
	}
	result = app.request(t, http.MethodDelete, "/api/v1/servers/"+server.ID+"/files?path="+url.QueryEscape(directory)+"&recursive=true", nil, csrf)
	if result.status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", result.status, result.body)
	}

	uploadPath := filepath.Join(workDir, "upload.txt")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "upload.txt")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("uploaded via HTTP")); err != nil {
		t.Fatalf("write multipart: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost,
		app.server.URL+"/api/v1/servers/"+server.ID+"/files/upload?path="+url.QueryEscape(uploadPath),
		&body)
	if err != nil {
		t.Fatalf("new upload request: %v", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", "http://example.com")
	request.Header.Set("X-CSRF-Token", csrf)
	upload, err := doRaw(app.client, request)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if upload.status != http.StatusCreated {
		t.Fatalf("upload: %d %s", upload.status, upload.body)
	}

	downloadURL := "/api/v1/servers/" + server.ID + "/files/download?path=" + url.QueryEscape(uploadPath)
	result = app.request(t, http.MethodGet, downloadURL, nil, "")
	if result.status != http.StatusOK || !strings.Contains(string(result.body), "uploaded via HTTP") {
		t.Fatalf("download: %d %s", result.status, result.body)
	}

	result = app.request(t, http.MethodGet, "/api/v1/servers/"+server.ID+"/files?path=relative", nil, "")
	if result.status != http.StatusUnprocessableEntity {
		t.Fatalf("expected invalid path, got %d %s", result.status, result.body)
	}
}
