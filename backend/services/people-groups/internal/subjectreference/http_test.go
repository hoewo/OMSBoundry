package subjectreference

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"omsbizfoundry/people-groups/internal/peoplegroups"
)

const (
	tenantID  = "11111111-1111-4111-8111-111111111111"
	subjectID = "22222222-2222-4222-8222-222222222222"
)

type credentialSpy struct {
	tenant      string
	invalidated bool
	calls       int
}

func (s *credentialSpy) CredentialForTenant(_ context.Context, tenant, audience, action, resourceType, resourceID string) (string, error) {
	s.tenant = tenant
	s.calls++
	return "credential", nil
}

func (s *credentialSpy) InvalidateForTenant(tenant, audience, action, resourceType, resourceID string) {
	s.invalidated = true
}

func TestHTTPValidatorUsesCallerTenantAndValidatesExactReference(t *testing.T) {
	credentials := &credentialSpy{}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(peoplegroups.IAMSubjectReference{SchemaVersion: "1", SourceApplication: "identity-access", ID: subjectID, TenantID: tenantID, Username: "employee", DisplayName: "员工", Status: "active", Revision: 2})
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(server.URL, credentials, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	value, err := validator.ValidateIAMSubject(t.Context(), tenantID, subjectID)
	if err != nil || value.Revision != 2 || credentials.tenant != tenantID || !credentials.invalidated || credentials.calls != 2 {
		t.Fatalf("value=%#v credentials=%#v err=%v", value, credentials, err)
	}
}

func TestHTTPValidatorRejectsCrossTenantReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(peoplegroups.IAMSubjectReference{SchemaVersion: "1", SourceApplication: "identity-access", ID: subjectID, TenantID: "33333333-3333-4333-8333-333333333333", Status: "active", Revision: 1})
	}))
	defer server.Close()
	validator, err := NewHTTPValidator(server.URL, &credentialSpy{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = validator.ValidateIAMSubject(t.Context(), tenantID, subjectID)
	if !errors.Is(err, peoplegroups.ErrSubjectReferenceInvalid) {
		t.Fatalf("error=%v", err)
	}
}
