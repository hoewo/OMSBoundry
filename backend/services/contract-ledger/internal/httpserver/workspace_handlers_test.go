package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/contractevidence"
	"omsbizfoundry/contract-ledger/internal/identity"
	"omsbizfoundry/contract-ledger/internal/partymigration"
)

type directoryPartyProjectorStub struct {
	value                                       partymigration.RuntimePartyReference
	err                                         error
	tenantID, partyID, reviewerID, reviewerName string
	revision                                    int64
}

type contractEvidenceStub struct {
	currentValue contractevidence.Snapshot
	currentFound bool
	replaceInput contractevidence.ReplaceInput
	replaceValue contractevidence.Result
	currentCalls int
	replaceCalls int
}

func (s *contractEvidenceStub) Current(context.Context, string, string) (contractevidence.Snapshot, bool, error) {
	s.currentCalls++
	return s.currentValue, s.currentFound, nil
}

func (s *contractEvidenceStub) Replace(_ context.Context, input contractevidence.ReplaceInput) (contractevidence.Result, error) {
	s.replaceCalls++
	s.replaceInput = input
	return s.replaceValue, nil
}

func (s *directoryPartyProjectorStub) ResolveOrProject(_ context.Context, tenantID, partyID string, revision int64, reviewerID, reviewerName string) (partymigration.RuntimePartyReference, error) {
	s.tenantID, s.partyID, s.revision, s.reviewerID, s.reviewerName = tenantID, partyID, revision, reviewerID, reviewerName
	return s.value, s.err
}

func workspacePrincipal(actions ...string) identity.Principal {
	p := identity.Principal{UserID: "11111111-1111-4111-8111-111111111111", DisplayName: "可信人员", Provider: "iam", TenantID: referenceTestTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{}}
	for _, action := range actions {
		p.AllowedActions[action] = true
	}
	return p
}

func workspaceRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test-session")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestWorkspaceContextDoesNotExpandExactContractScope(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*identity.Principal)
		status int
		create bool
	}{
		{"tenant draft manager", func(p *identity.Principal) {}, 200, true},
		{"tenant viewer", func(p *identity.Principal) { delete(p.AllowedActions, "contract-ledger.draft.manage") }, 200, false},
		{"exact contract draft manager", func(p *identity.Principal) {
			p.ScopeKind = "resources"
			p.Resources = map[string]map[string]bool{"contract": {referenceTestContract: true}}
		}, 200, false},
		{"no granted contracts", func(p *identity.Principal) {
			p.ScopeKind = "resources"
			p.Resources = map[string]map[string]bool{"contract": {referenceTestContract: false}}
		}, 403, false},
		{"different resource type", func(p *identity.Principal) {
			p.ScopeKind = "resources"
			p.Resources = map[string]map[string]bool{"settlement": {referenceTestContract: true}}
		}, 403, false},
		{"reference template", func(p *identity.Principal) { p.Provider = "reference" }, 403, false},
		{"missing native view", func(p *identity.Principal) { delete(p.AllowedActions, "contract-ledger.view") }, 403, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := workspacePrincipal("contract-ledger.view", "contract-ledger.draft.manage")
			test.mutate(&p)
			c := &contractUseCasesStub{}
			h := newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, c, Options{})
			w := workspaceRequest(h, "GET", "/api/v1/contracts/workspace-context", "")
			if w.Code != test.status || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if test.status == 200 {
				var value struct {
					AllowedOperations []string `json:"allowed_operations"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
					t.Fatal(err)
				}
				want := []string{}
				if test.create {
					want = append(want, "draft.create")
				}
				if !reflect.DeepEqual(value.AllowedOperations, want) {
					t.Fatalf("context expanded scope: %s", w.Body.String())
				}
				if strings.Contains(w.Body.String(), referenceTestContract) {
					t.Fatal("context contains business resource")
				}
			}
			if p.ScopeKind == "resources" {
				if workspaceRequest(h, "POST", "/api/v1/contracts", `{"command_id":"must-not-create"}`).Code != 403 {
					t.Fatal("context authorized tenant creation")
				}
				if workspaceRequest(h, "GET", "/api/v1/contracts/"+referenceTestOther+"/workspace", "").Code != 403 {
					t.Fatal("context authorized neighbor read")
				}
			}
		})
	}
}

func TestWorkspacePartyCandidatesRequireTenantView(t *testing.T) {
	p := workspacePrincipal("contract-ledger.view")
	h := newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{})
	w := workspaceRequest(h, "GET", "/api/v1/contracts/party-candidates", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"directory_kind":"contract_local"`) || !strings.Contains(w.Body.String(), `"parties":[]`) {
		t.Fatalf("directory=%d %s", w.Code, w.Body.String())
	}
	p.ScopeKind = "resources"
	p.Resources = map[string]map[string]bool{"contract": {referenceTestContract: true}}
	h = newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{})
	if workspaceRequest(h, "GET", "/api/v1/contracts/party-candidates", "").Code != 403 {
		t.Fatal("exact Contract scope expanded to whole Party directory")
	}
}

