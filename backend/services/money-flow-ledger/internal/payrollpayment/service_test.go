package payrollpayment

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
)

const (
	testTenant      = "11000000-0000-4000-8000-000000000001"
	testRun         = "11000000-0000-4000-8000-000000000002"
	testLine        = "11000000-0000-4000-8000-000000000003"
	testPerson      = "11000000-0000-4000-8000-000000000004"
	testProfile     = "11000000-0000-4000-8000-000000000005"
	testOwner       = "11000000-0000-4000-8000-000000000006"
	testPersonParty = "11000000-0000-4000-8000-000000000007"
	testObligation  = "11000000-0000-4000-8000-000000000008"
	testActor       = "11000000-0000-4000-8000-000000000009"
)

type memoryRepository struct {
	resolution Resolution
	results    map[string]struct {
		digest string
		result Result
	}
	recordErr error
}

func (r *memoryRepository) Replay(_ context.Context, _, command, requestDigest string) (Result, bool, error) {
	stored, found := r.results[command]
	if !found {
		return Result{}, false, nil
	}
	if stored.digest != requestDigest {
		return Result{}, false, ErrCommandConflict
	}
	return stored.result, true, nil
}
func (r *memoryRepository) Resolve(_ context.Context, _ Request) (Resolution, error) {
	if r.resolution.OwnerPartyID == "" {
		return Resolution{}, ErrOwnerMissing
	}
	if r.resolution.PersonPartyID == "" {
		return Resolution{}, ErrPersonMapping
	}
	return r.resolution, nil
}
func (r *memoryRepository) Bind(_ context.Context, in Request, obligationID string, revision int64) (Binding, error) {
	binding := Binding{ObligationID: obligationID, ObligationRevision: revision, TotalNetPayableMinor: in.TotalNetPayableMinor, PayrollLineID: in.PayrollLineID}
	r.resolution.Binding = &binding
	return binding, nil
}
func (r *memoryRepository) Record(_ context.Context, in Request, requestDigest string, result Result, _ audit.ActorRef) error {
	if r.recordErr != nil {
		err := r.recordErr
		r.recordErr = nil
		return err
	}
	if r.results == nil {
		r.results = map[string]struct {
			digest string
			result Result
		}{}
	}
	r.results[in.CommandID] = struct {
		digest string
		result Result
	}{requestDigest, result}
	return nil
}

type fakeObligations struct {
	created, confirmed int
	outstanding        int64
}

func (o *fakeObligations) CreateSuggestedDraft(_ context.Context, _ obligation.CreateSuggestedDraftInput) (obligation.CreateResult, error) {
	o.created++
	return obligation.CreateResult{Obligation: obligation.Obligation{ID: testObligation, DraftVersion: 1}}, nil
}
func (o *fakeObligations) Confirm(_ context.Context, _ obligation.ConfirmInput) (obligation.ConfirmResult, error) {
	o.confirmed++
	return obligation.ConfirmResult{Obligation: obligation.Obligation{ID: testObligation}, Revision: obligation.Revision{ObligationID: testObligation, Revision: 1}}, nil
}
func (o *fakeObligations) GetDetails(_ context.Context, id string) (obligation.Details, bool, error) {
	if id != testObligation {
		return obligation.Details{}, false, nil
	}
	return obligation.Details{Obligation: obligation.Obligation{ID: id, OutstandingAmount: minorToDecimal(o.outstanding), CurrentRevision: 1}}, true, nil
}

type fakeCash struct{ calls int }

func (c *fakeCash) Create(_ context.Context, _ cashtransaction.CreateInput) (cashtransaction.CreateResult, error) {
	c.calls++
	return cashtransaction.CreateResult{Transaction: cashtransaction.Transaction{ID: fmt.Sprintf("22000000-0000-4000-8000-%012d", c.calls)}}, nil
}

type fakeAllocations struct {
	calls       int
	obligations *fakeObligations
}

func (a *fakeAllocations) Apply(_ context.Context, input allocation.ApplyInput) (allocation.ApplyResult, error) {
	a.calls++
	minor, err := decimalToMinor(input.Amount)
	if err != nil {
		return allocation.ApplyResult{}, err
	}
	a.obligations.outstanding -= minor
	return allocation.ApplyResult{Allocation: allocation.Allocation{ID: fmt.Sprintf("33000000-0000-4000-8000-%012d", a.calls)}}, nil
}

