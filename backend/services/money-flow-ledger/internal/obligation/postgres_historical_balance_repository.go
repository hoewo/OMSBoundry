package obligation

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) GetHistoricalSnapshot(
	ctx context.Context,
	obligationID string,
	asOf time.Time,
) (HistoricalSnapshotLookup, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return HistoricalSnapshotLookup{}, fmt.Errorf("begin historical obligation read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var obligationFound bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM obligations WHERE id = $1)`, obligationID).Scan(&obligationFound); err != nil {
		return HistoricalSnapshotLookup{}, fmt.Errorf("find historical obligation root: %w", err)
	}
	if !obligationFound {
		if err := tx.Commit(ctx); err != nil {
			return HistoricalSnapshotLookup{}, fmt.Errorf("commit missing historical obligation read: %w", err)
		}
		return HistoricalSnapshotLookup{}, nil
	}

	var snapshot HistoricalSnapshot
	var closureID, closureBasis string
	var closureAt time.Time
	err = tx.QueryRow(ctx, `
		WITH historical_revision AS (
			SELECT *
			FROM obligation_revisions
			WHERE obligation_id = $1
			  AND recorded_at <= $2
			ORDER BY revision DESC
			LIMIT 1
		), allocation_totals AS (
			SELECT
				COALESCE(SUM(
					allocation.amount
					- CASE WHEN reversal.reversed_at <= $2 THEN reversal.amount ELSE 0 END
				), 0) AS net_allocated,
				COUNT(allocation.id)::bigint AS allocation_count,
				(COUNT(reversal.id) FILTER (WHERE reversal.reversed_at <= $2))::bigint AS reversal_count
			FROM allocations allocation
			LEFT JOIN allocation_reversals reversal ON reversal.allocation_id = allocation.id
			WHERE allocation.obligation_id = $1
			  AND allocation.applied_at <= $2
		), remission_totals AS (
			SELECT
				COALESCE(SUM(remission.amount), 0) AS remitted,
				COUNT(remission.id)::bigint AS remission_count
			FROM obligation_remissions remission
			WHERE remission.obligation_id = $1
			  AND remission.recorded_at <= $2
		), historical_closure AS (
			SELECT id, settlement_basis, closed_at
			FROM obligation_closures
			WHERE obligation_id = $1
			  AND closed_at <= $2
		)
		SELECT
			revision.obligation_id::text,
			revision.revision,
			revision.direction,
			revision.title,
			revision.ledger_owner_party_id::text,
			revision.creditor_party_id::text,
			revision.creditor_party_revision,
			revision.creditor_kind,
			revision.creditor_legal_name,
			COALESCE(revision.creditor_registration_identifier, ''),
			revision.debtor_party_id::text,
			revision.debtor_party_revision,
			revision.debtor_kind,
			revision.debtor_legal_name,
			COALESCE(revision.debtor_registration_identifier, ''),
			revision.effective_amount::text,
			allocation_totals.net_allocated::text,
			remission_totals.remitted::text,
			(revision.effective_amount - allocation_totals.net_allocated - remission_totals.remitted)::text,
			(allocation_totals.allocation_count + allocation_totals.reversal_count + remission_totals.remission_count)::bigint,
			revision.currency,
			revision.due_on::text,
			revision.recorded_at,
			CASE WHEN closure.id IS NULL THEN 'open' ELSE 'closed' END,
			COALESCE(closure.id::text, ''),
			COALESCE(closure.settlement_basis, ''),
			COALESCE(closure.closed_at, 'epoch'::timestamptz)
		FROM historical_revision revision
		CROSS JOIN allocation_totals
		CROSS JOIN remission_totals
		LEFT JOIN historical_closure closure ON true
	`, obligationID, asOf).Scan(
		&snapshot.ObligationID,
		&snapshot.Revision,
		&snapshot.Direction,
		&snapshot.Title,
		&snapshot.LedgerOwnerPartyID,
		&snapshot.Creditor.PartyID,
		&snapshot.Creditor.PartyRevision,
		&snapshot.Creditor.Kind,
		&snapshot.Creditor.LegalName,
		&snapshot.Creditor.RegistrationIdentifier,
		&snapshot.Debtor.PartyID,
		&snapshot.Debtor.PartyRevision,
		&snapshot.Debtor.Kind,
		&snapshot.Debtor.LegalName,
		&snapshot.Debtor.RegistrationIdentifier,
		&snapshot.EffectiveAmount,
		&snapshot.NetAllocatedAmount,
		&snapshot.RemittedAmount,
		&snapshot.OutstandingAmount,
		&snapshot.BalanceSequence,
		&snapshot.Currency,
		&snapshot.DueOn,
		&snapshot.RevisionRecordedAt,
		&snapshot.Status,
		&closureID,
		&closureBasis,
		&closureAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return HistoricalSnapshotLookup{}, fmt.Errorf("commit pre-confirmation historical read: %w", err)
		}
		return HistoricalSnapshotLookup{ObligationFound: true}, nil
	}
	if err != nil {
		return HistoricalSnapshotLookup{}, fmt.Errorf("query historical obligation snapshot: %w", err)
	}

	if err := normalizeHistoricalSnapshotAmounts(&snapshot); err != nil {
		return HistoricalSnapshotLookup{}, err
	}
	if closureID != "" {
		snapshot.Closure = &HistoricalClosure{
			ID:              closureID,
			SettlementBasis: ClosureBasis(closureBasis),
			ClosedAt:        closureAt,
		}
	}
	if err := validateHistoricalSnapshot(snapshot, asOf); err != nil {
		return HistoricalSnapshotLookup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return HistoricalSnapshotLookup{}, fmt.Errorf("commit historical obligation read: %w", err)
	}
	return HistoricalSnapshotLookup{ObligationFound: true, SnapshotFound: true, Snapshot: snapshot}, nil
}

