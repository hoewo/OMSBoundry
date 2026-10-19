package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/partymigration"
)

const (
	referenceTestTenant      = "22222222-2222-4222-8222-222222222222"
	referenceTestObligation  = "44444444-4444-4444-8444-444444444444"
	referenceTestTransaction = "55555555-5555-4555-8555-555555555555"
	referenceTestOther       = "66666666-6666-4666-8666-666666666666"
	referenceTestToken       = "local-reference-token-1234567890"
)

func newReferenceHandler(t *testing.T, identities IdentityUseCases, obligations ObligationUseCases, transactions CashTransactionUseCases, options Options) http.Handler {
	t.Helper()
	handler, err := New(
		readinessStub{}, identities, &partyStub{}, obligations, transactions, &allocationStub{},
		&operatingSummaryStub{}, &ledgerExportStub{}, options, &internalTransferStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func TestMoneyObligationReferenceLabelsCreditorAndDebtorSnapshots(t *testing.T) {
	creditorLocal := "11111111-1111-4111-8111-111111111111"
	debtorLocal := "22222222-2222-4222-8222-222222222223"
	obligations := &obligationStub{getFound: true, detailsResult: obligation.Details{Obligation: obligation.Obligation{
		ID: referenceTestObligation, Title: "应付义务", Status: obligation.StatusOpen, CurrentRevision: 2,
		Direction: obligation.DirectionPayable, Currency: "CNY", EffectiveAmount: "100.00", AllocatedAmount: "20.00", RemittedAmount: "0.00", OutstandingAmount: "80.00",
		Creditor: obligation.PartySnapshot{PartyID: creditorLocal}, Debtor: obligation.PartySnapshot{PartyID: debtorLocal},
	}}}
	handler := newReferenceHandler(t, &identityStub{}, obligations, &cashTransactionStub{}, Options{
		ReferenceTenantID: referenceTestTenant, ReferenceAccessToken: referenceTestToken,
		PartyRuntime: partyRuntimeStub{values: []partymigration.RuntimePartyReference{
			{PartyID: "33333333-3333-4333-8333-333333333333", Revision: 3, Kind: "organization", LegalName: "施工方", Source: partymigration.RuntimeModeDirectory},
			{PartyID: "44444444-4444-4444-8444-444444444445", Revision: 4, Kind: "organization", LegalName: "资质单位", Source: partymigration.RuntimeModeDirectory},
		}},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/references/obligations/"+referenceTestObligation, nil)
	request.Header.Set("Authorization", "Bearer "+referenceTestToken)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var value resourceReferenceView
	if response.Code != http.StatusOK || json.NewDecoder(response.Body).Decode(&value) != nil || len(value.PartyReferences) != 2 || value.PartyReferences[0].Role != "creditor" || value.PartyReferences[1].Role != "debtor" {
		t.Fatalf("party roles missing status=%d view=%#v", response.Code, value.PartyReferences)
	}
}

func TestMoneyReferencesUseTenantScopedTemplateCredential(t *testing.T) {
	obligations := &obligationStub{getFound: true, detailsResult: obligation.Details{Obligation: obligation.Obligation{
		ID: referenceTestObligation, Title: "设计费应收义务", Status: obligation.StatusOpen, CurrentRevision: 3,
		Direction: obligation.DirectionReceivable, Currency: "CNY", EffectiveAmount: "1000.00",
		AllocatedAmount: "400.00", RemittedAmount: "100.00", OutstandingAmount: "500.00", BalanceSequence: 7,
	}}}
	transactions := &cashTransactionStub{getFound: true, getResult: cashtransaction.Transaction{
		ID: referenceTestTransaction, Description: "首期回款", Status: "recorded",
		Type: cashtransaction.TypeReceipt, Currency: "CNY", EffectiveAmount: "400.00",
		AllocatedAmount: "300.00", UnallocatedAmount: "100.00", OccurredOn: "2026-09-27",
	}}
	handler := newReferenceHandler(t, &identityStub{}, obligations, transactions, Options{
		ReferenceTenantID: referenceTestTenant, ReferenceAccessToken: referenceTestToken,
	})

	tests := []struct {
		path, id, resourceType, summary, status string
		revision                                int64
	}{
		{"/api/v1/references/obligations/" + referenceTestObligation, referenceTestObligation, "obligation", "设计费应收义务", obligation.StatusOpen, 3},
		{"/api/v1/references/transactions/" + referenceTestTransaction, referenceTestTransaction, "transaction", "首期回款", "recorded", 1},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.Header.Set("Authorization", "Bearer "+referenceTestToken)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%q", test.path, response.Code, response.Body.String())
		}
		var value resourceReferenceView
		if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
			t.Fatal(err)
		}
		if value.ID != test.id || value.TenantID != referenceTestTenant || value.ResourceType != test.resourceType || value.DisplaySummary != test.summary || value.Status != test.status || value.Revision != test.revision {
			t.Fatalf("%s reference = %#v", test.path, value)
		}
		if value.SchemaVersion != "1" || value.SourceApplication != "money-flow-ledger" || value.Currency != "CNY" || value.EffectiveAmount == "" || value.AllocatedAmount == "" {
			t.Fatalf("%s commercial reference = %#v", test.path, value)
		}
	}
}

func TestMoneyReferenceRejectsInvalidCredentialAndMissingResource(t *testing.T) {
	handler := newReferenceHandler(t, &identityStub{}, &obligationStub{}, &cashTransactionStub{}, Options{
		ReferenceTenantID: referenceTestTenant, ReferenceAccessToken: referenceTestToken,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/references/obligations/"+referenceTestObligation, nil)
	request.Header.Set("Authorization", "Bearer wrong-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid credential status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/references/obligations/"+referenceTestObligation, nil)
	request.Header.Set("Authorization", "Bearer "+referenceTestToken)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing resource status = %d body=%q", response.Code, response.Body.String())
	}
}

func TestMoneyServerRejectsPartialReferenceCredential(t *testing.T) {
	_, err := New(
		readinessStub{}, &identityStub{}, &partyStub{}, &obligationStub{}, &cashTransactionStub{},
		&allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{ReferenceAccessToken: referenceTestToken},
		&internalTransferStub{},
	)
	if err == nil {
		t.Fatal("partial reference credential accepted")
	}
}

func TestMoneyReferenceAcceptsIAMAudienceCredential(t *testing.T) {
	identities := &identityStub{found: true, principal: identity.Principal{
		UserID: "iam-subject", Provider: "iam", TenantID: referenceTestTenant,
		AllowedActions: map[string]bool{"money-flow-ledger.reference.read": true}, ScopeKind: "resources",
		Resources: map[string]map[string]bool{"obligation": {referenceTestObligation: true}},
	}}
	obligations := &obligationStub{getFound: true, getResult: obligation.Obligation{
		ID: referenceTestObligation, Status: obligation.StatusOpen, CurrentRevision: 2,
	}}
	handler := newReferenceHandler(t, identities, obligations, &cashTransactionStub{}, Options{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/references/obligations/"+referenceTestObligation, nil)
	request.Header.Set("Authorization", "Bearer signed-audience-credential")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
}

func TestMoneyReferenceRejectsAdjacentTypeAndTenantViewCredentials(t *testing.T) {
	obligations := &obligationStub{getFound: true, getResult: obligation.Obligation{ID: referenceTestObligation, Title: "义务", Status: obligation.StatusOpen, CurrentRevision: 2}}
	for _, test := range []struct {
		name      string
		principal identity.Principal
	}{
		{"adjacent resource", identity.Principal{UserID: "iam-subject", Provider: "iam", TenantID: referenceTestTenant, AllowedActions: map[string]bool{"money-flow-ledger.reference.read": true}, ScopeKind: "resources", Resources: map[string]map[string]bool{"obligation": {referenceTestOther: true}}}},
		{"wrong type", identity.Principal{UserID: "iam-subject", Provider: "iam", TenantID: referenceTestTenant, AllowedActions: map[string]bool{"money-flow-ledger.reference.read": true}, ScopeKind: "resources", Resources: map[string]map[string]bool{"transaction": {referenceTestObligation: true}}}},
		{"tenant view", identity.Principal{UserID: "iam-subject", Provider: "iam", TenantID: referenceTestTenant, AllowedActions: map[string]bool{"money-flow-ledger.view": true}, ScopeKind: "tenant", Resources: map[string]map[string]bool{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := newReferenceHandler(t, &identityStub{found: true, principal: test.principal}, obligations, &cashTransactionStub{}, Options{})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/references/obligations/"+referenceTestObligation, nil)
			request.Header.Set("Authorization", "Bearer signed-audience-credential")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
			}
		})
	}
}
