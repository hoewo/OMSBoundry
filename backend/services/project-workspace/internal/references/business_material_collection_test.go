package references

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBusinessMaterialV1AlreadyAcceptsSeveralFilesForOneRelation(t *testing.T) {
	const tenant = "11111111-1111-4111-8111-111111111111"
	const project = "22222222-2222-4222-8222-222222222222"
	const resource = "33333333-3333-4333-8333-333333333333"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"business-engagement","id":"` + resource + `","tenant_id":"` + tenant + `","project_id":"` + project + `","resource_type":"business-engagement","status":"collecting_evidence","revision":7,"digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","items":[{"relation_key":"bid_document","file_id":"44444444-4444-4444-8444-444444444444","version":1,"evidence_source_revision":1,"display_name":"材料一.pdf"},{"relation_key":"bid_document","file_id":"55555555-5555-4555-8555-555555555555","version":1,"evidence_source_revision":1,"display_name":"材料二.pdf"}]}`))
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"business-engagement": {BaseURL: server.URL, AccessToken: "test"}})
	if err != nil {
		t.Fatal(err)
	}
	value, err := validator.ResolveResource(context.Background(), tenant, "business-engagement", "business-engagement", resource)
	if err != nil || value.MaterialSource == nil || len(value.MaterialSource.Items) != 2 {
		t.Fatalf("projection=%#v err=%v", value, err)
	}
}
