package obligation

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type HistoricalBalanceInput struct {
	ObligationID string
	AsOf         string
}

type HistoricalClosure struct {
	ID              string
	SettlementBasis ClosureBasis
	ClosedAt        time.Time
}

type HistoricalSnapshot struct {
	AsOf               time.Time
	ObligationID       string
	Revision           int64
	Direction          Direction
	Title              string
	LedgerOwnerPartyID string
	Creditor           PartySnapshot
	Debtor             PartySnapshot
	EffectiveAmount    string
	NetAllocatedAmount string
	RemittedAmount     string
	OutstandingAmount  string
	BalanceSequence    int64
	Currency           string
	DueOn              string
	RevisionRecordedAt time.Time
	Status             string
	Closure            *HistoricalClosure
}

func (s HistoricalSnapshot) LedgerOwner() PartySnapshot {
	if s.Creditor.PartyID == s.LedgerOwnerPartyID {
		return s.Creditor
	}
	return s.Debtor
}

func (s HistoricalSnapshot) Counterparty() PartySnapshot {
	if s.Creditor.PartyID == s.LedgerOwnerPartyID {
		return s.Debtor
	}
	return s.Creditor
}

type HistoricalSnapshotLookup struct {
	ObligationFound bool
	SnapshotFound   bool
	Snapshot        HistoricalSnapshot
}

type HistoricalNotConfirmedError struct {
	ObligationID string
	AsOf         time.Time
}

func (e *HistoricalNotConfirmedError) Error() string {
	return "该时刻义务尚未确认，draft 不形成正式余额"
}

type HistoricalInvariantError struct {
	ObligationID string
	AsOf         time.Time
	Reason       string
}

func (e *HistoricalInvariantError) Error() string {
	return "历史义务事实无法形成一致余额"
}

func (s *Service) HistoricalBalance(ctx context.Context, input HistoricalBalanceInput) (HistoricalSnapshot, error) {
	id := strings.ToLower(strings.TrimSpace(input.ObligationID))
	if !uuidPattern.MatchString(id) {
		return HistoricalSnapshot{}, &ValidationError{Field: "obligation_id", Message: "义务标识无效"}
	}
	now := s.now().UTC()
	asOf, err := normalizeHistoricalAsOf(input.AsOf, now)
	if err != nil {
		return HistoricalSnapshot{}, err
	}
	lookup, err := s.repository.GetHistoricalSnapshot(ctx, id, asOf)
	if err != nil {
		return HistoricalSnapshot{}, fmt.Errorf("get historical obligation balance: %w", err)
	}
	if !lookup.ObligationFound {
		return HistoricalSnapshot{}, &ObligationNotFoundError{ObligationID: id}
	}
	if !lookup.SnapshotFound {
		return HistoricalSnapshot{}, &HistoricalNotConfirmedError{ObligationID: id, AsOf: asOf}
	}
	lookup.Snapshot.AsOf = asOf
	return lookup.Snapshot, nil
}

func normalizeHistoricalAsOf(raw string, now time.Time) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return now, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, &ValidationError{Field: "as_of", Message: "历史时刻必须是带时区的 RFC3339 时间"}
	}
	parsed = parsed.UTC()
	if parsed.After(now) {
		return time.Time{}, &ValidationError{Field: "as_of", Message: "历史余额不支持未来时刻；未来值不等于预测"}
	}
	return parsed, nil
}
