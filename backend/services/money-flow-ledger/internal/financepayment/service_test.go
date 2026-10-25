package financepayment

import (
	"context"
	"testing"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/party"
)

type fakeRepository struct {
	requestDigest string
	result        Result
	recorded      int
}

func (f *fakeRepository) Replay(_ context.Context, _, _, digest string) (Result, bool, error) {
	if f.requestDigest == "" {
		return Result{}, false, nil
	}
	if f.requestDigest != digest {
		return Result{}, false, ErrCommandConflict
	}
	return f.result, true, nil
}
func (f *fakeRepository) Resolve(context.Context, Request) (Resolution, error) {
	return Resolution{OwnerPartyID: "11111111-1111-4111-8111-111111111111"}, nil
}
func (f *fakeRepository) Bind(_ context.Context, input Request, id string, revision int64) (Binding, error) {
	return Binding{ApplicationID: input.ApplicationID, ObligationID: id, ObligationRevision: revision, TotalPayableMinor: input.TotalPayableMinor}, nil
}
func (f *fakeRepository) Record(_ context.Context, _ Request, digest string, result Result, _ audit.ActorRef) error {
	f.requestDigest, f.result = digest, result
	f.recorded++
	return nil
}

type fakeObligations struct{ created int }

func (f *fakeObligations) CreateSuggestedDraft(_ context.Context, input obligation.CreateSuggestedDraftInput) (obligation.CreateResult, error) {
	f.created++
	return obligation.CreateResult{Obligation: obligation.Obligation{ID: "22222222-2222-4222-8222-222222222222", DraftVersion: 1, OriginalAmount: input.OriginalAmount}}, nil
}
func (f *fakeObligations) Confirm(context.Context, obligation.ConfirmInput) (obligation.ConfirmResult, error) {
	return obligation.ConfirmResult{Obligation: obligation.Obligation{ID: "22222222-2222-4222-8222-222222222222", CurrentRevision: 1}, Revision: obligation.Revision{Revision: 1}}, nil
}
func (f *fakeObligations) GetDetails(context.Context, string) (obligation.Details, bool, error) {
	return obligation.Details{Obligation: obligation.Obligation{ID: "22222222-2222-4222-8222-222222222222", OutstandingAmount: "100.00", CurrentRevision: 1}}, true, nil
}

type fakeCash struct{ input cashtransaction.CreateInput }

func (f *fakeCash) Create(_ context.Context, input cashtransaction.CreateInput) (cashtransaction.CreateResult, error) {
	f.input = input
	return cashtransaction.CreateResult{Transaction: cashtransaction.Transaction{ID: "33333333-3333-4333-8333-333333333333"}}, nil
}

type fakeAllocations struct{ input allocation.ApplyInput }

func (f *fakeAllocations) Apply(_ context.Context, input allocation.ApplyInput) (allocation.ApplyResult, error) {
	f.input = input
	return allocation.ApplyResult{Allocation: allocation.Allocation{ID: "44444444-4444-4444-8444-444444444444"}}, nil
}

type fakeParties struct{ input party.CreateInput }

func (f *fakeParties) Create(_ context.Context, input party.CreateInput) (party.CreateResult, error) {
	f.input = input
	return party.CreateResult{Party: party.Party{ID: "55555555-5555-4555-8555-555555555555"}}, nil
}

func TestRecordOneTimeIndividualCreatesExactPaymentAndOverage(t *testing.T) {
	repository, obligations, cash, allocations, parties := &fakeRepository{}, &fakeObligations{}, &fakeCash{}, &fakeAllocations{}, &fakeParties{}
	service, err := NewService(repository, obligations, cash, allocations, parties)
	if err != nil {
		t.Fatal(err)
	}
	input := Request{SchemaVersion: "1", CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", TenantID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ApplicationID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ApplicationRevision: 2, ApplicationDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RequestType: "payment", Title: "顾问费", TotalPayableMinor: 10000, ActualAmountMinor: 12000, OccurredOn: "2026-10-25", Payee: PayeeSnapshot{Kind: "one_time", EntityKind: "individual", DisplayName: "外部顾问", BankName: "测试银行", AccountLast4: "1234"}}
	actor := audit.ActorRef{Kind: "service", ID: "internal-finance-requests"}
	result, err := service.Record(context.Background(), actor, input)
	if err != nil {
		t.Fatal(err)
	}
	if result.AllocatedAmountMinor != 10000 || result.OverageAmountMinor != 2000 || cash.input.Amount != "120.00" || allocations.input.Amount != "100.00" || parties.input.Kind != "individual" || repository.recorded != 1 {
		t.Fatalf("unexpected result=%#v cash=%#v allocation=%#v party=%#v recorded=%d", result, cash.input, allocations.input, parties.input, repository.recorded)
	}
	replayed, err := service.Record(context.Background(), actor, input)
	if err != nil || !replayed.Replayed || repository.recorded != 1 || obligations.created != 1 {
		t.Fatalf("replay=%#v recorded=%d obligations=%d err=%v", replayed, repository.recorded, obligations.created, err)
	}
}

func TestOneTimePayeeRequiresEntityKind(t *testing.T) {
	service, _ := NewService(&fakeRepository{}, &fakeObligations{}, &fakeCash{}, &fakeAllocations{}, &fakeParties{})
	input := Request{SchemaVersion: "1", CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", TenantID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ApplicationID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", ApplicationRevision: 1, ApplicationDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RequestType: "payment", Title: "顾问费", TotalPayableMinor: 10000, ActualAmountMinor: 10000, OccurredOn: "2026-10-25", Payee: PayeeSnapshot{Kind: "one_time", DisplayName: "外部顾问"}}
	if _, err := service.Record(context.Background(), audit.ActorRef{Kind: "service", ID: "internal-finance-requests"}, input); err != ErrInvalid {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}
