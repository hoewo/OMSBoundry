package references

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const (
	referenceTenant  = "11111111-1111-4111-8111-111111111111"
	referenceProject = "22222222-2222-4222-8222-222222222222"
	referenceFile    = "33333333-3333-4333-8333-333333333333"
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

func TestDynamicCredentialIsExactAndRetriesOneUnauthorizedResponse(t *testing.T) {
	spy := &credentialSpy{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/references/projects/"+referenceProject {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "Bearer stale" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","status":"active","revision":7}`, referenceProject, referenceTenant)
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(time.Second,
		Endpoint{BaseURL: server.URL, CredentialSource: spy, Audience: "project-workspace", Action: "project-workspace.read"},
		Endpoint{BaseURL: server.URL, AccessToken: "static-evidence-token"},
	)
	if err != nil {
		t.Fatal(err)
	}
	value, err := validator.ValidateProject(context.Background(), referenceTenant, referenceProject, 7)
	if err != nil || value.ID != referenceProject || spy.calls != 2 || !spy.invalidated {
		t.Fatalf("value=%#v err=%v spy=%#v", value, err, spy)
	}
	if spy.audience != "project-workspace" || spy.action != "project-workspace.read" || spy.resource != "project" || spy.id != referenceProject {
		t.Fatalf("credential request=%#v", spy)
	}
}

func TestEvidenceReferenceRejectsRevisionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tenant_id":%q,"file_id":%q,"version":2,"revision":8,"display_name":"中标通知书.pdf"}`, referenceTenant, referenceFile)
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(time.Second,
		Endpoint{BaseURL: server.URL, AccessToken: "static-project-token"},
		Endpoint{BaseURL: server.URL, AccessToken: "static-evidence-token"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validator.ValidateEvidence(context.Background(), referenceTenant, referenceFile, 2, 9); err == nil {
		t.Fatal("mismatched Evidence revision was accepted")
	}
}
