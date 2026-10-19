package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/design-operations/internal/auth"
	"omsbizfoundry/design-operations/internal/design"
)

const (
	authTenant  = "11111111-1111-4111-8111-111111111111"
	authSubject = "22222222-2222-4222-8222-222222222222"
	authProject = "33333333-3333-4333-8333-333333333333"
	authItem    = "44444444-4444-4444-8444-444444444444"
)

type authorizationUseCases struct {
	createCalls int
	lastCreate  design.CreateInput
	listResult  []design.WorkItem
	workItems   []design.WorkItemCandidate
	workItemErr error
	workSubject string
}

func (*authorizationUseCases) Ping(context.Context) error { return nil }
func (s *authorizationUseCases) List(context.Context, string, int) ([]design.WorkItem, error) {
	return s.listResult, nil
}
func (s *authorizationUseCases) WorkItemsForSubject(_ context.Context, _, subject string, _ int) ([]design.WorkItemCandidate, error) {
	s.workSubject = subject
	return s.workItems, s.workItemErr
}

func TestDemoEntryAndListRespectResourceScope(t *testing.T) {
	useCases := &authorizationUseCases{listResult: []design.WorkItem{
		{ID: authItem, ProjectID: authProject, TypeKey: design.DesignTypeKey, Status: design.StatusInProgress},
		{ID: "55555555-5555-4555-8555-555555555555", ProjectID: "66666666-6666-4666-8666-666666666666", Status: design.StatusDraft},
		{ID: "77777777-7777-4777-8777-777777777777", ProjectID: authProject, TypeKey: design.ConsultationTypeKey, Status: design.StatusDraft},
	}}
	authenticator := &authorizationAuthenticator{principal: auth.Principal{TenantID: authTenant, SubjectID: authSubject, AllowedActions: map[string]bool{auth.ActionRead: true}, ScopeKind: "resources", Resources: map[string]map[string]bool{"project": {authProject: true}}}}
	server, _ := New(useCases, authenticator)
	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "设计工作项") || !strings.Contains(page.Body.String(), "assets/demo.js") {
		t.Fatalf("demo page status=%d body=%q", page.Code, page.Body.String())
	}
	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/design-work-items", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), authItem) || strings.Contains(list.Body.String(), "55555555-5555-4555-8555-555555555555") || strings.Contains(list.Body.String(), "77777777-7777-4777-8777-777777777777") {
		t.Fatalf("scoped list status=%d body=%q", list.Code, list.Body.String())
	}
}
func (s *authorizationUseCases) Create(_ context.Context, _ design.Actor, input design.CreateInput) (design.Details, error) {
	s.createCalls++
	s.lastCreate = input
	return design.Details{WorkItem: design.WorkItem{ID: authItem, ProjectID: input.ProjectID}}, nil
}
func (*authorizationUseCases) SetOwner(context.Context, design.Actor, string, design.OwnerInput) (design.Details, error) {
	return design.Details{}, nil
}
func (*authorizationUseCases) PutDeliverable(context.Context, design.Actor, string, string, design.DeliverableInput) (design.Deliverable, error) {
	return design.Deliverable{}, nil
}
func (*authorizationUseCases) SetApplicability(context.Context, design.Actor, string, string, design.ApplicabilityInput) (design.Details, error) {
	return design.Details{}, nil
}
func (*authorizationUseCases) CompleteStage(context.Context, design.Actor, string, string, design.RevisionInput) (design.Details, error) {
	return design.Details{}, nil
}
func (*authorizationUseCases) ReopenStage(context.Context, design.Actor, string, string, design.ReasonInput) (design.Details, error) {
	return design.Details{}, nil
}
func (*authorizationUseCases) Cancel(context.Context, design.Actor, string, design.ReasonInput) (design.Details, error) {
	return design.Details{}, nil
}
func (*authorizationUseCases) GetOwned(_ context.Context, _ string, id string) (design.Details, error) {
	return design.Details{WorkItem: design.WorkItem{ID: id, ProjectID: authProject}}, nil
}
func (*authorizationUseCases) Get(context.Context, string, string) (design.Details, error) {
	return design.Details{WorkItem: design.WorkItem{ID: authItem, ProjectID: authProject}}, nil
}
func (*authorizationUseCases) History(context.Context, string, string, int64, int) (design.HistoryPage, error) {
	return design.HistoryPage{}, nil
}
func (*authorizationUseCases) Reference(context.Context, string, string) (design.ReferenceView, error) {
	return design.ReferenceView{}, nil
}
func (*authorizationUseCases) ProductionReference(context.Context, string, string) (design.ReferenceView, error) {
	return design.ReferenceView{}, nil
}
func (*authorizationUseCases) ReferenceMaterials(context.Context, string, string) (design.MaterialReferenceProjection, error) {
	return design.MaterialReferenceProjection{}, nil
}

