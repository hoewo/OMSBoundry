package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashevidence"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/partymigration"
)

type directoryPartyProjectorCall struct {
	tenantID, partyID, subjectID, displayName string
	revision                                  int64
}

type directoryPartyProjectorStub struct {
	values map[string]partymigration.RuntimePartyReference
	err    error
	calls  []directoryPartyProjectorCall
}

type cashEvidenceStub struct {
	currentValue cashevidence.Snapshot
	currentFound bool
	currentErr   error
	replaceInput cashevidence.ReplaceInput
	replaceValue cashevidence.Result
	replaceErr   error
	currentCalls int
	replaceCalls int
}

func (s *cashEvidenceStub) Current(context.Context, string) (cashevidence.Snapshot, bool, error) {
	s.currentCalls++
	return s.currentValue, s.currentFound, s.currentErr
}

func (s *cashEvidenceStub) Replace(_ context.Context, input cashevidence.ReplaceInput) (cashevidence.Result, error) {
	s.replaceCalls++
	s.replaceInput = input
	return s.replaceValue, s.replaceErr
}

func (s *directoryPartyProjectorStub) ResolveOrProject(_ context.Context, tenantID, partyID string, revision int64, subjectID, displayName string) (partymigration.RuntimePartyReference, error) {
	s.calls = append(s.calls, directoryPartyProjectorCall{tenantID: tenantID, partyID: partyID, revision: revision, subjectID: subjectID, displayName: displayName})
	if s.err != nil {
		return partymigration.RuntimePartyReference{}, s.err
	}
	return s.values[partyID], nil
}

