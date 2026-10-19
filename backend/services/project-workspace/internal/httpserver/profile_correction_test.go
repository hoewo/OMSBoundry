package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

func TestProfileCorrectionRequiresItsOwnCapabilityAndProjectScope(t *testing.T) {
	for _, kind := range []string{"old update only", "wrong project", "tenant", "resource", "unknown field"} {
		t.Run(kind, func(t *testing.T) {
			principal := fullPrincipal()
			principal.AllowedActions = map[string]bool{auth.ActionUpdate: true}
			want := 403
			if kind != "old update only" {
				principal.AllowedActions[auth.ActionProfileCorrect] = true
			}
			if kind == "wrong project" || kind == "resource" {
				principal.ScopeKind = "resources"
				principal.ResourceIDs = map[string]bool{httpSubject: true}
				if kind == "resource" {
					principal.ResourceIDs = map[string]bool{httpProject: true}
				}
			}
			if kind == "tenant" || kind == "resource" {
				want = 200
			}
			body := `{"command_id":"` + httpCommand + `","expected_revision":3,"project_number":"OA-EDIT","name":"项目更正","project_type_keys":["design"],"correction_reason":"修改项目基本资料"}`
			if kind == "unknown field" {
				body = strings.TrimSuffix(body, "}") + `,"status":"cancelled"}`
				want = 400
			}
			useCases := &useCasesStub{}
			server, _ := New(useCases, &authenticatorStub{principal: principal})
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/profile-corrections", strings.NewReader(body)))
			if response.Code != want || (useCases.correctionCalls == 1) != (want == 200) {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, useCases.correctionCalls, response.Body.String())
			}
		})
	}
}

func TestProfileCorrectionAllowedActionOnlyForActiveProjects(t *testing.T) {
	for _, status := range []string{projects.StatusDraft, projects.StatusPendingCompletion, projects.StatusActive, projects.StatusCancelled} {
		details := projects.Details{Project: projects.Project{ID: httpProject, Status: status}}
		if got := containsAction(detailsAllowedActions(fullPrincipal(), details), auth.ActionProfileCorrect); got != (status == projects.StatusActive) {
			t.Fatalf("status=%s correction=%v", status, got)
		}
		if status == projects.StatusActive && containsAction(detailsAllowedActions(fullPrincipal(), details), auth.ActionUpdate) {
			t.Fatal("old update expanded to active")
		}
	}
}
