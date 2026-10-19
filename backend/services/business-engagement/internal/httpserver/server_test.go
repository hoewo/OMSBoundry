package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/business-engagement/internal/auth"
	"omsbizfoundry/business-engagement/internal/engagement"
)

const (
	tenantID    = "11111111-1111-4111-8111-111111111111"
	subjectID   = "22222222-2222-4222-8222-222222222222"
	projectA    = "33333333-3333-4333-8333-333333333333"
	projectB    = "44444444-4444-4444-8444-444444444444"
	engagementA = "55555555-5555-4555-8555-555555555555"
	engagementB = "66666666-6666-4666-8666-666666666666"
	commandID   = "77777777-7777-4777-8777-777777777777"
)

type fakeAuthenticator struct {
	principal auth.Principal
	err       error
	header    string
}

func (f *fakeAuthenticator) ProviderName() string { return "fake" }
func (f *fakeAuthenticator) Authenticate(r *http.Request) (auth.Principal, error) {
	f.header = r.Header.Get("X-Actor-ID")
	return f.principal, f.err
}

type fakeUseCases struct {
	items              []engagement.Engagement
	detail             engagement.Details
	materialProjection engagement.MaterialReferenceProjection
	called             string
	listLimit          int
	materialInput      engagement.MaterialInput
}

func (f *fakeUseCases) Ping(context.Context) error { return nil }
func (f *fakeUseCases) List(context.Context, string, int) ([]engagement.Engagement, error) {
	return f.items, nil
}
func (f *fakeUseCases) ListProject(_ context.Context, tenant, project string, limit int) ([]engagement.Engagement, error) {
	f.called = "list-project:" + tenant + ":" + project
	f.listLimit = limit
	return f.items, nil
}

func TestExactProjectListUsesNativeQueryAndRetainsScopeAuthorization(t *testing.T) {
	for _, scope := range []string{"tenant", "current-project", "current-engagement", "adjacent-project"} {
		t.Run(scope, func(t *testing.T) {
			useCases := &fakeUseCases{items: []engagement.Engagement{{ID: engagementA, TenantID: tenantID, ProjectID: projectA}, {ID: engagementB, TenantID: tenantID, ProjectID: projectB}}}
			principal := auth.Principal{SubjectID: subjectID, TenantID: tenantID, ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionRead: true}}
			switch scope {
			case "tenant":
				principal.ScopeKind = "tenant"
			case "current-project":
				principal.Resources = map[string]map[string]bool{"project": {projectA: true}}
			case "current-engagement":
				principal.Resources = map[string]map[string]bool{"business-engagement": {engagementA: true}}
			case "adjacent-project":
				principal.Resources = map[string]map[string]bool{"project": {projectB: true}}
			}
			server, _ := New(useCases, &fakeAuthenticator{principal: principal})
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/business-engagements?project_id="+projectA+"&limit=2", nil))
			var value engagementListResponse
			if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			want := 1
			if scope == "adjacent-project" {
				want = 0
			}
			if response.Code != 200 || value.ProjectID != projectA || len(value.Items) != want || useCases.called != "list-project:"+tenantID+":"+projectA || useCases.listLimit != 2 {
				t.Fatalf("status=%d value=%#v called=%s limit=%d", response.Code, value, useCases.called, useCases.listLimit)
			}
		})
	}
}

func TestExactProjectListRejectsInvalidQueryAndMissingCapabilityBeforeUseCase(t *testing.T) {
	for _, query := range []string{"?project_id=", "?project_id=bad", "?project_id=" + projectA + "&limit=101"} {
		useCases := &fakeUseCases{}
		server, _ := New(useCases, &fakeAuthenticator{principal: auth.Principal{TenantID: tenantID, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionRead: true}}})
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/business-engagements"+query, nil))
		if response.Code != 400 || useCases.called != "" {
			t.Fatalf("query=%s status=%d called=%s", query, response.Code, useCases.called)
		}
	}
	useCases := &fakeUseCases{}
	server, _ := New(useCases, &fakeAuthenticator{principal: auth.Principal{TenantID: tenantID, ScopeKind: "tenant"}})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/business-engagements?project_id="+projectA, nil))
	if response.Code != 403 || useCases.called != "" {
		t.Fatalf("status=%d called=%s", response.Code, useCases.called)
	}
}

