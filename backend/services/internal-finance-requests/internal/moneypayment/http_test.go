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
	input := finance.MoneyPaymentRequest{SchemaVersion: "1", CommandID: "11111111-1111-4111-8111-111111111111"}
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
			client, err := NewHTTPClient(server.URL, "test-money-token-1234567890", time.Second)
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
