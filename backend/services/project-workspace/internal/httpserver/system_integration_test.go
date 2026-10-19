//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/projects"
	"omsbizfoundry/project-workspace/internal/references"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

const (
	systemTenant      = "11111111-1111-4111-8111-111111111111"
	systemSubject     = "22222222-2222-4222-8222-222222222222"
	systemPerson      = "33333333-3333-4333-8333-333333333333"
	systemGroup       = "44444444-4444-4444-8444-444444444444"
	systemContract    = "55555555-5555-4555-8555-555555555555"
	systemParty       = "66666666-6666-4666-8666-666666666666"
	systemAccessToken = "project-workspace-system-test-token"
)

func TestHTTPPostgresJ1VerticalSlice(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := projects.NewPostgresStore(environment.Pool)
	var peopleDown atomic.Bool
	var contractDown atomic.Bool
	var partyDown atomic.Bool
	var partyRevision atomic.Int64
	var partyCalls atomic.Int64
	partyRevision.Store(1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer reference-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/persons/" + systemPerson:
			if peopleDown.Load() {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"id":"` + systemPerson + `","tenant_id":"` + systemTenant + `","display_name":"经营负责人","status":"active","revision":1}`))
		case "/api/v1/groups/" + systemGroup:
			_, _ = w.Write([]byte(`{"group":{"id":"` + systemGroup + `","tenant_id":"` + systemTenant + `","name":"设计生产组","status":"active","revision":2}}`))
		case "/api/v1/references/contracts/" + systemContract:
			if contractDown.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"contract-ledger","id":"` + systemContract + `","tenant_id":"` + systemTenant + `","resource_type":"contract","display_summary":"设计合同","currency":"CNY","current_effective_amount":"1200.00","status":"active","revision":3}`))
		case "/api/v1/references/parties/" + systemParty:
			partyCalls.Add(1)
			if partyDown.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			revision := partyRevision.Load()
			name := "创建时甲方名称"
			if revision == 2 {
				name = "后来更正的甲方名称"
			}
			_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"party-directory","id":"` + systemParty + `","tenant_id":"` + systemTenant + `","kind":"organization","legal_name":"` + name + `","status":"active","revision":` + strconv.FormatInt(revision, 10) + `}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	validator, _ := references.NewHTTPValidator(
		2_000_000_000,
		references.Endpoint{BaseURL: upstream.URL, AccessToken: "reference-token"},
		map[string]references.Endpoint{
			"contract-ledger": {BaseURL: upstream.URL, AccessToken: "reference-token"},
			"party-directory": {BaseURL: upstream.URL, AccessToken: "reference-token"},
		},
	)
	service, _ := projects.NewService(store, validator)
	authenticator, _ := auth.NewTemplateProvider(systemTenant, systemSubject, "系统验收管理员", systemAccessToken)
	server, _ := New(service, authenticator)

	partyCreatedBody := commandBody(90, `"project_number":"OA-PARTY-001","name":"主体项目","project_type_keys":["design"],"client_party":{"party_id":"`+systemParty+`","source_revision":1}`)
	partyCreatedResponse := requestJSON(t, server, http.MethodPost, "/api/v1/projects", partyCreatedBody, http.StatusCreated)
	var partyProject projects.Project
	decodeResponse(t, partyCreatedResponse, &partyProject)
	if partyProject.ClientParty == nil || partyProject.ClientParty.LegalName != "创建时甲方名称" || partyProject.ClientParty.SourceRevision != 1 || partyCalls.Load() != 1 {
		t.Fatalf("party project=%#v calls=%d", partyProject, partyCalls.Load())
	}
	partyRevision.Store(2)
	partyReplayResponse := requestJSON(t, server, http.MethodPost, "/api/v1/projects", partyCreatedBody, http.StatusCreated)
	var partyReplay projects.Project
	decodeResponse(t, partyReplayResponse, &partyReplay)
	if partyReplay.ID != partyProject.ID || partyReplay.ClientParty == nil || partyReplay.ClientParty.LegalName != "创建时甲方名称" || partyCalls.Load() != 1 {
		t.Fatalf("party replay=%#v calls=%d", partyReplay, partyCalls.Load())
	}
	partyDetailsResponse := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+partyProject.ID, "", http.StatusOK)
	var partyDetails projects.Details
	decodeResponse(t, partyDetailsResponse, &partyDetails)
	if partyDetails.Project.ClientParty == nil || partyDetails.Project.ClientParty.LegalName != "创建时甲方名称" {
		t.Fatalf("party snapshot changed with current Party: %#v", partyDetails.Project.ClientParty)
	}
	requestJSON(t, server, http.MethodPost, "/api/v1/projects", commandBody(91, `"project_number":"OA-PARTY-STALE","name":"过期主体","project_type_keys":["design"],"client_party":{"party_id":"`+systemParty+`","source_revision":1}`), http.StatusUnprocessableEntity)
	partyDown.Store(true)
	requestJSON(t, server, http.MethodPost, "/api/v1/projects", commandBody(92, `"project_number":"OA-PARTY-DOWN","name":"不可用主体","project_type_keys":["design"],"client_party":{"party_id":"`+systemParty+`","source_revision":2}`), http.StatusServiceUnavailable)
	partyDown.Store(false)
	var rejectedProjects int
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM projects WHERE tenant_id=$1 AND project_number IN ('OA-PARTY-STALE','OA-PARTY-DOWN')`, systemTenant).Scan(&rejectedProjects); err != nil || rejectedProjects != 0 {
		t.Fatalf("rejected Party references left projects=%d error=%v", rejectedProjects, err)
	}

	created := requestJSON(t, server, http.MethodPost, "/api/v1/projects", commandBody(1, `"project_number":"OA-DEMO-001","name":"设计 OA 演示","summary":"J1 纵向闭环","project_type_keys":["design"]`), http.StatusCreated)
	var project projects.Project
	decodeResponse(t, created, &project)
	if project.Revision != 1 || project.Status != projects.StatusDraft {
		t.Fatalf("created project=%#v", project)
	}
	replayed := requestJSON(t, server, http.MethodPost, "/api/v1/projects", commandBody(1, `"project_number":"OA-DEMO-001","name":"设计 OA 演示","summary":"J1 纵向闭环","project_type_keys":["design"]`), http.StatusCreated)
	var replay projects.Project
	decodeResponse(t, replayed, &replay)
	if replay.ID != project.ID {
		t.Fatalf("idempotent create returned %s, want %s", replay.ID, project.ID)
	}
	requestJSON(t, server, http.MethodPost, "/api/v1/projects", commandBody(1, `"project_number":"OA-DEMO-001","name":"不同内容","project_type_keys":["design"]`), http.StatusConflict)
	incompleteResponse := requestJSON(t, server, http.MethodPost, "/api/v1/projects", commandBody(20, `"project_number":"OA-INCOMPLETE","name":"待补全项目","project_type_keys":["design"]`), http.StatusCreated)
	var incompleteProject projects.Project
	decodeResponse(t, incompleteResponse, &incompleteProject)
	requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+incompleteProject.ID+"/complete-initiation", commandBody(21, `"expected_revision":1`), http.StatusUnprocessableEntity)
	incompleteDetailsResponse := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+incompleteProject.ID, "", http.StatusOK)
	var incompleteDetails projects.Details
	decodeResponse(t, incompleteDetailsResponse, &incompleteDetails)
	if incompleteDetails.Project.Status != projects.StatusPendingCompletion || incompleteDetails.Project.Revision != 2 {
		t.Fatalf("incomplete project=%#v", incompleteDetails.Project)
	}

	business := requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/assignments", commandBody(2, `"expected_revision":1,"operation":"assign","subject_kind":"person","subject_id":"`+systemPerson+`","source_revision":1,"role_key":"business_owner"`), http.StatusOK)
	var businessResult projects.AssignmentResult
	decodeResponse(t, business, &businessResult)
	if businessResult.Project.Revision != 2 || businessResult.Assignment.DisplaySnapshot != "经营负责人" {
		t.Fatalf("business result=%#v", businessResult)
	}
	requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/assignments", commandBody(30, `"expected_revision":2,"operation":"assign","subject_kind":"person","subject_id":"99999999-9999-4999-8999-999999999999","source_revision":1,"role_key":"participant"`), http.StatusUnprocessableEntity)
	afterInvalidReference := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+project.ID, "", http.StatusOK)
	var detailsAfterInvalid projects.Details
	decodeResponse(t, afterInvalidReference, &detailsAfterInvalid)
	if detailsAfterInvalid.Project.Revision != 2 || len(detailsAfterInvalid.Assignments) != 1 {
		t.Fatalf("invalid reference caused partial write: %#v", detailsAfterInvalid)
	}
	peopleDown.Store(true)
	businessReplay := requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/assignments", commandBody(2, `"expected_revision":1,"operation":"assign","subject_kind":"person","subject_id":"`+systemPerson+`","source_revision":1,"role_key":"business_owner"`), http.StatusOK)
	var replayedBusiness projects.AssignmentResult
	decodeResponse(t, businessReplay, &replayedBusiness)
	if replayedBusiness.Assignment.ID != businessResult.Assignment.ID || replayedBusiness.Project.Revision != 2 {
		t.Fatalf("assignment replay=%#v", replayedBusiness)
	}
	requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/assignments", commandBody(3, `"expected_revision":2,"operation":"assign","subject_kind":"person","subject_id":"`+systemPerson+`","source_revision":1,"role_key":"participant"`), http.StatusServiceUnavailable)
	peopleDown.Store(false)
	production := requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/assignments", commandBody(4, `"expected_revision":2,"operation":"assign","subject_kind":"group","subject_id":"`+systemGroup+`","source_revision":2,"role_key":"production_owner"`), http.StatusOK)
	var productionResult projects.AssignmentResult
	decodeResponse(t, production, &productionResult)
	if productionResult.Project.Revision != 3 {
		t.Fatalf("production revision=%d", productionResult.Project.Revision)
	}
	active := requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/complete-initiation", commandBody(5, `"expected_revision":3`), http.StatusOK)
	decodeResponse(t, active, &project)
	if project.Status != projects.StatusActive || project.Revision != 4 {
		t.Fatalf("active project=%#v", project)
	}

	contractDown.Store(true)
	requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/resource-links", commandBody(6, `"expected_revision":4,"operation":"link","target_service":"contract-ledger","resource_type":"contract","resource_id":"`+systemContract+`","source_revision":3,"relation_key":"primary-contract"`), http.StatusServiceUnavailable)
	contractDown.Store(false)
	linked := requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/resource-links", commandBody(6, `"expected_revision":4,"operation":"link","target_service":"contract-ledger","resource_type":"contract","resource_id":"`+systemContract+`","source_revision":3,"relation_key":"primary-contract"`), http.StatusOK)
	var linkResult projects.ResourceLinkResult
	decodeResponse(t, linked, &linkResult)
	if linkResult.Project.Revision != 5 || linkResult.ResourceLink.ResourceID != systemContract {
		t.Fatalf("link result=%#v", linkResult)
	}
	contractDown.Store(true)
	linkReplay := requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/resource-links", commandBody(6, `"expected_revision":4,"operation":"link","target_service":"contract-ledger","resource_type":"contract","resource_id":"`+systemContract+`","source_revision":3,"relation_key":"primary-contract"`), http.StatusOK)
	var replayedLink projects.ResourceLinkResult
	decodeResponse(t, linkReplay, &replayedLink)
	if replayedLink.ResourceLink.ID != linkResult.ResourceLink.ID || replayedLink.Project.Revision != 5 {
		t.Fatalf("link replay=%#v", replayedLink)
	}

	detailsResponse := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+project.ID, "", http.StatusOK)
	var details projects.Details
	decodeResponse(t, detailsResponse, &details)
	if details.Project.Revision != 5 || len(details.Assignments) != 2 || len(details.ResourceLinks) != 1 || len(details.ReferenceStatuses) != 1 || details.ReferenceStatuses[0].Status != "unavailable" {
		t.Fatalf("details=%#v", details)
	}
	cancelled := requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/cancel", commandBody(7, `"expected_revision":5,"reason":"演示取消验收"`), http.StatusOK)
	decodeResponse(t, cancelled, &project)
	if project.Status != projects.StatusCancelled || project.Revision != 6 {
		t.Fatalf("cancelled project=%#v", project)
	}
	requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+project.ID+"/assignments", commandBody(8, `"expected_revision":6,"operation":"end","assignment_id":"`+businessResult.Assignment.ID+`","end_reason":"不应执行"`), http.StatusConflict)

	// The reference OA's initial dispatch is not strict Create. Its business
	// owner and blank draft are one transaction; production can follow activation.
	for index, nodeType := range []string{projects.NodeTypeProject, projects.NodeTypePackage} {
		sequence := 100 + index*10
		dispatchBody := commandBody(sequence, `"node_type":"`+nodeType+`","name":"OA 后补生产负责人","business_owner":{"person_id":"`+systemPerson+`","source_revision":1}`)
		dispatchResponse := requestJSON(t, server, http.MethodPost, "/api/v1/initiation-dispatches", dispatchBody, http.StatusCreated)
		var dispatched projects.Project
		decodeResponse(t, dispatchResponse, &dispatched)
		if dispatched.ProjectNumber != "" || len(dispatched.ProjectTypeKeys) != 0 || dispatched.Revision != 1 || dispatched.Status != projects.StatusDraft {
			t.Fatalf("dispatch prematurely required completion fields: %#v", dispatched)
		}
		peopleDown.Store(true)
		replayResponse := requestJSON(t, server, http.MethodPost, "/api/v1/initiation-dispatches", dispatchBody, http.StatusCreated)
		var dispatchedReplay projects.Project
		decodeResponse(t, replayResponse, &dispatchedReplay)
		if dispatchedReplay.ID != dispatched.ID {
			t.Fatal("dispatch replay created another project")
		}
		peopleDown.Store(false)
		number := "OA-DISPATCH-001"
		if nodeType == projects.NodeTypePackage {
			number = ""
		}
		requestJSON(t, server, http.MethodPut, "/api/v1/projects/"+dispatched.ID, commandBody(sequence+1, `"expected_revision":1,"name":"OA 后补生产负责人","project_number":"`+number+`","project_type_keys":["design"],"summary":"原 OA 输入时机"`), http.StatusOK)
		completedResponse := requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+dispatched.ID+"/complete-initiation", commandBody(sequence+2, `"expected_revision":2,"initiation_profile":"business-owner-v1"`), http.StatusOK)
		var completed projects.Project
		decodeResponse(t, completedResponse, &completed)
		if completed.ID != dispatched.ID || completed.Status != projects.StatusActive || completed.Revision != 3 {
			t.Fatalf("completion did not retain project identity: %#v", completed)
		}
		beforeLateOwner := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+dispatched.ID, "", http.StatusOK)
		var beforeLateDetails projects.Details
		decodeResponse(t, beforeLateOwner, &beforeLateDetails)
		if len(beforeLateDetails.Assignments) != 1 || beforeLateDetails.Assignments[0].RoleKey != projects.RoleBusinessOwner {
			t.Fatalf("activation required or fabricated a production owner: %#v", beforeLateDetails.Assignments)
		}
		requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+dispatched.ID+"/assignments", commandBody(sequence+3, `"expected_revision":3,"operation":"assign","subject_kind":"person","subject_id":"`+systemPerson+`","source_revision":1,"role_key":"production_owner"`), http.StatusOK)
		lateOwnerResponse := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+dispatched.ID, "", http.StatusOK)
		var lateDetails projects.Details
		decodeResponse(t, lateOwnerResponse, &lateDetails)
		if lateDetails.Project.ID != dispatched.ID || lateDetails.Project.Status != projects.StatusActive || lateDetails.Project.Revision != 4 || len(lateDetails.Assignments) != 2 {
			t.Fatalf("later production owner did not persist on same active project: %#v", lateDetails)
		}
		peopleDown.Store(true)
		correctionBody := commandBody(sequence+4, `"expected_revision":4,"name":"OA 基本资料更正","project_number":"`+number+`","project_type_keys":["design","consultation"],"summary":"已立项后修改","correction_reason":"修改项目基本资料"`)
		requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+dispatched.ID+"/profile-corrections", correctionBody, http.StatusOK)
		requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+dispatched.ID+"/profile-corrections", correctionBody, http.StatusOK)
		correctedResponse := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+dispatched.ID, "", http.StatusOK)
		var correctedDetails projectDetailsResponse
		decodeResponse(t, correctedResponse, &correctedDetails)
		if correctedDetails.Project.ID != dispatched.ID || correctedDetails.Project.Revision != 5 || correctedDetails.Project.Name != "OA 基本资料更正" || len(correctedDetails.Assignments) != 2 || !containsAction(correctedDetails.AllowedActions, auth.ActionProfileCorrect) || containsAction(correctedDetails.AllowedActions, auth.ActionUpdate) {
			t.Fatalf("profile correction did not persist: %#v", correctedDetails)
		}
		peopleDown.Store(false)
	}
	directoryResponse := requestJSON(t, server, http.MethodGet, "/api/v1/project-directory?keyword=OA%20基本资料更正&limit=100&offset=0", "", http.StatusOK)
	var directory projects.DirectoryPage
	decodeResponse(t, directoryResponse, &directory)
	if directory.Total != 2 || len(directory.Items) != 2 || directory.HasMore {
		t.Fatalf("native persisted directory: %#v", directory)
	}
	for _, entry := range directory.Items {
		if entry.Revision != 5 || len(entry.Assignments) != 2 {
			t.Fatalf("directory lacks current facts: %#v", entry)
		}
		clearBody := commandBody(200+int(entry.Revision)+len(entry.ProjectNumber), `"expected_revision":5,"reason":"原 OA 统一基本信息保存","profile":{"name":"OA 原子基本信息","project_number":"`+entry.ProjectNumber+`","project_type_keys":["design"],"summary":"两类负责人同次清除"},"owners":[{"role_key":"business_owner","operation":"clear"},{"role_key":"production_owner","operation":"clear"}]`)
		clearedResponse := requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+entry.ID+"/basic-info-changes", clearBody, http.StatusOK)
		var cleared projects.BasicInfoChangeResult
		decodeResponse(t, clearedResponse, &cleared)
		if cleared.Project.ID != entry.ID || cleared.Project.Revision != 6 || len(cleared.Owners) != 2 || !cleared.Owners[0].Changed || !cleared.Owners[1].Changed || cleared.Owners[0].Current != nil || cleared.Owners[1].Current != nil {
			t.Fatalf("HTTP atomic clear result: %#v", cleared)
		}
		requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+entry.ID+"/basic-info-changes", clearBody, http.StatusOK)
		currentOnlyResponse := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+entry.ID, "", http.StatusOK)
		var currentOnly projects.Details
		decodeResponse(t, currentOnlyResponse, &currentOnly)
		if currentOnly.Project.Revision != 6 || len(currentOnly.Assignments) != 0 {
			t.Fatal("default detail leaked ended assignments as current")
		}
		clearedDetailsResponse := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+entry.ID+"?include_inactive=true", "", http.StatusOK)
		var clearedDetails projects.Details
		decodeResponse(t, clearedDetailsResponse, &clearedDetails)
		if clearedDetails.Project.Revision != 6 || clearedDetails.Project.Name != "OA 原子基本信息" || len(clearedDetails.Assignments) != 2 || clearedDetails.Assignments[0].EndedAt == nil || clearedDetails.Assignments[1].EndedAt == nil {
			t.Fatalf("HTTP clear did not retain ended history: %#v", clearedDetails)
		}
		setBody := commandBody(250+len(entry.ProjectNumber), `"expected_revision":6,"reason":"重新分配两类负责人","owners":[{"role_key":"business_owner","operation":"set","person":{"person_id":"`+systemPerson+`","source_revision":1}},{"role_key":"production_owner","operation":"set","person":{"person_id":"`+systemPerson+`","source_revision":1}}]`)
		peopleDown.Store(true)
		requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+entry.ID+"/basic-info-changes", setBody, http.StatusServiceUnavailable)
		unchangedResponse := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+entry.ID+"?include_inactive=true", "", http.StatusOK)
		var unchanged projects.Details
		decodeResponse(t, unchangedResponse, &unchanged)
		if unchanged.Project.Revision != 6 || len(unchanged.Assignments) != 2 {
			t.Fatal("failed People validation wrote partial basic info")
		}
		peopleDown.Store(false)
		requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+entry.ID+"/basic-info-changes", setBody, http.StatusOK)
		peopleDown.Store(true)
		requestJSON(t, server, http.MethodPost, "/api/v1/projects/"+entry.ID+"/basic-info-changes", setBody, http.StatusOK)
		peopleDown.Store(false)
		restoredResponse := requestJSON(t, server, http.MethodGet, "/api/v1/projects/"+entry.ID+"?include_inactive=true", "", http.StatusOK)
		var restored projects.Details
		decodeResponse(t, restoredResponse, &restored)
		if restored.Project.Revision != 7 || len(restored.Assignments) != 4 {
			t.Fatalf("atomic owner restore/replay: %#v", restored)
		}
	}

	var projectColumns []string
	rows, err := environment.Pool.Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='projects' ORDER BY column_name`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		_ = rows.Scan(&name)
		projectColumns = append(projectColumns, name)
	}
	rows.Close()
	for _, forbidden := range []string{"contract_amount", "contract_status", "object_key", "money_balance", "person_name"} {
		for _, column := range projectColumns {
			if column == forbidden {
				t.Fatalf("Project schema copied external fact column %s", forbidden)
			}
		}
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, path, body string, wantStatus int) []byte {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer "+systemAccessToken)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, response.Code, wantStatus, response.Body.String())
	}
	return response.Body.Bytes()
}

func commandBody(sequence int, fields string) string {
	commandID := "aaaaaaaa-aaaa-4aaa-8aaa-" + leftPadSystem(sequence)
	if fields == "" {
		return `{"command_id":"` + commandID + `"}`
	}
	return `{"command_id":"` + commandID + `",` + fields + `}`
}

func leftPadSystem(value int) string {
	raw := "000000000000" + strconv.Itoa(value)
	return raw[len(raw)-12:]
}

func decodeResponse(t *testing.T, body []byte, destination any) {
	t.Helper()
	if err := json.Unmarshal(body, destination); err != nil {
		t.Fatalf("decode response %s: %v", string(body), err)
	}
}
