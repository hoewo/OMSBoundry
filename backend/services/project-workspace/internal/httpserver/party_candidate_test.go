package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

func TestProjectPartyCandidatesUseTenantScopeAndCreateCapability(t *testing.T) {
	useCases := &useCasesStub{partyCandidateResult: projects.PartyCandidatePage{
		SchemaVersion: "1",
		Items:         []projects.PartyCandidate{{PartyID: httpSubject, RoleKey: projects.PartyCandidateRoleContractor, SourceRevision: 2}},
	}}
	server, _ := New(useCases, &authenticatorStub{principal: fullPrincipal()})

	listed := httptest.NewRecorder()
	server.ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "/api/v1/project-party-candidates", nil))
	if listed.Code != http.StatusOK || useCases.partyCandidateCalls != 1 || useCases.partyCandidateTenant != httpTenant {
		t.Fatalf("list status=%d calls=%d tenant=%q body=%s", listed.Code, useCases.partyCandidateCalls, useCases.partyCandidateTenant, listed.Body.String())
	}
	var page partyCandidatePageResponse
	if err := json.Unmarshal(listed.Body.Bytes(), &page); err != nil || len(page.Items) != 1 || page.Items[0].RoleKey != projects.PartyCandidateRoleContractor || !containsAction(page.AllowedActions, auth.ActionCreate) {
		t.Fatalf("unexpected list page=%#v error=%v", page, err)
	}

	useCases.partyCandidateRegistered = projects.PartyCandidate{PartyID: httpSubject, RoleKey: projects.PartyCandidateRoleContractor, SourceRevision: 2}
	created := httptest.NewRecorder()
	body := `{"command_id":"` + httpCommand + `","party_id":"` + httpSubject + `","source_revision":2,"role_key":"contractor"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/project-party-candidates", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(created, request)
	if created.Code != http.StatusCreated || useCases.partyCandidateCalls != 2 || useCases.partyCandidateInput.PartyID != httpSubject || useCases.partyCandidateInput.RoleKey != projects.PartyCandidateRoleContractor {
		t.Fatalf("create status=%d calls=%d input=%#v body=%s", created.Code, useCases.partyCandidateCalls, useCases.partyCandidateInput, created.Body.String())
	}

	denied := fullPrincipal()
	delete(denied.AllowedActions, auth.ActionCreate)
	server, _ = New(useCases, &authenticatorStub{principal: denied})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/project-party-candidates", strings.NewReader(body)))
	if response.Code != http.StatusForbidden || useCases.partyCandidateCalls != 2 {
		t.Fatalf("denied status=%d calls=%d", response.Code, useCases.partyCandidateCalls)
	}

	resourceScoped := fullPrincipal()
	resourceScoped.ScopeKind = "resources"
	resourceScoped.ResourceIDs = map[string]bool{httpProject: true}
	server, _ = New(useCases, &authenticatorStub{principal: resourceScoped})
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/project-party-candidates", nil))
	if response.Code != http.StatusForbidden || useCases.partyCandidateCalls != 2 {
		t.Fatalf("resource scoped status=%d calls=%d", response.Code, useCases.partyCandidateCalls)
	}
}
