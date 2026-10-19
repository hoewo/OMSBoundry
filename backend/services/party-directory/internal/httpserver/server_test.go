package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/party-directory/internal/auth"
	"omsbizfoundry/party-directory/internal/party"
)

const (
	testTenant  = "11111111-1111-4111-8111-111111111111"
	testSubject = "22222222-2222-4222-8222-222222222222"
	testParty   = "33333333-3333-4333-8333-333333333333"
)

type fixedAuth struct {
	principal    auth.Principal
	err          error
	sawUntrusted bool
}

func (a *fixedAuth) ProviderName() string { return "test" }
func (a *fixedAuth) Authenticate(r *http.Request) (auth.Principal, error) {
	a.sawUntrusted = r.Header.Get("X-Tenant-ID") != "" || r.Header.Get("X-OMS-Actor") != ""
	return a.principal, a.err
}

type fakeCases struct {
	value      party.Party
	reference  party.Reference
	err        error
	actor      party.Actor
	listFilter party.ListFilter
}

func (f *fakeCases) Ping(context.Context) error { return f.err }
func (f *fakeCases) Create(_ context.Context, actor party.Actor, _ party.CreateInput) (party.Party, error) {
	f.actor = actor
	return f.value, f.err
}
func (f *fakeCases) List(_ context.Context, _ string, filter party.ListFilter, _ string) (party.Page, error) {
	f.listFilter = filter
	return party.Page{Items: []party.Party{f.value}}, f.err
}
func (f *fakeCases) Get(context.Context, string, string) (party.Party, error) { return f.value, f.err }
func (f *fakeCases) Reference(context.Context, string, string) (party.Reference, error) {
	return f.reference, f.err
}
func (f *fakeCases) History(context.Context, string, string, *int64, int) (party.HistoryPage, error) {
	return party.HistoryPage{}, f.err
}
func (f *fakeCases) Update(_ context.Context, actor party.Actor, _ string, _ party.UpdateInput) (party.Party, error) {
	f.actor = actor
	return f.value, f.err
}
func (f *fakeCases) ChangeStatus(_ context.Context, actor party.Actor, _ string, _ party.StatusInput) (party.Party, error) {
	f.actor = actor
	return f.value, f.err
}

func TestProtectedRoutesFailClosedAndStripIdentityHeaders(t *testing.T) {
	cases := &fakeCases{value: party.Party{ID: testParty, TenantID: testTenant, Status: party.StatusActive}}
	authenticator := &fixedAuth{err: auth.ErrUnauthenticated}
	server, _ := New(cases, authenticator)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/parties", nil)
	req.Header.Set("X-Tenant-ID", "attacker")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
	if authenticator.sawUntrusted {
		t.Fatal("untrusted identity headers reached authenticator")
	}
	authenticator.err = nil
	authenticator.principal = principal(auth.ActionCreate, "tenant")
	res = httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/parties", nil))
	if res.Code != http.StatusForbidden {
		t.Fatalf("missing read capability should be 403, got %d", res.Code)
	}
}

func TestDemoEntryAndListActionsAreHumanUsable(t *testing.T) {
	cases := &fakeCases{value: party.Party{ID: testParty, TenantID: testTenant, Kind: party.KindOrganization, LegalName: "示范设计院", Status: party.StatusActive, Revision: 1}}
	actions := map[string]bool{}
	for _, action := range auth.AllActions {
		actions[action] = true
	}
	server, _ := New(cases, &fixedAuth{principal: auth.Principal{SubjectID: testSubject, TenantID: testTenant, DisplayName: "主体管理员", ScopeKind: "tenant", AllowedActions: actions}})

	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "组织与个人主体") || !strings.Contains(page.Body.String(), "登记主体") || !strings.Contains(page.Body.String(), "assets/demo.js") {
		t.Fatalf("demo page status=%d body=%q", page.Code, page.Body.String())
	}
	asset := httptest.NewRecorder()
	server.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/demo.js", nil))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Header().Get("Content-Type"), "application/javascript") || !strings.Contains(asset.Body.String(), "/gateway/authorize/party-directory") || !strings.Contains(asset.Body.String(), "PARTY_REVISION_CONFLICT") {
		t.Fatalf("demo asset status=%d type=%q", asset.Code, asset.Header().Get("Content-Type"))
	}
	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/parties", nil))
	var payload party.Page
	if list.Code != http.StatusOK || json.Unmarshal(list.Body.Bytes(), &payload) != nil || len(payload.AllowedActions) != 2 || payload.AllowedActions[1] != auth.ActionCreate {
		t.Fatalf("list status=%d actions=%#v body=%q", list.Code, payload.AllowedActions, list.Body.String())
	}
}

