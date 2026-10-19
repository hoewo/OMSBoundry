package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
)

const (
	httpTenant  = "11111111-1111-4111-8111-111111111111"
	httpSubject = "22222222-2222-4222-8222-222222222222"
	httpProject = "33333333-3333-4333-8333-333333333333"
	httpCommand = "44444444-4444-4444-8444-444444444444"
)

type useCasesStub struct {
	namesCalls               int
	namesTenant              string
	namesQuery               projects.CounterpartyNamesQuery
	contextCalls             int
	contextReads             int
	contextInput             projects.CounterpartyContextInput
	contextResult            projects.CounterpartyContextView
	historyResult            projects.HistoryPage
	basicChangeCalls         int
	basicChangeInput         projects.BasicInfoChangeInput
	directoryQuery           projects.DirectoryQuery
	directoryTenant          string
	directoryCalls           int
	directoryResult          projects.DirectoryPage
	correctionCalls          int
	correctionInput          projects.ProfileCorrectionInput
	listResult               []projects.Project
	listTenant               string
	listLimit                int
	createResult             projects.Project
	createErr                error
	createCalls              int
	assignmentResult         projects.AssignmentResult
	assignmentErr            error
	assignmentCalls          int
	assignmentInput          projects.AssignmentCommand
	referenceAssignment      projects.AssignmentReference
	referenceAssignmentErr   error
	referenceAssignmentCalls int
	completeResult           projects.Project
	completeErr              error
	completeCalls            int
	completeInput            projects.RevisionCommand
	dispatchCalls            int
	dispatchInput            projects.InitiationDispatchInput
	getResult                projects.Details
	getErr                   error
	commercialResult         projects.CommercialSummary
	commercialErr            error
	materialsResult          projects.MaterialCollection
	materialsErr             error
	materialsCalls           int
	completionResult         projects.CompletionAssessment
	completionErr            error
	completionCalls          int
	completionTenant         string
	completionProject        string
	completionProfile        string
	confirmationResult       projects.CompletionConfirmation
	confirmationErr          error
	confirmationCalls        int
	confirmationInput        projects.CompletionCommand
}

func (*useCasesStub) Ping(context.Context) error { return nil }
func (s *useCasesStub) CounterpartyNameSuggestions(_ context.Context, tenant string, q projects.CounterpartyNamesQuery) (projects.CounterpartyNames, error) {
	s.namesCalls++
	s.namesTenant = tenant
	s.namesQuery = q
	return projects.CounterpartyNames{SchemaVersion: projects.CounterpartyNamesSchema, Names: []string{"示例甲方"}}, nil
}
func (s *useCasesStub) CounterpartyContext(context.Context, string, string) (projects.CounterpartyContextView, error) {
	s.contextReads++
	return s.contextResult, nil
}
func (s *useCasesStub) ChangeCounterpartyContext(_ context.Context, _ projects.Actor, _ string, in projects.CounterpartyContextInput) (projects.CounterpartyContextView, error) {
	s.contextCalls++
	s.contextInput = in
	return s.contextResult, nil
}
func (s *useCasesStub) ChangeInitiationInfo(ctx context.Context, actor projects.Actor, id string, in projects.BasicInfoChangeInput) (projects.BasicInfoChangeResult, error) {
	return s.ChangeBasicInfo(ctx, actor, id, in)
}
func (s *useCasesStub) ChangeBasicInfo(_ context.Context, _ projects.Actor, _ string, in projects.BasicInfoChangeInput) (projects.BasicInfoChangeResult, error) {
	s.basicChangeCalls++
	s.basicChangeInput = in
	return projects.BasicInfoChangeResult{}, nil
}
func (s *useCasesStub) Directory(_ context.Context, tenant string, query projects.DirectoryQuery) (projects.DirectoryPage, error) {
	s.directoryCalls++
	s.directoryTenant, s.directoryQuery = tenant, query
	return s.directoryResult, nil
}
func (s *useCasesStub) List(_ context.Context, tenant string, limit int) ([]projects.Project, error) {
	s.listTenant, s.listLimit = tenant, limit
	return s.listResult, nil
}

