package money

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	contract "omsbizfoundry/payable-suggestion-contract"
)

const (
	testTenantID     = "21000000-0000-4000-8000-000000000001"
	testSuggestionID = "21000000-0000-4000-8000-000000000002"
)

func TestClientSubmitsExactSuggestionWithStaticCredential(t *testing.T) {
	var received contract.Submission
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/payable-suggestion-intakes" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer static-money-token-1234567890" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"intake": contract.Intake{
			SchemaVersion: contract.SchemaVersion, SuggestionID: received.SuggestionID,
			TenantID: received.TenantID, RequestDigest: received.RequestDigest,
			Status: contract.StatusPending, Revision: 1,
		}})
	}))
	defer server.Close()

	client, err := NewClient(time.Second, Endpoint{BaseURL: server.URL, AccessToken: "static-money-token-1234567890"})
	if err != nil {
		t.Fatal(err)
	}
	input := validSubmission()
	result, err := client.SubmitPayableSuggestion(context.Background(), testTenantID, testSuggestionID, input)
	if err != nil {
		t.Fatal(err)
	}
	if received.Source.Application != contract.SourceApplication || received.Source.ResourceType != contract.SourceEngagement || received.Source.Revision != 7 || received.AmountMinor != 885000 || received.RequestDigest != input.RequestDigest {
		t.Fatalf("received submission = %#v", received)
	}
	if result.SuggestionID != testSuggestionID || result.Status != contract.StatusPending || result.Revision != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestClientRefreshesWorkloadCredentialOnceAfterUnauthorized(t *testing.T) {
	credentials := &credentialStub{tokens: []string{"expired-token", "fresh-token"}}
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			if r.Header.Get("Authorization") != "Bearer expired-token" {
				t.Fatalf("first authorization = %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fresh-token" {
			t.Fatalf("refreshed authorization = %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(contract.Intake{
			SchemaVersion: contract.SchemaVersion, SuggestionID: testSuggestionID,
			TenantID: testTenantID, Status: contract.StatusAccepted, Revision: 2,
			Obligation: &contract.ObligationReference{ID: "21000000-0000-4000-8000-000000000009", Revision: 1},
		})
	}))
	defer server.Close()

	client, err := NewClient(time.Second, Endpoint{BaseURL: server.URL, Credentials: credentials})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.GetPayableSuggestion(context.Background(), testTenantID, testSuggestionID)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || credentials.invalidations != 1 || len(credentials.calls) != 2 {
		t.Fatalf("requests=%d credential calls=%d invalidations=%d", requests, len(credentials.calls), credentials.invalidations)
	}
	for _, call := range credentials.calls {
		if call != "money-flow-ledger|money-flow-ledger.payable-suggestion.read|payable_suggestion|"+testSuggestionID {
			t.Fatalf("credential scope = %q", call)
		}
	}
	if result.Status != contract.StatusAccepted || result.Obligation == nil || result.Obligation.Revision != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestClientMapsBadResponseAndTransportFailureToTargetUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"Party mapping unavailable"}`))
	}))
	client, err := NewClient(time.Second, Endpoint{BaseURL: server.URL, AccessToken: "static-money-token-1234567890"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.SubmitPayableSuggestion(context.Background(), testTenantID, testSuggestionID, validSubmission()); err != costoutsourcing.ErrTargetUnavailable {
		t.Fatalf("bad response error = %v", err)
	}
	server.Close()
	if _, err = client.GetPayableSuggestion(context.Background(), testTenantID, testSuggestionID); err != costoutsourcing.ErrTargetUnavailable {
		t.Fatalf("transport error = %v", err)
	}
}

type credentialStub struct {
	mu            sync.Mutex
	tokens        []string
	calls         []string
	invalidations int
}

func (s *credentialStub) Credential(_ context.Context, audience, action, resourceType, resourceID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, audience+"|"+action+"|"+resourceType+"|"+resourceID)
	token := s.tokens[0]
	s.tokens = s.tokens[1:]
	return token, nil
}

func (s *credentialStub) Invalidate(_, _, _, _ string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidations++
}

func validSubmission() costoutsourcing.PayableSubmission {
	return costoutsourcing.PayableSubmission{
		SuggestionID: testSuggestionID, TenantID: testTenantID,
		ProjectID: "21000000-0000-4000-8000-000000000003",
		Source: costoutsourcing.PayableSourceSnapshot{
			Type: contract.SourceEngagement, ID: "21000000-0000-4000-8000-000000000004",
			Revision: 7, Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		LedgerOwner:  costoutsourcing.PartySnapshot{ID: "21000000-0000-4000-8000-000000000005", Revision: 3, Kind: "organization", LegalName: "设计单位"},
		Counterparty: costoutsourcing.PartySnapshot{ID: "21000000-0000-4000-8000-000000000006", Revision: 5, Kind: "organization", LegalName: "协作单位"},
		AmountMinor:  885000, Currency: "CNY", DueOn: "2026-10-31", Purpose: "外协成果付款",
		RequestDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
}
