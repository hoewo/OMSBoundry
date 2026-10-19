package references

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/project-workspace/internal/projects"
)

const productionMaterialProject = "44444444-4444-4444-8444-444444444444"
const productionMaterialFile = "33333333-3333-4333-8333-333333333333"

func productionMaterialBody(items string) string {
	digest := sha256.Sum256([]byte(items))
	return fmt.Sprintf(`{"schema_version":"1","source_application":"production-operations","id":%q,"tenant_id":%q,"project_id":%q,"resource_type":"production-work-item","status":"in-progress","revision":5,"digest":%q,"items":%s}`, resourceID, tenantID, productionMaterialProject, hex.EncodeToString(digest[:]), items)
}

func productionMaterialItem(relation, file string, version int) string {
	return fmt.Sprintf(`{"relation_key":%q,"file_id":%q,"version":%d,"evidence_source_revision":7,"display_name":"成果.pdf"}`, relation, file, version)
}

func TestProductionMaterialConsumerExactNativePortAndDigest(t *testing.T) {
	for _, items := range []string{"[]", "[" + productionMaterialItem("report.primary-output", productionMaterialFile, 2) + "]"} {
		credentials := &credentialSpy{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/references/production-work-items/"+resourceID+"/materials" || r.Header.Get("Authorization") != "Bearer dynamic-token" {
				t.Fatalf("wrong exact native request: path=%q", r.URL.Path)
			}
			_, _ = w.Write([]byte(productionMaterialBody(items)))
		}))
		validator, _ := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"production-operations": {BaseURL: server.URL, Audience: "production-operations", Action: "production-work-item.reference.read", Credentials: credentials}})
		value, err := validator.ResolveResource(context.Background(), tenantID, "production-operations", "production-work-item", resourceID)
		server.Close()
		if err != nil || value.MaterialSource == nil || value.MaterialSource.ProjectID != productionMaterialProject || value.SourceApplication != "production-operations" || value.Revision != 5 {
			t.Fatalf("native material reference=%#v err=%v", value, err)
		}
		if credentials.audience != "production-operations" || credentials.action != "production-work-item.reference.read" || credentials.resourceType != "production-work-item" || credentials.resourceID != resourceID {
			t.Fatalf("expanded credential request=%#v", credentials)
		}
		encoded, _ := json.Marshal(value)
		if strings.Contains(string(encoded), "object_key") || strings.Contains(string(encoded), "access_token") {
			t.Fatal("storage coordinates crossed the reference boundary")
		}
	}
}

func TestProductionMaterialConsumerRejectsWrongIdentityDigestAndMalformedArray(t *testing.T) {
	valid := productionMaterialBody("[" + productionMaterialItem("report.primary-output", productionMaterialFile, 2) + "]")
	cases := map[string]string{
		"wrong tenant":       strings.Replace(valid, tenantID, productionMaterialProject, 1),
		"wrong id":           strings.Replace(valid, resourceID, productionMaterialProject, 1),
		"wrong source":       strings.Replace(valid, `"source_application":"production-operations"`, `"source_application":"design-operations"`, 1),
		"wrong type":         strings.Replace(valid, `"resource_type":"production-work-item"`, `"resource_type":"design-work-item"`, 1),
		"wrong schema":       strings.Replace(valid, `"schema_version":"1"`, `"schema_version":"2"`, 1),
		"bad project":        strings.Replace(valid, productionMaterialProject, "not-a-project", 1),
		"bad revision":       strings.Replace(valid, `"revision":5`, `"revision":0`, 1),
		"unknown status":     strings.Replace(valid, `"status":"in-progress"`, `"status":"unknown"`, 1),
		"cancelled":          strings.Replace(valid, `"status":"in-progress"`, `"status":"cancelled"`, 1),
		"tampered items":     strings.Replace(valid, `"version":2`, `"version":3`, 1),
		"null array":         productionMaterialBody("null"),
		"bad file":           productionMaterialBody("[" + productionMaterialItem("report.primary-output", "bad-file", 2) + "]"),
		"zero version":       productionMaterialBody("[" + productionMaterialItem("report.primary-output", productionMaterialFile, 0) + "]"),
		"bad relation":       productionMaterialBody("[" + productionMaterialItem("arbitrary/path", productionMaterialFile, 2) + "]"),
		"duplicate relation": productionMaterialBody("[" + productionMaterialItem("report.primary-output", productionMaterialFile, 2) + "," + productionMaterialItem("report.primary-output", productionMaterialProject, 3) + "]"),
		"unsorted":           productionMaterialBody("[" + productionMaterialItem("scheme.primary-output", productionMaterialFile, 2) + "," + productionMaterialItem("report.primary-output", productionMaterialProject, 3) + "]"),
	}
	var missing map[string]any
	_ = json.Unmarshal([]byte(productionMaterialBody("[]")), &missing)
	delete(missing, "items")
	missingBody, _ := json.Marshal(missing)
	cases["absent array"] = string(missingBody)
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			validator, _ := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"production-operations": {BaseURL: server.URL, AccessToken: "test-token"}})
			if _, err := validator.ResolveResource(context.Background(), tenantID, "production-operations", "production-work-item", resourceID); !errors.Is(err, projects.ErrReferenceInvalid) {
				t.Fatalf("malformed source did not fail closed: %v", err)
			}
		})
	}
}

func TestProductionMaterialConsumerUnselectedAndSourceErrorsStayDistinct(t *testing.T) {
	validator, _ := NewHTTPValidator(time.Second, Endpoint{}, nil)
	if _, err := validator.ResolveResource(context.Background(), tenantID, "production-operations", "production-work-item", resourceID); !errors.Is(err, projects.ErrReferenceUnavailable) {
		t.Fatalf("unselected source became empty success: %v", err)
	}
	for _, test := range []struct {
		status int
		err    error
	}{{403, projects.ErrReferenceForbidden}, {503, projects.ErrReferenceUnavailable}, {404, projects.ErrReferenceInvalid}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(test.status) }))
		validator, _ := NewHTTPValidator(time.Second, Endpoint{}, map[string]Endpoint{"production-operations": {BaseURL: server.URL, AccessToken: "test-token"}})
		_, err := validator.ResolveResource(context.Background(), tenantID, "production-operations", "production-work-item", resourceID)
		server.Close()
		if !errors.Is(err, test.err) {
			t.Fatalf("source status=%d err=%v", test.status, err)
		}
	}
}