type authorizationAuthenticator struct {
	principal      auth.Principal
	err            error
	sawActorHeader string
}

func (*authorizationAuthenticator) ProviderName() string { return "test" }
func (a *authorizationAuthenticator) Authenticate(r *http.Request) (auth.Principal, error) {
	a.sawActorHeader = r.Header.Get("X-Actor-ID")
	return a.principal, a.err
}

func TestAuthenticationCapabilityScopeAndHeaderBoundary(t *testing.T) {
	useCases := &authorizationUseCases{}
	authenticator := &authorizationAuthenticator{err: auth.ErrUnauthenticated}
	server, _ := New(useCases, authenticator)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/design-work-items/"+authItem, nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", response.Code)
	}

	authenticator.err = nil
	authenticator.principal = auth.Principal{TenantID: authTenant, SubjectID: authSubject, AllowedActions: map[string]bool{auth.ActionRead: true}, ScopeKind: "resources", Resources: map[string]map[string]bool{"project": {"55555555-5555-4555-8555-555555555555": true}}}
	response = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/design-work-items/"+authItem, nil)
	request.Header.Set("X-Actor-ID", "forged")
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || authenticator.sawActorHeader != "" {
		t.Fatalf("scope status=%d actor_header=%q", response.Code, authenticator.sawActorHeader)
	}

	authenticator.principal = auth.Principal{TenantID: authTenant, SubjectID: authSubject, AllowedActions: map[string]bool{auth.ActionCreate: true}, ScopeKind: "resources", Resources: map[string]map[string]bool{"project": {authProject: true}}}
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/design-work-items", strings.NewReader(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-000000000001","project_id":"`+authProject+`","project_source_revision":1}`))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || useCases.createCalls != 1 {
		t.Fatalf("create status=%d calls=%d body=%s", response.Code, useCases.createCalls, response.Body.String())
	}
	if useCases.lastCreate.TypeKey != design.DesignTypeKey {
		t.Fatalf("legacy Design API did not pin design-v1: %#v", useCases.lastCreate)
	}
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/design-work-items", strings.NewReader(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-000000000002","project_id":"`+authProject+`","project_source_revision":1,"type_key":"consultation-v1"}`))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || useCases.createCalls != 1 {
		t.Fatalf("cross-type legacy create status=%d calls=%d body=%s", response.Code, useCases.createCalls, response.Body.String())
	}
}