func TestWorkspaceAddPartyProjectsExactDirectoryReference(t *testing.T) {
	p := workspacePrincipal("contract-ledger.view", "contract-ledger.draft.manage")
	c := &contractUseCasesStub{}
	projector := &directoryPartyProjectorStub{value: partymigration.RuntimePartyReference{
		LocalPartyID: "22222222-2222-4222-8222-222222222222", PartyID: "33333333-3333-4333-8333-333333333333", Revision: 7, Source: partymigration.RuntimeModeDirectory,
	}}
	h := newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, c, Options{DirectoryParties: projector})
	body := `{"command_id":"44444444-4444-4444-8444-444444444444","expected_draft_version":1,"party_reference":{"party_id":"33333333-3333-4333-8333-333333333333","revision":7},"role":"施工方"}`
	w := workspaceRequest(h, http.MethodPost, "/api/v1/contracts/"+referenceTestContract+"/parties", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if c.lastAdd.PartyID != projector.value.LocalPartyID || c.lastAdd.Role != "施工方" || c.lastAdd.Actor.ID != p.UserID {
		t.Fatalf("native Contract input=%#v", c.lastAdd)
	}
	if projector.tenantID != p.TenantID || projector.partyID != projector.value.PartyID || projector.revision != 7 || projector.reviewerID != p.UserID || projector.reviewerName != p.DisplayName {
		t.Fatalf("projection input=%#v", projector)
	}

	mixed := `{"command_id":"55555555-5555-4555-8555-555555555555","expected_draft_version":1,"party_id":"22222222-2222-4222-8222-222222222222","party_reference":{"party_id":"33333333-3333-4333-8333-333333333333","revision":7},"role":"施工方"}`
	if got := workspaceRequest(h, http.MethodPost, "/api/v1/contracts/"+referenceTestContract+"/parties", mixed); got.Code != http.StatusBadRequest {
		t.Fatalf("mixed authority status=%d body=%s", got.Code, got.Body.String())
	}
	projector.err = partymigration.ErrTargetInvalid
	if got := workspaceRequest(h, http.MethodPost, "/api/v1/contracts/"+referenceTestContract+"/parties", body); got.Code != http.StatusUnprocessableEntity {
		t.Fatalf("stale reference status=%d body=%s", got.Code, got.Body.String())
	}
}

