package obligation

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateDraft(
	ctx context.Context,
	value Obligation,
	command CreateCommand,
	entry audit.Entry,
) (Obligation, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Obligation{}, false, fmt.Errorf("begin create obligation draft: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, command.CommandID); err != nil {
		return Obligation{}, false, fmt.Errorf("lock obligation create command: %w", err)
	}
	var existingObligationID string
	var existingPayloadHash []byte
	err = tx.QueryRow(ctx, `
		SELECT obligation_id::text, payload_hash
		FROM obligation_create_commands
		WHERE command_id = $1
	`, command.CommandID).Scan(&existingObligationID, &existingPayloadHash)
	switch {
	case err == nil:
		if !bytes.Equal(existingPayloadHash, command.PayloadHash) {
			return Obligation{}, false, &IdempotencyConflictError{CommandID: command.CommandID}
		}
		existing, found, err := readObligation(ctx, tx, existingObligationID)
		if err != nil {
			return Obligation{}, false, err
		}
		if !found {
			return Obligation{}, false, fmt.Errorf("obligation create command references missing obligation %s", existingObligationID)
		}
		if err := tx.Commit(ctx); err != nil {
			return Obligation{}, false, fmt.Errorf("commit obligation draft replay: %w", err)
		}
		return existing, true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Obligation{}, false, fmt.Errorf("read obligation create command: %w", err)
	}

	snapshots, err := loadPartySnapshots(ctx, tx, value.Creditor.PartyID, value.Debtor.PartyID)
	if err != nil {
		return Obligation{}, false, err
	}
	creditor, found := snapshots[value.Creditor.PartyID]
	if !found {
		return Obligation{}, false, &PartyNotFoundError{PartyID: value.Creditor.PartyID}
	}
	debtor, found := snapshots[value.Debtor.PartyID]
	if !found {
		return Obligation{}, false, &PartyNotFoundError{PartyID: value.Debtor.PartyID}
	}
	value.Creditor = creditor
	value.Debtor = debtor

	if _, err := tx.Exec(ctx, `
		INSERT INTO obligations (
			id, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			original_amount, currency, due_on,
			source_kind, manual_source_reference,
			source_application, source_resource_type, source_resource_id, source_revision, source_digest,
			status, draft_version, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, NULLIF($9, ''),
			$10, $11, $12, $13, NULLIF($14, ''),
			$15::numeric, $16, $17::date,
			$18, NULLIF($19, ''),
			NULLIF($20, ''), NULLIF($21, ''), NULLIF($22, '')::uuid, NULLIF($23, 0), NULLIF($24, ''),
			$25, $26, $27, $28
		)
	`,
		value.ID,
		value.Direction,
		value.Title,
		value.LedgerOwnerPartyID,
		value.Creditor.PartyID,
		value.Creditor.PartyRevision,
		value.Creditor.Kind,
		value.Creditor.LegalName,
		value.Creditor.RegistrationIdentifier,
		value.Debtor.PartyID,
		value.Debtor.PartyRevision,
		value.Debtor.Kind,
		value.Debtor.LegalName,
		value.Debtor.RegistrationIdentifier,
		value.OriginalAmount,
		value.Currency,
		value.DueOn,
		value.SourceKind,
		value.ManualSourceReference,
		value.SourceApplication,
		value.SourceResourceType,
		value.SourceResourceID,
		value.SourceFactRevision,
		value.SourceDigest,
		value.Status,
		value.DraftVersion,
		value.CreatedAt,
		value.UpdatedAt,
	); err != nil {
		return Obligation{}, false, fmt.Errorf("insert obligation draft: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_create_commands (
			command_id, obligation_id, payload_hash,
			actor_kind, actor_id, actor_display_name, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		command.CommandID,
		value.ID,
		command.PayloadHash,
		command.Actor.Kind,
		command.Actor.ID,
		command.Actor.DisplayName,
		command.CreatedAt,
	); err != nil {
		return Obligation{}, false, fmt.Errorf("insert obligation create command: %w", err)
	}
	if err := audit.Insert(ctx, tx, entry); err != nil {
		return Obligation{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Obligation{}, false, fmt.Errorf("commit create obligation draft: %w", err)
	}
	return value, false, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]Obligation, error) {
	rows, err := r.pool.Query(ctx, obligationSelect+`
		ORDER BY o.created_at DESC, o.id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query obligation drafts: %w", err)
	}
	defer rows.Close()

	values := make([]Obligation, 0)
	for rows.Next() {
		value, err := scanObligation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan obligation draft: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate obligation drafts: %w", err)
	}
	return values, nil
}

func (r *PostgresRepository) Get(ctx context.Context, obligationID string) (Obligation, bool, error) {
	value, found, err := readObligation(ctx, r.pool, obligationID)
	if err != nil {
		return Obligation{}, false, fmt.Errorf("query obligation draft: %w", err)
	}
	return value, found, nil
}

func (r *PostgresRepository) GetDetails(ctx context.Context, obligationID string) (Details, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Details{}, false, fmt.Errorf("begin obligation details read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, found, err := readObligation(ctx, tx, obligationID)
	if err != nil {
		return Details{}, false, fmt.Errorf("query obligation: %w", err)
	}
	if !found {
		if err := tx.Commit(ctx); err != nil {
			return Details{}, false, fmt.Errorf("commit missing obligation details read: %w", err)
		}
		return Details{}, false, nil
	}
	revisions, err := readRevisions(ctx, tx, obligationID)
	if err != nil {
		return Details{}, false, fmt.Errorf("query obligation revisions: %w", err)
	}
	adjustments, err := readAdjustments(ctx, tx, obligationID)
	if err != nil {
		return Details{}, false, fmt.Errorf("query obligation amount adjustments: %w", err)
	}
	remissions, err := readRemissions(ctx, tx, obligationID)
	if err != nil {
		return Details{}, false, fmt.Errorf("query obligation remissions: %w", err)
	}
	closure, closureFound, err := readClosureForObligation(ctx, tx, obligationID)
	if err != nil {
		return Details{}, false, fmt.Errorf("query obligation closure: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Details{}, false, fmt.Errorf("commit obligation details read: %w", err)
	}
	details := Details{Obligation: value, Revisions: revisions, Adjustments: adjustments, Remissions: remissions}
	if closureFound {
		details.Closure = &closure
	}
	return details, true, nil
}

func loadPartySnapshots(ctx context.Context, tx pgx.Tx, firstPartyID, secondPartyID string) (map[string]PartySnapshot, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			id::text,
			revision,
			kind,
			legal_name,
			COALESCE(registration_identifier, '')
		FROM parties
		WHERE id = $1 OR id = $2
		ORDER BY id
		FOR SHARE
	`, firstPartyID, secondPartyID)
	if err != nil {
		return nil, fmt.Errorf("read obligation party snapshots: %w", err)
	}
	defer rows.Close()

	values := make(map[string]PartySnapshot, 2)
	for rows.Next() {
		var value PartySnapshot
		if err := rows.Scan(
			&value.PartyID,
			&value.PartyRevision,
			&value.Kind,
			&value.LegalName,
			&value.RegistrationIdentifier,
		); err != nil {
			return nil, fmt.Errorf("scan obligation party snapshot: %w", err)
		}
		values[value.PartyID] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate obligation party snapshots: %w", err)
	}
	return values, nil
}

const obligationSelect = `
	SELECT
		o.id::text,
		COALESCE(r.direction, o.direction),
		COALESCE(r.title, o.title),
		COALESCE(r.ledger_owner_party_id, o.ledger_owner_party_id)::text,
		COALESCE(r.creditor_party_id, o.creditor_party_id)::text,
		COALESCE(r.creditor_party_revision, o.creditor_party_revision),
		COALESCE(r.creditor_kind, o.creditor_kind),
		COALESCE(r.creditor_legal_name, o.creditor_legal_name),
		COALESCE(r.creditor_registration_identifier, o.creditor_registration_identifier, ''),
		COALESCE(r.debtor_party_id, o.debtor_party_id)::text,
		COALESCE(r.debtor_party_revision, o.debtor_party_revision),
		COALESCE(r.debtor_kind, o.debtor_kind),
		COALESCE(r.debtor_legal_name, o.debtor_legal_name),
		COALESCE(r.debtor_registration_identifier, o.debtor_registration_identifier, ''),
		COALESCE(initial_revision.effective_amount, o.original_amount)::text,
		COALESCE(r.effective_amount, o.original_amount)::text,
		allocation_totals.allocated_amount::text,
		remission_totals.remitted_amount::text,
		(COALESCE(r.effective_amount, o.original_amount) - allocation_totals.allocated_amount - remission_totals.remitted_amount)::text,
		(allocation_totals.allocation_count + allocation_totals.reversal_count + remission_totals.remission_count)::bigint,
		COALESCE(r.currency, o.currency),
		COALESCE(r.due_on, o.due_on)::text,
		COALESCE(r.source_kind, o.source_kind),
		COALESCE(r.manual_source_reference, o.manual_source_reference, ''),
		COALESCE(r.source_application, o.source_application, ''),
		COALESCE(r.source_resource_type, o.source_resource_type, ''),
		COALESCE(r.source_resource_id::text, o.source_resource_id::text, ''),
		COALESCE(r.source_fact_revision, o.source_revision, 0),
		COALESCE(r.source_digest, o.source_digest, ''),
		o.status,
		o.draft_version,
		COALESCE(o.current_revision, 0),
		o.created_at,
		o.updated_at
	FROM obligations o
	LEFT JOIN obligation_revisions r
		ON r.obligation_id = o.id AND r.revision = o.current_revision
	LEFT JOIN obligation_revisions initial_revision
		ON initial_revision.obligation_id = o.id AND initial_revision.revision = 1
	LEFT JOIN LATERAL (
		SELECT
			COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0) AS allocated_amount,
			COUNT(a.id)::bigint AS allocation_count,
			COUNT(ar.id)::bigint AS reversal_count
		FROM allocations a
		LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
		WHERE a.obligation_id = o.id
	) allocation_totals ON true
	LEFT JOIN LATERAL (
		SELECT
			COALESCE(SUM(orx.amount), 0) AS remitted_amount,
			COUNT(orx.id)::bigint AS remission_count
		FROM obligation_remissions orx
		WHERE orx.obligation_id = o.id
	) remission_totals ON true
`

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readObligation(ctx context.Context, queryer queryRower, obligationID string) (Obligation, bool, error) {
	value, err := scanObligation(queryer.QueryRow(ctx, obligationSelect+` WHERE o.id = $1`, obligationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Obligation{}, false, nil
	}
	if err != nil {
		return Obligation{}, false, err
	}
	return value, true, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanObligation(row rowScanner) (Obligation, error) {
	var value Obligation
	err := row.Scan(
		&value.ID,
		&value.Direction,
		&value.Title,
		&value.LedgerOwnerPartyID,
		&value.Creditor.PartyID,
		&value.Creditor.PartyRevision,
		&value.Creditor.Kind,
		&value.Creditor.LegalName,
		&value.Creditor.RegistrationIdentifier,
		&value.Debtor.PartyID,
		&value.Debtor.PartyRevision,
		&value.Debtor.Kind,
		&value.Debtor.LegalName,
		&value.Debtor.RegistrationIdentifier,
		&value.OriginalAmount,
		&value.EffectiveAmount,
		&value.AllocatedAmount,
		&value.RemittedAmount,
		&value.OutstandingAmount,
		&value.BalanceSequence,
		&value.Currency,
		&value.DueOn,
		&value.SourceKind,
		&value.ManualSourceReference,
		&value.SourceApplication,
		&value.SourceResourceType,
		&value.SourceResourceID,
		&value.SourceFactRevision,
		&value.SourceDigest,
		&value.Status,
		&value.DraftVersion,
		&value.CurrentRevision,
		&value.CreatedAt,
		&value.UpdatedAt,
	)
	if err != nil {
		return Obligation{}, err
	}
	normalizedAmount, err := normalizeAmount(value.OriginalAmount)
	if err != nil {
		return Obligation{}, fmt.Errorf("invalid stored obligation amount: %w", err)
	}
	value.OriginalAmount = normalizedAmount
	normalizedEffectiveAmount, err := normalizeAmount(value.EffectiveAmount)
	if err != nil {
		return Obligation{}, fmt.Errorf("invalid stored obligation effective amount: %w", err)
	}
	value.EffectiveAmount = normalizedEffectiveAmount
	normalizedAllocatedAmount, err := normalizeAmountAllowZero(value.AllocatedAmount)
	if err != nil {
		return Obligation{}, fmt.Errorf("invalid stored obligation allocated amount: %w", err)
	}
	value.AllocatedAmount = normalizedAllocatedAmount
	normalizedRemittedAmount, err := normalizeAmountAllowZero(value.RemittedAmount)
	if err != nil {
		return Obligation{}, fmt.Errorf("invalid stored obligation remitted amount: %w", err)
	}
	value.RemittedAmount = normalizedRemittedAmount
	normalizedOutstandingAmount, err := normalizeAmountAllowZero(value.OutstandingAmount)
	if err != nil {
		return Obligation{}, fmt.Errorf("invalid stored obligation outstanding amount: %w", err)
	}
	value.OutstandingAmount = normalizedOutstandingAmount
	return value, nil
}
