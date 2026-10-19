//go:build integration

package httpserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/projects"
	"omsbizfoundry/project-workspace/internal/testdatabase"
	"strings"
	"testing"
)

type noContextReferences struct{ calls int }

func (v *noContextReferences) ValidatePeople(context.Context, string, string, string, int64) (projects.ReferenceSnapshot, error) {
	v.calls++
	return projects.ReferenceSnapshot{}, projects.ErrReferenceUnavailable
}
func (v *noContextReferences) ValidateParty(context.Context, string, string, int64) (projects.PartySnapshot, error) {
	v.calls++
	return projects.PartySnapshot{}, projects.ErrReferenceUnavailable
}
func (v *noContextReferences) ValidateResource(context.Context, string, string, string, string, *int64) error {
	v.calls++
	return projects.ErrReferenceUnavailable
}
func TestHTTPPostgresCounterpartyContextSaveReadRedactionAndNoMasterCalls(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := projects.NewPostgresStore(env.Pool)
	actor := projects.Actor{ID: httpSubject, TenantID: httpTenant}
	p, err := store.DispatchInitiation(ctx, actor, projects.InitiationDispatchInput{CommandID: "a0000000-0000-4000-8000-000000000301", NodeType: projects.NodeTypePackage, Name: "项目包", BusinessOwner: projects.InitiationOwnerReference{PersonID: systemPerson, SourceRevision: 1}})
	// Dispatch needs a validated People snapshot, so create the independent
	// package and its assignment using Store preconditions, not a shared table.
	if err == nil {
		t.Fatal("unvalidated dispatch unexpectedly succeeded")
	}
	p, err = store.CreatePackage(ctx, actor, projects.PackageCreateInput{CommandID: "a0000000-0000-4000-8000-000000000302", Name: "项目包", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	a, err := store.ApplyAssignment(ctx, actor, p.ID, projects.AssignmentCommand{CommandID: "a0000000-0000-4000-8000-000000000303", ExpectedRevision: p.Revision, Operation: "assign", SubjectKind: "person", SubjectID: systemPerson, SourceRevision: 1, DisplaySnapshot: "经营", RoleKey: projects.RoleBusinessOwner})
	if err != nil {
		t.Fatal(err)
	}
	p, err = store.CompleteInitiation(ctx, actor, p.ID, projects.RevisionCommand{CommandID: "a0000000-0000-4000-8000-000000000304", ExpectedRevision: a.Project.Revision, InitiationProfile: projects.InitiationProfileBusinessOwnerV1})
	if err != nil {
		t.Fatal(err)
	}
	refs := &noContextReferences{}
	service, _ := projects.NewService(store, refs)
	principal := fullPrincipal()
	server, _ := New(service, &authenticatorStub{principal: principal})
	send := func(method, path, body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(method, "/api/v1/projects/"+p.ID+path, strings.NewReader(body)))
		return response
	}
	body := `{"command_id":"a0000000-0000-4000-8000-000000000305","expected_revision":3,"reason":"录入甲方","name":"本项目甲方","contact_name":"项目联系人","contact_phone":"123-456"}`
	response := send("POST", "/counterparty-context", body)
	if response.Code != 200 {
		t.Fatalf("save=%s", response.Body.String())
	}
	replay := send("POST", "/counterparty-context", body)
	if replay.Code != 200 || response.Body.String() != replay.Body.String() {
		t.Fatal("HTTP replay changed result")
	}
	read := send("GET", "/counterparty-context", "")
	var value counterpartyContextResponse
	if err := json.Unmarshal(read.Body.Bytes(), &value); err != nil || value.ProjectRevision != 4 || value.Context.Name != "本项目甲方" || !containsAction(value.AllowedActions, auth.ActionCounterpartyContextManage) {
		t.Fatalf("read=%s err=%v", read.Body.String(), err)
	}
	facts := func() string {
		t.Helper()
		var out string
		if err := env.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM projects)||':'||(SELECT sum(revision) FROM projects)||':'||(SELECT count(*) FROM project_history_events)||':'||(SELECT count(*) FROM project_commands)||':'||(SELECT count(*) FROM project_assignments)`).Scan(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	beforeNames := facts()
	sendNames := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		server.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/counterparty-context/name-suggestions?q=甲方", nil))
		return w
	}
	names := sendNames()
	var list projects.CounterpartyNames
	if err := json.Unmarshal(names.Body.Bytes(), &list); err != nil || names.Code != 200 || len(list.Names) != 1 || list.Names[0] != "本项目甲方" || strings.Contains(names.Body.String(), "项目联系人") || strings.Contains(names.Body.String(), "123-456") || strings.Contains(names.Body.String(), p.ID) {
		t.Fatalf("names=%s %v", names.Body.String(), err)
	}
	principal.ScopeKind = "resources"
	principal.ResourceIDs = map[string]bool{}
	server, _ = New(service, &authenticatorStub{principal: principal})
	names = sendNames()
	if err := json.Unmarshal(names.Body.Bytes(), &list); err != nil || names.Code != 200 || len(list.Names) != 0 || facts() != beforeNames {
		t.Fatal("names scope leaked or read wrote facts")
	}
	principal = fullPrincipal()
	server, _ = New(service, &authenticatorStub{principal: principal})
	ordinary := send("GET", "", "")
	if ordinary.Code != 200 || strings.Contains(ordinary.Body.String(), "项目联系人") || strings.Contains(ordinary.Body.String(), "123-456") {
		t.Fatalf("ordinary leaked=%s", ordinary.Body.String())
	}
	principal.AllowedActions = map[string]bool{auth.ActionRead: true, auth.ActionHistoryRead: true}
	server, _ = New(service, &authenticatorStub{principal: principal})
	if sendNames().Code != 403 {
		t.Fatal("ordinary Project read disclosed counterparty names")
	}
	history := send("GET", "/history", "")
	if history.Code != 200 || strings.Contains(history.Body.String(), "项目联系人") || strings.Contains(history.Body.String(), "123-456") || strings.Contains(history.Body.String(), "本项目甲方") {
		t.Fatalf("history leaked=%s", history.Body.String())
	}
	denied := send("GET", "/counterparty-context", "")
	if denied.Code != 403 {
		t.Fatalf("read denied=%d", denied.Code)
	}
	denied = send("POST", "/counterparty-context", body)
	if denied.Code != 403 {
		t.Fatalf("write denied=%d", denied.Code)
	}
	principal.AllowedActions[auth.ActionCounterpartyContextRead] = true
	server, _ = New(service, &authenticatorStub{principal: principal})
	history = send("GET", "/history", "")
	if !strings.Contains(history.Body.String(), "项目联系人") {
		t.Fatal("authorized history missing")
	}
	if refs.calls != 0 {
		t.Fatalf("project-local context called master=%d", refs.calls)
	}
}
