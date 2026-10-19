package references

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"omsbizfoundry/project-workspace/internal/projects"
	"strings"
	"testing"
	"time"
)

var recordedMaterialTypes = []struct{ service, kind, path, action string }{
	{"approval-audit-results", "approval_audit_result", "approval-audit-results", "approval-audit-result.read"},
	{"cost-outsourcing", "cost_entry", "cost-entries", "cost-entry.read"},
	{"cost-outsourcing", "outsourcing_engagement", "outsourcing-engagements", "outsourcing-engagement.read"},
}

func recordedBody(service, kind, items string) string {
	digest := sha256.Sum256([]byte(items))
	return fmt.Sprintf(`{"schema_version":"1","source_application":%q,"id":%q,"tenant_id":%q,"project_id":%q,"resource_type":%q,"result_kind":"approval","status":"recorded","revision":5,"digest":%q,"items":%s}`, service, resourceID, tenantID, productionMaterialProject, kind, hex.EncodeToString(digest[:]), items)
}
func recordedItem(slot, file string, version int) string {
	return fmt.Sprintf(`{"relation_key":%q,"file_id":%q,"version":%d,"evidence_source_revision":%d,"display_name":"资料.pdf"}`, slot, file, version, version)
}
func TestRecordedMaterialConsumerClosedTypesExactActionsAndSlotTuples(t *testing.T) {
	for _, typ := range recordedMaterialTypes {
		t.Run(typ.kind, func(t *testing.T) {
			for _, items := range []string{"[]", "[" + recordedItem("发票", productionMaterialFile, 2) + "]", "[" + recordedItem("a", productionMaterialFile, 2) + "," + recordedItem("a", productionMaterialFile, 3) + "]", "[" + recordedItem("a", productionMaterialFile, 2) + "," + recordedItem("a/0", productionMaterialFile, 3) + "]"} {
				credentials := &credentialSpy{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/api/v1/references/"+typ.path+"/"+resourceID+"/materials" || r.Header.Get("Authorization") != "Bearer dynamic-token" {
						t.Errorf("wrong exact request %s", r.URL.Path)
						w.WriteHeader(400)
						return
					}
					fmt.Fprint(w, recordedBody(typ.service, typ.kind, items))
				}))
				validator, err := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{typ.service: {BaseURL: server.URL, Audience: typ.service, Action: "cost-entry.read", Credentials: credentials}})
				if typ.service == "approval-audit-results" {
					validator.resources[typ.service] = Endpoint{BaseURL: server.URL, Audience: typ.service, Action: typ.action, Credentials: credentials}
				}
				if err != nil {
					t.Fatal(err)
				}
				value, err := validator.ResolveResource(context.Background(), tenantID, typ.service, typ.kind, resourceID)
				server.Close()
				if err != nil || value.MaterialSource == nil || value.MaterialSource.ProjectID != productionMaterialProject || value.Status != "recorded" {
					t.Fatalf("source=%#v err=%v", value, err)
				}
				if credentials.audience != typ.service || credentials.action != typ.action || credentials.resourceType != typ.kind || credentials.resourceID != resourceID {
					t.Fatalf("expanded credential=%#v", credentials)
				}
			}
		})
	}
}
func TestRecordedMaterialConsumerRejectsMalformedSources(t *testing.T) {
	for _, typ := range recordedMaterialTypes {
		t.Run(typ.kind, func(t *testing.T) {
			valid := recordedBody(typ.service, typ.kind, "["+recordedItem("发票", productionMaterialFile, 2)+"]")
			cases := map[string]string{
				"tenant":          strings.Replace(valid, tenantID, productionMaterialProject, 1),
				"id":              strings.Replace(valid, resourceID, productionMaterialProject, 1),
				"source":          strings.Replace(valid, `"source_application":`+fmt.Sprintf("%q", typ.service), `"source_application":"other"`, 1),
				"type":            strings.Replace(valid, `"resource_type":`+fmt.Sprintf("%q", typ.kind), `"resource_type":"other"`, 1),
				"schema":          strings.Replace(valid, `"schema_version":"1"`, `"schema_version":"2"`, 1),
				"project":         strings.Replace(valid, productionMaterialProject, "bad", 1),
				"revision":        strings.Replace(valid, `"revision":5`, `"revision":0`, 1),
				"status":          strings.Replace(valid, `"status":"recorded"`, `"status":"unknown"`, 1),
				"digest":          strings.Replace(valid, `"version":2`, `"version":3`, 1),
				"null":            recordedBody(typ.service, typ.kind, "null"),
				"absent":          strings.Replace(recordedBody(typ.service, typ.kind, "[]"), `,"items":[]`, "", 1),
				"file":            recordedBody(typ.service, typ.kind, "["+recordedItem("发票", "bad", 2)+"]"),
				"zero":            recordedBody(typ.service, typ.kind, "["+recordedItem("发票", productionMaterialFile, 0)+"]"),
				"slot whitespace": recordedBody(typ.service, typ.kind, "["+recordedItem(" 发票 ", productionMaterialFile, 2)+"]"),
				"slot long":       recordedBody(typ.service, typ.kind, "["+recordedItem(strings.Repeat("票", 101), productionMaterialFile, 2)+"]"),
				"duplicate":       recordedBody(typ.service, typ.kind, "["+recordedItem("票", productionMaterialFile, 2)+","+recordedItem("票", productionMaterialFile, 2)+"]"),
				"unsorted":        recordedBody(typ.service, typ.kind, "["+recordedItem("z", productionMaterialFile, 2)+","+recordedItem("a", productionMaterialFile, 2)+"]"),
				"frozen revision": recordedBody(typ.service, typ.kind, "["+strings.Replace(recordedItem("票", productionMaterialFile, 2), `"evidence_source_revision":2`, `"evidence_source_revision":3`, 1)+"]"),
				"name whitespace": recordedBody(typ.service, typ.kind, "["+strings.Replace(recordedItem("票", productionMaterialFile, 2), "资料.pdf", " 资料.pdf ", 1)+"]"),
				"name long":       recordedBody(typ.service, typ.kind, "["+strings.Replace(recordedItem("票", productionMaterialFile, 2), "资料.pdf", strings.Repeat("名", 501), 1)+"]"),
			}
			if typ.service == "approval-audit-results" {
				cases["result kind"] = strings.Replace(valid, `"result_kind":"approval"`, `"result_kind":"other"`, 1)
			}
			for name, body := range cases {
				t.Run(name, func(t *testing.T) {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) }))
					defer server.Close()
					validator, _ := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{typ.service: {BaseURL: server.URL, AccessToken: "test-provider-token"}})
					if _, err := validator.ResolveResource(context.Background(), tenantID, typ.service, typ.kind, resourceID); !errors.Is(err, projects.ErrReferenceInvalid) {
						t.Fatalf("malformed source accepted: %v", err)
					}
				})
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, strings.Replace(valid, `"status":"recorded"`, `"status":"withdrawn"`, 1))
			}))
			defer server.Close()
			validator, _ := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{typ.service: {BaseURL: server.URL, AccessToken: "test-provider-token"}})
			value, err := validator.ResolveResource(context.Background(), tenantID, typ.service, typ.kind, resourceID)
			if err != nil || value.Status != "withdrawn" || value.MaterialSource == nil {
				t.Fatalf("withdrawn explanation lost: %#v %v", value, err)
			}
		})
	}
}
