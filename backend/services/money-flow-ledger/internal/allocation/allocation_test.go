package allocation

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const (
	testCommandID    = "11111111-1111-4111-8111-111111111111"
	testActorID      = "22222222-2222-4222-8222-222222222222"
	testCashID       = "33333333-3333-4333-8333-333333333333"
	testObligationID = "44444444-4444-4444-8444-444444444444"
	testAllocationID = "55555555-5555-4555-8555-555555555555"
	testAuditID      = "66666666-6666-4666-8666-666666666666"
)

type repositoryStub struct {
	appliedValue            Allocation
	appliedCommand          ApplyCommand
	appliedAudit            audit.Entry
	applyResult             Allocation
	replayed                bool
	applyErr                error
	listResult              []Allocation
	listErr                 error
	listForCashResult       []Allocation
	listForCashErr          error
	listForCashID           string
	listForCashCalls        int
	listForObligationResult []Allocation
	listForObligationErr    error
	listForObligationID     string
	listForObligationCalls  int
	getResult               Allocation
	getFound                bool
	getErr                  error
	getID                   string
	getCalls                int
	reversedValue           Reversal
	reversedCommand         ReverseCommand
	reversedAudit           audit.Entry
	reverseResult           Reversal
	reverseReplayed         bool
	reverseErr              error
}

func (r *repositoryStub) Apply(_ context.Context, value Allocation, command ApplyCommand, entry audit.Entry) (Allocation, bool, error) {
	r.appliedValue = value
	r.appliedCommand = command
	r.appliedAudit = entry
	if r.applyErr != nil {
		return Allocation{}, false, r.applyErr
	}
	if r.applyResult.ID != "" {
		return r.applyResult, r.replayed, nil
	}
	return value, r.replayed, nil
}

func (r *repositoryStub) Reverse(_ context.Context, value Reversal, command ReverseCommand, entry audit.Entry) (Reversal, bool, error) {
	r.reversedValue = value
	r.reversedCommand = command
	r.reversedAudit = entry
	if r.reverseErr != nil {
		return Reversal{}, false, r.reverseErr
	}
	if r.reverseResult.ID != "" {
		return r.reverseResult, r.reverseReplayed, nil
	}
	return value, r.reverseReplayed, nil
}

func (r *repositoryStub) List(context.Context) ([]Allocation, error) {
	return r.listResult, r.listErr
}

func (r *repositoryStub) ListForCashTransaction(_ context.Context, id string) ([]Allocation, error) {
	r.listForCashCalls++
	r.listForCashID = id
	return r.listForCashResult, r.listForCashErr
}

func (r *repositoryStub) ListForObligation(_ context.Context, id string) ([]Allocation, error) {
	r.listForObligationCalls++
	r.listForObligationID = id
	return r.listForObligationResult, r.listForObligationErr
}

func (r *repositoryStub) Get(_ context.Context, id string) (Allocation, bool, error) {
	r.getCalls++
	r.getID = id
	return r.getResult, r.getFound, r.getErr
}

func testActor() audit.ActorRef {
	return audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "资金管理员"}
}

func validApplyInput() ApplyInput {
	return ApplyInput{
		CommandID:                  testCommandID,
		CashTransactionID:          testCashID,
		ObligationID:               testObligationID,
		ExpectedObligationRevision: 1,
		Amount:                     "60.5",
		ManualSourceReference:      "人工核对回单与应收义务",
		Actor:                      testActor(),
	}
}

func testService(repository Repository) *Service {
	service := NewService(repository)
	ids := []string{testAllocationID, testAuditID}
	service.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	service.now = func() time.Time {
		return time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)
	}
	return service
}

