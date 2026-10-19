package obligation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHistoricalBalanceNormalizesAsOfAndReturnsSnapshot(t *testing.T) {
	repository := &repositoryStub{historicalResult: HistoricalSnapshotLookup{
		ObligationFound: true,
		SnapshotFound:   true,
		Snapshot: HistoricalSnapshot{
			ObligationID:      testObligationID,
			Revision:          2,
			EffectiveAmount:   "120",
			OutstandingAmount: "70",
		},
	}}
	service := testService(repository)

	result, err := service.HistoricalBalance(context.Background(), HistoricalBalanceInput{
		ObligationID: " 55555555-5555-4555-8555-555555555555 ",
		AsOf:         "2026-09-19T19:30:00+08:00",
	})
	if err != nil {
		t.Fatalf("HistoricalBalance() error = %v", err)
	}
	wantAsOf := time.Date(2026, 9, 19, 11, 30, 0, 0, time.UTC)
	if repository.historicalCalls != 1 || repository.historicalID != testObligationID ||
		!repository.historicalAsOf.Equal(wantAsOf) {
		t.Fatalf("repository input = calls %d, id %q, as-of %v", repository.historicalCalls, repository.historicalID, repository.historicalAsOf)
	}
	if !result.AsOf.Equal(wantAsOf) || result.Revision != 2 || result.OutstandingAmount != "70" {
		t.Fatalf("snapshot = %#v", result)
	}
}

func TestHistoricalBalanceDefaultsToCurrentServiceTime(t *testing.T) {
	repository := &repositoryStub{historicalResult: HistoricalSnapshotLookup{
		ObligationFound: true,
		SnapshotFound:   true,
		Snapshot:        HistoricalSnapshot{ObligationID: testObligationID},
	}}
	service := testService(repository)

	result, err := service.HistoricalBalance(context.Background(), HistoricalBalanceInput{ObligationID: testObligationID})
	if err != nil {
		t.Fatalf("HistoricalBalance() error = %v", err)
	}
	want := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	if !repository.historicalAsOf.Equal(want) || !result.AsOf.Equal(want) {
		t.Fatalf("as-of = repository %v, snapshot %v", repository.historicalAsOf, result.AsOf)
	}
}

func TestHistoricalBalanceValidationDoesNotCallRepository(t *testing.T) {
	tests := []struct {
		name  string
		input HistoricalBalanceInput
		field string
	}{
		{name: "obligation ID", input: HistoricalBalanceInput{ObligationID: "bad"}, field: "obligation_id"},
		{name: "timezone required", input: HistoricalBalanceInput{ObligationID: testObligationID, AsOf: "2026-09-19T11:00:00"}, field: "as_of"},
		{name: "invalid time", input: HistoricalBalanceInput{ObligationID: testObligationID, AsOf: "not-a-time"}, field: "as_of"},
		{name: "future", input: HistoricalBalanceInput{ObligationID: testObligationID, AsOf: "2026-09-19T12:00:00.000000001Z"}, field: "as_of"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &repositoryStub{}
			_, err := testService(repository).HistoricalBalance(context.Background(), test.input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("HistoricalBalance() error = %v", err)
			}
			if repository.historicalCalls != 0 {
				t.Fatalf("repository calls = %d", repository.historicalCalls)
			}
		})
	}
}

func TestHistoricalBalanceMapsLookupStatesAndPreservesRepositoryError(t *testing.T) {
	asOf := "2026-09-19T11:00:00Z"
	tests := []struct {
		name       string
		repository *repositoryStub
		assert     func(*testing.T, error)
	}{
		{
			name:       "missing obligation",
			repository: &repositoryStub{},
			assert: func(t *testing.T, err error) {
				var target *ObligationNotFoundError
				if !errors.As(err, &target) || target.ObligationID != testObligationID {
					t.Fatalf("error = %v", err)
				}
			},
		},
		{
			name: "before confirmation",
			repository: &repositoryStub{historicalResult: HistoricalSnapshotLookup{
				ObligationFound: true,
			}},
			assert: func(t *testing.T, err error) {
				var target *HistoricalNotConfirmedError
				if !errors.As(err, &target) || target.ObligationID != testObligationID ||
					!target.AsOf.Equal(time.Date(2026, 9, 19, 11, 0, 0, 0, time.UTC)) {
					t.Fatalf("error = %v", err)
				}
			},
		},
		{
			name:       "repository failure",
			repository: &repositoryStub{historicalErr: errors.New("database unavailable")},
			assert: func(t *testing.T, err error) {
				if err == nil || err.Error() != "get historical obligation balance: database unavailable" {
					t.Fatalf("error = %v", err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := testService(test.repository).HistoricalBalance(context.Background(), HistoricalBalanceInput{
				ObligationID: testObligationID,
				AsOf:         asOf,
			})
			test.assert(t, err)
		})
	}
}

func TestHistoricalSnapshotPartyHelpers(t *testing.T) {
	snapshot := HistoricalSnapshot{
		LedgerOwnerPartyID: testOwnerPartyID,
		Creditor:           PartySnapshot{PartyID: testOwnerPartyID},
		Debtor:             PartySnapshot{PartyID: testCounterpartyID},
	}
	if snapshot.LedgerOwner().PartyID != testOwnerPartyID || snapshot.Counterparty().PartyID != testCounterpartyID {
		t.Fatalf("receivable helpers = owner %#v counterparty %#v", snapshot.LedgerOwner(), snapshot.Counterparty())
	}
	snapshot.LedgerOwnerPartyID = testCounterpartyID
	if snapshot.LedgerOwner().PartyID != testCounterpartyID || snapshot.Counterparty().PartyID != testOwnerPartyID {
		t.Fatalf("payable helpers = owner %#v counterparty %#v", snapshot.LedgerOwner(), snapshot.Counterparty())
	}
}
