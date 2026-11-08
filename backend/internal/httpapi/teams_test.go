package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/storage"
)

func TestTeamsShareServersWithMembers(t *testing.T) {
	app := newTestApp(t)
	setup := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin", Password: "correct horse battery staple",
	}, "")
	if setup.status != http.StatusCreated {
		t.Fatalf("setup: %d %s", setup.status, setup.body)
	}
	csrf := decodeAuth(t, setup.body).CSRFToken

	createdUser := app.request(t, http.MethodPost, "/api/v1/users", createUserRequest{
		Username: "operator", Password: "operator password", Role: "operator",
	}, csrf)
	if createdUser.status != http.StatusCreated {
		t.Fatalf("create user: %d %s", createdUser.status, createdUser.body)
	}
	var managed struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createdUser.body, &managed); err != nil {
		t.Fatalf("decode user: %v", err)
	}

	createdServer := app.request(t, http.MethodPost, "/api/v1/servers", createServerRequest{
		DisplayName: "Team host", Host: "192.0.2.20", Port: 22, Username: "deploy",
		AuthMethod: "password", Password: "test-only",
	}, csrf)
	if createdServer.status != http.StatusCreated {
		t.Fatalf("create server: %d %s", createdServer.status, createdServer.body)
	}
	var server serverResponse
	if err := json.Unmarshal(createdServer.body, &server); err != nil {
		t.Fatalf("decode server: %v", err)
	}

	createdTeam := app.request(t, http.MethodPost, "/api/v1/teams", teamRequest{
		Name: "Operations", Description: "Shared production access",
	}, csrf)
	if createdTeam.status != http.StatusCreated {
		t.Fatalf("create team: %d %s", createdTeam.status, createdTeam.body)
	}
	var team teamResponse
	if err := json.Unmarshal(createdTeam.body, &team); err != nil {
		t.Fatalf("decode team: %v", err)
	}
	if added := app.request(t, http.MethodPut, "/api/v1/teams/"+team.ID+"/members/"+managed.ID, nil, csrf); added.status != http.StatusNoContent {
		t.Fatalf("add member: %d %s", added.status, added.body)
	}
	if assigned := app.request(t, http.MethodPut, "/api/v1/teams/"+team.ID+"/servers/"+server.ID, teamServerRequest{
		AccessLevel: "read",
	}, csrf); assigned.status != http.StatusNoContent {
		t.Fatalf("assign server: %d %s", assigned.status, assigned.body)
	}

	jar, _ := cookiejar.New(nil)
	memberClient := &http.Client{Jar: jar}
	login := requestAs(t, app, memberClient, http.MethodPost, "/api/v1/auth/login", loginRequest{
		Username: "operator", Password: "operator password",
	}, "")
	if login.status != http.StatusOK {
		t.Fatalf("member login: %d %s", login.status, login.body)
	}
	memberCSRF := decodeAuth(t, login.body).CSRFToken

	accessible := requestAs(t, app, memberClient, http.MethodGet, "/api/v1/servers/"+server.ID, nil, "")
	if accessible.status != http.StatusOK {
		t.Fatalf("team member server access: %d %s", accessible.status, accessible.body)
	}
	list := requestAs(t, app, memberClient, http.MethodGet, "/api/v1/servers", nil, "")
	if list.status != http.StatusOK {
		t.Fatalf("team member server list: %d %s", list.status, list.body)
	}
	var servers struct {
		Servers []serverResponse `json:"servers"`
	}
	if err := json.Unmarshal(list.body, &servers); err != nil || len(servers.Servers) != 1 {
		t.Fatalf("team server missing from list: %s (%v)", list.body, err)
	}
	serverID := server.ID
	if err := storage.NewAuditRepository(app.db).Insert(t.Context(), storage.AuditEvent{
		ActorUserID: &managed.ID, ServerID: &serverID, EventType: "test.team_visible",
		TargetSummary: "team-scoped event", Outcome: "success", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("insert team audit event: %v", err)
	}
	teamActivity := requestAs(t, app, memberClient, http.MethodGet, "/api/v1/activity?q=team-scoped", nil, "")
	if teamActivity.status != http.StatusOK || !strings.Contains(string(teamActivity.body), "test.team_visible") {
		t.Fatalf("team member cannot see server audit event: %d %s", teamActivity.status, teamActivity.body)
	}
	if forbidden := requestAs(t, app, memberClient, http.MethodPost, "/api/v1/teams", teamRequest{Name: "Nope"}, memberCSRF); forbidden.status != http.StatusForbidden {
		t.Fatalf("non-admin created a team: %d %s", forbidden.status, forbidden.body)
	}
	if forbidden := requestAs(t, app, memberClient, http.MethodDelete, "/api/v1/teams/"+team.ID, nil, memberCSRF); forbidden.status != http.StatusForbidden {
		t.Fatalf("non-admin deleted a team: %d %s", forbidden.status, forbidden.body)
	}

	detail := app.request(t, http.MethodGet, "/api/v1/teams/"+team.ID, nil, "")
	if detail.status != http.StatusOK {
		t.Fatalf("team detail: %d %s", detail.status, detail.body)
	}
	var teamDetail teamDetailResponse
	if err := json.Unmarshal(detail.body, &teamDetail); err != nil {
		t.Fatalf("decode team detail: %v", err)
	}
	if len(teamDetail.Members) != 1 || len(teamDetail.Servers) != 1 || teamDetail.Servers[0].AccessLevel != "read" {
		t.Fatalf("unexpected team detail: %+v", teamDetail)
	}
}

