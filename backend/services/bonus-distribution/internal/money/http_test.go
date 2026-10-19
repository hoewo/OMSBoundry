package money

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"omsbizfoundry/bonus-distribution/internal/bonus"
	contract "omsbizfoundry/payable-suggestion-contract"
)

func TestSubmitPayableSuggestionBindsBonusAndPeopleSnapshots(t *testing.T) {
	var captured contract.Submission
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/payable-suggestion-intakes" || r.Header.Get("Authorization") != "Bearer static-money-token" {
			t.Fatalf("unexpected request %s %s authorization=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"intake": contract.Intake{SchemaVersion: contract.SchemaVersion, SuggestionID: captured.SuggestionID, TenantID: captured.TenantID, RequestDigest: captured.RequestDigest, Status: contract.StatusPending, Revision: 1}})
	}))
	defer server.Close()
	client, err := NewClient(time.Second, Endpoint{BaseURL: server.URL, AccessToken: "static-money-token"})
	if err != nil {
		t.Fatal(err)
	}
	submission := bonus.PayableSubmission{SuggestionID: "21000000-0000-4000-8000-000000000001", TenantID: "21000000-0000-4000-8000-000000000002", ProjectID: "21000000-0000-4000-8000-000000000003",
		Source:      bonus.PayableSourceSnapshot{AllocationID: "21000000-0000-4000-8000-000000000004", AllocationRevision: 2, AllocationDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Person: bonus.PersonSnapshot{ID: "21000000-0000-4000-8000-000000000005", Revision: 3, DisplayName: "奖金参与人", Digest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
		LedgerOwner: bonus.MoneyPartyRef{ID: "21000000-0000-4000-8000-000000000006", Revision: 1, Kind: "organization", LegalName: "设计单位"}, AmountMinor: 10650, Currency: "CNY", DueOn: "2026-11-30", Purpose: "经营奖金应付", RequestDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}
	result, err := client.SubmitPayableSuggestion(context.Background(), submission.TenantID, submission.SuggestionID, submission)
	if err != nil || result.Status != contract.StatusPending || result.Revision != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if captured.Source.Application != contract.SourceApplicationBonusDistribution || captured.Source.ResourceType != contract.SourceBonusAllocation || captured.Source.ResourceID != submission.Source.AllocationID || captured.Source.Revision != submission.Source.AllocationRevision || captured.Source.Digest != submission.Source.AllocationDigest {
		t.Fatalf("unexpected source snapshot: %#v", captured.Source)
	}
	if captured.LedgerOwner.SourceApplication != contract.PartySourceApplicationMoneyFlow || captured.LedgerOwner.ResourceType != contract.PartyResourceParty || captured.LedgerOwner.ID != submission.LedgerOwner.ID {
		t.Fatalf("unexpected ledger owner snapshot: %#v", captured.LedgerOwner)
	}
	if captured.Counterparty.SourceApplication != contract.PartySourceApplicationPeopleGroups || captured.Counterparty.ResourceType != contract.PartyResourcePerson || captured.Counterparty.ID != submission.Source.Person.ID || captured.Counterparty.Digest != submission.Source.Person.Digest {
		t.Fatalf("unexpected Person snapshot: %#v", captured.Counterparty)
	}
}

func TestMoneyFailureDoesNotBecomeAFalseSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client, err := NewClient(time.Second, Endpoint{BaseURL: server.URL, AccessToken: "static-money-token"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.GetPayableSuggestion(context.Background(), "21000000-0000-4000-8000-000000000002", "21000000-0000-4000-8000-000000000001"); err != bonus.ErrTargetUnavailable {
		t.Fatalf("Money 503 must remain target unavailable, got %v", err)
	}
}

func TestClientRequestsMoneyNativeWorkloadCapability(t *testing.T) {
	credentials := &credentialStub{tokens: []string{"expired-token", "fresh-token"}}
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(contract.Intake{
			SchemaVersion: contract.SchemaVersion,
			SuggestionID:  "21000000-0000-4000-8000-000000000001",
			TenantID:      "21000000-0000-4000-8000-000000000002",
			Status:        contract.StatusAccepted,
			Revision:      2,
		})
	}))
	defer server.Close()

	client, err := NewClient(time.Second, Endpoint{BaseURL: server.URL, Credentials: credentials})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetPayableSuggestion(context.Background(), "21000000-0000-4000-8000-000000000002", "21000000-0000-4000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || credentials.invalidations != 1 || len(credentials.calls) != 2 {
		t.Fatalf("requests=%d credential calls=%d invalidations=%d", requests, len(credentials.calls), credentials.invalidations)
	}
	for _, call := range credentials.calls {
		if call != "money-flow-ledger|money-flow-ledger.payable-suggestion.read|payable_suggestion|21000000-0000-4000-8000-000000000001" {
			t.Fatalf("credential scope = %q", call)
		}
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
