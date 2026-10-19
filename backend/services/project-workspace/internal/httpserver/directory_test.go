package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

func TestProjectDetailsCannotLeakUnscopedParentOrChildren(t *testing.T) {
	principal := fullPrincipal()
	principal.ScopeKind = "resources"
	principal.ResourceIDs = map[string]bool{httpProject: true}
	useCases := &useCasesStub{getResult: projects.Details{Project: projects.Project{ID: httpProject}, Parent: &projects.ProjectSummary{ID: httpCommand, Name: "secret parent"}, Children: []projects.ProjectSummary{{ID: httpCommand, Name: "secret child"}}}}
	server, _ := New(useCases, &authenticatorStub{principal: principal})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject, nil))
	var details projects.Details
	if err := json.Unmarshal(response.Body.Bytes(), &details); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || details.Parent != nil || len(details.Children) != 0 {
		t.Fatalf("leaked %s", response.Body.String())
	}
	principal.ResourceIDs[httpCommand] = true
	server, _ = New(useCases, &authenticatorStub{principal: principal})
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject, nil))
	_ = json.Unmarshal(response.Body.Bytes(), &details)
	if response.Code != 200 || details.Parent == nil || len(details.Children) != 1 {
		t.Fatalf("authorized context missing %s", response.Body.String())
	}
}

func TestDirectoryUsesOnlyAuthenticatedTenantScopeAndRejectsInvalidQueries(t *testing.T) {
	for _, query := range []string{"limit=0", "limit=101", "offset=-1", "offset=1.2", "limit=10&limit=20", "status=draft", "resource_ids=*", "tenant_id=other", "scope_kind=tenant"} {
		useCases := &useCasesStub{}
		server, _ := New(useCases, &authenticatorStub{principal: fullPrincipal()})
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/project-directory?"+query, nil))
		if response.Code != 400 || useCases.directoryCalls != 0 {
			t.Fatalf("%s status=%d calls=%d", query, response.Code, useCases.directoryCalls)
		}
	}
	useCases := &useCasesStub{directoryResult: projects.DirectoryPage{SchemaVersion: "1", Items: []projects.DirectoryEntry{}, Total: 108, Offset: 100, Limit: 10}}
	principal := fullPrincipal()
	principal.ScopeKind, principal.ResourceIDs = "resources", map[string]bool{httpProject: true, httpCommand: false}
	authenticator := &authenticatorStub{principal: principal}
	server, _ := New(useCases, authenticator)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/project-directory?offset=100&limit=10&keyword=child&status=all", nil)
	request.Header.Set("X-Actor-ID", "spoofed")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != 200 || useCases.directoryTenant != httpTenant || authenticator.sawHeader != "" || useCases.directoryQuery.ScopeKind != "resources" || len(useCases.directoryQuery.ResourceIDs) != 1 || useCases.directoryQuery.ResourceIDs[0] != httpProject || useCases.directoryQuery.Offset != 100 || useCases.directoryQuery.Status != "all" {
		t.Fatalf("scope query %#v status=%d", useCases.directoryQuery, response.Code)
	}
	// The new endpoint does not widen the old tenant-only List contract.
	legacy := httptest.NewRecorder()
	server.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/api/v1/projects?limit=100", nil))
	if legacy.Code != 403 {
		t.Fatalf("legacy resource list=%d", legacy.Code)
	}
}

func TestDirectoryReturnsCurrentPartyRelationshipForOMSProjection(t *testing.T) {
	partyRevision := int64(7)
	useCases := &useCasesStub{directoryResult: projects.DirectoryPage{
		SchemaVersion: "1",
		Items: []projects.DirectoryEntry{{
			Project: projects.Project{
				ID:       httpProject,
				Name:     "道路养护项目",
				Revision: 4,
			},
			ResourceLinks: []projects.ResourceLink{{
				ID:             httpCommand,
				ProjectID:      httpProject,
				TargetService:  "party-directory",
				ResourceType:   "party",
				ResourceID:     httpSubject,
				SourceRevision: &partyRevision,
				RelationKey:    "contractor-party",
			}},
		}},
		Total: 1,
		Limit: 100,
	}}
	server, _ := New(useCases, &authenticatorStub{principal: fullPrincipal()})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/project-directory?limit=100", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []projects.DirectoryEntry `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 1 || len(payload.Items[0].ResourceLinks) != 1 {
		t.Fatalf("missing relationship: %s", response.Body.String())
	}
	link := payload.Items[0].ResourceLinks[0]
	if link.TargetService != "party-directory" || link.ResourceType != "party" || link.ResourceID != httpSubject || link.RelationKey != "contractor-party" || link.SourceRevision == nil || *link.SourceRevision != partyRevision {
		t.Fatalf("wrong relationship: %#v", link)
	}
}

func TestDirectoryRejectsMissingCredentialCapabilityAndScopeBeforeReading(t *testing.T) {
	for _, fixture := range []struct {
		principal auth.Principal
		err       error
		status    int
	}{
		{err: errors.New("expired"), status: 401},
		{principal: auth.Principal{TenantID: httpTenant, ScopeKind: "tenant"}, status: 403},
		{principal: auth.Principal{TenantID: httpTenant, ScopeKind: "none", AllowedActions: map[string]bool{auth.ActionRead: true}}, status: 403},
	} {
		useCases := &useCasesStub{}
		server, _ := New(useCases, &authenticatorStub{principal: fixture.principal, err: fixture.err})
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/project-directory", nil))
		if response.Code != fixture.status || useCases.directoryCalls != 0 {
			t.Fatalf("status=%d calls=%d", response.Code, useCases.directoryCalls)
		}
	}
}
