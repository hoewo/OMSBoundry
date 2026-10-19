package cashevidence

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Current(ctx context.Context, id string) (Snapshot, bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cash_transactions WHERE id=$1)`, id).Scan(&exists); err != nil {
		return Snapshot{}, false, err
	}
	if !exists {
		return Snapshot{}, false, nil
	}
	value := Snapshot{CashTransactionID: id, EvidenceRefs: []Ref{}}
	var refs []byte
	err := r.pool.QueryRow(ctx, `
		SELECT revision,evidence_refs,reason,actor_kind,actor_id,actor_display_name,recorded_at
		FROM cash_transaction_evidence_revisions
		WHERE cash_transaction_id=$1 ORDER BY revision DESC LIMIT 1
	`, id).Scan(&value.Revision, &refs, &value.Reason, &value.Actor.Kind, &value.Actor.ID, &value.Actor.DisplayName, &value.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, true, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	if err := json.Unmarshal(refs, &value.EvidenceRefs); err != nil {
		return Snapshot{}, false, err
	}
	return value, true, nil
}

func (r *PostgresRepository) Replace(ctx context.Context, input ReplaceInput, hash []byte, now time.Time) (Result, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('cash-evidence:'||$1,0))`, input.CashTransactionID); err != nil {
		return Result{}, err
	}
	var existingHash, existingRefs []byte
	var existingRevision int64
	err = tx.QueryRow(ctx, `SELECT intent_sha256,revision,evidence_refs FROM cash_transaction_evidence_revisions WHERE command_id=$1`, input.CommandID).Scan(&existingHash, &existingRevision, &existingRefs)
	if err == nil {
		if !bytes.Equal(existingHash, hash) {
			return Result{}, ErrIdempotencyConflict
		}
		var refs []Ref
		if err = json.Unmarshal(existingRefs, &refs); err != nil {
			return Result{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return Result{}, err
		}
		return Result{Snapshot: Snapshot{CashTransactionID: input.CashTransactionID, Revision: existingRevision, EvidenceRefs: refs}, Replayed: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}
	var cashTransactionID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM cash_transactions WHERE id=$1 FOR SHARE`, input.CashTransactionID).Scan(&cashTransactionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, err
	}
	var current int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(revision),0) FROM cash_transaction_evidence_revisions WHERE cash_transaction_id=$1`, input.CashTransactionID).Scan(&current); err != nil {
		return Result{}, err
	}
	if current != input.ExpectedRevision {
		return Result{}, ErrRevisionConflict
	}
	refs, err := json.Marshal(input.EvidenceRefs)
	if err != nil {
		return Result{}, err
	}
	revision := current + 1
	if _, err = tx.Exec(ctx, `
		INSERT INTO cash_transaction_evidence_revisions(cash_transaction_id,revision,command_id,intent_sha256,evidence_refs,reason,actor_kind,actor_id,actor_display_name,recorded_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, input.CashTransactionID, revision, input.CommandID, hash, refs, input.Reason, input.Actor.Kind, input.Actor.ID, input.Actor.DisplayName, now); err != nil {
		return Result{}, err
	}
	auditID, err := randomUUID()
	if err != nil {
		return Result{}, err
	}
	if err = audit.Insert(ctx, tx, audit.Entry{ID: auditID, ObjectType: audit.ObjectCashTransaction, ObjectID: input.CashTransactionID, Action: "cash_transaction_evidence_replaced", CommandID: input.CommandID, Actor: input.Actor, OccurredAt: now, Summary: "更新实际收付款凭证引用", Details: map[string]any{"evidence_revision": revision, "evidence_count": len(input.EvidenceRefs), "physical_delete": false}}); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{Snapshot: Snapshot{CashTransactionID: input.CashTransactionID, Revision: revision, EvidenceRefs: append([]Ref(nil), input.EvidenceRefs...), Reason: input.Reason, Actor: snapshotActor(input.Actor), RecordedAt: now}}, nil
}

func randomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate audit id: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