func TestDemoEntryAndProjectListAreHumanUsableAndProtected(t *testing.T) {
	useCases := &useCasesStub{listResult: []projects.Project{{ID: httpProject, Name: "演示项目", Status: projects.StatusActive, Revision: 3}}}
	server, _ := New(useCases, &authenticatorStub{principal: fullPrincipal()})

	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "项目与立项工作区") || !strings.Contains(page.Body.String(), "新建项目包") || !strings.Contains(page.Body.String(), "客户主体（可选）") || !strings.Contains(page.Body.String(), "加载主体目录") || !strings.Contains(page.Body.String(), "经营链路") || !strings.Contains(page.Body.String(), "项目资料") || !strings.Contains(page.Body.String(), "Evidence 资料") || !strings.Contains(page.Body.String(), "业务承接资料源") || !strings.Contains(page.Body.String(), "下载必须另行取得 Evidence 的精确授权") || !strings.Contains(page.Body.String(), "billing-invoice") || !strings.Contains(page.Body.String(), "刷新权限") || !strings.Contains(page.Body.String(), "assets/demo.js") {
		t.Fatalf("demo page status=%d body=%q", page.Code, page.Body.String())
	}
	asset := httptest.NewRecorder()
	server.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/demo.js", nil))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Header().Get("Content-Type"), "application/javascript") || !strings.Contains(asset.Body.String(), "sessionStorage") || !strings.Contains(asset.Body.String(), "/gateway/authorize/project-workspace") || !strings.Contains(asset.Body.String(), "authorizeApplication('party-directory')") || !strings.Contains(asset.Body.String(), "authorizeApplication('evidence-service')") || !strings.Contains(asset.Body.String(), "commercial-summary") || !strings.Contains(asset.Body.String(), "/materials") || !strings.Contains(asset.Body.String(), "project-workspace.materials.read") || !strings.Contains(asset.Body.String(), "前往 Evidence 获取下载授权") || !strings.Contains(asset.Body.String(), "未知（不按 0 计算）") || !strings.Contains(asset.Body.String(), "invoice_registration") || !strings.Contains(asset.Body.String(), "business-engagement") || !strings.Contains(asset.Body.String(), "client_party") || !strings.Contains(asset.Body.String(), "allowed_actions") {
		t.Fatalf("demo asset status=%d type=%q", asset.Code, asset.Header().Get("Content-Type"))
	}
	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/projects?limit=12", nil))
	if list.Code != http.StatusOK || useCases.listTenant != httpTenant || useCases.listLimit != 12 || !strings.Contains(list.Body.String(), "演示项目") {
		t.Fatalf("list status=%d tenant=%q limit=%d body=%q", list.Code, useCases.listTenant, useCases.listLimit, list.Body.String())
	}
	var payload projectListResponse
	if err := json.Unmarshal(list.Body.Bytes(), &payload); err != nil || !containsAction(payload.AllowedActions, auth.ActionHierarchyManage) {
		t.Fatalf("list allowed_actions=%#v error=%v", payload.AllowedActions, err)
	}
}

func TestProductionMaterialResourcePickerUsesCanonicalSource(t *testing.T) {
	if !strings.Contains(projectDemoPage, `<option value="production-operations">生产资料源</option>`) ||
		!strings.Contains(projectDemoJavaScript, `"production-operations":[['production-work-item','production-materials']]`) {
		t.Fatal("production picker must expose only the canonical Work Item reference")
	}
}

