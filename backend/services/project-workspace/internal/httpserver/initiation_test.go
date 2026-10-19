package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

func TestDispatchRequiresBothCreationAndBusinessAssignmentCapabilities(t *testing.T) {
	tests := []struct {
		name, node, scope string
		actions           []string
		want              int
	}{
		{"create only", "project", "tenant", []string{auth.ActionCreate}, 403},
		{"assign only", "project", "tenant", []string{auth.ActionAssignmentBusinessOwnerManage}, 403},
		{"wrong assignment", "project", "tenant", []string{auth.ActionCreate, auth.ActionAssignmentProductionOwnerManage}, 403},
		{"scoped credential", "project", "projects", []string{auth.ActionCreate, auth.ActionAssignmentManage}, 403},
		{"ordinary dispatch", "project", "tenant", []string{auth.ActionCreate, auth.ActionAssignmentBusinessOwnerManage}, 201},
		{"package needs hierarchy", "package", "tenant", []string{auth.ActionCreate, auth.ActionAssignmentManage}, 403},
		{"package dispatch", "package", "tenant", []string{auth.ActionHierarchyManage, auth.ActionAssignmentBusinessOwnerManage}, 201},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			principal := fullPrincipal()
			principal.ScopeKind = test.scope
			principal.AllowedActions = map[string]bool{}
			for _, action := range test.actions {
				principal.AllowedActions[action] = true
			}
			useCases := &useCasesStub{createResult: projects.Project{ID: httpProject, Status: projects.StatusDraft}}
			server, _ := New(useCases, &authenticatorStub{principal: principal})
			body := `{"command_id":"` + httpCommand + `","node_type":"` + test.node + `","name":"派发项目","business_owner":{"person_id":"` + httpSubject + `","source_revision":2}}`
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/initiation-dispatches", strings.NewReader(body)))
			if response.Code != test.want || (useCases.dispatchCalls == 1) != (test.want == 201) {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, useCases.dispatchCalls, response.Body.String())
			}
		})
	}
}

func TestInitiationProfileDoesNotGrantCompletionOrExpandProjectScope(t *testing.T) {
	for _, denied := range []string{"capability", "scope"} {
		t.Run(denied, func(t *testing.T) {
			principal := fullPrincipal()
			if denied == "capability" {
				delete(principal.AllowedActions, auth.ActionComplete)
			} else {
				principal.ScopeKind = "resources"
				principal.ResourceIDs = map[string]bool{httpSubject: true}
			}
			useCases := &useCasesStub{}
			server, _ := New(useCases, &authenticatorStub{principal: principal})
			body := `{"command_id":"` + httpCommand + `","expected_revision":1,"initiation_profile":"business-owner-v1"}`
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/complete-initiation", strings.NewReader(body)))
			if response.Code != 403 || useCases.completeCalls != 0 {
				t.Fatalf("status=%d calls=%d", response.Code, useCases.completeCalls)
			}
		})
	}
}

func TestPackageInitiationUsesTheSameNativeCompletionCapability(t *testing.T) {
	draft := projects.Details{Project: projects.Project{ID: httpProject, NodeType: projects.NodeTypePackage, Status: projects.StatusDraft}}
	if !containsAction(detailsAllowedActions(fullPrincipal(), draft), auth.ActionComplete) {
		t.Fatal("draft package must expose its supported native completion action")
	}
	principal := fullPrincipal()
	delete(principal.AllowedActions, auth.ActionComplete)
	if containsAction(detailsAllowedActions(principal, draft), auth.ActionComplete) {
		t.Fatal("package type must not grant completion")
	}
}