func TestProductionAndLegacyRoutesUseSeparateCapabilitiesAndScopes(t *testing.T) {
	consultationID := "77777777-7777-4777-8777-777777777777"
	useCases := &authorizationUseCases{listResult: []design.WorkItem{
		{ID: authItem, ProjectID: authProject, TypeKey: design.DesignTypeKey, Status: design.StatusInProgress},
		{ID: consultationID, ProjectID: authProject, TypeKey: design.ConsultationTypeKey, Status: design.StatusDraft},
	}}
	designAuthenticator := &authorizationAuthenticator{principal: auth.Principal{
		TenantID: authTenant, SubjectID: authSubject,
		AllowedActions: map[string]bool{auth.ActionRead: true, auth.ActionCreate: true},
		ScopeKind:      "resources",
		Resources:      map[string]map[string]bool{"project": {authProject: true}},
	}}
	productionAuthenticator := &authorizationAuthenticator{principal: auth.Principal{
		TenantID: authTenant, SubjectID: authSubject,
		AllowedActions: map[string]bool{auth.ProductionActionRead: true, auth.ProductionActionCreate: true},
		ScopeKind:      "resources",
		Resources:      map[string]map[string]bool{"project": {authProject: true}},
	}}
	server, err := NewWithProduction(useCases, designAuthenticator, productionAuthenticator)
	if err != nil {
		t.Fatal(err)
	}
	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Production Operations") || !strings.Contains(page.Body.String(), "consultation-v1") || !strings.Contains(page.Body.String(), "assets/demo.js") {
		t.Fatalf("production page status=%d body=%s", page.Code, page.Body.String())
	}
	script := httptest.NewRecorder()
	server.ServeHTTP(script, httptest.NewRequest(http.MethodGet, "/assets/demo.js", nil))
	for _, marker := range []string{"api/v1/production-work-items", "type_key:f.get('type_key')", "refreshPermissions", "allowed_actions"} {
		if !strings.Contains(script.Body.String(), marker) {
			t.Fatalf("production browser script is missing %q", marker)
		}
	}

	legacyList := httptest.NewRecorder()
	server.ServeHTTP(legacyList, httptest.NewRequest(http.MethodGet, "/api/v1/design-work-items", nil))
	if legacyList.Code != http.StatusOK || !strings.Contains(legacyList.Body.String(), authItem) || strings.Contains(legacyList.Body.String(), consultationID) {
		t.Fatalf("legacy list status=%d body=%s", legacyList.Code, legacyList.Body.String())
	}

	productionList := httptest.NewRecorder()
	server.ServeHTTP(productionList, httptest.NewRequest(http.MethodGet, "/api/v1/production-work-items", nil))
	if productionList.Code != http.StatusOK || !strings.Contains(productionList.Body.String(), authItem) || !strings.Contains(productionList.Body.String(), consultationID) || !strings.Contains(productionList.Body.String(), `"allowed_actions"`) || !strings.Contains(productionList.Body.String(), auth.ProductionActionCreate) {
		t.Fatalf("production list status=%d body=%s", productionList.Code, productionList.Body.String())
	}

	productionDetails := httptest.NewRecorder()
	server.ServeHTTP(productionDetails, httptest.NewRequest(http.MethodGet, "/api/v1/production-work-items/"+consultationID, nil))
	if productionDetails.Code != http.StatusOK || !strings.Contains(productionDetails.Body.String(), `"allowed_actions"`) || !strings.Contains(productionDetails.Body.String(), auth.ProductionActionRead) || !strings.Contains(productionDetails.Body.String(), auth.ProductionActionCreate) {
		t.Fatalf("production details status=%d body=%s", productionDetails.Code, productionDetails.Body.String())
	}

	create := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/production-work-items", strings.NewReader(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-000000000003","project_id":"`+authProject+`","project_source_revision":1,"type_key":"consultation-v1"}`))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(create, request)
	if create.Code != http.StatusCreated || useCases.lastCreate.TypeKey != design.ConsultationTypeKey {
		t.Fatalf("production create status=%d type=%q body=%s", create.Code, useCases.lastCreate.TypeKey, create.Body.String())
	}

	productionAuthenticator.principal.AllowedActions = map[string]bool{auth.ActionRead: true}
	denied := httptest.NewRecorder()
	server.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/api/v1/production-work-items", nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("legacy capability reached production route: status=%d body=%s", denied.Code, denied.Body.String())
	}

	productionAuthenticator.principal.AllowedActions = map[string]bool{auth.ProductionActionRead: true}
	productionAuthenticator.principal.Resources = map[string]map[string]bool{"production-work-item": {authItem: true}}
	allowedByItem := httptest.NewRecorder()
	server.ServeHTTP(allowedByItem, httptest.NewRequest(http.MethodGet, "/api/v1/production-work-items/"+authItem, nil))
	if allowedByItem.Code != http.StatusOK {
		t.Fatalf("exact production work item scope status=%d body=%s", allowedByItem.Code, allowedByItem.Body.String())
	}
	adjacentItem := "99999999-9999-4999-8999-999999999999"
	deniedByItem := httptest.NewRecorder()
	server.ServeHTTP(deniedByItem, httptest.NewRequest(http.MethodGet, "/api/v1/production-work-items/"+adjacentItem, nil))
	if deniedByItem.Code != http.StatusForbidden {
		t.Fatalf("adjacent production work item scope status=%d body=%s", deniedByItem.Code, deniedByItem.Body.String())
	}
}

func TestWorkItemProjectionUsesCredentialSubjectScopeAndActions(t *testing.T) {
	stage := "scheme"
	useCases := &authorizationUseCases{workItems: []design.WorkItemCandidate{
		{WorkItem: design.WorkItem{ID: authItem, TenantID: authTenant, ProjectID: authProject, TypeKey: design.DesignTypeKey, Status: design.StatusInProgress, Revision: 7, CurrentStageKey: &stage}},
		{WorkItem: design.WorkItem{ID: "99999999-9999-4999-8999-999999999999", TenantID: authTenant, ProjectID: "88888888-8888-4888-8888-888888888888", TypeKey: design.ConsultationTypeKey, Status: design.StatusInProgress, Revision: 2}},
	}}
	authenticator := &authorizationAuthenticator{principal: auth.Principal{
		TenantID: authTenant, SubjectID: authSubject, ScopeKind: "resources",
		AllowedActions: map[string]bool{auth.ProductionActionWorkItemsRead: true, auth.ProductionActionStageManage: true},
		Resources:      map[string]map[string]bool{"project": {authProject: true}},
	}}
	server, err := NewWithProduction(useCases, &authorizationAuthenticator{}, authenticator)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/work-items", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || useCases.workSubject != authSubject || !strings.Contains(body, `"schema_version":"1"`) || !strings.Contains(body, `"subject_id":"`+authSubject+`"`) || !strings.Contains(body, authItem) || !strings.Contains(body, auth.ProductionActionStageManage) || strings.Contains(body, "99999999-9999-4999-8999-999999999999") {
		t.Fatalf("work item projection status=%d body=%s", response.Code, body)
	}

	authenticator.principal.AllowedActions = map[string]bool{auth.ProductionActionStageManage: true}
	denied := httptest.NewRecorder()
	server.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/api/v1/work-items", nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing work-items.read status=%d body=%s", denied.Code, denied.Body.String())
	}

	authenticator.principal.AllowedActions = map[string]bool{auth.ProductionActionWorkItemsRead: true}
	readOnly := httptest.NewRecorder()
	server.ServeHTTP(readOnly, httptest.NewRequest(http.MethodGet, "/api/v1/work-items", nil))
	if readOnly.Code != http.StatusOK || !strings.Contains(readOnly.Body.String(), `"items":[]`) {
		t.Fatalf("revoked executable actions status=%d body=%s", readOnly.Code, readOnly.Body.String())
	}

	authenticator.principal.AllowedActions = map[string]bool{auth.ProductionActionWorkItemsRead: true, auth.ProductionActionStageManage: true}
	useCases.workItemErr = design.ErrReferenceUnavailable
	unavailable := httptest.NewRecorder()
	server.ServeHTTP(unavailable, httptest.NewRequest(http.MethodGet, "/api/v1/work-items", nil))
	if unavailable.Code != http.StatusServiceUnavailable || !strings.Contains(unavailable.Body.String(), "REFERENCE_SERVICE_UNAVAILABLE") {
		t.Fatalf("source reference outage status=%d body=%s", unavailable.Code, unavailable.Body.String())
	}
}