func TestProjectDetailsAllowedActionsFollowCredentialScopeAndLifecycle(t *testing.T) {
	draft := projects.Details{Project: projects.Project{ID: httpProject, NodeType: projects.NodeTypeProject, Status: projects.StatusDraft}}
	full := detailsAllowedActions(fullPrincipal(), draft)
	for _, action := range []string{auth.ActionRead, auth.ActionCommercialSummaryRead, auth.ActionMaterialsRead, auth.ActionUpdate, auth.ActionAssignmentManage, auth.ActionResourceManage, auth.ActionComplete, auth.ActionCancel, auth.ActionHistoryRead, auth.ActionHierarchyManage} {
		if !containsAction(full, action) {
			t.Fatalf("draft full actions missing %q: %#v", action, full)
		}
	}
	readOnly := fullPrincipal()
	readOnly.AllowedActions = map[string]bool{auth.ActionRead: true, auth.ActionHistoryRead: true}
	if got := detailsAllowedActions(readOnly, draft); len(got) != 2 || got[0] != auth.ActionRead || got[1] != auth.ActionHistoryRead {
		t.Fatalf("read-only actions=%#v", got)
	}
	active := draft
	active.Project.Status = projects.StatusActive
	activeActions := detailsAllowedActions(fullPrincipal(), active)
	if containsAction(activeActions, auth.ActionUpdate) || containsAction(activeActions, auth.ActionComplete) || !containsAction(activeActions, auth.ActionHierarchyManage) {
		t.Fatalf("active actions=%#v", activeActions)
	}
	child := active
	parentID := "55555555-5555-4555-8555-555555555555"
	child.Project.ParentProjectID = &parentID
	if got := detailsAllowedActions(fullPrincipal(), child); containsAction(got, auth.ActionHierarchyManage) {
		t.Fatalf("untracked child exposed hierarchy action: %#v", got)
	}
	child.ActivePackageConversion = &projects.PackageConversion{ID: "66666666-6666-4666-8666-666666666666", Status: projects.ConversionActive}
	if got := detailsAllowedActions(fullPrincipal(), child); !containsAction(got, auth.ActionHierarchyManage) {
		t.Fatalf("reversible conversion hid hierarchy action: %#v", got)
	}
	cancelled := draft
	cancelled.Project.Status = projects.StatusCancelled
	if got := detailsAllowedActions(fullPrincipal(), cancelled); len(got) != 5 || got[0] != auth.ActionRead || got[1] != auth.ActionCommercialSummaryRead || got[2] != auth.ActionMaterialsRead || got[3] != auth.ActionHistoryRead || got[4] != auth.ActionCounterpartyContextRead {
		t.Fatalf("cancelled actions=%#v", got)
	}
}

func TestCommercialSummaryRequiresDedicatedCapabilityAndReturnsTypedSegments(t *testing.T) {
	useCases := &useCasesStub{commercialResult: projects.CommercialSummary{
		ProjectID: httpProject, ProjectRevision: 4,
		Segments: []projects.CommercialSegment{{
			SourceApplication: "billing-invoice", ResourceType: "invoice_registration",
			State:   projects.CommercialSegmentAvailable,
			Invoice: &projects.InvoiceCommercialFacts{IssuedOn: "2026-09-27", AmountMinor: 120000, Currency: "CNY"},
		}},
	}}
	server, _ := New(useCases, &authenticatorStub{principal: fullPrincipal()})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject+"/commercial-summary", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"amount_minor":120000`) || !strings.Contains(response.Body.String(), `"source_application":"billing-invoice"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	denied := fullPrincipal()
	denied.AllowedActions[auth.ActionCommercialSummaryRead] = false
	server, _ = New(useCases, &authenticatorStub{principal: denied})
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject+"/commercial-summary", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("denied status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMaterialsRequireDedicatedCapabilityAndExactProjectScope(t *testing.T) {
	useCases := &useCasesStub{materialsResult: projects.MaterialCollection{
		ProjectID: httpProject, ProjectRevision: 4, State: projects.MaterialCollectionAvailable,
		Segments: []projects.MaterialSegment{{ResourceID: httpCommand, State: projects.MaterialSegmentAvailable, FileName: "成果.pdf"}},
	}}
	server, _ := New(useCases, &authenticatorStub{principal: fullPrincipal()})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject+"/materials", nil))
	if response.Code != http.StatusOK || useCases.materialsCalls != 1 || !strings.Contains(response.Body.String(), `"file_name":"成果.pdf"`) {
		t.Fatalf("allowed status=%d calls=%d body=%s", response.Code, useCases.materialsCalls, response.Body.String())
	}

	missingAction := fullPrincipal()
	delete(missingAction.AllowedActions, auth.ActionMaterialsRead)
	server, _ = New(useCases, &authenticatorStub{principal: missingAction})
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject+"/materials", nil))
	if response.Code != http.StatusForbidden || useCases.materialsCalls != 1 {
		t.Fatalf("missing action status=%d calls=%d body=%s", response.Code, useCases.materialsCalls, response.Body.String())
	}

	adjacentProject := fullPrincipal()
	adjacentProject.ScopeKind = "resources"
	adjacentProject.ResourceIDs = map[string]bool{"55555555-5555-4555-8555-555555555555": true}
	server, _ = New(useCases, &authenticatorStub{principal: adjacentProject})
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject+"/materials", nil))
	if response.Code != http.StatusForbidden || useCases.materialsCalls != 1 {
		t.Fatalf("adjacent project status=%d calls=%d body=%s", response.Code, useCases.materialsCalls, response.Body.String())
	}
}

