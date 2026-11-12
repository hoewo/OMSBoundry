package personreference

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSnapshotBuildsMultipleEmploymentIntervals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer static-token-long-enough" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/references/persons/77777777-7777-4777-8777-777777777777":
			_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"people-groups","id":"77777777-7777-4777-8777-777777777777","tenant_id":"55555555-5555-4555-8555-555555555555","resource_type":"person","display_name":"张工程师","status":"active","revision":4}`))
		case "/api/v1/persons/77777777-7777-4777-8777-777777777777/employment-history":
			_, _ = w.Write([]byte(`[{"sequence":1,"person_revision":1,"event_kind":"hired","effective_date":"2026-01-10"},{"sequence":2,"person_revision":2,"event_kind":"offboarded","effective_date":"2026-03-01"},{"sequence":3,"person_revision":4,"event_kind":"rehired","effective_date":"2026-05-16"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "static-token-long-enough", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.Snapshot(context.Background(), "55555555-5555-4555-8555-555555555555", "77777777-7777-4777-8777-777777777777")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.DisplayName != "张工程师" || snapshot.SourceRevision != 4 || len(snapshot.Intervals) != 2 || snapshot.Intervals[0].Through != "2026-02-28" || snapshot.Intervals[1].From != "2026-05-16" || snapshot.Intervals[1].Through != "" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}
