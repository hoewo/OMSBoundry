package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/payrollpayment"
)

const payrollCommandID = "92000000-0000-4000-8000-000000000001"

type payrollPaymentStub struct {
	actor audit.ActorRef
	input payrollpayment.Request
}

func (s *payrollPaymentStub) Record(_ context.Context, actor audit.ActorRef, input payrollpayment.Request) (payrollpayment.Result, error) {
	s.actor, s.input = actor, input
	return payrollpayment.Result{Status: "succeeded", MoneyPaymentID: "92000000-0000-4000-8000-000000000002", MoneyPaymentRevision: 1, MoneyAllocationID: "92000000-0000-4000-8000-000000000003", MoneyAllocationRevision: 1, MoneyObligationID: "92000000-0000-4000-8000-000000000004", MoneyObligationRevision: 1, AllocatedAmountMinor: 10000}, nil
}

func payrollPaymentHandler(t *testing.T, principal identity.Principal, payments PayrollPaymentUseCases) http.Handler {
	t.Helper()
	handler, err := New(readinessStub{}, &identityStub{found: true, principal: principal}, &partyStub{}, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{PayrollPayments: payments}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func TestPayrollPaymentRequiresExactCapabilityResourceAndTenant(t *testing.T) {
	stub := &payrollPaymentStub{}
	exact := identity.Principal{UserID: "92000000-0000-4000-8000-000000000005", DisplayName: "Payroll", Provider: "iam", TenantID: referenceTestTenant, ScopeKind: "resources", AllowedActions: map[string]bool{"money-flow-ledger.payroll-payment.record": true}, Resources: map[string]map[string]bool{"payroll_payment": {payrollCommandID: true}}}
	body := `{"schema_version":"1","command_id":"` + payrollCommandID + `","tenant_id":"` + referenceTestTenant + `","payroll_run_id":"92000000-0000-4000-8000-000000000006","payroll_period":"2026-10","payroll_label":"十月工资","payroll_line_id":"92000000-0000-4000-8000-000000000007","payroll_line_revision":2,"payroll_line_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","person_id":"92000000-0000-4000-8000-000000000008","person_revision":3,"person_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","person_display_name":"成员甲","total_net_payable_minor":10000,"actual_amount_minor":10000,"occurred_on":"2026-10-31","note":"工资实付","payment_profile_id":"92000000-0000-4000-8000-000000000009","payment_profile_revision":1,"payment_bank_name":"测试银行","masked_payment_account":"****1234"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/payroll-payments", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer exact-workload-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	payrollPaymentHandler(t, exact, stub).ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.input.CommandID != payrollCommandID || stub.actor.ID != exact.UserID {
		t.Fatalf("status=%d input=%#v actor=%#v body=%s", response.Code, stub.input, stub.actor, response.Body.String())
	}

	for _, adjacent := range []identity.Principal{
		{UserID: exact.UserID, Provider: "iam", TenantID: referenceTestTenant, ScopeKind: "resources", AllowedActions: map[string]bool{"money-flow-ledger.cash.record": true}, Resources: exact.Resources},
		{UserID: exact.UserID, Provider: "iam", TenantID: referenceTestTenant, ScopeKind: "resources", AllowedActions: exact.AllowedActions, Resources: map[string]map[string]bool{"payroll_payment": {"92000000-0000-4000-8000-000000000099": true}}},
		{UserID: exact.UserID, Provider: "iam", TenantID: "92000000-0000-4000-8000-000000000099", ScopeKind: "resources", AllowedActions: exact.AllowedActions, Resources: exact.Resources},
	} {
		response = httptest.NewRecorder()
		adjacentRequest := httptest.NewRequest(http.MethodPost, "/api/v1/payroll-payments", strings.NewReader(body))
		adjacentRequest.Header.Set("Authorization", "Bearer adjacent-workload-token")
		adjacentRequest.Header.Set("Content-Type", "application/json")
		payrollPaymentHandler(t, adjacent, &payrollPaymentStub{}).ServeHTTP(response, adjacentRequest.WithContext(context.Background()))
		if response.Code != http.StatusForbidden {
			t.Fatalf("adjacent principal status=%d body=%s", response.Code, response.Body.String())
		}
	}
}
