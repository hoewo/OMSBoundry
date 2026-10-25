package moneypayment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"omsbizfoundry/payroll/internal/payroll"
)

func TestDeliverUsesExactMoneyContractWithoutFullAccountNumber(t *testing.T) {
	intent := paymentIntent()
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/payroll-payments" || r.Header.Get("Authorization") != "Bearer static-money-token" {
			t.Fatalf("unexpected request %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "succeeded", "money_payment_id": "70000000-0000-4000-8000-000000000001", "money_payment_revision": 1,
			"money_allocation_id": "70000000-0000-4000-8000-000000000002", "money_allocation_revision": 1,
			"money_obligation_id": "70000000-0000-4000-8000-000000000003", "money_obligation_revision": 2,
			"allocated_amount_minor": 50000, "overage_amount_minor": 0,
		})
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "static-money-token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Deliver(context.Background(), intent)
	if err != nil || result.Status != "succeeded" || result.AllocatedAmountMinor != intent.AmountMinor || result.MoneyAllocationRevision != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if captured["schema_version"] != "1" || captured["command_id"] != intent.CommandID || captured["person_digest"] != intent.PersonDigest || captured["masked_payment_account"] != intent.MaskedAccount {
		t.Fatalf("unexpected payload %#v", captured)
	}
	if _, found := captured["account_name"]; found {
		t.Fatal("Money payment request must not expose the payroll account holder name")
	}
	if _, found := captured["payment_account_number"]; found {
		t.Fatal("Money payment request must not expose the full account number")
	}
}

func TestDeliverRefreshesExactWorkloadCredentialOnce(t *testing.T) {
	credentials := &credentialSpy{tokens: []string{"expired", "fresh"}}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fresh" {
			t.Fatalf("unexpected refreshed token %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "succeeded", "money_payment_id": "70000000-0000-4000-8000-000000000001", "money_payment_revision": 1,
			"money_allocation_id": "70000000-0000-4000-8000-000000000002", "money_allocation_revision": 1,
			"allocated_amount_minor": 50000, "overage_amount_minor": 0,
		})
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "", credentials, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Deliver(context.Background(), paymentIntent()); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || credentials.invalidations != 1 || len(credentials.calls) != 2 {
		t.Fatalf("requests=%d calls=%d invalidations=%d", requests, len(credentials.calls), credentials.invalidations)
	}
	for _, call := range credentials.calls {
		if call != "11111111-1111-4111-8111-111111111111|money-flow-ledger|money-flow-ledger.payroll-payment.record|payroll_payment|22222222-2222-4222-8222-222222222222" {
			t.Fatalf("unexpected credential scope %q", call)
		}
	}
}

func TestDeliverClassifiesDeterministicAndAmbiguousFailures(t *testing.T) {
	for _, test := range []struct {
		name        string
		status      int
		wantStatus  string
		wantCode    string
		wantUnknown bool
	}{
		{name: "validation", status: http.StatusUnprocessableEntity, wantStatus: "failed", wantCode: "MONEY_PAYMENT_REJECTED"},
		{name: "conflict", status: http.StatusConflict, wantStatus: "failed", wantCode: "MONEY_COMMAND_CONFLICT"},
		{name: "rate limit", status: http.StatusTooManyRequests, wantUnknown: true},
		{name: "server", status: http.StatusServiceUnavailable, wantUnknown: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(test.status) }))
			defer server.Close()
			client, err := NewHTTPClient(server.URL, "static-money-token", nil, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Deliver(context.Background(), paymentIntent())
			if test.wantUnknown {
				if err != payroll.ErrPaymentUnknown {
					t.Fatalf("expected unknown result, got %#v %v", result, err)
				}
				return
			}
			if err != nil || result.Status != test.wantStatus || result.FailureCode != test.wantCode {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
}

func TestDeliverRejectsMalformedSuccessAsUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "succeeded", "money_payment_id": "70000000-0000-4000-8000-000000000001", "money_payment_revision": 1,
			"allocated_amount_minor": 49999, "overage_amount_minor": 0,
		})
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "static-money-token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := client.Deliver(context.Background(), paymentIntent()); err != payroll.ErrPaymentUnknown {
		t.Fatalf("malformed success must remain unknown: %#v %v", result, err)
	}
}

func paymentIntent() payroll.PaymentIntent {
	return payroll.PaymentIntent{
		CommandID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111",
		PayrollRunID: "33333333-3333-4333-8333-333333333333", PayrollPeriod: "2026-10", PayrollLabel: "十月工资",
		PayrollLineID: "44444444-4444-4444-8444-444444444444", PayrollLineRevision: 3, PayrollLineDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PersonID: "55555555-5555-4555-8555-555555555555", PersonRevision: 4, PersonDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", PersonDisplayName: "成员甲",
		PaymentProfileID: "66666666-6666-4666-8666-666666666666", PaymentProfileRevision: 2, TotalNetPayableMinor: 100000, AmountMinor: 50000,
		OccurredOn: "2026-10-31", Note: "首笔", AccountName: "不应传输的账户名", BankName: "测试银行", MaskedAccount: "****1234",
	}
}

type credentialSpy struct {
	mu            sync.Mutex
	tokens        []string
	calls         []string
	invalidations int
}

func (s *credentialSpy) CredentialForTenant(_ context.Context, tenant, targetAudience, targetAction, targetType, targetID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, stringsJoin(tenant, targetAudience, targetAction, targetType, targetID))
	token := s.tokens[0]
	s.tokens = s.tokens[1:]
	return token, nil
}

func (s *credentialSpy) InvalidateForTenant(_, _, _, _, _ string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidations++
}

func stringsJoin(values ...string) string {
	result := ""
	for index, value := range values {
		if index > 0 {
			result += "|"
		}
		result += value
	}
	return result
}
