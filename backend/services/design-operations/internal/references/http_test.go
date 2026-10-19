package references

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"omsbizfoundry/design-operations/internal/design"
)

type credentialSpy struct {
	calls       int
	invalidated bool
	audience    string
	action      string
	resource    string
	id          string
}

func (s *credentialSpy) Credential(_ context.Context, audience, action, resourceType, resourceID string) (string, error) {
	s.calls++
	s.audience, s.action, s.resource, s.id = audience, action, resourceType, resourceID
	if s.calls == 1 {
		return "stale", nil
	}
	return "fresh", nil
}

func (s *credentialSpy) Invalidate(_, _, _, _ string) { s.invalidated = true }

const (
	refTenant     = "11111111-1111-4111-8111-111111111111"
	refProject    = "22222222-2222-4222-8222-222222222222"
	refAssignment = "33333333-3333-4333-8333-333333333333"
	refFile       = "44444444-4444-4444-8444-444444444444"
	refPerson     = "55555555-5555-4555-8555-555555555555"
	refSubject    = "66666666-6666-4666-8666-666666666666"
)

func TestHTTPValidatorAcceptsExactReferences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer reference-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/references/projects/" + refProject:
			_, _ = w.Write([]byte(`{"id":"` + refProject + `","tenant_id":"` + refTenant + `","status":"active","revision":4,"project_type_keys":["design"]}`))
		case "/api/v1/references/projects/" + refProject + "/assignments/" + refAssignment:
			_, _ = w.Write([]byte(`{"id":"` + refAssignment + `","tenant_id":"` + refTenant + `","project_id":"` + refProject + `","subject_kind":"person","subject_id":"` + refPerson + `","source_revision":7,"role_key":"production_owner","status":"active","display_name":"设计组","revision":3}`))
		case "/api/v1/policy-facts/persons/" + refPerson:
			_, _ = w.Write([]byte(`{"schema_version":"1","source_application":"people-groups","fact_kind":"person-iam-subject-link","id":"` + refPerson + `","tenant_id":"` + refTenant + `","status":"active","revision":7,"iam_subject_id":"` + refSubject + `"}`))
		case "/v1/references/files/" + refFile + "/versions/1":
			_, _ = w.Write([]byte(`{"file_id":"` + refFile + `","tenant_id":"` + refTenant + `","status":"available","display_name":"方案.pdf","version":1,"revision":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	endpoint := Endpoint{BaseURL: server.URL, AccessToken: "reference-token"}
	validator, err := NewHTTPValidator(time.Second, endpoint, endpoint, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validator.ValidateProject(context.Background(), refTenant, refProject, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := validator.ValidateAssignment(context.Background(), refTenant, refProject, refAssignment, 3); err != nil {
		t.Fatal(err)
	}
	if value, err := validator.ValidatePersonIAMSubject(context.Background(), refTenant, refPerson, 7); err != nil || value.IAMSubjectID != refSubject {
		t.Fatalf("person policy fact=%#v err=%v", value, err)
	}
	if _, err := validator.ValidateEvidence(context.Background(), refTenant, refFile, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := validator.ValidateProject(context.Background(), refTenant, refProject, 5); !errors.Is(err, design.ErrReferenceInvalid) {
		t.Fatalf("revision mismatch=%v", err)
	}
	if _, err := validator.ValidateProject(context.Background(), "99999999-9999-4999-8999-999999999999", refProject, 4); !errors.Is(err, design.ErrReferenceInvalid) {
		t.Fatalf("tenant mismatch=%v", err)
	}
	if _, err := validator.ValidateEvidence(context.Background(), refTenant, "77777777-7777-4777-8777-777777777777", 1, 1); !errors.Is(err, design.ErrReferenceInvalid) {
		t.Fatalf("not found=%v", err)
	}
}

func TestHTTPValidatorDistinguishesUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer server.Close()
	endpoint := Endpoint{BaseURL: server.URL, AccessToken: "token"}
	validator, _ := NewHTTPValidator(time.Second, endpoint, endpoint, endpoint)
	if _, err := validator.ValidateProject(context.Background(), refTenant, refProject, 4); !errors.Is(err, design.ErrReferenceUnavailable) {
		t.Fatalf("unavailable=%v", err)
	}
}

func TestDynamicCredentialIsExactAndRetriesOneUnauthorizedResponse(t *testing.T) {
	spy := &credentialSpy{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer stale" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"status":"active","revision":4,"project_type_keys":["design","consultation"]}`, refProject, refTenant)
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(time.Second,
		Endpoint{BaseURL: server.URL, CredentialSource: spy, Audience: "project-workspace", Action: "project-workspace.read"},
		Endpoint{BaseURL: server.URL, AccessToken: "static-people-token"},
		Endpoint{BaseURL: server.URL, AccessToken: "static-evidence-token"},
	)
	if err != nil {
		t.Fatal(err)
	}
	value, err := validator.ValidateProject(context.Background(), refTenant, refProject, 4)
	if err != nil || value.ID != refProject || spy.calls != 2 || !spy.invalidated {
		t.Fatalf("value=%#v err=%v spy=%#v", value, err, spy)
	}
	if spy.audience != "project-workspace" || spy.action != "project-workspace.read" || spy.resource != "project" || spy.id != refProject {
		t.Fatalf("credential request=%#v", spy)
	}
}