func TestWorkspaceContractEvidenceUsesNativeCapabilityAndExactScope(t *testing.T) {
	p := workspacePrincipal("contract-ledger.view", "contract-ledger.evidence.manage")
	evidence := &contractEvidenceStub{
		currentFound: true,
		currentValue: contractevidence.Snapshot{ContractID: referenceTestContract, TenantID: referenceTestTenant, Revision: 0, EvidenceRefs: []contractevidence.Ref{}},
		replaceValue: contractevidence.Result{Snapshot: contractevidence.Snapshot{ContractID: referenceTestContract, TenantID: referenceTestTenant, Revision: 1, EvidenceRefs: []contractevidence.Ref{{FileID: "77777777-7777-4777-8777-777777777777", Version: 1, Purpose: "contract_document"}}}},
	}
	h := newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{ContractEvidence: evidence})
	path := "/api/v1/contracts/" + referenceTestContract + "/evidence"
	if w := workspaceRequest(h, http.MethodGet, path, ""); w.Code != http.StatusOK || evidence.currentCalls != 1 || !strings.Contains(w.Body.String(), `"revision":0`) {
		t.Fatalf("read status=%d body=%s calls=%d", w.Code, w.Body.String(), evidence.currentCalls)
	}
	body := `{"command_id":"44444444-4444-4444-8444-444444444444","expected_revision":0,"reason":"上传合同","evidence_refs":[{"file_id":"77777777-7777-4777-8777-777777777777","version":1,"purpose":"contract_document"}]}`
	if w := workspaceRequest(h, http.MethodPut, path, body); w.Code != http.StatusOK || evidence.replaceCalls != 1 {
		t.Fatalf("replace status=%d body=%s calls=%d", w.Code, w.Body.String(), evidence.replaceCalls)
	}
	if evidence.replaceInput.ContractID != referenceTestContract || evidence.replaceInput.TenantID != p.TenantID || evidence.replaceInput.Actor.ID != p.UserID || evidence.replaceInput.Actor.Kind != audit.ActorKindIAMSubject {
		t.Fatalf("replace input=%#v", evidence.replaceInput)
	}
	p.ScopeKind = "resources"
	p.Resources = map[string]map[string]bool{"contract": {referenceTestContract: true}}
	h = newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{ContractEvidence: evidence})
	if w := workspaceRequest(h, http.MethodPut, "/api/v1/contracts/"+referenceTestOther+"/evidence", body); w.Code != http.StatusForbidden || evidence.replaceCalls != 1 {
		t.Fatalf("neighbor status=%d calls=%d", w.Code, evidence.replaceCalls)
	}
	delete(p.AllowedActions, "contract-ledger.evidence.manage")
	h = newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{ContractEvidence: evidence})
	if w := workspaceRequest(h, http.MethodPut, path, body); w.Code != http.StatusForbidden || evidence.replaceCalls != 1 {
		t.Fatalf("missing capability status=%d calls=%d", w.Code, evidence.replaceCalls)
	}
}