func TestPayrollPaymentCreatesOneObligationAndSupportsPartialOverageAndReplay(t *testing.T) {
	repository := &memoryRepository{resolution: Resolution{OwnerPartyID: testOwner, PersonPartyID: testPersonParty}, results: map[string]struct {
		digest string
		result Result
	}{}}
	obligations := &fakeObligations{outstanding: 100000}
	cash := &fakeCash{}
	allocations := &fakeAllocations{obligations: obligations}
	service, err := NewService(repository, obligations, cash, allocations)
	if err != nil {
		t.Fatal(err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: testActor, DisplayName: "工资经办人"}
	first := validRequest("44000000-0000-4000-8000-000000000001", 40000)
	result, err := service.Record(context.Background(), actor, first)
	if err != nil || result.AllocatedAmountMinor != 40000 || result.OverageAmountMinor != 0 || result.MoneyAllocationID == "" {
		t.Fatalf("first=%#v err=%v", result, err)
	}
	second := validRequest("44000000-0000-4000-8000-000000000002", 70000)
	second.PayrollLineRevision = 4
	result, err = service.Record(context.Background(), actor, second)
	if err != nil || result.AllocatedAmountMinor != 60000 || result.OverageAmountMinor != 10000 {
		t.Fatalf("second=%#v err=%v", result, err)
	}
	if obligations.created != 1 || obligations.confirmed != 1 || cash.calls != 2 || allocations.calls != 2 || obligations.outstanding != 0 {
		t.Fatalf("created=%d confirmed=%d cash=%d allocations=%d outstanding=%d", obligations.created, obligations.confirmed, cash.calls, allocations.calls, obligations.outstanding)
	}
	replay, err := service.Record(context.Background(), actor, second)
	if err != nil || !replay.Replayed || cash.calls != 2 || allocations.calls != 2 {
		t.Fatalf("replay=%#v err=%v", replay, err)
	}
	conflict := second
	conflict.ActualAmountMinor++
	if _, err = service.Record(context.Background(), actor, conflict); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("changed command payload must conflict: %v", err)
	}
}

func TestPayrollPaymentRecoversAfterMoneyFactsButBeforeDeliveryRecord(t *testing.T) {
	repository := &memoryRepository{resolution: Resolution{OwnerPartyID: testOwner, PersonPartyID: testPersonParty}, results: map[string]struct {
		digest string
		result Result
	}{}, recordErr: errors.New("simulated delivery record interruption")}
	obligations := &fakeObligations{outstanding: 100000}
	cash := &idempotentCash{results: map[string]cashtransaction.CreateResult{}}
	allocations := &idempotentAllocations{results: map[string]allocation.ApplyResult{}, obligations: obligations}
	service, _ := NewService(repository, obligations, cash, allocations)
	input := validRequest("44000000-0000-4000-8000-000000000003", 50000)
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: testActor, DisplayName: "工资经办人"}
	if _, err := service.Record(context.Background(), actor, input); err == nil {
		t.Fatal("simulated post-Money interruption was not returned")
	}
	result, err := service.Record(context.Background(), actor, input)
	if err != nil || result.AllocatedAmountMinor != 50000 || cash.unique != 1 || allocations.unique != 1 {
		t.Fatalf("recovered=%#v unique cash=%d allocations=%d err=%v", result, cash.unique, allocations.unique, err)
	}
}

type idempotentCash struct {
	results map[string]cashtransaction.CreateResult
	unique  int
}

func (c *idempotentCash) Create(_ context.Context, in cashtransaction.CreateInput) (cashtransaction.CreateResult, error) {
	if result, found := c.results[in.CommandID]; found {
		result.Replayed = true
		return result, nil
	}
	c.unique++
	result := cashtransaction.CreateResult{Transaction: cashtransaction.Transaction{ID: "55000000-0000-4000-8000-000000000001"}}
	c.results[in.CommandID] = result
	return result, nil
}

type idempotentAllocations struct {
	results     map[string]allocation.ApplyResult
	obligations *fakeObligations
	unique      int
}

func (a *idempotentAllocations) Apply(_ context.Context, in allocation.ApplyInput) (allocation.ApplyResult, error) {
	if result, found := a.results[in.CommandID]; found {
		result.Replayed = true
		return result, nil
	}
	minor, err := decimalToMinor(in.Amount)
	if err != nil {
		return allocation.ApplyResult{}, err
	}
	a.obligations.outstanding -= minor
	a.unique++
	result := allocation.ApplyResult{Allocation: allocation.Allocation{ID: "66000000-0000-4000-8000-000000000001"}}
	a.results[in.CommandID] = result
	return result, nil
}

func TestPayrollPaymentRequiresTenantOwnerAndExactPersonMapping(t *testing.T) {
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: testActor, DisplayName: "工资经办人"}
	for _, test := range []struct {
		name       string
		resolution Resolution
		want       error
	}{
		{name: "owner", resolution: Resolution{}, want: ErrOwnerMissing},
		{name: "person", resolution: Resolution{OwnerPartyID: testOwner}, want: ErrPersonMapping},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &memoryRepository{resolution: test.resolution, results: map[string]struct {
				digest string
				result Result
			}{}}
			obligations := &fakeObligations{outstanding: 100000}
			service, _ := NewService(repository, obligations, &fakeCash{}, &fakeAllocations{obligations: obligations})
			if _, err := service.Record(context.Background(), actor, validRequest("44000000-0000-4000-8000-000000000004", 10000)); !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func validRequest(command string, amount int64) Request {
	return Request{SchemaVersion: "1", CommandID: command, TenantID: testTenant, PayrollRunID: testRun, PayrollPeriod: "2026-10", PayrollLabel: "十月工资", PayrollLineID: testLine, PayrollLineRevision: 3, PayrollLineDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PersonID: testPerson, PersonRevision: 2, PersonDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", PersonDisplayName: "成员甲", TotalNetPayableMinor: 100000, ActualAmountMinor: amount, OccurredOn: "2026-10-31", Note: "工资实付", PaymentProfileID: testProfile, PaymentProfileRevision: 1, PaymentBankName: "测试银行", MaskedPaymentAccount: "****1234"}
}
