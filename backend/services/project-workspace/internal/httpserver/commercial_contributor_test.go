package httpserver

import (
	"net/http"
	"net/http/httptest"
	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
	"strings"
	"testing"
)

func TestCommercialContributorCapabilityNeverExpandsLegacyAssignmentGrant(t *testing.T) {
	assign := `{"command_id":"` + httpCommand + `","expected_revision":1,"operation":"assign","subject_kind":"person","subject_id":"` + httpSubject + `","source_revision":1,"role_key":"commercial_contributor"}`
	end := `{"command_id":"` + httpCommand + `","expected_revision":2,"operation":"end","assignment_id":"66666666-6666-4666-8666-666666666666","role_key":"business_owner","end_reason":"移出"}`
	tests := []struct {
		name, action, body, actualRole string
		scopeWrong                     bool
		want                           int
	}{
		{name: "old broad grant cannot add", action: auth.ActionAssignmentManage, body: assign, want: 403},
		{name: "old participant grant cannot add", action: auth.ActionAssignmentParticipantManage, body: assign, want: 403},
		{name: "business owner grant cannot add", action: auth.ActionAssignmentBusinessOwnerManage, body: assign, want: 403},
		{name: "new explicit grant", action: auth.ActionAssignmentCommercialContributorManage, body: assign, want: 200},
		{name: "normalization cannot bypass", action: auth.ActionAssignmentManage, body: strings.ReplaceAll(assign, "commercial_contributor", " COMMERCIAL_CONTRIBUTOR "), want: 403},
		{name: "end resolves actual role", action: auth.ActionAssignmentManage, body: end, actualRole: projects.RoleCommercialContributor, want: 403},
		{name: "normalized end resolves role", action: auth.ActionAssignmentManage, body: strings.ReplaceAll(end, `"end"`, `" END "`), actualRole: projects.RoleCommercialContributor, want: 403},
		{name: "new grant cannot end owner", action: auth.ActionAssignmentCommercialContributorManage, body: end, actualRole: projects.RoleBusinessOwner, want: 403},
		{name: "new grant ends commercial only", action: auth.ActionAssignmentCommercialContributorManage, body: end, actualRole: projects.RoleCommercialContributor, want: 200},
		{name: "resource scope still enforced", action: auth.ActionAssignmentCommercialContributorManage, body: assign, scopeWrong: true, want: 403},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := fullPrincipal()
			p.AllowedActions = map[string]bool{tc.action: true}
			p.ScopeKind = "resources"
			p.ResourceIDs = map[string]bool{httpProject: !tc.scopeWrong}
			cases := &useCasesStub{referenceAssignment: projects.AssignmentReference{RoleKey: tc.actualRole}}
			s, _ := New(cases, &authenticatorStub{principal: p})
			r := httptest.NewRecorder()
			s.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/assignments", strings.NewReader(tc.body)))
			if r.Code != tc.want || (cases.assignmentCalls == 1) != (tc.want == 200) {
				t.Fatalf("status=%d writes=%d body=%s", r.Code, cases.assignmentCalls, r.Body.String())
			}
		})
	}
}

func TestCommercialContributorAllowedActionsRequiresGrantAndMutableProject(t *testing.T) {
	for _, status := range []string{projects.StatusDraft, projects.StatusActive, projects.StatusCancelled} {
		p := fullPrincipal()
		p.AllowedActions = map[string]bool{auth.ActionAssignmentCommercialContributorManage: true}
		d := projects.Details{Project: projects.Project{ID: httpProject, TenantID: httpTenant, Status: status}}
		if containsAction(detailsAllowedActions(p, d), auth.ActionAssignmentCommercialContributorManage) != (status != projects.StatusCancelled) {
			t.Fatalf("status=%s action list wrong", status)
		}
		p.AllowedActions = map[string]bool{auth.ActionAssignmentManage: true}
		if containsAction(detailsAllowedActions(p, d), auth.ActionAssignmentCommercialContributorManage) {
			t.Fatal("legacy grant widened")
		}
	}
}
