package httpserver

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/design-operations/internal/auth"
	"omsbizfoundry/design-operations/internal/design"
)

type stageAuthorizationCases struct {
	authorizationUseCases
	calls int
	actor design.Actor
}

func (s *stageAuthorizationCases) GetOwned(_ context.Context, tenant, id string) (design.Details, error) {
	if tenant != authTenant || id != authItem {
		return design.Details{}, design.ErrNotFound
	}
	return design.Details{WorkItem: design.WorkItem{ID: id, TenantID: tenant, ProjectID: authProject, TypeKey: design.DesignTypeKey, TypeDefinitionVersion: 2, Status: design.StatusDraft, Revision: 1}}, nil
}
func (s *stageAuthorizationCases) StageWorkItems(context.Context, string, string, string) ([]design.WorkItem, error) {
	s.calls++
	return []design.WorkItem{}, nil
}
func (s *stageAuthorizationCases) ProductionProgress(_ context.Context, tenant, project, _ string) (design.ProductionProgressProjection, error) {
	s.calls++
	return design.ProductionProgressProjection{SchemaVersion: "1.0", Profile: design.ProductionProgressProfile, TenantID: tenant, ProjectID: project, Items: []design.ProductionWorkItemProgress{}}, nil
}
func (s *stageAuthorizationCases) StageMaterials(context.Context, string, string, string) (design.StageMaterialCollection, error) {
	s.calls++
	return design.StageMaterialCollection{WorkItem: design.WorkItem{ID: authItem, TypeDefinitionVersion: 2, Status: design.StatusDraft}}, nil
}
func (s *stageAuthorizationCases) RegisterStageMaterials(_ context.Context, a design.Actor, _, _ string, _ design.StageMaterialInput) (design.StageMaterialCollection, error) {
	s.actor = a
	return s.StageMaterials(context.Background(), "", "", "")
}
func (s *stageAuthorizationCases) WithdrawStageMaterial(context.Context, design.Actor, string, string, string, design.ReasonInput) (design.StageMaterialCollection, error) {
	return s.StageMaterials(context.Background(), "", "", "")
}
func (s *stageAuthorizationCases) StageMaterialHistory(context.Context, string, string, int64, int) (design.HistoryPage, error) {
	s.calls++
	return design.HistoryPage{Items: []design.HistoryEvent{}}, nil
}
func (s *stageAuthorizationCases) UpgradeStageMaterialDefinition(_ context.Context, a design.Actor, id string, input design.DefinitionUpgradeInput) (design.Details, error) {
	s.calls++
	s.actor = a
	return design.Details{WorkItem: design.WorkItem{ID: id, TenantID: a.TenantID, ProjectID: authProject, TypeKey: design.StageMaterialTypeKey, TypeDefinitionVersion: input.TargetVersion, Status: design.StatusDraft, Revision: input.ExpectedRevision + 1}}, nil
}

