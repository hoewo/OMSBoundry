package httpserver

import (
	"context"
	"net/http/httptest"
	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
	personnel "omsbizfoundry/project-workspace/internal/referenceoa/productionpersonnel"
	"strings"
	"testing"
)

type oaHTTPStub struct {
	calls int
	actor projects.Actor
	view  projects.OAProductionView
}

func (s *oaHTTPStub) AssignmentReferences(context.Context, string, string, string) (projects.OAProductionAssignmentReferences, error) {
	s.calls++
	return projects.OAProductionAssignmentReferences{Items: []projects.OAProductionAssignmentReference{{ID: httpProject, ResourceType: projects.OAProductionAssignmentResource}}}, nil
}

func (s *oaHTTPStub) Personnel(context.Context, string, string, bool) (projects.OAProductionView, error) {
	s.calls++
	return s.view, nil
}
func (s *oaHTTPStub) ProductionProfile(context.Context, string, string) (projects.OAProductionProfileContext, error) {
	s.calls++
	return projects.OAProductionProfileContext{Profile: personnel.Profile, ProjectID: httpProject, ProjectRevision: 4, ProjectStatus: s.view.ProjectStatus, Installed: false}, nil
}
func (s *oaHTTPStub) Enroll(_ context.Context, a projects.Actor, _ string, _ projects.RevisionCommand) (projects.OAProductionReceipt, error) {
	s.calls++
	s.actor = a
	return projects.OAProductionReceipt{}, nil
}
func (s *oaHTTPStub) ApplyPersonnel(_ context.Context, a projects.Actor, _ string, _ personnel.Intention) (projects.OAProductionReceipt, error) {
	s.calls++
	s.actor = a
	return projects.OAProductionReceipt{}, nil
}
func (s *oaHTTPStub) SetParticipantApplicable(_ context.Context, a projects.Actor, _ string, _ projects.RevisionCommand, _ bool) (projects.OAProductionReceipt, error) {
	s.calls++
	s.actor = a
	return projects.OAProductionReceipt{}, nil
}
func (s *oaHTTPStub) Disciplines(context.Context, string, bool) ([]projects.OADiscipline, error) {
	s.calls++
	return []projects.OADiscipline{}, nil
}
func (s *oaHTTPStub) ChangeDiscipline(_ context.Context, a projects.Actor, _ projects.OADisciplineCommand) (projects.OADiscipline, error) {
	s.calls++
	s.actor = a
	return projects.OADiscipline{}, nil
}
func TestOAProductionHTTPOptInCapabilitiesScopeAndTrustedActor(t *testing.T) {
	path := "/api/v1/projects/" + httpProject + "/reference-oa/"
	for _, tc := range []struct {
		name, method, url, action, scope, body string
		enabled                                bool
		want                                   int
	}{
		{"off despite grant", "GET", path + "production-personnel", auth.ActionOAProductionPersonnelRead, "tenant", "", false, 404},
		{"reference off", "GET", "/api/v1/references/projects/" + httpProject + "/reference-oa/production-assignments", auth.ActionOAProductionPersonnelRead, "exact", "", false, 404},
		{"reference old read forbidden", "GET", "/api/v1/references/projects/" + httpProject + "/reference-oa/production-assignments", auth.ActionRead, "exact", "", true, 403},
		{"reference exact new read", "GET", "/api/v1/references/projects/" + httpProject + "/reference-oa/production-assignments", auth.ActionOAProductionPersonnelRead, "exact", "", true, 200},
		{"reference wrong scope", "GET", "/api/v1/references/projects/" + httpProject + "/reference-oa/production-assignments/" + httpProject, auth.ActionOAProductionPersonnelRead, "wrong", "", true, 403},
		{"reference single exact", "GET", "/api/v1/references/projects/" + httpProject + "/reference-oa/production-assignments/" + httpProject, auth.ActionOAProductionPersonnelRead, "exact", "", true, 200},
		{"old read not personnel", "GET", path + "production-personnel", auth.ActionRead, "tenant", "", true, 403},
		{"old broad not personnel", "POST", path + "production-personnel", auth.ActionAssignmentManage, "tenant", "{}", true, 403},
		{"new read exact", "GET", path + "production-personnel", auth.ActionOAProductionPersonnelRead, "exact", "", true, 200},
		{"new read wrong scope", "GET", path + "production-personnel", auth.ActionOAProductionPersonnelRead, "wrong", "", true, 403},
		{"read no write", "POST", path + "production-personnel", auth.ActionOAProductionPersonnelRead, "exact", "{}", true, 403},
		{"write no read", "GET", path + "production-personnel", auth.ActionOAProductionPersonnelManage, "exact", "", true, 403},
		{"new write trusted actor", "POST", path + "production-personnel", auth.ActionOAProductionPersonnelManage, "exact", "{}", true, 200},
		{"install context requires manage", "GET", path + "production-profile", auth.ActionOAProductionPersonnelRead, "exact", "", true, 403},
		{"install context exact", "GET", path + "production-profile", auth.ActionOAProductionPersonnelManage, "exact", "", true, 200},
		{"install context wrong scope", "GET", path + "production-profile", auth.ActionOAProductionPersonnelManage, "wrong", "", true, 403},
		{"install exact", "POST", path + "production-profile", auth.ActionOAProductionPersonnelManage, "exact", "{}", true, 200},
		{"missing explicit applicable", "POST", path + "participant-applicability", auth.ActionOAProductionPersonnelManage, "exact", "{}", true, 400},
		{"explicit applicable", "POST", path + "participant-applicability", auth.ActionOAProductionPersonnelManage, "exact", `{"not_applicable":false}`, true, 200},
		{"project catalogue discovery", "GET", path + "production-disciplines", auth.ActionOAProductionPersonnelRead, "exact", "", true, 200},
		{"personnel not dictionary admin", "POST", "/api/v1/reference-oa/production-disciplines", auth.ActionOAProductionPersonnelManage, "tenant", "{}", true, 403},
		{"dictionary management tenant", "POST", "/api/v1/reference-oa/production-disciplines", auth.ActionOAProductionDisciplineManage, "tenant", "{}", true, 200},
		{"dictionary management not project scope", "POST", "/api/v1/reference-oa/production-disciplines", auth.ActionOAProductionDisciplineManage, "exact", "{}", true, 403},
		{"dictionary read tenant", "GET", "/api/v1/reference-oa/production-disciplines", auth.ActionOAProductionDisciplineRead, "tenant", "", true, 200},
		{"actor JSON rejected", "POST", path + "production-personnel", auth.ActionOAProductionPersonnelManage, "exact", `{"actor":{"id":"admin"}}`, true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fullPrincipal()
			p.AllowedActions = map[string]bool{tc.action: true}
			if tc.scope != "tenant" {
				p.ScopeKind = "resources"
				p.ResourceIDs = map[string]bool{httpProject: tc.scope == "exact"}
			}
			oa := &oaHTTPStub{view: projects.OAProductionView{Profile: personnel.Profile, ProjectStatus: projects.StatusActive, Facts: []projects.OAProductionFact{}}}
			var selected OAProductionUseCases
			if tc.enabled {
				selected = oa
			}
			s, _ := NewWithOAProduction(&useCasesStub{}, &authenticatorStub{principal: p}, selected)
			r := httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Actor-ID", "forged")
			r.Header.Set("X-Tenant-ID", "forged")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if tc.want != 200 && oa.calls != 0 {
				t.Fatal("denied reached domain")
			}
			if oa.actor.ID != "" && (oa.actor.ID != p.SubjectID || oa.actor.TenantID != p.TenantID) {
				t.Fatal("client headers became Actor")
			}
		})
	}
}
func TestOAProductionHTTPReadonlyAndCancelledAllowedActions(t *testing.T) {
	for _, status := range []string{projects.StatusActive, projects.StatusCancelled} {
		p := fullPrincipal()
		p.ScopeKind = "resources"
		p.ResourceIDs = map[string]bool{httpProject: true}
		p.AllowedActions = map[string]bool{auth.ActionOAProductionPersonnelRead: true, auth.ActionOAProductionPersonnelManage: true, auth.ActionOAProductionDisciplineManage: true}
		oa := &oaHTTPStub{view: projects.OAProductionView{ProjectStatus: status, Facts: []projects.OAProductionFact{}}}
		s, _ := NewWithOAProduction(&useCasesStub{}, &authenticatorStub{principal: p}, oa)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/projects/"+httpProject+"/reference-oa/production-personnel", nil))
		if strings.Contains(w.Body.String(), auth.ActionOAProductionDisciplineManage) {
			t.Fatal("resource scope became tenant catalogue authority")
		}
		if strings.Contains(w.Body.String(), auth.ActionOAProductionPersonnelManage) != (status != projects.StatusCancelled) {
			t.Fatal("cancelled or live allowed actions wrong")
		}
	}
}