func containsAction(actions []string, target string) bool {
	for _, action := range actions {
		if action == target {
			return true
		}
	}
	return false
}
func (s *useCasesStub) Create(context.Context, projects.Actor, projects.CreateInput) (projects.Project, error) {
	s.createCalls++
	return s.createResult, s.createErr
}
func (*useCasesStub) CreatePackage(context.Context, projects.Actor, projects.PackageCreateInput) (projects.Project, error) {
	return projects.Project{}, nil
}
func (*useCasesStub) ConvertPackage(context.Context, projects.Actor, string, projects.PackageConversionCommand) (projects.PackageConversionResult, error) {
	return projects.PackageConversionResult{}, nil
}
func (*useCasesStub) RollbackPackageConversion(context.Context, projects.Actor, string, string, projects.PackageRollbackCommand) (projects.PackageConversionResult, error) {
	return projects.PackageConversionResult{}, nil
}
func (*useCasesStub) Update(context.Context, projects.Actor, string, projects.UpdateInput) (projects.Project, error) {
	return projects.Project{}, nil
}
func (s *useCasesStub) CorrectProfile(_ context.Context, _ projects.Actor, id string, input projects.ProfileCorrectionInput) (projects.Project, error) {
	s.correctionCalls++
	s.correctionInput = input
	return projects.Project{ID: id, Name: input.Name, Status: projects.StatusActive, Revision: input.ExpectedRevision + 1}, nil
}
func (s *useCasesStub) ApplyAssignment(_ context.Context, _ projects.Actor, _ string, input projects.AssignmentCommand) (projects.AssignmentResult, error) {
	s.assignmentCalls++
	s.assignmentInput = input
	return s.assignmentResult, s.assignmentErr
}
func (*useCasesStub) ApplyResourceLink(context.Context, projects.Actor, string, projects.ResourceLinkCommand) (projects.ResourceLinkResult, error) {
	return projects.ResourceLinkResult{}, nil
}
func (s *useCasesStub) CompleteInitiation(_ context.Context, _ projects.Actor, _ string, input projects.RevisionCommand) (projects.Project, error) {
	s.completeCalls++
	s.completeInput = input
	return s.completeResult, s.completeErr
}
func (s *useCasesStub) DispatchInitiation(_ context.Context, _ projects.Actor, input projects.InitiationDispatchInput) (projects.Project, error) {
	s.dispatchCalls++
	s.dispatchInput = input
	return s.createResult, s.createErr
}
func (*useCasesStub) Cancel(context.Context, projects.Actor, string, projects.CancelCommand) (projects.Project, error) {
	return projects.Project{}, nil
}
func (s *useCasesStub) Get(context.Context, string, string, bool) (projects.Details, error) {
	return s.getResult, s.getErr
}
func (s *useCasesStub) CommercialSummary(context.Context, string, string) (projects.CommercialSummary, error) {
	return s.commercialResult, s.commercialErr
}
func (s *useCasesStub) Materials(context.Context, string, string) (projects.MaterialCollection, error) {
	s.materialsCalls++
	return s.materialsResult, s.materialsErr
}
func (s *useCasesStub) History(context.Context, string, string, string, int) (projects.HistoryPage, error) {
	return s.historyResult, nil
}
func (*useCasesStub) ReferenceProject(context.Context, string, string) (projects.ProjectReference, error) {
	return projects.ProjectReference{}, nil
}
func (s *useCasesStub) ReferenceAssignment(context.Context, string, string, string) (projects.AssignmentReference, error) {
	s.referenceAssignmentCalls++
	return s.referenceAssignment, s.referenceAssignmentErr
}
func (s *useCasesStub) EvaluateCompletion(_ context.Context, tenant, projectID, profile string) (projects.CompletionAssessment, error) {
	s.completionCalls++
	s.completionTenant, s.completionProject, s.completionProfile = tenant, projectID, profile
	return s.completionResult, s.completionErr
}
func (s *useCasesStub) ConfirmCompletion(_ context.Context, _ projects.Actor, _ string, input projects.CompletionCommand) (projects.CompletionConfirmation, error) {
	s.confirmationCalls++
	s.confirmationInput = input
	return s.confirmationResult, s.confirmationErr
}