func TestWorkspaceContextAdvertisesGenericEvidenceManagement(t *testing.T) {
	p := workspacePrincipal("contract-ledger.view", "contract-ledger.evidence.manage")
	h := newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{})
	w := workspaceRequest(h, http.MethodGet, "/api/v1/contracts/workspace-context", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"evidence.manage"`) {
		t.Fatalf("tenant evidence context status=%d body=%s", w.Code, w.Body.String())
	}
	p.ScopeKind = "resources"
	p.Resources = map[string]map[string]bool{"contract": {referenceTestContract: true}}
	h = newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{})
	w = workspaceRequest(h, http.MethodGet, "/api/v1/contracts/workspace-context", "")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), `"evidence.manage"`) {
		t.Fatalf("resource context expanded evidence scope status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestWorkspaceNativeCapabilityScopeAndLegacyIsolation(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*identity.Principal)
		path   string
		status int
	}{
		{"tenant create", func(p *identity.Principal) {}, "/api/v1/contracts", 200},
		{"local session denied", func(p *identity.Principal) { p.Provider = "local" }, "/api/v1/contracts", 403},
		{"missing tenant denied", func(p *identity.Principal) { p.TenantID = "" }, "/api/v1/contracts", 403},
		{"view does not manage draft", func(p *identity.Principal) { p.AllowedActions = map[string]bool{"contract-ledger.view": true} }, "/api/v1/contracts", 403},
		{"exact contract write", func(p *identity.Principal) {
			p.ScopeKind = "resources"
			p.Resources = map[string]map[string]bool{"contract": {referenceTestContract: true}}
		}, "/api/v1/contracts/" + referenceTestContract, 200},
		{"exact scope cannot create tenant fact", func(p *identity.Principal) {
			p.ScopeKind = "resources"
			p.Resources = map[string]map[string]bool{"contract": {referenceTestContract: true}}
		}, "/api/v1/contracts", 403},
		{"neighbor contract denied", func(p *identity.Principal) {
			p.ScopeKind = "resources"
			p.Resources = map[string]map[string]bool{"contract": {referenceTestOther: true}}
		}, "/api/v1/contracts/" + referenceTestContract, 403},
		{"wrong resource type denied", func(p *identity.Principal) {
			p.ScopeKind = "resources"
			p.Resources = map[string]map[string]bool{"settlement": {referenceTestContract: true}}
		}, "/api/v1/contracts/" + referenceTestContract, 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := workspacePrincipal("contract-ledger.view", "contract-ledger.draft.manage")
			test.mutate(&p)
			contracts := &contractUseCasesStub{}
			h := newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, contracts, Options{})
			w := workspaceRequest(h, "POST", test.path, `{"command_id":"cmd","total_amount":"9007199254740993.01"}`)
			if w.Code != test.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("credential workface must not cache")
			}
			if test.status != 200 && (contracts.lastCreate.CommandID != "" || contracts.lastUpdate.CommandID != "") {
				t.Fatal("denied request reached domain")
			}
		})
	}
	p := workspacePrincipal("contract-ledger.view", "contract-ledger.draft.manage")
	p.ScopeKind = "resources"
	p.Resources = map[string]map[string]bool{"contract": {referenceTestContract: true}}
	h := newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, &contractUseCasesStub{}, Options{ExternalIdentity: true, SessionCookieName: "oms_cap_contract_ledger"})
	for _, path := range []string{"/", "/contracts", "/parties"} {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(&http.Cookie{Name: "oms_cap_contract_ledger", Value: "test-session"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("exact credential leaked tenant HTML %s: %d", path, w.Code)
		}
	}
}

func TestWorkspaceStrictCommandsExactMoneyAndTrustedActor(t *testing.T) {
	p := workspacePrincipal("contract-ledger.view", "contract-ledger.draft.manage")
	for _, test := range []struct {
		name, body, contentType string
		status                  int
	}{
		{"exact decimal", `{"command_id":"cmd","total_amount":"9007199254740993.01"}`, "application/json", 200},
		{"floating payload forbidden", `{"command_id":"cmd","total_amount":9007199254740993.01}`, "application/json", 400},
		{"forged actor forbidden", `{"command_id":"cmd","actor":{"id":"forged"}}`, "application/json", 400},
		{"unknown field forbidden", `{"command_id":"cmd","project_id":"foreign"}`, "application/json", 400},
		{"null command", `null`, "application/json", 400},
		{"array command", `[]`, "application/json", 400},
		{"trailing command", `{} {}`, "application/json", 400},
		{"oversized command", `{"title":"` + strings.Repeat("a", 65536) + `"}`, "application/json", 400},
		{"wrong media type", `{}`, "text/plain", 415},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := &contractUseCasesStub{}
			h := newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, c, Options{})
			r := httptest.NewRequest("POST", "/api/v1/contracts", strings.NewReader(test.body))
			r.Header.Set("Authorization", "Bearer test-session")
			r.Header.Set("Content-Type", test.contentType)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != test.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if test.status == 200 {
				if c.lastCreate.TotalAmount != "9007199254740993.01" || c.lastCreate.Actor.Kind != audit.ActorKindIAMSubject || c.lastCreate.Actor.ID != p.UserID {
					t.Fatalf("domain input=%#v", c.lastCreate)
				}
			} else if c.lastCreate.CommandID != "" {
				t.Fatal("invalid command reached domain")
			}
		})
	}
	c := &contractUseCasesStub{}
	h := newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, c, Options{ReferenceTenantID: referenceTestTenant, ReferenceAccessToken: referenceTestToken})
	for _, token := range []string{"", referenceTestToken, "forged"} {
		r := httptest.NewRequest("POST", "/api/v1/contracts", strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "test-session"})
		r.Header.Set("X-Actor-ID", p.UserID)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("untrusted credential accepted %d", w.Code)
		}
	}
}

func TestWorkspaceRoutesReuseNativeCommandsWithoutPreRead(t *testing.T) {
	p := workspacePrincipal("contract-ledger.view", "contract-ledger.draft.manage", "contract-ledger.lifecycle.manage")
	c := &contractUseCasesStub{getErr: errors.New("must not pre-read before idempotent command replay")}
	h := newRawTestHandler(t, &identityUseCasesStub{principal: p}, &partyUseCasesStub{}, c, Options{})
	base := "/api/v1/contracts/" + referenceTestContract
	for _, cmd := range []struct{ path, body string }{
		{base, `{"command_id":"update","expected_draft_version":2,"total_amount":"19.99"}`},
		{base + "/cancel", `{"command_id":"cancel","expected_draft_version":3,"reason":"撤回"}`},
		{base + "/parties", `{"command_id":"add","expected_draft_version":4,"party_id":"party","role":"client"}`},
		{base + "/parties/link/remove", `{"command_id":"remove","expected_draft_version":5}`},
		{base + "/activate", `{"command_id":"activate","expected_draft_version":6}`},
		{base + "/changes", `{"command_id":"change","expected_revision":1,"amount_effect":"adjustment","amount_value":"-0.01"}`},
		{base + "/settlements", `{"command_id":"settle","expected_contract_revision":2,"expected_settlement_sequence":0,"kind":"progress","amount":"9007199254740993.01","confirming_party_positions":[1]}`},
	} {
		w := workspaceRequest(h, "POST", cmd.path, cmd.body)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", cmd.path, w.Code, w.Body.String())
		}
		var v map[string]any
		if json.Unmarshal(w.Body.Bytes(), &v) != nil || v["tenant_id"] != referenceTestTenant || v["source_application"] != "contract-ledger" || v["schema_version"] != "1" {
			t.Fatalf("receipt=%s", w.Body.String())
		}
	}
	if c.lastUpdate.ExpectedDraftVersion != 2 || c.lastCancel.Reason != "撤回" || c.lastAdd.Role != "client" || c.lastRemove.PartyLinkID != "link" || c.lastActivate.ExpectedDraftVersion != 6 || c.lastChange.AmountValue != "-0.01" || c.lastSettlement.Amount != "9007199254740993.01" || c.lastSettlement.ExpectedContractRevision != 2 {
		t.Fatal("native command fields lost")
	}
	if c.lastSettlement.Actor.ID != p.UserID || c.lastChange.ContractID != referenceTestContract {
		t.Fatal("trusted actor / resource routing lost")
	}
	// Internal diagnostics must never be returned to a browser.
	c.updateErr = errors.New("postgres://secret-password@private")
	w := workspaceRequest(h, "POST", base, `{"command_id":"fail"}`)
	if w.Code != 500 || strings.Contains(w.Body.String(), "secret-password") {
		t.Fatal("private domain error leaked")
	}
}

func TestWorkspaceStateDependentActionsAndReadDTO(t *testing.T) {
	for _, test := range []struct {
		name   string
		status contract.Status
		final  bool
		want   []string
	}{
		{"draft", contract.StatusDraft, false, []string{"contract-ledger.view", "contract-ledger.draft.manage", "contract-ledger.lifecycle.manage"}},
		{"active", contract.StatusActive, false, []string{"contract-ledger.view", "contract-ledger.lifecycle.manage"}},
		{"terminated final", contract.StatusTerminated, true, []string{"contract-ledger.view"}},
		{"cancelled", contract.StatusCancelled, false, []string{"contract-ledger.view"}},
		{"closed", contract.StatusClosed, false, []string{"contract-ledger.view"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := &contractUseCasesStub{details: contract.DraftDetails{Contract: contract.Contract{ID: referenceTestContract, Status: test.status, TotalAmount: "19.99"}}}
			if test.final {
				c.details.Settlements = []contract.ContractSettlement{{Kind: contract.SettlementKindFinal}}
			}
			h := newRawTestHandler(t, &identityUseCasesStub{principal: workspacePrincipal("contract-ledger.view", "contract-ledger.draft.manage", "contract-ledger.lifecycle.manage")}, &partyUseCasesStub{}, c, Options{})
			w := workspaceRequest(h, "GET", "/api/v1/contracts/"+referenceTestContract+"/workspace", "")
			if w.Code != 200 {
				t.Fatal(w.Body.String())
			}
			var v struct {
				AllowedActions []string          `json:"allowed_actions"`
				Contract       workspaceContract `json:"contract"`
				Parties        []any             `json:"parties"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(v.AllowedActions, test.want) || v.Contract.TotalAmount != "19.99" || v.Parties == nil {
				t.Fatalf("value=%#v", v)
			}
			if strings.Contains(w.Body.String(), "AllowedActions") || strings.Contains(w.Body.String(), "Actor") {
				t.Fatal("domain serialization leaked")
			}
		})
	}
}