func TestUnfilteredListRetainsOriginalResponseContract(t *testing.T) {
	server, _ := New(&fakeUseCases{items: []engagement.Engagement{}}, &fakeAuthenticator{principal: auth.Principal{TenantID: tenantID, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionRead: true}}})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/business-engagements", nil))
	if response.Code != 200 || strings.Contains(response.Body.String(), `"project_id"`) || !strings.Contains(response.Body.String(), `"items":[]`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
func (f *fakeUseCases) Get(context.Context, string, string) (engagement.Details, error) {
	return f.detail, nil
}
func (f *fakeUseCases) ReferenceMaterials(context.Context, string, string) (engagement.MaterialReferenceProjection, error) {
	return f.materialProjection, nil
}
func (f *fakeUseCases) History(context.Context, string, string, int64, int) (engagement.HistoryPage, error) {
	return engagement.HistoryPage{}, nil
}
func (f *fakeUseCases) Create(context.Context, engagement.Actor, engagement.CreateInput) (engagement.Details, error) {
	f.called = "create"
	return f.detail, nil
}
func (f *fakeUseCases) SetApplicability(context.Context, engagement.Actor, string, engagement.ApplicabilityInput) (engagement.Details, error) {
	f.called = "applicability"
	return f.detail, nil
}
func (f *fakeUseCases) SelectMethod(context.Context, engagement.Actor, string, engagement.MethodInput) (engagement.Details, error) {
	f.called = "method"
	return f.detail, nil
}
func (f *fakeUseCases) ApplyMaterial(_ context.Context, _ engagement.Actor, _ string, input engagement.MaterialInput) (engagement.Details, error) {
	f.called = "material"
	f.materialInput = input
	return f.detail, nil
}
func (f *fakeUseCases) Cancel(context.Context, engagement.Actor, string, engagement.CancelInput) (engagement.Details, error) {
	f.called = "cancel"
	return f.detail, nil
}

func TestProtectedEndpointRejectsMissingCredential(t *testing.T) {
	authenticator := &fakeAuthenticator{err: errors.New("missing")}
	server, _ := New(&fakeUseCases{}, authenticator)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/business-engagements", nil)
	request.Header.Set("X-Actor-ID", subjectID)
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || authenticator.header != "" {
		t.Fatalf("status=%d leaked_header=%q", response.Code, authenticator.header)
	}
}

func TestCapabilityAndResourceScopeAreBothRequired(t *testing.T) {
	useCases := &fakeUseCases{detail: engagement.Details{Engagement: engagement.Engagement{ID: engagementA, ProjectID: projectA}}}
	principal := auth.Principal{SubjectID: subjectID, TenantID: tenantID, ScopeKind: "resources", AllowedActions: map[string]bool{}, Resources: map[string]map[string]bool{"project": {projectA: true}}}
	authenticator := &fakeAuthenticator{principal: principal}
	server, _ := New(useCases, authenticator)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/business-engagements/"+engagementA+"/method", strings.NewReader(`{"command_id":"`+commandID+`","expected_revision":1,"method":"comparison"}`))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || useCases.called != "" {
		t.Fatalf("capability status=%d called=%q", response.Code, useCases.called)
	}

	principal.AllowedActions[auth.ActionMethodSelect] = true
	principal.Resources = map[string]map[string]bool{"project": {projectB: true}}
	authenticator.principal = principal
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/business-engagements/"+engagementA+"/method", strings.NewReader(`{"command_id":"`+commandID+`","expected_revision":1,"method":"comparison"}`))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || useCases.called != "" {
		t.Fatalf("scope status=%d called=%q", response.Code, useCases.called)
	}

	principal.Resources = map[string]map[string]bool{"business-engagement": {engagementA: true}}
	authenticator.principal = principal
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/business-engagements/"+engagementA+"/method", strings.NewReader(`{"command_id":"`+commandID+`","expected_revision":1,"method":"comparison"}`))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || useCases.called != "method" {
		t.Fatalf("allowed status=%d called=%q body=%s", response.Code, useCases.called, response.Body.String())
	}
}

