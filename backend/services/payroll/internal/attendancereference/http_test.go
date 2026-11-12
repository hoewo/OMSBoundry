package attendancereference

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetMonthlySummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer attendance-token-long-enough" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"schema_version":"attendance.monthly-summary/v1","id":"77777777-7777-4777-8777-777777777777","tenant_id":"55555555-5555-4555-8555-555555555555","person_id":"66666666-6666-4666-8666-666666666666","person_display_name":"张工程师","person_source_revision":2,"person_source_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","period":"2026-11","policy_key":"beijing-calendar","policy_revision":1,"policy_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","basis":"calendar_days","standard_units":300000,"employment_eligible_units":300000,"unpaid_leave_deduction_units":10000,"absence_deduction_units":0,"payable_units":290000,"source_revision":1}`))
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "attendance-token-long-enough", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := client.GetMonthlySummary(context.Background(), "55555555-5555-4555-8555-555555555555", "77777777-7777-4777-8777-777777777777")
	if err != nil || summary.PayableUnits != 290000 {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
}
