package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"omsbizfoundry/project-workspace/internal/auth"
	"omsbizfoundry/project-workspace/internal/projects"
	"strings"
	"testing"
)

func TestCounterpartyContextCapabilitiesScopeValidationAndActions(t *testing.T) {
	for _, test := range []struct {
		name, method string
		actions      []string
		scope        bool
		code         int
	}{
		{"read", "GET", []string{auth.ActionCounterpartyContextRead}, true, 200}, {"manage no read", "GET", []string{auth.ActionCounterpartyContextManage}, true, 403}, {"read no write", "POST", []string{auth.ActionCounterpartyContextRead}, true, 403}, {"core not business", "POST", []string{auth.ActionProfileCorrect}, true, 403}, {"manage", "POST", []string{auth.ActionCounterpartyContextManage}, true, 200}, {"wrong scope", "POST", []string{auth.ActionCounterpartyContextManage}, false, 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := fullPrincipal()
			p.AllowedActions = map[string]bool{}
			for _, a := range test.actions {
				p.AllowedActions[a] = true
			}
			if !test.scope {
				p.ScopeKind = "resources"
				p.ResourceIDs = map[string]bool{}
			}
			stub := &useCasesStub{contextResult: projects.CounterpartyContextView{ProjectID: httpProject, ProjectRevision: 3, ProjectStatus: projects.StatusActive}}
			server, _ := New(stub, &authenticatorStub{principal: p})
			body := `{"command_id":"` + httpCommand + `","expected_revision":3,"reason":"录入甲方","name":"甲方","contact_name":"","contact_phone":""}`
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(test.method, "/api/v1/projects/"+httpProject+"/counterparty-context", strings.NewReader(body)))
			if response.Code != test.code {
				t.Fatalf("code=%d body=%s", response.Code, response.Body.String())
			}
			if test.code != 200 && (stub.contextCalls != 0 || stub.contextReads != 0) {
				t.Fatal("denied hit domain")
			}
		})
	}
	stub := &useCasesStub{}
	server, _ := New(stub, &authenticatorStub{principal: fullPrincipal()})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest("POST", "/api/v1/projects/"+httpProject+"/counterparty-context", strings.NewReader(`{"command_id":"`+httpCommand+`","expected_revision":3,"reason":"x","name":"甲方","contact_phone":"abc"}`)))
	if response.Code != 400 || stub.contextCalls != 0 {
		t.Fatal("invalid phone reached domain")
	}
}
func TestCounterpartyContextHistoryReadDoesNotLeakContacts(t *testing.T) {
	page := projects.HistoryPage{Items: []projects.HistoryEvent{{ID: httpCommand, ProjectRevision: 4, Payload: map[string]any{"schema_version": projects.CounterpartyContextSchema, "context": map[string]any{"name": "秘密甲方", "contact_phone": "123456"}, "reason": "个人信息"}}}}
	p := fullPrincipal()
	delete(p.AllowedActions, auth.ActionCounterpartyContextRead)
	stub := &useCasesStub{historyResult: page}
	server, _ := New(stub, &authenticatorStub{principal: p})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject+"/history", nil))
	if response.Code != 200 || strings.Contains(response.Body.String(), "秘密") || strings.Contains(response.Body.String(), "123456") || strings.Contains(response.Body.String(), "个人信息") {
		t.Fatalf("leak=%s", response.Body.String())
	}
	if page.Items[0].Payload["context"] == nil {
		t.Fatal("redaction mutated stored history")
	}
	p.AllowedActions[auth.ActionCounterpartyContextRead] = true
	server, _ = New(stub, &authenticatorStub{principal: p})
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+httpProject+"/history", nil))
	var decoded projects.HistoryPage
	_ = json.Unmarshal(response.Body.Bytes(), &decoded)
	if decoded.Items[0].Payload["context"] == nil {
		t.Fatal("authorized context missing")
	}
}