func TestWorkspaceReadProjectsAuthoritativePartyReferences(t *testing.T) {
	localPartyID := "11111111-1111-4111-8111-111111111112"
	directoryPartyID := "22222222-2222-4222-8222-222222222223"
	c := &contractUseCasesStub{details: contract.DraftDetails{
		Contract: contract.Contract{ID: referenceTestContract, Status: contract.StatusDraft, DraftVersion: 3},
		Parties: []contract.DraftParty{{
			ID: "33333333-3333-4333-8333-333333333333", ContractID: referenceTestContract,
			PartyID: localPartyID, PartyRevision: 1, PartyLegalName: "施工主体", Role: "施工方",
		}},
	}}
	runtime := partyRuntimeStub{values: []partymigration.RuntimePartyReference{{
		LocalPartyID: localPartyID, PartyID: directoryPartyID, Revision: 7,
		Kind: "organization", LegalName: "施工主体", Source: partymigration.RuntimeModeDirectory,
	}}}
	h := newRawTestHandler(t,
		&identityUseCasesStub{principal: workspacePrincipal("contract-ledger.view", "contract-ledger.draft.manage")},
		&partyUseCasesStub{}, c, Options{PartyRuntime: runtime},
	)
	w := workspaceRequest(h, http.MethodGet, "/api/v1/contracts/"+referenceTestContract+"/workspace", "")
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	var value struct {
		Parties []struct {
			PartyID        string `json:"party_id"`
			PartyReference struct {
				PartyID      string `json:"party_id"`
				Revision     int64  `json:"revision"`
				Authority    string `json:"authority"`
				LocalPartyID string `json:"local_party_id"`
			} `json:"party_reference"`
		} `json:"parties"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if len(value.Parties) != 1 || value.Parties[0].PartyID != localPartyID ||
		value.Parties[0].PartyReference.PartyID != directoryPartyID ||
		value.Parties[0].PartyReference.Revision != 7 ||
		value.Parties[0].PartyReference.Authority != partymigration.RuntimeModeDirectory ||
		value.Parties[0].PartyReference.LocalPartyID != localPartyID {
		t.Fatalf("Party reference projection = %#v", value.Parties)
	}
}
