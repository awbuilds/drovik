package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/awbuilds/drovik/backend/internal/logs"
	"github.com/awbuilds/drovik/backend/internal/processes"
	systemservices "github.com/awbuilds/drovik/backend/internal/services"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

const operationsProcessFixture = `  4242     1 root      Ssl   1.5  2.3  40960   5000 /usr/sbin/sshd -D
`

const operationsUnitFixture = `ssh.service loaded active running OpenBSD Secure Shell server
@@drovik-unit-files@@
ssh.service enabled enabled
`

const operationsJournalFixture = `{"__REALTIME_TIMESTAMP":"1700000000123456","PRIORITY":"3","MESSAGE":"failed to start","_SYSTEMD_UNIT":"ssh.service","SYSLOG_IDENTIFIER":"systemd"}
`

func TestOperationsHTTPFlow(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	sshServer.CommandHook = func(command string) (string, string, int, bool) {
		switch {
		case strings.Contains(command, "ps -eo"):
			return operationsProcessFixture, "", 0, true
		case command == "LC_ALL=C ps -p 4242 -o etimes=":
			return "5000\n", "", 0, true
		case command == "kill -s TERM 4242":
			return "", "", 0, true
		case command == "ps -p 4242 -o stat=":
			return "", "", 1, true
		case strings.Contains(command, "systemctl list-units"):
			return operationsUnitFixture, "", 0, true
		case command == "systemctl --no-ask-password --no-pager restart ssh.service":
			return "", "", 0, true
		case command == "systemctl show --no-pager --property=ActiveState,SubState,UnitFileState ssh.service":
			return "ActiveState=active\nSubState=running\nUnitFileState=enabled\n", "", 0, true
		case strings.Contains(command, "journalctl"):
			return operationsJournalFixture, "", 0, true
		}
		return "", "", 0, false
	}

	app := newTestApp(t)
	server, csrf := setupFilesServer(t, app, sshServer)
	base := "/api/v1/servers/" + server.ID

	result := app.request(t, http.MethodGet, base+"/processes", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("list processes: %d %s", result.status, result.body)
	}
	var listing processes.ListResult
	if err := json.Unmarshal(result.body, &listing); err != nil {
		t.Fatalf("decode processes: %v", err)
	}
	if len(listing.Processes) != 1 || listing.Processes[0].PID != 4242 {
		t.Fatalf("unexpected processes: %+v", listing)
	}

	signalBody := signalProcessRequest{Signal: processes.SignalTerm, ExpectedStart: listing.Processes[0].StartedAt}
	result = app.request(t, http.MethodPost, base+"/processes/4242/signal", signalBody, "")
	if result.status != http.StatusForbidden {
		t.Fatalf("expected CSRF rejection, got %d %s", result.status, result.body)
	}
	result = app.request(t, http.MethodPost, base+"/processes/4242/signal", signalBody, csrf)
	if result.status != http.StatusOK {
		t.Fatalf("signal process: %d %s", result.status, result.body)
	}
	var signalResult processes.SignalResult
	if err := json.Unmarshal(result.body, &signalResult); err != nil {
		t.Fatalf("decode signal result: %v", err)
	}
	if !signalResult.Delivered || signalResult.Running {
		t.Fatalf("unexpected signal result: %+v", signalResult)
	}

	result = app.request(t, http.MethodGet, base+"/services", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("list services: %d %s", result.status, result.body)
	}
	var services systemservices.ListResult
	if err := json.Unmarshal(result.body, &services); err != nil {
		t.Fatalf("decode services: %v", err)
	}
	if len(services.Units) != 1 || services.Units[0].Name != "ssh.service" {
		t.Fatalf("unexpected services: %+v", services)
	}

	result = app.request(t, http.MethodPost, base+"/services/ssh.service/action", serviceActionRequest{Action: "restart"}, csrf)
	if result.status != http.StatusOK {
		t.Fatalf("service action: %d %s", result.status, result.body)
	}
	var action systemservices.ActionResult
	if err := json.Unmarshal(result.body, &action); err != nil {
		t.Fatalf("decode action: %v", err)
	}
	if !action.StateKnown || action.ActiveState != "active" {
		t.Fatalf("unexpected action result: %+v", action)
	}

	result = app.request(t, http.MethodGet, base+"/logs?unit=ssh.service&since=1h&lines=50", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("logs: %d %s", result.status, result.body)
	}
	var journal logs.Result
	if err := json.Unmarshal(result.body, &journal); err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	if len(journal.Entries) != 1 || journal.Entries[0].Message != "failed to start" {
		t.Fatalf("unexpected logs: %+v", journal)
	}

	result = app.request(t, http.MethodGet, "/api/v1/activity?limit=50", nil, "")
	if result.status != http.StatusOK {
		t.Fatalf("activity: %d %s", result.status, result.body)
	}
	var activity struct {
		Events []auditEventResponse `json:"events"`
	}
	if err := json.Unmarshal(result.body, &activity); err != nil {
		t.Fatalf("decode activity: %v", err)
	}
	foundProcess := false
	foundService := false
	for _, event := range activity.Events {
		if event.EventType == "process.signal" {
			foundProcess = true
		}
		if event.EventType == "service.action" {
			foundService = true
		}
	}
	if !foundProcess || !foundService {
		t.Fatalf("expected process and service audit events: %+v", activity.Events)
	}

	result = app.request(t, http.MethodGet, base+"/logs?lines=9999", nil, "")
	if result.status != http.StatusUnprocessableEntity {
		t.Fatalf("expected line limit rejection, got %d %s", result.status, result.body)
	}
	result = app.request(t, http.MethodPost, base+"/processes/999999/signal", signalProcessRequest{Signal: processes.SignalKill}, csrf)
	if result.status != http.StatusUnprocessableEntity && result.status != http.StatusNotFound {
		t.Fatalf("expected an invalid process response, got %d %s", result.status, result.body)
	}
}