func TestStageMaterialsRequireNativeCapabilitiesAndScope(t *testing.T) {
	uc := &stageAuthorizationCases{}
	provider := &authorizationAuthenticator{principal: auth.Principal{TenantID: authTenant, SubjectID: authSubject, Provider: "iam", ScopeKind: "resources", Resources: map[string]map[string]bool{"project": {authProject: true}}}}
	server, err := NewWithProduction(uc, provider, provider)
	if err != nil {
		t.Fatal(err)
	}
	route := "/api/v1/production-work-items/" + authItem + "/stages/scheme/material-collection"
	send := func(method, path, body string) int {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Actor-ID", "forged")
		server.ServeHTTP(w, r)
		return w.Code
	}
	body := `{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-000000000001","expected_revision":1,"main_files":[],"review_sheets":[],"score":"0"}`
	for _, action := range []string{auth.ProductionActionRead, auth.ProductionActionStageManage, auth.ActionDeliverableManage, auth.ProductionActionReferenceRead} {
		provider.principal.AllowedActions = map[string]bool{action: true}
		if status := send("GET", route, ""); status != 403 {
			t.Fatalf("old action %s read %d", action, status)
		}
		if status := send("POST", route, body); status != 403 {
			t.Fatalf("old action %s wrote %d", action, status)
		}
	}
	provider.principal.AllowedActions = map[string]bool{auth.ProductionActionStageMaterialRegister: true}
	if send("POST", route, body) != 403 || uc.calls != 0 {
		t.Fatal("write without read")
	}
	provider.principal.AllowedActions[auth.ProductionActionStageMaterialRead] = true
	provider.principal.Resources = map[string]map[string]bool{"project": {"55555555-5555-4555-8555-555555555555": true}}
	if send("POST", route, body) != 403 || uc.calls != 0 {
		t.Fatal("cross project write")
	}
	provider.principal.Resources = map[string]map[string]bool{"production-work-item": {authItem: true}}
	if send("POST", route, body) != 200 || uc.calls != 1 || uc.actor.ID != authSubject || uc.actor.TenantID != authTenant || uc.actor.Kind != "iam_subject" || provider.sawActorHeader != "" {
		t.Fatal("trusted actor or scoped write")
	}
	if send("POST", route, strings.TrimSuffix(body, "}")+`,"actor":"forged"}`) != 400 || uc.calls != 1 {
		t.Fatal("unknown actor payload accepted")
	}
	if send("GET", "/api/v1/production-stage-work-items?project_id="+authProject+"&type_key=design-v1", "") != 403 {
		t.Fatal("work item scope broadened to project list")
	}
	provider.principal.Resources = map[string]map[string]bool{"project": {authProject: true}}
	if send("GET", "/api/v1/production-stage-work-items?project_id="+authProject+"&type_key=design-v1", "") != 200 {
		t.Fatal("exact project query")
	}
	if send("GET", "/api/v1/production-stage-progress?project_id="+authProject+"&type_key="+design.StageMaterialTypeKey, "") != 200 {
		t.Fatal("exact project progress query")
	}
	provider.principal.TenantID = "55555555-5555-4555-8555-555555555555"
	if send("GET", route, "") != 404 {
		t.Fatal("cross tenant collection")
	}
}

func TestV2CreateLegacyHistoryAndAllowedActionsBoundary(t *testing.T) {
	uc := &stageAuthorizationCases{}
	provider := &authorizationAuthenticator{principal: auth.Principal{TenantID: authTenant, SubjectID: authSubject, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ProductionActionCreate: true, auth.ActionCreate: true, auth.ActionRead: true, auth.ProductionActionRead: true, auth.ProductionActionHistoryRead: true}}}
	server, _ := NewWithProduction(uc, provider, provider)
	send := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		server.ServeHTTP(w, r)
		return w
	}
	body := `{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-000000000001","project_id":"` + authProject + `","project_source_revision":1,"type_key":"design-v1","type_definition_version":2}`
	if send("POST", "/api/v1/production-work-items", body).Code != 403 || uc.createCalls != 0 {
		t.Fatal("v2 created with old capability")
	}
	provider.principal.AllowedActions[auth.ProductionActionStageMaterialRead] = true
	provider.principal.AllowedActions[auth.ProductionActionStageMaterialRegister] = true
	if send("POST", "/api/v1/production-work-items", body).Code != 201 || uc.createCalls != 1 || uc.lastCreate.TypeDefinitionVersion != 2 {
		t.Fatal("v2 create")
	}
	if send("POST", "/api/v1/design-work-items", body).Code != 400 || uc.createCalls != 1 {
		t.Fatal("v2 legacy create")
	}
	if send("GET", "/api/v1/design-work-items/"+authItem, "").Code != 404 {
		t.Fatal("v2 legacy get")
	}
	if send("GET", "/api/v1/production-work-items/"+authItem+"/history", "").Code != 422 {
		t.Fatal("new history leaked through old permission")
	}
	owned, _ := uc.GetOwned(context.Background(), authTenant, authItem)
	provider.principal.AllowedActions[auth.ProductionActionStageManage] = true
	provider.principal.AllowedActions[auth.ProductionActionDeliverableManage] = true
	for _, a := range detailsAllowedActions(owned, provider.principal, true) {
		if strings.Contains(a, "stage-material") || a == auth.ProductionActionStageManage || a == auth.ProductionActionDeliverableManage || a == auth.ProductionActionHistoryRead {
			t.Fatalf("generic details false action %s", a)
		}
	}
}
