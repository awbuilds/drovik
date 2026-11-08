package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/awbuilds/drovik/backend/internal/advanced"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

const advancedDockerFixture = `{"ID":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","Names":"/web","Image":"nginx:latest","State":"running","Status":"Up 1 hour","Ports":"0.0.0.0:8080->80/tcp","CreatedAt":"2026-11-08 01:00:00 +0000 UTC","RunningFor":"1 hour ago"}
`

const advancedInspectFixture = `[{"Id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","Name":"/web","Image":"sha256:cccc","Path":"/docker-entrypoint.sh","Config":{"Env":["DB_PASSWORD=super-secret"],"Cmd":["nginx"]},"State":{"Status":"running"},"Mounts":[],"NetworkSettings":{"Ports":{},"IPAddress":"172.17.0.2"}}]
`

func TestAdvancedHTTPFlow(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	sshServer.CommandHook = func(command string) (string, string, int, bool) {
		switch {
		case strings.Contains(command, "docker ps -a"):
			return advancedDockerFixture, "", 0, true
		case strings.Contains(command, "docker images"):
			return "", "", 0, true
		case strings.Contains(command, "docker inspect"):
			return advancedInspectFixture, "", 0, true
		case strings.Contains(command, "docker logs"):
			return "log line\n", "", 0, true
		case strings.Contains(command, "docker restart "):
			return "", "", 0, true
		case strings.Contains(command, "echo"):
			return "ran\n", "", 0, true
		}
		return "", "", 0, false
	}

	app := newTestApp(t)
	server, csrf := setupFilesServer(t, app, sshServer)
	base := "/api/v1/servers/" + server.ID

	docker := app.request(t, http.MethodGet, base+"/docker", nil, "")
	if docker.status != http.StatusOK {
		t.Fatalf("docker: %d %s", docker.status, docker.body)
	}
	var overview advanced.DockerOverview
	if err := json.Unmarshal(docker.body, &overview); err != nil {
		t.Fatalf("decode docker: %v", err)
	}
	if len(overview.Containers) != 1 {
		t.Fatalf("unexpected containers: %+v", overview)
	}
	containerID := overview.Containers[0].ID

	action := app.request(t, http.MethodPost, base+"/docker/"+containerID+"/action",
		dockerActionRequest{Action: "restart"}, csrf)
	if action.status != http.StatusNoContent {
		t.Fatalf("docker action: %d %s", action.status, action.body)
	}
	invalid := app.request(t, http.MethodPost, base+"/docker/not-a-container/action",
		dockerActionRequest{Action: "restart"}, csrf)
	if invalid.status != http.StatusUnprocessableEntity {
		t.Fatalf("expected invalid container rejection, got %d %s", invalid.status, invalid.body)
	}

	inspect := app.request(t, http.MethodGet, base+"/docker/"+containerID+"/inspect", nil, "")
	if inspect.status != http.StatusOK {
		t.Fatalf("inspect: %d %s", inspect.status, inspect.body)
	}
	if strings.Contains(string(inspect.body), "super-secret") || strings.Contains(string(inspect.body), "Env") {
		t.Fatalf("inspect leaked container environment: %s", inspect.body)
	}

	created := app.request(t, http.MethodPost, "/api/v1/snippets", snippetRequest{
		Name: "Echo", Command: "echo {{value}}",
		Variables: []advanced.SnippetVariable{{Name: "value", Default: "hello"}},
	}, csrf)
	if created.status != http.StatusCreated {
		t.Fatalf("create snippet: %d %s", created.status, created.body)
	}
	var snippet advanced.Snippet
	if err := json.Unmarshal(created.body, &snippet); err != nil {
		t.Fatalf("decode snippet: %v", err)
	}
	preview := app.request(t, http.MethodPost, "/api/v1/snippets/"+snippet.ID+"/preview",
		snippetRunRequest{Values: map[string]string{"value": "it's ok"}}, csrf)
	if preview.status != http.StatusOK {
		t.Fatalf("preview: %d %s", preview.status, preview.body)
	}
	var previewBody struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(preview.body, &previewBody); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if previewBody.Command != `echo 'it'"'"'s ok'` {
		t.Fatalf("unexpected preview command: %q", previewBody.Command)
	}
	unconfirmed := app.request(t, http.MethodPost, base+"/snippets/"+snippet.ID+"/run",
		snippetRunRequest{Values: map[string]string{"value": "x"}}, csrf)
	if unconfirmed.status != http.StatusUnprocessableEntity {
		t.Fatalf("expected confirmation rejection, got %d %s", unconfirmed.status, unconfirmed.body)
	}
	run := app.request(t, http.MethodPost, base+"/snippets/"+snippet.ID+"/run",
		snippetRunRequest{Values: map[string]string{"value": "x"}, Confirm: true}, csrf)
	if run.status != http.StatusOK {
		t.Fatalf("run snippet: %d %s", run.status, run.body)
	}

	history := app.request(t, http.MethodGet, base+"/history?range=1h", nil, "")
	if history.status != http.StatusOK {
		t.Fatalf("history: %d %s", history.status, history.body)
	}
	rule := app.request(t, http.MethodPut, base+"/alerts/new", alertRuleRequest{
		Metric: "cpu_percent", Comparator: "gt", Threshold: 80, Enabled: true,
	}, csrf)
	if rule.status != http.StatusOK {
		t.Fatalf("save alert rule: %d %s", rule.status, rule.body)
	}
}