func TestCompletionWorkspaceAssessmentAndConfirmationAreCapabilityBound(t *testing.T) {
	pageUseCases := &useCasesStub{}
	server, _ := New(pageUseCases, &authenticatorStub{principal: fullPrincipal()})
	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/completion/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Project 完成条件检查点") || !strings.Contains(page.Body.String(), "不等于整个 OA Project 已完成") || !strings.Contains(page.Body.String(), "assets/completion.js") {
		t.Fatalf("page status=%d body=%q", page.Code, page.Body.String())
	}
	asset := httptest.NewRecorder()
	server.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/completion.js", nil))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), "completion-assessment") || !strings.Contains(asset.Body.String(), "completion-confirmations") || !strings.Contains(asset.Body.String(), "allowed_actions") {
		t.Fatalf("asset status=%d body=%q", asset.Code, asset.Body.String())
	}

	readOnly := fullPrincipal()
	readOnly.AllowedActions = map[string]bool{auth.ActionCompletionRead: true}
	useCases := &useCasesStub{completionResult: projects.CompletionAssessment{
		ProjectID: httpProject, ProjectRevision: 8, ProfileKey: projects.CompletionProfileReferenceOAC5V1,
		OverallStatus: projects.CompletionSatisfied,
	}}
	server, _ = New(useCases, &authenticatorStub{principal: readOnly})
	assessment := httptest.NewRecorder()
	server.ServeHTTP(assessment, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject+"/completion-assessment", nil))
	if assessment.Code != http.StatusOK || useCases.completionCalls != 1 || useCases.completionTenant != httpTenant || useCases.completionProject != httpProject || useCases.completionProfile != projects.CompletionProfileReferenceOAC5V1 {
		t.Fatalf("assessment status=%d calls=%d body=%s", assessment.Code, useCases.completionCalls, assessment.Body.String())
	}
	var payload completionAssessmentResponse
	if err := json.Unmarshal(assessment.Body.Bytes(), &payload); err != nil || len(payload.AllowedActions) != 1 || payload.AllowedActions[0] != auth.ActionCompletionRead {
		t.Fatalf("assessment actions=%#v err=%v", payload.AllowedActions, err)
	}
	confirm := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/completion-confirmations", strings.NewReader(`{"command_id":"`+httpCommand+`","expected_revision":8,"profile_key":"reference-oa-c5-v1","expected_sources":[]}`))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(confirm, request)
	if confirm.Code != http.StatusForbidden || useCases.confirmationCalls != 0 {
		t.Fatalf("confirm status=%d calls=%d body=%s", confirm.Code, useCases.confirmationCalls, confirm.Body.String())
	}

	useCases.confirmationResult = projects.CompletionConfirmation{ID: httpCommand, ProjectID: httpProject, ProfileKey: projects.CompletionProfileReferenceOAC5V1}
	server, _ = New(useCases, &authenticatorStub{principal: fullPrincipal()})
	confirm = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/completion-confirmations", strings.NewReader(`{"command_id":"`+httpCommand+`","expected_revision":8,"profile_key":"reference-oa-c5-v1","expected_sources":[]}`))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(confirm, request)
	if confirm.Code != http.StatusOK || useCases.confirmationCalls != 1 || useCases.confirmationInput.ExpectedRevision != 8 {
		t.Fatalf("allowed confirm status=%d calls=%d body=%s", confirm.Code, useCases.confirmationCalls, confirm.Body.String())
	}
}

type authenticatorStub struct {
	principal auth.Principal
	err       error
	sawHeader string
}

func (a *authenticatorStub) ProviderName() string { return "test" }
func (a *authenticatorStub) Authenticate(r *http.Request) (auth.Principal, error) {
	a.sawHeader = r.Header.Get("X-Actor-ID")
	return a.principal, a.err
}

func fullPrincipal() auth.Principal {
	actions := map[string]bool{}
	for _, action := range auth.AllActions {
		actions[action] = true
	}
	return auth.Principal{SubjectID: httpSubject, TenantID: httpTenant, DisplayName: "测试管理员", Provider: "test", AllowedActions: actions, ScopeKind: "tenant", ResourceIDs: map[string]bool{}}
}