func normalizeHistoricalSnapshotAmounts(snapshot *HistoricalSnapshot) error {
	fields := []struct {
		name  string
		value *string
	}{
		{name: "effective", value: &snapshot.EffectiveAmount},
		{name: "net allocated", value: &snapshot.NetAllocatedAmount},
		{name: "remitted", value: &snapshot.RemittedAmount},
		{name: "outstanding", value: &snapshot.OutstandingAmount},
	}
	for _, field := range fields {
		normalized, err := normalizeAmountAllowZero(*field.value)
		if err != nil {
			return fmt.Errorf("invalid historical %s amount: %w", field.name, err)
		}
		*field.value = normalized
	}
	return nil
}

func validateHistoricalSnapshot(snapshot HistoricalSnapshot, asOf time.Time) error {
	effective, err := decimalToMicros(snapshot.EffectiveAmount)
	if err != nil {
		return err
	}
	allocated, err := decimalToMicros(snapshot.NetAllocatedAmount)
	if err != nil {
		return err
	}
	remitted, err := decimalToMicros(snapshot.RemittedAmount)
	if err != nil {
		return err
	}
	outstanding, err := decimalToMicros(snapshot.OutstandingAmount)
	if err != nil {
		return err
	}
	wantOutstanding := new(big.Int).Sub(new(big.Int).Set(effective), allocated)
	wantOutstanding.Sub(wantOutstanding, remitted)
	if allocated.Sign() < 0 || outstanding.Sign() < 0 || outstanding.Cmp(wantOutstanding) != 0 ||
		(snapshot.Status == StatusClosed && outstanding.Sign() != 0) ||
		(snapshot.Status == StatusOpen && snapshot.Closure != nil) {
		return &HistoricalInvariantError{ObligationID: snapshot.ObligationID, AsOf: asOf, Reason: "balance or status mismatch"}
	}
	return nil
}
