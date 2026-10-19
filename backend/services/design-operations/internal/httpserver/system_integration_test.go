//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"omsbizfoundry/design-operations/internal/auth"
	"omsbizfoundry/design-operations/internal/database"
	"omsbizfoundry/design-operations/internal/design"
	"omsbizfoundry/design-operations/internal/references"
	"omsbizfoundry/design-operations/internal/testdatabase"
)

const (
	systemTenant     = "11111111-1111-4111-8111-111111111111"
	systemSubject    = "22222222-2222-4222-8222-222222222222"
	systemProject    = "33333333-3333-4333-8333-333333333333"
	systemAssignment = "44444444-4444-4444-8444-444444444444"
	systemFile       = "55555555-5555-4555-8555-555555555555"
	systemToken      = "design-operations-system-test-token"
)

func TestHTTPPostgresDesignVerticalSlice(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer reference-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/references/projects/" + systemProject:
			_, _ = w.Write([]byte(`{"id":"` + systemProject + `","tenant_id":"` + systemTenant + `","status":"active","revision":4,"project_type_keys":["design"]}`))
		case "/api/v1/references/projects/" + systemProject + "/assignments/" + systemAssignment:
			_, _ = w.Write([]byte(`{"id":"` + systemAssignment + `","tenant_id":"` + systemTenant + `","project_id":"` + systemProject + `","role_key":"production_owner","status":"active","display_name":"设计生产组","revision":3}`))
		case "/v1/references/files/" + systemFile + "/versions/1":
			_, _ = w.Write([]byte(`{"file_id":"` + systemFile + `","tenant_id":"` + systemTenant + `","status":"available","display_name":"方案成果.pdf","version":1,"revision":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	store, _ := design.NewPostgresStore(environment.Pool)
	endpoint := references.Endpoint{BaseURL: upstream.URL, AccessToken: "reference-token"}
	validator, _ := references.NewHTTPValidator(time.Second, endpoint, endpoint, endpoint)
	service, _ := design.NewService(store, validator)
	authenticator, _ := auth.NewTemplateProvider(systemTenant, systemSubject, "系统验收管理员", systemToken)
	server, _ := New(service, authenticator)

	createdBody := requestJSON(t, server, http.MethodPost, "/api/v1/design-work-items", commandBody(1, `"project_id":"`+systemProject+`","project_source_revision":4`), http.StatusCreated)
	var created design.Details
	decodeResponse(t, createdBody, &created)
	itemID := created.WorkItem.ID
	requestJSON(t, server, http.MethodPost, "/api/v1/design-work-items/"+itemID+"/owner", commandBody(2, `"expected_revision":1,"assignment_id":"`+systemAssignment+`","assignment_source_revision":3`), http.StatusOK)
	requestJSON(t, server, http.MethodPost, "/api/v1/design-work-items/"+itemID+"/stages/scheme/deliverables", commandBody(3, `"expected_revision":2,"requirement_key":"primary-output","evidence_file_id":"`+systemFile+`","evidence_version":1,"evidence_source_revision":1`), http.StatusOK)
	completedBody := requestJSON(t, server, http.MethodPost, "/api/v1/design-work-items/"+itemID+"/stages/scheme/complete", commandBody(4, `"expected_revision":3`), http.StatusOK)
	var completed design.Details
	decodeResponse(t, completedBody, &completed)
	if completed.WorkItem.Revision != 4 || completed.Stages[0].Status != design.StageCompleted || completed.Stages[1].Status != design.StageReady {
		t.Fatalf("completed=%#v", completed)
	}
	referenceBody := requestJSON(t, server, http.MethodGet, "/api/v1/references/design-work-items/"+itemID, "", http.StatusOK)
	var reference design.ReferenceView
	decodeResponse(t, referenceBody, &reference)
	if reference.ID != itemID || reference.WorkItemID != itemID || reference.ResourceType != "design-work-item" || reference.Status != design.StatusInProgress {
		t.Fatalf("reference=%#v", reference)
	}
	requestJSON(t, server, http.MethodPost, "/api/v1/design-work-items/"+itemID+"/stages/construction-drawing/applicability", commandBody(5, `"expected_revision":4,"operation":"set-not-applicable","reason":"非法跳过"`), http.StatusUnprocessableEntity)
	getBody := requestJSON(t, server, http.MethodGet, "/api/v1/design-work-items/"+itemID, "", http.StatusOK)
	var details design.Details
	decodeResponse(t, getBody, &details)
	if details.WorkItem.Revision != 4 || len(details.ReferenceStatuses) != 2 {
		t.Fatalf("details=%#v", details)
	}
	historyBody := requestJSON(t, server, http.MethodGet, "/api/v1/design-work-items/"+itemID+"/history?limit=2", "", http.StatusOK)
	var history design.HistoryPage
	decodeResponse(t, historyBody, &history)
	if len(history.Items) != 2 || history.NextAfterSequence == nil {
		t.Fatalf("history=%#v", history)
	}
}

func TestHTTPPostgresProductionConsultationVerticalSlice(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer reference-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/references/projects/" + systemProject:
			_, _ = w.Write([]byte(`{"id":"` + systemProject + `","tenant_id":"` + systemTenant + `","status":"active","revision":4,"project_type_keys":["consultation"]}`))
		case "/api/v1/references/projects/" + systemProject + "/assignments/" + systemAssignment:
			_, _ = w.Write([]byte(`{"id":"` + systemAssignment + `","tenant_id":"` + systemTenant + `","project_id":"` + systemProject + `","role_key":"production_owner","status":"active","display_name":"咨询生产组","revision":3}`))
		case "/v1/references/files/" + systemFile + "/versions/1":
			_, _ = w.Write([]byte(`{"file_id":"` + systemFile + `","tenant_id":"` + systemTenant + `","status":"available","display_name":"咨询报告.pdf","version":1,"revision":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	store, _ := design.NewPostgresStore(environment.Pool)
	endpoint := references.Endpoint{BaseURL: upstream.URL, AccessToken: "reference-token"}
	validator, _ := references.NewHTTPValidator(time.Second, endpoint, endpoint, endpoint)
	service, _ := design.NewService(store, validator)
	designAuthenticator, _ := auth.NewTemplateProvider(systemTenant, systemSubject, "系统验收管理员", systemToken)
	productionAuthenticator, _ := auth.NewProductionTemplateProvider(systemTenant, systemSubject, "系统验收管理员", systemToken)
	server, _ := NewWithProduction(service, designAuthenticator, productionAuthenticator)

	createdBody := requestJSON(t, server, http.MethodPost, "/api/v1/production-work-items", commandBody(21, `"project_id":"`+systemProject+`","project_source_revision":4,"type_key":"consultation-v1"`), http.StatusCreated)
	var created design.Details
	decodeResponse(t, createdBody, &created)
	itemID := created.WorkItem.ID
	if created.WorkItem.TypeKey != design.ConsultationTypeKey || len(created.Stages) != 2 || created.Stages[1].Key != "report" {
		t.Fatalf("created=%#v", created)
	}
	requestJSON(t, server, http.MethodGet, "/api/v1/design-work-items/"+itemID, "", http.StatusNotFound)
	requestJSON(t, server, http.MethodPost, "/api/v1/production-work-items/"+itemID+"/owner", commandBody(22, `"expected_revision":1,"assignment_id":"`+systemAssignment+`","assignment_source_revision":3`), http.StatusOK)
	requestJSON(t, server, http.MethodPost, "/api/v1/production-work-items/"+itemID+"/stages/scheme/applicability", commandBody(23, `"expected_revision":2,"operation":"set-not-applicable","reason":"本咨询任务无需独立方案"`), http.StatusOK)
	requestJSON(t, server, http.MethodPost, "/api/v1/production-work-items/"+itemID+"/stages/report/deliverables", commandBody(24, `"expected_revision":3,"requirement_key":"primary-output","evidence_file_id":"`+systemFile+`","evidence_version":1,"evidence_source_revision":1`), http.StatusOK)
	completedBody := requestJSON(t, server, http.MethodPost, "/api/v1/production-work-items/"+itemID+"/stages/report/complete", commandBody(25, `"expected_revision":4`), http.StatusOK)
	var completed design.Details
	decodeResponse(t, completedBody, &completed)
	if completed.WorkItem.Status != design.StatusCompleted || completed.WorkItem.Revision != 5 || completed.Stages[0].Status != design.StageNotApplicable || completed.Stages[1].Status != design.StageCompleted {
		t.Fatalf("completed=%#v", completed)
	}
	referenceBody := requestJSON(t, server, http.MethodGet, "/api/v1/references/production-work-items/"+itemID, "", http.StatusOK)
	var reference design.ReferenceView
	decodeResponse(t, referenceBody, &reference)
	if reference.TypeKey != design.ConsultationTypeKey || reference.ResourceType != "production-work-item" || reference.Revision != 5 || len(reference.CompletionGaps) != 0 {
		t.Fatalf("reference=%#v", reference)
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, path, body string, wantStatus int) []byte {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer "+systemToken)
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