func TestProtectedCreateRequiresCredentialCapabilityAndTenantScope(t *testing.T) {
	body := `{"command_id":"` + httpCommand + `","project_number":"OA-001","name":"演示","project_type_keys":["design"]}`
	tests := []struct {
		name      string
		principal auth.Principal
		authErr   error
		want      int
	}{
		{"missing credential", auth.Principal{}, auth.ErrUnauthenticated, http.StatusUnauthorized},
		{"missing capability", auth.Principal{ScopeKind: "tenant", AllowedActions: map[string]bool{}}, nil, http.StatusForbidden},
		{"resource scope", auth.Principal{ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionCreate: true}, ResourceIDs: map[string]bool{httpProject: true}}, nil, http.StatusForbidden},
		{"allowed", fullPrincipal(), nil, http.StatusCreated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCases := &useCasesStub{createResult: projects.Project{ID: httpProject, TenantID: httpTenant}}
			authenticator := &authenticatorStub{principal: test.principal, err: test.authErr}
			server, _ := New(useCases, authenticator)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Actor-ID", "untrusted")
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if authenticator.sawHeader != "" {
				t.Fatalf("untrusted identity header reached authenticator: %q", authenticator.sawHeader)
			}
			if (test.want == http.StatusCreated) != (useCases.createCalls == 1) {
				t.Fatalf("create calls = %d", useCases.createCalls)
			}
		})
	}
}

