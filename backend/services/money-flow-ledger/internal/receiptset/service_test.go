package receiptset

import (
	"context"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type memoryRepository struct {
	value Set
	ids   []string
}

func (r *memoryRepository) Confirm(_ context.Context, value Set, ids []string, _ Command, _ audit.Entry) (Set, bool, error) {
	r.value, r.ids = value, append([]string(nil), ids...)
	value.Items = []Item{{TransactionID: "10000000-0000-4000-8000-000000000009", TransactionRevision: 1, AllocationID: ids[0], AllocationRevision: 1, AmountMinor: 1250, Currency: CurrencyCNY, Status: StatusConfirmed}}
	value.TotalAmountMinor = 1250
	return value, false, nil
}
func (r *memoryRepository) Get(_ context.Context, tenantID, setID string) (Set, error) {
	if r.value.TenantID != tenantID || r.value.ID != setID {
		return Set{}, ErrNotFound
	}
	return r.value, nil
}

func TestConfirmNormalizesAndSortsExactAllocationSet(t *testing.T) {
	repository := &memoryRepository{}
	service := NewService(repository)
	service.now = func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) }
	ids := []string{"30000000-0000-4000-8000-000000000002", "30000000-0000-4000-8000-000000000001"}
	result, err := service.Confirm(context.Background(), ConfirmInput{
		CommandID: "20000000-0000-4000-8000-000000000001", TenantID: "10000000-0000-4000-8000-000000000001",
		ProjectID: "10000000-0000-4000-8000-000000000002", AllocationIDs: ids,
		Actor: audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "10000000-0000-4000-8000-000000000003", DisplayName: "资金负责人"},
	})
	if err != nil || result.Set.Status != StatusConfirmed || result.Set.Currency != CurrencyCNY || result.Set.Revision != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(repository.ids) != 2 || repository.ids[0] != "30000000-0000-4000-8000-000000000001" || repository.ids[1] != "30000000-0000-4000-8000-000000000002" {
		t.Fatalf("normalized ids=%#v", repository.ids)
	}
}

func TestConfirmRejectsDuplicateOrInvalidBoundary(t *testing.T) {
	service := NewService(&memoryRepository{})
	valid := ConfirmInput{CommandID: "20000000-0000-4000-8000-000000000001", TenantID: "10000000-0000-4000-8000-000000000001", ProjectID: "10000000-0000-4000-8000-000000000002", AllocationIDs: []string{"30000000-0000-4000-8000-000000000001"}, Actor: audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "10000000-0000-4000-8000-000000000003", DisplayName: "资金负责人"}}
	duplicate := valid
	duplicate.AllocationIDs = []string{valid.AllocationIDs[0], valid.AllocationIDs[0]}
	if _, err := service.Confirm(context.Background(), duplicate); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate error=%v", err)
	}
	empty := valid
	empty.AllocationIDs = nil
	if _, err := service.Confirm(context.Background(), empty); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty error=%v", err)
	}
}

func TestDecimalToMinorIsExactAndNeverRounds(t *testing.T) {
	for raw, want := range map[string]int64{"1065.000000": 106500, "0.01": 1, "12.3": 1230} {
		got, err := decimalToMinor(raw)
		if err != nil || got != want {
			t.Fatalf("decimalToMinor(%q)=%d,%v want=%d", raw, got, err, want)
		}
	}
	for _, raw := range []string{"1.001", "999999999999999999999999.00", "bad"} {
		if _, err := decimalToMinor(raw); !errors.Is(err, ErrStateConflict) {
			t.Fatalf("decimalToMinor(%q) error=%v", raw, err)
		}
	}
}