func TestReferenceRequiresExactPartyScope(t *testing.T) {
	cases := &fakeCases{reference: party.Reference{SchemaVersion: "1", SourceApplication: "party-directory", ID: testParty, TenantID: testTenant, Status: party.StatusActive, Revision: 1}}
	authenticator := &fixedAuth{principal: principal(auth.ActionReferenceRead, "tenant")}
	server, _ := New(cases, authenticator)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/references/parties/"+testParty, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("tenant scope must not call workload reference API: %d", response.Code)
	}
	authenticator.principal = principal(auth.ActionReferenceRead, "resources")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("exact resource should pass: %d %s", response.Code, response.Body.String())
	}
	authenticator.principal.Resources["party"] = map[string]bool{"44444444-4444-4444-8444-444444444444": true}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("other party scope should fail: %d", response.Code)
	}
}

func TestCreateUsesTrustedActorAndStrictJSON(t *testing.T) {
	cases := &fakeCases{value: party.Party{ID: testParty, TenantID: testTenant, Status: party.StatusActive, Revision: 1}}
	authenticator := &fixedAuth{principal: principal(auth.ActionCreate, "tenant")}
	server, _ := New(cases, authenticator)
	body := []byte(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","kind":"organization","legal_name":"甲方","actor":"forged"}`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/parties", bytes.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown actor field must fail: %d", response.Code)
	}
	body = []byte(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","kind":"organization","legal_name":"甲方"}`)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/parties", bytes.NewReader(body)))
	if response.Code != http.StatusCreated {
		t.Fatalf("expected create: %d %s", response.Code, response.Body.String())
	}
	if cases.actor.ID != testSubject || cases.actor.TenantID != testTenant {
		t.Fatalf("actor must come from credential: %#v", cases.actor)
	}
}

func TestErrorMappingAndAllowedActions(t *testing.T) {
	cases := &fakeCases{value: party.Party{ID: testParty, TenantID: testTenant, Status: party.StatusInactive}, err: party.ErrRevisionConflict}
	authenticator := &fixedAuth{principal: principal(auth.ActionUpdate, "resources")}
	server, _ := New(cases, authenticator)
	body := []byte(`{"command_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","expected_revision":1,"kind":"organization","legal_name":"甲方","reason":"纠正"}`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/parties/"+testParty, bytes.NewReader(body)))
	if response.Code != http.StatusConflict {
		t.Fatalf("revision conflict should map to 409: %d", response.Code)
	}
	cases.err = nil
	authenticator.principal = principal(auth.ActionRead, "resources")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/parties/"+testParty, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("get failed: %d", response.Code)
	}
	var decoded party.Party
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.AllowedActions) != 1 || decoded.AllowedActions[0] != "read" {
		t.Fatalf("unexpected allowed actions: %#v", decoded.AllowedActions)
	}
}

func principal(action, scope string) auth.Principal {
	resources := map[string]map[string]bool{}
	if scope == "resources" {
		resources["party"] = map[string]bool{testParty: true}
	}
	return auth.Principal{SubjectID: testSubject, TenantID: testTenant, DisplayName: "测试用户", Provider: "iam", CredentialID: "55555555-5555-4555-8555-555555555555", ScopeKind: scope, AllowedActions: map[string]bool{action: true}, Resources: resources}
}
