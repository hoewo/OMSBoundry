package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/receiptset"
)

const (
	receiptSetID       = "91000000-0000-4000-8000-000000000001"
	receiptProject     = "91000000-0000-4000-8000-000000000002"
	receiptAllocation  = "91000000-0000-4000-8000-000000000003"
	receiptTransaction = "91000000-0000-4000-8000-000000000004"
)

type receiptSetStub struct {
	input  receiptset.ConfirmInput
	result receiptset.ConfirmResult
	get    receiptset.Set
	err    error
}

func (s *receiptSetStub) Confirm(_ context.Context, input receiptset.ConfirmInput) (receiptset.ConfirmResult, error) {
	s.input = input
	return s.result, s.err
}
func (s *receiptSetStub) Get(context.Context, string, string) (receiptset.Set, error) {
	return s.get, s.err
}

func receiptSetHandler(t *testing.T, principal identity.Principal, sets ReceiptSetUseCases) http.Handler {
	t.Helper()
	handler, err := New(readinessStub{}, &identityStub{found: true, principal: principal}, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{ReceiptSets: sets}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func TestReceiptSetConfirmationRequiresFormalTenantCapability(t *testing.T) {
	sets := &receiptSetStub{result: receiptset.ConfirmResult{Set: receiptset.Set{ID: receiptSetID, TenantID: referenceTestTenant, ProjectID: receiptProject, Status: receiptset.StatusConfirmed, Currency: receiptset.CurrencyCNY, Revision: 1, TotalAmountMinor: 106500}}}
	principal := identity.Principal{UserID: "91000000-0000-4000-8000-000000000005", DisplayName: "资金负责人", Provider: "iam", TenantID: referenceTestTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{"money-flow-ledger.receipt-set.confirm": true}}
	handler := receiptSetHandler(t, principal, sets)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/receipt-sets", strings.NewReader(`{"command_id":"91000000-0000-4000-8000-000000000006","project_id":"`+receiptProject+`","allocation_ids":["`+receiptAllocation+`"]}`))
	request.Header.Set("Authorization", "Bearer signed-confirm-credential")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || sets.input.TenantID != referenceTestTenant || sets.input.ProjectID != receiptProject || sets.input.Actor.ID != principal.UserID {
		t.Fatalf("status=%d input=%#v body=%s", response.Code, sets.input, response.Body.String())
	}

	adjacent := principal
	adjacent.AllowedActions = map[string]bool{"money-flow-ledger.view": true}
	response = httptest.NewRecorder()
	handler = receiptSetHandler(t, adjacent, sets)
	handler.ServeHTTP(response, request.Clone(context.Background()))
	if response.Code != http.StatusForbidden {
		t.Fatalf("adjacent action status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestReceiptSetReferenceRequiresExactActionAndResource(t *testing.T) {
	value := receiptset.Set{ID: receiptSetID, TenantID: referenceTestTenant, ProjectID: receiptProject, Status: receiptset.StatusConfirmed, Currency: receiptset.CurrencyCNY, Revision: 1, TotalAmountMinor: 106500, Items: []receiptset.Item{{TransactionID: receiptTransaction, TransactionRevision: 1, AllocationID: receiptAllocation, AllocationRevision: 1, AmountMinor: 106500, Currency: receiptset.CurrencyCNY, Status: receiptset.StatusConfirmed}}}
	sets := &receiptSetStub{get: value}
	exact := identity.Principal{UserID: "91000000-0000-4000-8000-000000000005", DisplayName: "Bonus Distribution", Provider: "iam", TenantID: referenceTestTenant, ScopeKind: "resources", AllowedActions: map[string]bool{"money-flow-ledger.receipt-set.reference.read": true}, Resources: map[string]map[string]bool{"receipt_set": {receiptSetID: true}}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/references/receipt-sets/"+receiptSetID, nil)
	request.Header.Set("Authorization", "Bearer signed-reference-credential")
	response := httptest.NewRecorder()
	receiptSetHandler(t, exact, sets).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["schema_version"] != "1" || body["source_application"] != serviceName || body["resource_type"] != "receipt-set" || body["id"] != receiptSetID {
		t.Fatalf("body=%#v", body)
	}

	for _, adjacent := range []identity.Principal{
		{UserID: exact.UserID, Provider: "iam", TenantID: referenceTestTenant, ScopeKind: "resources", AllowedActions: map[string]bool{"money-flow-ledger.reference.read": true}, Resources: map[string]map[string]bool{"receipt_set": {receiptSetID: true}}},
		{UserID: exact.UserID, Provider: "iam", TenantID: referenceTestTenant, ScopeKind: "resources", AllowedActions: map[string]bool{"money-flow-ledger.receipt-set.reference.read": true}, Resources: map[string]map[string]bool{"receipt_set": {referenceTestOther: true}}},
	} {
		response = httptest.NewRecorder()
		receiptSetHandler(t, adjacent, sets).ServeHTTP(response, request.Clone(context.Background()))
		if response.Code != http.StatusForbidden {
			t.Fatalf("adjacent credential status=%d body=%s", response.Code, response.Body.String())
		}
	}
}