func TestApplyNormalizesInputAndBuildsAudit(t *testing.T) {
	repository := &repositoryStub{}
	input := validApplyInput()
	input.CommandID = " 11111111-1111-4111-8111-111111111111 "
	input.CashTransactionID = " 33333333-3333-4333-8333-333333333333 "
	input.ObligationID = " 44444444-4444-4444-8444-444444444444 "
	input.Amount = "000060.500000"
	input.ManualSourceReference = "  人工核对回单与应收义务  "

	result, err := testService(repository).Apply(context.Background(), input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	value := result.Allocation
	if result.Replayed || value.ID != testAllocationID || value.CashTransactionID != testCashID ||
		value.ObligationID != testObligationID || value.ObligationRevision != 1 || value.Amount != "60.5" ||
		value.SourceKind != SourceKindManual || value.ManualSourceReference != "人工核对回单与应收义务" ||
		value.Status != StatusActive || value.AppliedBy != testActor() || !value.AppliedAt.Equal(time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("allocation = %#v", value)
	}
	if repository.appliedCommand.CommandID != testCommandID || len(repository.appliedCommand.PayloadHash) != 32 ||
		repository.appliedCommand.Actor != testActor() {
		t.Fatalf("command = %#v", repository.appliedCommand)
	}
	entry := repository.appliedAudit
	if entry.ID != testAuditID || entry.ObjectType != audit.ObjectAllocation || entry.ObjectID != testAllocationID ||
		entry.Action != AuditActionApplied || entry.CommandID != testCommandID || entry.Actor != testActor() {
		t.Fatalf("audit = %#v", entry)
	}
	if entry.Details["cash_transaction_id"] != testCashID || entry.Details["obligation_id"] != testObligationID ||
		entry.Details["obligation_revision"] != int64(1) || entry.Details["amount"] != "60.5" ||
		entry.Details["source_kind"] != SourceKindManual {
		t.Fatalf("audit details = %#v", entry.Details)
	}
}

func TestApplyValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ApplyInput)
		field  string
	}{
		{name: "command", mutate: func(v *ApplyInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "cash", mutate: func(v *ApplyInput) { v.CashTransactionID = "bad" }, field: "cash_transaction_id"},
		{name: "obligation", mutate: func(v *ApplyInput) { v.ObligationID = "bad" }, field: "obligation_id"},
		{name: "revision", mutate: func(v *ApplyInput) { v.ExpectedObligationRevision = 0 }, field: "expected_obligation_revision"},
		{name: "amount empty", mutate: func(v *ApplyInput) { v.Amount = "" }, field: "amount"},
		{name: "amount zero", mutate: func(v *ApplyInput) { v.Amount = "0.000000" }, field: "amount"},
		{name: "amount negative", mutate: func(v *ApplyInput) { v.Amount = "-1" }, field: "amount"},
		{name: "amount exponent", mutate: func(v *ApplyInput) { v.Amount = "1e3" }, field: "amount"},
		{name: "amount integer precision", mutate: func(v *ApplyInput) { v.Amount = "1234567890123456789" }, field: "amount"},
		{name: "amount scale", mutate: func(v *ApplyInput) { v.Amount = "1.0000001" }, field: "amount"},
		{name: "source empty", mutate: func(v *ApplyInput) { v.ManualSourceReference = " " }, field: "manual_source_reference"},
		{name: "source long", mutate: func(v *ApplyInput) { v.ManualSourceReference = strings.Repeat("界", 201) }, field: "manual_source_reference"},
		{name: "actor", mutate: func(v *ApplyInput) { v.Actor.ID = "bad" }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validApplyInput()
			test.mutate(&input)
			repository := &repositoryStub{}
			_, err := testService(repository).Apply(context.Background(), input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("Apply() error = %v", err)
			}
			if repository.appliedValue.ID != "" {
				t.Fatalf("repository called with %#v", repository.appliedValue)
			}
		})
	}
}