func TestProjectScopeIsEnforcedBeforeLookup(t *testing.T) {
	principal := fullPrincipal()
	principal.ScopeKind = "resources"
	principal.ResourceIDs = map[string]bool{"55555555-5555-4555-8555-555555555555": true}
	server, _ := New(&useCasesStub{}, &authenticatorStub{principal: principal})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject, nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestProjectReferenceRequiresExactProjectScope(t *testing.T) {
	for _, test := range []struct {
		name       string
		resourceID string
		want       int
	}{
		{"exact resource", httpProject, http.StatusOK},
		{"adjacent resource", "55555555-5555-4555-8555-555555555555", http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			principal := auth.Principal{
				SubjectID: httpSubject, TenantID: httpTenant, DisplayName: "Billing workload", Provider: "iam",
				AllowedActions: map[string]bool{auth.ActionRead: true}, ScopeKind: "resources", ResourceIDs: map[string]bool{test.resourceID: true},
			}
			server, _ := New(&useCasesStub{}, &authenticatorStub{principal: principal})
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/references/projects/"+httpProject, nil))
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestHierarchyManagementRequiresPrivilegedTenantCredential(t *testing.T) {
	body := `{"command_id":"` + httpCommand + `","operation":"wrap_as_package","expected_project_revision":1,"sort_order":10,"new_package":{"name":"项目包","project_type_keys":["design"]}}`
	resourcePrincipal := fullPrincipal()
	resourcePrincipal.ScopeKind = "resources"
	resourcePrincipal.ResourceIDs = map[string]bool{httpProject: true}
	withoutCapability := fullPrincipal()
	delete(withoutCapability.AllowedActions, auth.ActionHierarchyManage)
	tests := []struct {
		name      string
		principal auth.Principal
		want      int
	}{
		{"tenant capability", fullPrincipal(), http.StatusOK},
		{"resource credential cannot mutate a package", resourcePrincipal, http.StatusForbidden},
		{"missing hierarchy capability", withoutCapability, http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, _ := New(&useCasesStub{}, &authenticatorStub{principal: test.principal})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/package-conversions", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestAssignmentManagementCapabilityIsBoundToRequestedRole(t *testing.T) {
	businessManager := auth.Principal{
		SubjectID: httpSubject, TenantID: httpTenant, DisplayName: "经营经理", Provider: "test",
		AllowedActions: map[string]bool{auth.ActionAssignmentBusinessOwnerManage: true},
		ScopeKind:      "resources", ResourceIDs: map[string]bool{httpProject: true},
	}
	tests := []struct {
		name          string
		principal     auth.Principal
		body          string
		reference     projects.AssignmentReference
		want          int
		wantApply     int
		wantReference int
	}{
		{
			name: "business manager assigns business owner", principal: businessManager,
			body: `{"command_id":"` + httpCommand + `","expected_revision":1,"operation":"assign","subject_kind":"person","subject_id":"55555555-5555-4555-8555-555555555555","source_revision":1,"role_key":"business_owner"}`,
			want: http.StatusOK, wantApply: 1,
		},
		{
			name: "business manager cannot assign production owner", principal: businessManager,
			body: `{"command_id":"` + httpCommand + `","expected_revision":1,"operation":"assign","subject_kind":"person","subject_id":"55555555-5555-4555-8555-555555555555","source_revision":1,"role_key":"production_owner"}`,
			want: http.StatusForbidden,
		},
		{
			name: "business manager ends business owner", principal: businessManager,
			body:      `{"command_id":"` + httpCommand + `","expected_revision":2,"operation":"end","assignment_id":"66666666-6666-4666-8666-666666666666","end_reason":"责任调整"}`,
			reference: projects.AssignmentReference{RoleKey: projects.RoleBusinessOwner},
			want:      http.StatusOK, wantApply: 1, wantReference: 1,
		},
		{
			name: "business manager cannot end production owner", principal: businessManager,
			body:      `{"command_id":"` + httpCommand + `","expected_revision":2,"operation":"end","assignment_id":"66666666-6666-4666-8666-666666666666","end_reason":"越权尝试"}`,
			reference: projects.AssignmentReference{RoleKey: projects.RoleProductionOwner},
			want:      http.StatusForbidden, wantReference: 1,
		},
		{
			name: "broad business administrator can assign any stable role", principal: fullPrincipal(),
			body: `{"command_id":"` + httpCommand + `","expected_revision":1,"operation":"assign","subject_kind":"person","subject_id":"55555555-5555-4555-8555-555555555555","source_revision":1,"role_key":"participant"}`,
			want: http.StatusOK, wantApply: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCases := &useCasesStub{referenceAssignment: test.reference}
			server, _ := New(useCases, &authenticatorStub{principal: test.principal})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/assignments", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != test.want || useCases.assignmentCalls != test.wantApply || useCases.referenceAssignmentCalls != test.wantReference {
				t.Fatalf("status=%d apply=%d reference=%d body=%s", response.Code, useCases.assignmentCalls, useCases.referenceAssignmentCalls, response.Body.String())
			}
		})
	}
}

func TestCompleteInitiationReturnsMachineReadableMissingFields(t *testing.T) {
	useCases := &useCasesStub{
		completeResult: projects.Project{ID: httpProject, Status: projects.StatusPendingCompletion, Revision: 2},
		completeErr:    &projects.InitiationError{Missing: []string{"production_owner"}},
	}
	server, _ := New(useCases, &authenticatorStub{principal: fullPrincipal()})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+httpProject+"/complete-initiation", strings.NewReader(`{"command_id":"`+httpCommand+`","expected_revision":1}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Error struct {
			Code    string   `json:"code"`
			Missing []string `json:"missing"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "initiation_incomplete" || len(payload.Error.Missing) != 1 || payload.Error.Missing[0] != "production_owner" {
		t.Fatalf("payload=%#v", payload)
	}
}

func TestErrorMappingDoesNotLeakInternalFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
		code string
	}{
		{"not found", projects.ErrNotFound, 404, "project_not_found"},
		{"revision", projects.ErrRevisionConflict, 409, "revision_conflict"},
		{"reference invalid", projects.ErrReferenceInvalid, 422, "reference_invalid"},
		{"reference down", projects.ErrReferenceUnavailable, 503, "reference_unavailable"},
		{"internal", errors.New("password=secret database exploded"), 500, "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, _ := New(&useCasesStub{getErr: test.err}, &authenticatorStub{principal: fullPrincipal()})
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject, nil))
			if response.Code != test.want || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "password=secret") {
				t.Fatal("internal error leaked")
			}
		})
	}
}

func TestDecodeRejectsUnknownAndMultipleJSONValues(t *testing.T) {
	for _, body := range []string{
		`{"command_id":"` + httpCommand + `","project_number":"OA-001","name":"演示","project_type_keys":["design"],"surprise":true}`,
		`{} {}`,
	} {
		useCases := &useCasesStub{}
		server, _ := New(useCases, &authenticatorStub{principal: fullPrincipal()})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || useCases.createCalls != 0 {
			t.Fatalf("status=%d calls=%d body=%s", response.Code, useCases.createCalls, response.Body.String())
		}
	}
}