func TestAuditSearchAndExport(t *testing.T) {
	app := newTestApp(t)
	setup := app.request(t, http.MethodPost, "/api/v1/setup", setupRequest{
		Username: "admin", Password: "correct horse battery staple",
	}, "")
	if setup.status != http.StatusCreated {
		t.Fatalf("setup: %d %s", setup.status, setup.body)
	}
	csrf := decodeAuth(t, setup.body).CSRFToken
	if created := app.request(t, http.MethodPost, "/api/v1/teams", teamRequest{
		Name: "Audit Team", Description: "Searchable team",
	}, csrf); created.status != http.StatusCreated {
		t.Fatalf("create team: %d %s", created.status, created.body)
	}

	search := app.request(t, http.MethodGet, "/api/v1/activity?event_type=team.created&q=Audit", nil, "")
	if search.status != http.StatusOK {
		t.Fatalf("search: %d %s", search.status, search.body)
	}
	var found struct {
		Events []auditEventResponse `json:"events"`
	}
	if err := json.Unmarshal(search.body, &found); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if len(found.Events) != 1 || found.Events[0].EventType != "team.created" {
		t.Fatalf("unexpected search results: %+v", found.Events)
	}
	invalid := app.request(t, http.MethodGet, "/api/v1/activity?outcome=made-up", nil, "")
	if invalid.status != http.StatusUnprocessableEntity {
		t.Fatalf("expected outcome validation, got %d %s", invalid.status, invalid.body)
	}

	csvExport := app.request(t, http.MethodGet, "/api/v1/activity/export?format=csv&event_type=team.created", nil, "")
	if csvExport.status != http.StatusOK {
		t.Fatalf("csv export: %d %s", csvExport.status, csvExport.body)
	}
	if got := csvExport.header.Get("Content-Type"); !strings.HasPrefix(got, "text/csv") {
		t.Fatalf("unexpected CSV content type: %q", got)
	}
	if got := csvExport.header.Get("Content-Disposition"); !strings.Contains(got, "attachment") {
		t.Fatalf("unexpected disposition: %q", got)
	}
	if !strings.Contains(string(csvExport.body), "team.created") || !strings.Contains(string(csvExport.body), "Audit Team") {
		t.Fatalf("CSV export missing audit event: %s", csvExport.body)
	}

	jsonExport := app.request(t, http.MethodGet, "/api/v1/activity/export?format=json&event_type=team.created", nil, "")
	if jsonExport.status != http.StatusOK || !strings.Contains(string(jsonExport.body), `"events"`) {
		t.Fatalf("json export: %d %s", jsonExport.status, jsonExport.body)
	}
}
