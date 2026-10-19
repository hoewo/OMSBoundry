package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/identity"
)

const (
	referenceTestTenant     = "22222222-2222-4222-8222-222222222222"
	referenceTestContract   = "33333333-3333-4333-8333-333333333333"
	referenceTestSettlement = "44444444-4444-4444-8444-444444444444"
	referenceTestOther      = "55555555-5555-4555-8555-555555555555"
	referenceTestToken      = "local-reference-token-1234567890"
)

func TestContractReferenceUsesTenantScopedTemplateCredential(t *testing.T) {
	contracts := &contractUseCasesStub{details: contract.DraftDetails{Contract: contract.Contract{
		ID: referenceTestContract, Title: "道路设计合同", Status: contract.StatusActive, CurrentRevision: 4,
		Category: "design_fee", Currency: "CNY", TotalAmount: "128000.00",
	},
		Parties: []contract.DraftParty{{PartyID: referenceTestContract}},
		PartySnapshots: []contract.ContractPartySnapshot{
			{Revision: 3, SourcePartyID: referenceTestContract},
			{Revision: 4, SourcePartyID: referenceTestOther},
		},
	}}
	handler := newRawTestHandler(t, &identityUseCasesStub{}, &partyUseCasesStub{}, contracts, Options{
		ReferenceTenantID: referenceTestTenant, ReferenceAccessToken: referenceTestToken,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/references/contracts/"+referenceTestContract, nil)
	request.Header.Set("Authorization", "Bearer "+referenceTestToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
	var value contractReferenceView
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "contract-ledger" || value.ID != referenceTestContract || value.TenantID != referenceTestTenant || value.ResourceType != "contract" || value.DisplaySummary != "道路设计合同" || value.Status != "active" || value.Revision != 4 || value.Category != "design_fee" || value.Currency != "CNY" || value.CurrentEffectiveAmount == nil || *value.CurrentEffectiveAmount != "128000.00" || len(value.PartyIDs) != 1 || value.PartyIDs[0] != referenceTestOther {
		t.Fatalf("reference = %#v", value)
	}
}

func TestContractReferenceUsesActiveDraftPartiesBeforeFirstRevision(t *testing.T) {
	contracts := &contractUseCasesStub{details: contract.DraftDetails{Contract: contract.Contract{
		ID: referenceTestContract, Title: "合同草稿", Status: contract.StatusDraft, DraftVersion: 2,
		Currency: "CNY", TotalAmount: "1.00",
	}, Parties: []contract.DraftParty{
		{PartyID: referenceTestOther},
		{PartyID: referenceTestContract, RemovedAt: time.Now().UTC()},
	}}}
	handler := newRawTestHandler(t, &identityUseCasesStub{}, &partyUseCasesStub{}, contracts, Options{
		ReferenceTenantID: referenceTestTenant, ReferenceAccessToken: referenceTestToken,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/references/contracts/"+referenceTestContract, nil)
	request.Header.Set("Authorization", "Bearer "+referenceTestToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
	var value contractReferenceView
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	if len(value.PartyIDs) != 1 || value.PartyIDs[0] != referenceTestOther {
		t.Fatalf("draft party references = %#v", value.PartyIDs)
	}
}

func TestContractReferenceRejectsInvalidCredentialAndAcceptsIAMCredential(t *testing.T) {
	contracts := &contractUseCasesStub{details: contract.DraftDetails{Contract: contract.Contract{
		ID: referenceTestContract, Status: contract.StatusActive, CurrentRevision: 1,
	}}}
	t.Run("invalid template credential", func(t *testing.T) {
		handler := newRawTestHandler(t, &identityUseCasesStub{}, &partyUseCasesStub{}, contracts, Options{
			ReferenceTenantID: referenceTestTenant, ReferenceAccessToken: referenceTestToken,
		})
		request := httptest.NewRequest(http.MethodGet, "/api/v1/references/contracts/"+referenceTestContract, nil)
		request.Header.Set("Authorization", "Bearer wrong-token")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", response.Code)
		}
	})

	t.Run("IAM exact-resource credential", func(t *testing.T) {
		principal := identity.Principal{
			UserID: "iam-subject", Provider: "iam", TenantID: referenceTestTenant,
			AllowedActions: map[string]bool{"contract-ledger.reference.read": true}, ScopeKind: "resources",
			Resources: map[string]map[string]bool{"contract": {referenceTestContract: true}},
		}
		handler := newRawTestHandler(t, &identityUseCasesStub{principal: principal}, &partyUseCasesStub{}, contracts, Options{})
		request := httptest.NewRequest(http.MethodGet, "/api/v1/references/contracts/"+referenceTestContract, nil)
		request.Header.Set("Authorization", "Bearer test-session")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
		}
	})

	for _, test := range []struct {
		name      string
		principal identity.Principal
	}{
		{"adjacent resource", identity.Principal{UserID: "iam-subject", Provider: "iam", TenantID: referenceTestTenant, AllowedActions: map[string]bool{"contract-ledger.reference.read": true}, ScopeKind: "resources", Resources: map[string]map[string]bool{"contract": {referenceTestOther: true}}}},
		{"wrong resource type", identity.Principal{UserID: "iam-subject", Provider: "iam", TenantID: referenceTestTenant, AllowedActions: map[string]bool{"contract-ledger.reference.read": true}, ScopeKind: "resources", Resources: map[string]map[string]bool{"settlement": {referenceTestContract: true}}}},
		{"tenant view is not workload reference", identity.Principal{UserID: "iam-subject", Provider: "iam", TenantID: referenceTestTenant, AllowedActions: map[string]bool{"contract-ledger.view": true}, ScopeKind: "tenant", Resources: map[string]map[string]bool{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := newRawTestHandler(t, &identityUseCasesStub{principal: test.principal}, &partyUseCasesStub{}, contracts, Options{})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/references/contracts/"+referenceTestContract, nil)
			request.Header.Set("Authorization", "Bearer test-session")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
			}
		})
	}
}

func TestSettlementReferenceUsesExactResourceAndStableSummary(t *testing.T) {
	contracts := &contractUseCasesStub{settlementReference: contract.ContractSettlement{
		ID: referenceTestSettlement, ContractID: referenceTestContract, Title: "最终结算书", Sequence: 2,
		Amount: "120000.50", Currency: "CNY", ConfirmingParties: []contract.ContractSettlementPartySnapshot{{SourcePartyID: referenceTestOther}},
	}, details: contract.DraftDetails{Contract: contract.Contract{ID: referenceTestContract, Category: "design_fee"}}}
	principal := identity.Principal{
		UserID: "iam-subject", Provider: "iam", TenantID: referenceTestTenant,
		AllowedActions: map[string]bool{"contract-ledger.reference.read": true}, ScopeKind: "resources",
		Resources: map[string]map[string]bool{"settlement": {referenceTestSettlement: true}},
	}
	handler := newRawTestHandler(t, &identityUseCasesStub{principal: principal}, &partyUseCasesStub{}, contracts, Options{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/references/settlements/"+referenceTestSettlement, nil)
	request.Header.Set("Authorization", "Bearer test-session")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
	var value contractReferenceView
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	if value.ID != referenceTestSettlement || value.ResourceType != "settlement" || value.DisplaySummary != "最终结算书" || value.Status != "recorded" || value.Revision != 1 || value.ContractID != referenceTestContract || value.Category != "design_fee" || value.Currency != "CNY" || value.CurrentEffectiveAmount == nil || *value.CurrentEffectiveAmount != "120000.50" || len(value.PartyIDs) != 1 || value.PartyIDs[0] != referenceTestOther {
		t.Fatalf("reference = %#v", value)
	}
}

func TestContractServerRejectsPartialReferenceCredential(t *testing.T) {
	_, err := NewServer(
		readinessStub{}, &partyUseCasesStub{}, &contractUseCasesStub{}, &evidenceUseCasesStub{},
		&ledgerExportUseCasesStub{}, &ledgerRestoreUseCasesStub{}, &identityUseCasesStub{}, &auditUseCasesStub{},
		Options{ReferenceTenantID: referenceTestTenant},
	)
	if err == nil {
		t.Fatal("partial reference credential accepted")
	}
}