func TestAmountNormalizationAndComparisonAreExact(t *testing.T) {
	tests := map[string]string{
		"0.000001":                  "0.000001",
		"000060.500000":             "60.5",
		"0000001":                   "1",
		"999999999999999999.999999": "999999999999999999.999999",
	}
	for input, want := range tests {
		got, err := normalizeAmount(input)
		if err != nil || got != want {
			t.Errorf("normalizeAmount(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, test := range []struct {
		left, right string
		want        int
	}{
		{left: "1", right: "1.000000", want: 0},
		{left: "0.000001", right: "0.000002", want: -1},
		{left: "999999999999999999.999999", right: "2", want: 1},
	} {
		got, err := compareAmounts(test.left, test.right)
		if err != nil || got != test.want {
			t.Errorf("compareAmounts(%q, %q) = %d, %v; want %d", test.left, test.right, got, err, test.want)
		}
	}
}

func TestPayloadHashUsesNormalizedIntentAndTrustedActor(t *testing.T) {
	first, err := normalizeApplyInput(validApplyInput())
	if err != nil {
		t.Fatal(err)
	}
	secondInput := validApplyInput()
	secondInput.CashTransactionID = " 33333333-3333-4333-8333-333333333333 "
	secondInput.ObligationID = " 44444444-4444-4444-8444-444444444444 "
	secondInput.Amount = "000060.500000"
	secondInput.ManualSourceReference = " 人工核对回单与应收义务 "
	second, err := normalizeApplyInput(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	firstHash, _ := applyPayloadHash(first)
	secondHash, _ := applyPayloadHash(second)
	if !bytes.Equal(firstHash, secondHash) {
		t.Fatalf("normalized hashes differ: %x %x", firstHash, secondHash)
	}
	second.Actor.ID = "77777777-7777-4777-8777-777777777777"
	changedActorHash, _ := applyPayloadHash(second)
	if bytes.Equal(firstHash, changedActorHash) {
		t.Fatal("actor identity is absent from payload hash")
	}
	second = first
	second.ExpectedObligationRevision = 2
	changedRevisionHash, _ := applyPayloadHash(second)
	if bytes.Equal(firstHash, changedRevisionHash) {
		t.Fatal("expected revision is absent from payload hash")
	}
}

func TestApplyReturnsReplayAndPreservesTypedErrors(t *testing.T) {
	replayed := Allocation{ID: "88888888-8888-4888-8888-888888888888", Amount: "10"}
	repository := &repositoryStub{applyResult: replayed, replayed: true}
	result, err := testService(repository).Apply(context.Background(), validApplyInput())
	if err != nil || !result.Replayed || result.Allocation.ID != replayed.ID {
		t.Fatalf("Apply() = %#v, %v", result, err)
	}
	for _, typedError := range []error{
		&IdempotencyConflictError{CommandID: testCommandID},
		&CashTransactionNotFoundError{CashTransactionID: testCashID},
		&ObligationNotFoundError{ObligationID: testObligationID},
		&ObligationStateConflictError{Status: "draft"},
		&ObligationRevisionConflictError{Expected: 1, Actual: 2},
		&CompatibilityConflictError{Reason: "currency"},
		&CashBalanceConflictError{Requested: "60.5", Available: "10"},
		&ObligationBalanceConflictError{Requested: "60.5", Available: "10"},
	} {
		repository = &repositoryStub{applyErr: typedError}
		_, err = testService(repository).Apply(context.Background(), validApplyInput())
		if !errors.Is(err, typedError) {
			t.Fatalf("Apply() error = %v, want wrapped %T", err, typedError)
		}
	}
}

func TestListAndGet(t *testing.T) {
	want := Allocation{ID: testAllocationID, Amount: "60.5"}
	repository := &repositoryStub{listResult: []Allocation{want}, getResult: want, getFound: true}
	service := NewService(repository)
	listed, err := service.List(context.Background())
	if err != nil || len(listed) != 1 || listed[0].ID != want.ID {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	got, found, err := service.Get(context.Background(), " 55555555-5555-4555-8555-555555555555 ")
	if err != nil || !found || got.ID != want.ID || repository.getID != testAllocationID {
		t.Fatalf("Get() = %#v, %v, %v; repository ID=%q", got, found, err, repository.getID)
	}
	_, _, err = service.Get(context.Background(), "bad")
	var validationError *ValidationError
	if !errors.As(err, &validationError) || repository.getCalls != 1 {
		t.Fatalf("invalid Get() error=%v calls=%d", err, repository.getCalls)
	}
}

func TestScopedAllocationHistory(t *testing.T) {
	want := Allocation{ID: testAllocationID, Amount: "60.5"}
	repository := &repositoryStub{
		listForCashResult:       []Allocation{want},
		listForObligationResult: []Allocation{want},
	}
	service := NewService(repository)

	byCash, err := service.ListForCashTransaction(context.Background(), " 33333333-3333-4333-8333-333333333333 ")
	if err != nil || len(byCash) != 1 || byCash[0].ID != want.ID ||
		repository.listForCashID != testCashID || repository.listForCashCalls != 1 {
		t.Fatalf("ListForCashTransaction() = %#v, %v; id=%q calls=%d", byCash, err, repository.listForCashID, repository.listForCashCalls)
	}
	byObligation, err := service.ListForObligation(context.Background(), " 44444444-4444-4444-8444-444444444444 ")
	if err != nil || len(byObligation) != 1 || byObligation[0].ID != want.ID ||
		repository.listForObligationID != testObligationID || repository.listForObligationCalls != 1 {
		t.Fatalf("ListForObligation() = %#v, %v; id=%q calls=%d", byObligation, err, repository.listForObligationID, repository.listForObligationCalls)
	}

	_, err = service.ListForCashTransaction(context.Background(), "bad")
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Field != "cash_transaction_id" || repository.listForCashCalls != 1 {
		t.Fatalf("invalid cash history error=%v calls=%d", err, repository.listForCashCalls)
	}
	_, err = service.ListForObligation(context.Background(), "bad")
	if !errors.As(err, &validationError) || validationError.Field != "obligation_id" || repository.listForObligationCalls != 1 {
		t.Fatalf("invalid obligation history error=%v calls=%d", err, repository.listForObligationCalls)
	}
}

func TestListAndGetPreserveRepositoryErrors(t *testing.T) {
	sentinel := errors.New("database unavailable")
	service := NewService(&repositoryStub{listErr: sentinel})
	if _, err := service.List(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("List() error = %v", err)
	}
	service = NewService(&repositoryStub{getErr: sentinel})
	if _, _, err := service.Get(context.Background(), testAllocationID); !errors.Is(err, sentinel) {
		t.Fatalf("Get() error = %v", err)
	}
	service = NewService(&repositoryStub{listForCashErr: sentinel})
	if _, err := service.ListForCashTransaction(context.Background(), testCashID); !errors.Is(err, sentinel) {
		t.Fatalf("ListForCashTransaction() error = %v", err)
	}
	service = NewService(&repositoryStub{listForObligationErr: sentinel})
	if _, err := service.ListForObligation(context.Background(), testObligationID); !errors.Is(err, sentinel) {
		t.Fatalf("ListForObligation() error = %v", err)
	}
}

func TestNewCommandID(t *testing.T) {
	id, err := NewCommandID()
	if err != nil || !uuidPattern.MatchString(id) {
		t.Fatalf("NewCommandID() = %q, %v", id, err)
	}
}
