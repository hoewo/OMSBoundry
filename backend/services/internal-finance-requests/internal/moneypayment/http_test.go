package moneypayment

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"omsbizfoundry/internal-finance-requests/internal/finance"
)

func TestRecordClassifiesDeterministicRejectionAndUnknownOutcome(t *testing.T) {
	input := finance.MoneyPaymentRequest{SchemaVersion: "1", CommandID: "11111111-1111-4111-8111-111111111111", TenantID: "22222222-2222-4222-8222-222222222222"}
	tests := []struct {
		name string
		code int
		body string
		want error
	}{
		{name: "recorded", code: 200, body: `{"status":"recorded","money_payment_id":"22222222-2222-4222-8222-222222222222"}`},
		{name: "deterministic validation rejection", code: 422, body: `{}`, want: finance.ErrPaymentRejected},
		{name: "rate limit is unknown", code: 429, body: `{}`, want: finance.ErrPaymentUnknown},
		{name: "dependency failure is unknown", code: 503, body: `{}`, want: finance.ErrPaymentUnknown},
		{name: "malformed success is unknown", code: 200, body: `{`, want: finance.ErrPaymentUnknown},
		{name: "trailing success is unknown", code: 200, body: `{} {}`, want: finance.ErrPaymentUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/internal-finance-payments" || r.Header.Get("Authorization") != "Bearer test-money-token-1234567890" {
					t.Fatalf("unexpected request %s %q", r.URL.Path, r.Header.Get("Authorization"))
				}
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			client, err := NewHTTPClient(server.URL, "test-money-token-1234567890", nil, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			value, err := client.Record(context.Background(), input)
			if tt.want == nil {
				if err != nil || value.Status != "recorded" {
					t.Fatalf("value=%#v err=%v", value, err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

func TestRecordRefreshesExactWorkloadCredentialOnce(t *testing.T) {
	const tenantID = "22222222-2222-4222-8222-222222222222"
	const commandID = "11111111-1111-4111-8111-111111111111"
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
		_, _ = w.Write([]byte(`{"status":"recorded","money_payment_id":"33333333-3333-4333-8333-333333333333"}`))
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "", credentials, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Record(context.Background(), finance.MoneyPaymentRequest{SchemaVersion: "1", CommandID: commandID, TenantID: tenantID}); err != nil {
		t.Fatal(err)
	}
	want := tenantID + "|money-flow-ledger|money-flow-ledger.internal-finance-payment.record|internal_finance_payment|" + commandID
	if requests != 2 || credentials.invalidations != 1 || len(credentials.calls) != 2 {
		t.Fatalf("requests=%d calls=%d invalidations=%d", requests, len(credentials.calls), credentials.invalidations)
	}
	for _, call := range credentials.calls {
		if call != want {
			t.Fatalf("unexpected credential scope %q", call)
		}
	}
}

type credentialSpy struct {
	tokens        []string
	calls         []string
	invalidations int
}

func (s *credentialSpy) CredentialForTenant(_ context.Context, tenantID, audience, action, resourceType, resourceID string) (string, error) {
	s.calls = append(s.calls, tenantID+"|"+audience+"|"+action+"|"+resourceType+"|"+resourceID)
	if len(s.tokens) == 0 {
		return "", errors.New("no token")
	}
	token := s.tokens[0]
	s.tokens = s.tokens[1:]
	return token, nil
}

func (s *credentialSpy) InvalidateForTenant(_, _, _, _, _ string) { s.invalidations++ }