func TestMaterialReferenceProjectionRequiresDedicatedActionAndExactScope(t *testing.T) {
	projection := engagement.MaterialReferenceProjection{
		SchemaVersion: "1", SourceApplication: "business-engagement", ID: engagementA,
		TenantID: tenantID, ProjectID: projectA, ResourceType: "business-engagement", Status: engagement.StatusReady,
		Revision: 5, Digest: strings.Repeat("a", 64), Items: []engagement.MaterialProjectionItem{{RelationKey: "award_document", FileID: commandID, Version: 2, EvidenceSourceRevision: 9, DisplayName: "中标通知书.pdf"}},
	}
	useCases := &fakeUseCases{materialProjection: projection}
	principal := auth.Principal{SubjectID: subjectID, TenantID: tenantID, ScopeKind: "resources", AllowedActions: map[string]bool{}, Resources: map[string]map[string]bool{"business-engagement": {engagementA: true}}}
	authenticator := &fakeAuthenticator{principal: principal}
	server, _ := New(useCases, authenticator)
	request := func() *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/references/business-engagements/"+engagementA+"/materials", nil))
		return response
	}
	if response := request(); response.Code != http.StatusForbidden {
		t.Fatalf("missing action status=%d body=%s", response.Code, response.Body.String())
	}
	principal.AllowedActions[auth.ActionMaterialReferenceRead] = true
	principal.Resources = map[string]map[string]bool{"business-engagement": {engagementB: true}}
	authenticator.principal = principal
	if response := request(); response.Code != http.StatusForbidden {
		t.Fatalf("adjacent scope status=%d body=%s", response.Code, response.Body.String())
	}
	principal.Resources = map[string]map[string]bool{"business-engagement": {engagementA: true}}
	authenticator.principal = principal
	response := request()
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"evidence_source_revision":9`) || strings.Contains(response.Body.String(), "superseded_at") {
		t.Fatalf("allowed status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestListFiltersResourcesAndCreateChecksProjectScope(t *testing.T) {
	useCases := &fakeUseCases{
		items:  []engagement.Engagement{{ID: engagementA, ProjectID: projectA}, {ID: engagementB, ProjectID: projectB}},
		detail: engagement.Details{Engagement: engagement.Engagement{ID: engagementA, ProjectID: projectA}},
	}
	principal := auth.Principal{
		SubjectID: subjectID, TenantID: tenantID, ScopeKind: "resources",
		AllowedActions: map[string]bool{auth.ActionRead: true, auth.ActionCreate: true},
		Resources:      map[string]map[string]bool{"project": {projectA: true}},
	}
	server, _ := New(useCases, &fakeAuthenticator{principal: principal})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/business-engagements", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), engagementA) || strings.Contains(response.Body.String(), engagementB) {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/business-engagements", strings.NewReader(`{"command_id":"`+commandID+`","project_id":"`+projectB+`","project_source_revision":1}`))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || useCases.called != "" {
		t.Fatalf("create status=%d called=%q", response.Code, useCases.called)
	}
}

func TestListAndDetailsExposeServerComputedAllowedActions(t *testing.T) {
	method := engagement.MethodTender
	useCases := &fakeUseCases{
		items: []engagement.Engagement{{ID: engagementA, ProjectID: projectA}},
		detail: engagement.Details{Engagement: engagement.Engagement{
			ID: engagementA, ProjectID: projectA, Status: engagement.StatusCollectingEvidence,
			Applicability: engagement.ApplicabilityApplicable, Method: &method,
		}},
	}
	actions := make(map[string]bool, len(auth.AllActions))
	for _, action := range auth.AllActions {
		actions[action] = true
	}
	server, _ := New(useCases, &fakeAuthenticator{principal: auth.Principal{
		SubjectID: subjectID, TenantID: tenantID, ScopeKind: "tenant", AllowedActions: actions,
	}})

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/business-engagements", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"allowed_actions"`) || !strings.Contains(response.Body.String(), auth.ActionCreate) {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/business-engagements/"+engagementA, nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), auth.ActionMethodSelect) || !strings.Contains(response.Body.String(), auth.ActionEvidenceManage) || strings.Contains(response.Body.String(), auth.ActionCreate) {
		t.Fatalf("details status=%d body=%s", response.Code, response.Body.String())
	}

	useCases.detail.Engagement.Status = engagement.StatusCancelled
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/business-engagements/"+engagementA, nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), auth.ActionMethodSelect) || strings.Contains(response.Body.String(), auth.ActionEvidenceManage) || strings.Contains(response.Body.String(), auth.ActionCancel) {
		t.Fatalf("cancelled details status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestWorkspaceIsPublicButBusinessDataRemainProtected(t *testing.T) {
	server, _ := New(&fakeUseCases{}, &fakeAuthenticator{err: errors.New("missing")})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "经营承接工作区") {
		t.Fatalf("workspace status=%d body=%s", response.Code, response.Body.String())
	}
	csp := response.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("workspace csp=%q", csp)
	}
	for _, marker := range []string{"faultRecovery", "retryCommand", "returnProject", "使用新命令重试"} {
		if !strings.Contains(response.Body.String(), marker) {
			t.Fatalf("workspace is missing recovery marker %q", marker)
		}
	}

	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/demo.js", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("workspace script status=%d", response.Code)
	}
	for _, marker := range []string{"error.status===409", "error.status===503", "command_id:uuid()", "../project/?project_id="} {
		if !strings.Contains(response.Body.String(), marker) {
			t.Fatalf("workspace script is missing recovery marker %q", marker)
		}
	}

	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/business-engagements", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("protected list status=%d", response.Code)
	}
}