func cashPrincipal() identity.Principal {
	return identity.Principal{Provider: "iam", UserID: "11111111-1111-4111-8111-111111111111", DisplayName: "trusted", TenantID: referenceTestTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{"money-flow-ledger.view": true, "money-flow-ledger.amount.read": true, "money-flow-ledger.cash.record": true, "money-flow-ledger.cash.reverse": true}}
}
func cashRequest(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer candidate-token")
	r.Header.Set("X-Actor-ID", "forged")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestStatisticsCashExportRequiresExportAmountAndExactResource(t *testing.T) {
	p := cashPrincipal()
	ids := &identityStub{found: true, principal: p}
	cash := &cashTransactionStub{getFound: true, getResult: cashtransaction.Transaction{ID: referenceTestTransaction, Amount: "100.01", EffectiveAmount: "100.01", Status: "recorded", Currency: "CNY"}}
	h := newReferenceHandler(t, ids, &obligationStub{}, cash, Options{ExternalIdentity: true})
	path := "/api/v1/cash-transactions/" + referenceTestTransaction + "/export-view"
	if w := cashRequest(h, "GET", path, ""); w.Code != 403 || cash.getCalls != 0 {
		t.Fatalf("read capability exported: %d", w.Code)
	}
	ids.principal.AllowedActions["money-flow-ledger.export"] = true
	ids.principal.ScopeKind = "resources"
	ids.principal.Resources = map[string]map[string]bool{"cash_transaction": {referenceTestTransaction: true}}
	if w := cashRequest(h, "GET", path, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "100.01") {
		t.Fatalf("authorized export failed: %d %s", w.Code, w.Body.String())
	}
	if w := cashRequest(h, "GET", "/api/v1/cash-transactions/"+referenceTestOther+"/export-view", ""); w.Code != 403 || cash.getCalls != 1 {
		t.Fatalf("neighbor exported: %d", w.Code)
	}
	delete(ids.principal.AllowedActions, "money-flow-ledger.amount.read")
	if w := cashRequest(h, "GET", path, ""); w.Code != 403 || cash.getCalls != 1 {
		t.Fatalf("redacted amount exported: %d", w.Code)
	}
}

func TestCashWorkspaceTrustedActorStrictJSONAndScope(t *testing.T) {
	p := cashPrincipal()
	ids := &identityStub{found: true, principal: p}
	cash := &cashTransactionStub{createResult: cashtransaction.CreateResult{Transaction: cashtransaction.Transaction{ID: referenceTestTransaction, Amount: "9007199254740993.01", Status: "recorded", AllocatedAmount: "0", Currency: "CNY"}}, getFound: true, getResult: cashtransaction.Transaction{ID: referenceTestTransaction, Amount: "9007199254740993.01", EffectiveAmount: "9007199254740993.01", Status: "recorded", AllocatedAmount: "0", Currency: "CNY"}}
	h := newReferenceHandler(t, ids, &obligationStub{}, cash, Options{ExternalIdentity: true})
	body := `{"command_id":"44444444-4444-4444-8444-444444444444","type":"receipt","ledger_owner_party_id":"55555555-5555-4555-8555-555555555555","counterparty_party_id":"66666666-6666-4666-8666-666666666666","description":"actual","amount":"9007199254740993.01","currency":"CNY","occurred_on":"2026-10-04","account_reference":"account","manual_source_reference":"manual"}`
	w := cashRequest(h, "POST", "/api/v1/cash-transactions", body)
	if w.Code != 200 || cash.createCalls != 1 || cash.createInput.Amount != "9007199254740993.01" || cash.createInput.Actor.ID != p.UserID || cash.createInput.Actor.Kind != audit.ActorKindIAMSubject {
		t.Fatalf("status=%d input=%#v", w.Code, cash.createInput)
	}
	for _, bad := range []string{strings.TrimSuffix(body, "}") + `,"actor_id":"forged"}`, body + `{}`, strings.Replace(body, `"9007199254740993.01"`, `9007199254740993.01`, 1)} {
		if w = cashRequest(h, "POST", "/api/v1/cash-transactions", bad); w.Code != 400 || cash.createCalls != 1 {
			t.Fatalf("strict JSON accepted %d", w.Code)
		}
	}
	ids.principal.ScopeKind = "resources"
	ids.principal.Resources = map[string]map[string]bool{"cash_transaction": {referenceTestTransaction: true}}
	if w = cashRequest(h, "GET", "/api/v1/cash-transactions/"+referenceTestTransaction, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = cashRequest(h, "GET", "/api/v1/cash-transactions/"+referenceTestOther, ""); w.Code != 403 || cash.getCalls != 1 {
		t.Fatalf("neighbor accepted %d", w.Code)
	}
	if w = cashRequest(h, "POST", "/api/v1/cash-transactions", body); w.Code != 403 || cash.createCalls != 1 {
		t.Fatalf("exact scope created cash %d", w.Code)
	}
	delete(ids.principal.AllowedActions, "money-flow-ledger.amount.read")
	w = cashRequest(h, "GET", "/api/v1/cash-transactions/"+referenceTestTransaction, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "9007199254740993.01") {
		t.Fatalf("amount leaked %s", w.Body.String())
	}
	var view struct {
		Transaction map[string]any `json:"transaction"`
	}
	if json.Unmarshal(w.Body.Bytes(), &view) != nil || view.Transaction["amount"] != nil || view.Transaction["currency"] != nil || view.Transaction["amount_visible"] != false {
		t.Fatal(view)
	}
	if w = cashRequest(h, "POST", "/api/v1/cash-transactions/"+referenceTestTransaction+"/reversal", `{"command_id":"44444444-4444-4444-8444-444444444444","reason":"wrong"}`); w.Code != 403 || cash.reverseCalls != 0 {
		t.Fatalf("blind reversal %d", w.Code)
	}
}

func TestCashWorkspaceEvidenceUsesNativeCapabilityActorAndExactTransaction(t *testing.T) {
	p := cashPrincipal()
	p.AllowedActions["money-flow-ledger.cash.evidence.manage"] = true
	ids := &identityStub{found: true, principal: p}
	evidence := &cashEvidenceStub{
		currentFound: true,
		currentValue: cashevidence.Snapshot{CashTransactionID: referenceTestTransaction, Revision: 0, EvidenceRefs: []cashevidence.Ref{}},
		replaceValue: cashevidence.Result{Snapshot: cashevidence.Snapshot{CashTransactionID: referenceTestTransaction, Revision: 1, EvidenceRefs: []cashevidence.Ref{{FileID: "77777777-7777-4777-8777-777777777777", Version: 1, Purpose: "receipt_proof"}}}},
	}
	h := newReferenceHandler(t, ids, &obligationStub{}, &cashTransactionStub{}, Options{ExternalIdentity: true, CashEvidence: evidence})
	path := "/api/v1/cash-transactions/" + referenceTestTransaction + "/evidence"
	if w := cashRequest(h, http.MethodGet, path, ""); w.Code != http.StatusOK || evidence.currentCalls != 1 || !strings.Contains(w.Body.String(), `"revision":0`) {
		t.Fatalf("read status=%d body=%s calls=%d", w.Code, w.Body.String(), evidence.currentCalls)
	}
	body := `{"command_id":"44444444-4444-4444-8444-444444444444","expected_revision":0,"reason":"登记回款凭证","evidence_refs":[{"file_id":"77777777-7777-4777-8777-777777777777","version":1,"purpose":"receipt_proof"}]}`
	if w := cashRequest(h, http.MethodPut, path, body); w.Code != http.StatusOK || evidence.replaceCalls != 1 {
		t.Fatalf("replace status=%d body=%s calls=%d", w.Code, w.Body.String(), evidence.replaceCalls)
	}
	if evidence.replaceInput.CashTransactionID != referenceTestTransaction || evidence.replaceInput.TenantID != p.TenantID || evidence.replaceInput.Actor.ID != p.UserID || evidence.replaceInput.Actor.Kind != audit.ActorKindIAMSubject {
		t.Fatalf("replace input=%#v", evidence.replaceInput)
	}
	ids.principal.ScopeKind = "resources"
	ids.principal.Resources = map[string]map[string]bool{"cash_transaction": {referenceTestTransaction: true}}
	if w := cashRequest(h, http.MethodPut, "/api/v1/cash-transactions/"+referenceTestOther+"/evidence", body); w.Code != http.StatusForbidden || evidence.replaceCalls != 1 {
		t.Fatalf("neighbor status=%d calls=%d", w.Code, evidence.replaceCalls)
	}
	delete(ids.principal.AllowedActions, "money-flow-ledger.cash.evidence.manage")
	if w := cashRequest(h, http.MethodPut, path, body); w.Code != http.StatusForbidden || evidence.replaceCalls != 1 {
		t.Fatalf("missing capability status=%d calls=%d", w.Code, evidence.replaceCalls)
	}
}

func TestCashWorkspaceProjectsExactPartyDirectoryReferencesBeforeNativeCreate(t *testing.T) {
	const ownerDirectory = "77777777-7777-4777-8777-777777777777"
	const counterpartyDirectory = "88888888-8888-4888-8888-888888888888"
	ids := &identityStub{found: true, principal: cashPrincipal()}
	cash := &cashTransactionStub{createResult: cashtransaction.CreateResult{Transaction: cashtransaction.Transaction{ID: referenceTestTransaction, Amount: "10.25", EffectiveAmount: "10.25", Status: "recorded", AllocatedAmount: "0", Currency: "CNY"}}}
	projector := &directoryPartyProjectorStub{values: map[string]partymigration.RuntimePartyReference{
		ownerDirectory:        {LocalPartyID: referenceTestTransaction, PartyID: ownerDirectory, Revision: 3, Source: partymigration.RuntimeModeDirectory},
		counterpartyDirectory: {LocalPartyID: referenceTestOther, PartyID: counterpartyDirectory, Revision: 4, Source: partymigration.RuntimeModeDirectory},
	}}
	h := newReferenceHandler(t, ids, &obligationStub{}, cash, Options{ExternalIdentity: true, DirectoryParties: projector})
	body := `{"command_id":"44444444-4444-4444-8444-444444444444","type":"receipt","ledger_owner_party_id":"","counterparty_party_id":"","ledger_owner_party_reference":{"party_id":"` + ownerDirectory + `","revision":3},"counterparty_party_reference":{"party_id":"` + counterpartyDirectory + `","revision":4},"description":"项目回款","amount":"10.25","currency":"CNY","occurred_on":"2026-10-12","account_reference":"银行流水","manual_source_reference":"project:aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","external_transaction_reference":""}`
	w := cashRequest(h, http.MethodPost, "/api/v1/cash-transactions", body)
	if w.Code != http.StatusOK || cash.createCalls != 1 || cash.createInput.LedgerOwnerPartyID != referenceTestTransaction || cash.createInput.CounterpartyPartyID != referenceTestOther {
		t.Fatalf("status=%d body=%s input=%#v", w.Code, w.Body.String(), cash.createInput)
	}
	if len(projector.calls) != 2 || projector.calls[0].tenantID != referenceTestTenant || projector.calls[0].partyID != ownerDirectory || projector.calls[0].revision != 3 || projector.calls[0].subjectID != ids.principal.UserID || projector.calls[1].partyID != counterpartyDirectory || projector.calls[1].revision != 4 {
		t.Fatalf("projection calls = %#v", projector.calls)
	}

	mixed := strings.Replace(body, `"ledger_owner_party_id":""`, `"ledger_owner_party_id":"`+referenceTestTransaction+`"`, 1)
	if w = cashRequest(h, http.MethodPost, "/api/v1/cash-transactions", mixed); w.Code != http.StatusBadRequest || cash.createCalls != 1 {
		t.Fatalf("mixed Party authorities accepted: status=%d body=%s", w.Code, w.Body.String())
	}
	projector.err = partymigration.ErrTargetInvalid
	if w = cashRequest(h, http.MethodPost, "/api/v1/cash-transactions", body); w.Code != http.StatusUnprocessableEntity || cash.createCalls != 1 {
		t.Fatalf("stale Party reference accepted: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCashWorkspaceContextDoesNotLeakDirectoryToExactScope(t *testing.T) {
	ids := &identityStub{found: true, principal: cashPrincipal()}
	parties := &partyStub{listResult: []party.Party{{ID: referenceTestOther, Revision: 1, LegalName: "Money native"}}}
	h, err := New(readinessStub{}, ids, parties, &obligationStub{}, &cashTransactionStub{}, &allocationStub{}, &operatingSummaryStub{}, &ledgerExportStub{}, Options{}, &internalTransferStub{})
	if err != nil {
		t.Fatal(err)
	}
	if w := cashRequest(h, "GET", "/api/v1/cash-workspace", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "Money native") || parties.listCalls != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	ids.principal.ScopeKind = "resources"
	ids.principal.Resources = map[string]map[string]bool{"cash_transaction": {referenceTestTransaction: true}}
	if w := cashRequest(h, "GET", "/api/v1/cash-workspace", ""); w.Code != 200 || strings.Contains(w.Body.String(), "Money native") || parties.listCalls != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	ids.principal.Provider = "reference-template"
	if w := cashRequest(h, "POST", "/api/v1/cash-transactions", `{}`); w.Code != 403 {
		t.Fatal("template wrote", w.Code)
	}
	ids.principal.Provider = "local"
	if w := cashRequest(h, "GET", "/api/v1/cash-workspace", ""); w.Code != 403 {
		t.Fatal("local credential used formal port", w.Code)
	}
}

func TestCashWorkspaceListFiltersSortsPaginatesAndRedactsAmounts(t *testing.T) {
	ids := &identityStub{found: true, principal: cashPrincipal()}
	receipt := sampleCashTransaction()
	receipt.ManualSourceReference = "project:aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	payment := sampleCashTransaction()
	payment.ID = referenceTestOther
	payment.Type = cashtransaction.TypePayment
	payment.Description = "施工方付款"
	payment.OccurredOn = "2026-10-06"
	payment.RecordedAt = receipt.RecordedAt.AddDate(0, 1, 0)
	payment.ManualSourceReference = "project:bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	reversed := sampleReversedCashTransaction()
	reversed.ID = "99999999-9999-4999-8999-999999999999"
	reversed.OccurredOn = "2026-10-07"
	cash := &cashTransactionStub{listResult: []cashtransaction.Transaction{receipt, reversed, payment}}
	h := newReferenceHandler(t, ids, &obligationStub{}, cash, Options{ExternalIdentity: true})

	w := cashRequest(h, http.MethodGet, "/api/v1/cash-transactions?type=payment&occurred_from=2026-10-01&limit=1", "")
	if w.Code != http.StatusOK || cash.listCalls != 1 {
		t.Fatalf("status=%d body=%s calls=%d", w.Code, w.Body.String(), cash.listCalls)
	}
	var page struct {
		Items []struct {
			ID            string  `json:"id"`
			Amount        *string `json:"amount"`
			AmountVisible bool    `json:"amount_visible"`
		} `json:"items"`
		Total  int `json:"total"`
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Total != 1 || page.Offset != 0 || page.Limit != 1 || len(page.Items) != 1 || page.Items[0].ID != payment.ID || !page.Items[0].AmountVisible || page.Items[0].Amount == nil || *page.Items[0].Amount != payment.Amount {
		t.Fatalf("page=%#v err=%v body=%s", page, err, w.Body.String())
	}

	delete(ids.principal.AllowedActions, "money-flow-ledger.amount.read")
	w = cashRequest(h, http.MethodGet, "/api/v1/cash-transactions?source_reference=project%3Aaaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "")
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &page) != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != receipt.ID || page.Items[0].AmountVisible || page.Items[0].Amount != nil || strings.Contains(w.Body.String(), receipt.Amount) {
		t.Fatalf("redacted page=%#v status=%d body=%s", page, w.Code, w.Body.String())
	}
}

func TestCashWorkspaceListAddsAuthoritativePartyReferences(t *testing.T) {
	ids := &identityStub{found: true, principal: cashPrincipal()}
	transaction := sampleCashTransaction()
	runtime := partyRuntimeStub{values: []partymigration.RuntimePartyReference{
		{
			LocalPartyID: transaction.Payer.PartyID,
			PartyID:      "77777777-7777-4777-8777-777777777777",
			Revision:     7,
			Kind:         string(transaction.Payer.Kind),
			LegalName:    transaction.Payer.LegalName,
			Source:       partymigration.RuntimeModeDirectory,
		},
		{
			LocalPartyID: transaction.Payee.PartyID,
			PartyID:      "88888888-8888-4888-8888-888888888888",
			Revision:     8,
			Kind:         string(transaction.Payee.Kind),
			LegalName:    transaction.Payee.LegalName,
			Source:       partymigration.RuntimeModeDirectory,
		},
	}}
	cash := &cashTransactionStub{listResult: []cashtransaction.Transaction{transaction}}
	h := newReferenceHandler(t, ids, &obligationStub{}, cash, Options{ExternalIdentity: true, PartyRuntime: runtime})

	w := cashRequest(h, http.MethodGet, "/api/v1/cash-transactions", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var page struct {
		Items []struct {
			Payer struct {
				ID             string `json:"id"`
				PartyReference struct {
					PartyID      string `json:"party_id"`
					Revision     int64  `json:"revision"`
					Authority    string `json:"authority"`
					LocalPartyID string `json:"local_party_id"`
				} `json:"party_reference"`
			} `json:"payer"`
			Payee struct {
				ID             string `json:"id"`
				PartyReference struct {
					PartyID      string `json:"party_id"`
					Revision     int64  `json:"revision"`
					Authority    string `json:"authority"`
					LocalPartyID string `json:"local_party_id"`
				} `json:"party_reference"`
			} `json:"payee"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%#v err=%v body=%s", page, err, w.Body.String())
	}
	item := page.Items[0]
	if item.Payer.ID != transaction.Payer.PartyID || item.Payer.PartyReference.PartyID != "77777777-7777-4777-8777-777777777777" || item.Payer.PartyReference.LocalPartyID != transaction.Payer.PartyID || item.Payer.PartyReference.Revision != 7 || item.Payer.PartyReference.Authority != partymigration.RuntimeModeDirectory {
		t.Fatalf("payer=%#v", item.Payer)
	}
	if item.Payee.ID != transaction.Payee.PartyID || item.Payee.PartyReference.PartyID != "88888888-8888-4888-8888-888888888888" || item.Payee.PartyReference.LocalPartyID != transaction.Payee.PartyID || item.Payee.PartyReference.Revision != 8 || item.Payee.PartyReference.Authority != partymigration.RuntimeModeDirectory {
		t.Fatalf("payee=%#v", item.Payee)
	}

	h = newReferenceHandler(t, ids, &obligationStub{}, cash, Options{ExternalIdentity: true, PartyRuntime: partyRuntimeStub{values: runtime.values[:1]}})
	if w = cashRequest(h, http.MethodGet, "/api/v1/cash-transactions", ""); w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "Money Party references unavailable") {
		t.Fatalf("incomplete projection status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCashWorkspaceListRejectsInvalidQueriesAndRequiresTenantScope(t *testing.T) {
	ids := &identityStub{found: true, principal: cashPrincipal()}
	cash := &cashTransactionStub{}
	h := newReferenceHandler(t, ids, &obligationStub{}, cash, Options{ExternalIdentity: true})
	for _, path := range []string{
		"/api/v1/cash-transactions?type=transfer",
		"/api/v1/cash-transactions?status=pending",
		"/api/v1/cash-transactions?occurred_from=2026-02-30",
		"/api/v1/cash-transactions?occurred_from=2026-10-02&occurred_to=2026-10-01",
		"/api/v1/cash-transactions?limit=101",
		"/api/v1/cash-transactions?unknown=value",
	} {
		if w := cashRequest(h, http.MethodGet, path, ""); w.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
	if cash.listCalls != 0 {
		t.Fatalf("invalid queries reached list %d times", cash.listCalls)
	}
	ids.principal.ScopeKind = "resources"
	ids.principal.Resources = map[string]map[string]bool{"cash_transaction": {referenceTestTransaction: true}}
	if w := cashRequest(h, http.MethodGet, "/api/v1/cash-transactions", ""); w.Code != http.StatusForbidden || cash.listCalls != 0 {
		t.Fatalf("exact scope listed tenant cash: status=%d calls=%d", w.Code, cash.listCalls)
	}
}

func TestCashWorkspaceReversalConflictAndReplayUseNativeCases(t *testing.T) {
	ids := &identityStub{found: true, principal: cashPrincipal()}
	cash := &cashTransactionStub{reverseResult: cashtransaction.ReverseResult{Replayed: true, Reversal: cashtransaction.Reversal{ID: referenceTestOther, CashTransactionID: referenceTestTransaction}}}
	h := newReferenceHandler(t, ids, &obligationStub{}, cash, Options{})
	body := `{"command_id":"44444444-4444-4444-8444-444444444444","reason":"错误记录"}`
	if w := cashRequest(h, "POST", "/api/v1/cash-transactions/"+referenceTestTransaction+"/reversal", body); w.Code != 200 || !strings.Contains(w.Body.String(), `"replayed":true`) || cash.reverseInput.Actor.ID != ids.principal.UserID {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, test := range []struct {
		err    error
		status int
	}{{&cashtransaction.TransactionAllocatedConflictError{}, 409}, {&cashtransaction.TransactionAlreadyReversedError{}, 409}, {&cashtransaction.ReversalIdempotencyConflictError{}, 409}, {&cashtransaction.TransactionNotFoundError{}, 404}, {&cashtransaction.ValidationError{Message: "invalid"}, 422}, {errors.New("database failure"), 503}} {
		cash.reverseErr = test.err
		if w := cashRequest(h, "POST", "/api/v1/cash-transactions/"+referenceTestTransaction+"/reversal", body); w.Code != test.status || strings.Contains(w.Body.String(), "database failure") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
