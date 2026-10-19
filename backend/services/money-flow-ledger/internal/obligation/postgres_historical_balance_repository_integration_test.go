//go:build integration

package obligation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresHistoricalObligationBalanceReplaysRecordedFacts(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "历史余额管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))

	draft, err := service.CreateDraft(ctx, CreateDraftInput{
		CommandID: "41000000-0000-4000-8000-000000000001", Direction: "receivable",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Title: "历史余额服务费", OriginalAmount: "100", Currency: "CNY",
		DueOn: "2026-12-31", ManualSourceReference: "历史余额义务依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	confirmed, err := service.Confirm(ctx, ConfirmInput{
		CommandID: "42000000-0000-4000-8000-000000000001", ObligationID: draft.Obligation.ID,
		ExpectedDraftVersion: 1, Actor: actor,
	})
	if err != nil {
		t.Fatalf("confirm obligation: %v", err)
	}
	confirmationAt := storedHistoricalFactTime(t, ctx, environment, `
		SELECT recorded_at FROM obligation_revisions WHERE obligation_id = $1 AND revision = 1
	`, draft.Obligation.ID)

	before, err := service.HistoricalBalance(ctx, HistoricalBalanceInput{
		ObligationID: draft.Obligation.ID,
		AsOf:         confirmationAt.Add(-time.Microsecond).Format(time.RFC3339Nano),
	})
	var notConfirmed *HistoricalNotConfirmedError
	if !errors.As(err, &notConfirmed) || before.ObligationID != "" {
		t.Fatalf("before confirmation = %#v, %v", before, err)
	}
	assertHistoricalSnapshot(t, historicalAt(t, ctx, service, draft.Obligation.ID, confirmationAt), 1, "100", "0", "0", "100", 0, StatusOpen)

	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	cash, err := cashService.Create(ctx, cashtransaction.CreateInput{
		CommandID: "43000000-0000-4000-8000-000000000001", Type: "receipt",
		LedgerOwnerPartyID: owner.ID, CounterpartyPartyID: counterparty.ID,
		Description: "历史余额测试收款", Amount: "100", Currency: "CNY",
		OccurredOn: "2026-09-20", AccountReference: "历史余额测试账户",
		ManualSourceReference: "历史余额测试回单", ExternalTransactionReference: "HISTORY-CASH-001", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create cash transaction: %v", err)
	}
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	applied, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: "44000000-0000-4000-8000-000000000001", CashTransactionID: cash.Transaction.ID,
		ObligationID: draft.Obligation.ID, ExpectedObligationRevision: 1,
		Amount: "40", ManualSourceReference: "历史余额首次核销", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply allocation: %v", err)
	}
	allocationAt := storedHistoricalFactTime(t, ctx, environment, `SELECT applied_at FROM allocations WHERE id = $1`, applied.Allocation.ID)
	assertHistoricalSnapshot(t, historicalAt(t, ctx, service, draft.Obligation.ID, allocationAt), 1, "100", "40", "0", "60", 1, StatusOpen)

	reversed, err := allocationService.Reverse(ctx, allocation.ReverseInput{
		CommandID: "45000000-0000-4000-8000-000000000001", AllocationID: applied.Allocation.ID,
		Reason: "核销对象选择错误", Actor: actor,
	})
	if err != nil {
		t.Fatalf("reverse allocation: %v", err)
	}
	reversalAt := storedHistoricalFactTime(t, ctx, environment, `SELECT reversed_at FROM allocation_reversals WHERE id = $1`, reversed.Reversal.ID)
	assertHistoricalSnapshot(t, historicalAt(t, ctx, service, draft.Obligation.ID, reversalAt), 1, "100", "0", "0", "100", 2, StatusOpen)

	adjusted, err := service.AdjustAmount(ctx, AdjustAmountInput{
		CommandID: "46000000-0000-4000-8000-000000000001", ObligationID: draft.Obligation.ID,
		ExpectedCurrentRevision: 1, Type: "increase", Amount: "20", Reason: "追加确认服务范围", Actor: actor,
	})
	if err != nil {
		t.Fatalf("adjust obligation: %v", err)
	}
	adjustmentAt := storedHistoricalFactTime(t, ctx, environment, `SELECT adjusted_at FROM obligation_amount_adjustments WHERE id = $1`, adjusted.Adjustment.ID)
	assertHistoricalSnapshot(t, historicalAt(t, ctx, service, draft.Obligation.ID, adjustmentAt), 2, "120", "0", "0", "120", 2, StatusOpen)

	remission, err := service.RecordRemission(ctx, RecordRemissionInput{
		CommandID: "47000000-0000-4000-8000-000000000001", ObligationID: draft.Obligation.ID,
		ExpectedCurrentRevision: 2, Amount: "20", Reason: "确认免除附加费用",
		ManualSourceReference: "历史余额减免确认函", Actor: actor,
	})
	if err != nil {
		t.Fatalf("record remission: %v", err)
	}
	remissionAt := storedHistoricalFactTime(t, ctx, environment, `SELECT recorded_at FROM obligation_remissions WHERE id = $1`, remission.Remission.ID)
	assertHistoricalSnapshot(t, historicalAt(t, ctx, service, draft.Obligation.ID, remissionAt), 2, "120", "0", "20", "100", 3, StatusOpen)

	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO obligation_remissions (
			id, obligation_id, obligation_revision, amount, recorded_effective_amount, currency,
			source_kind, remission_reason, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id, recorded_by_actor_display_name, recorded_at
		) VALUES (
			'48000000-0000-4000-8000-000000000001', $1, 2, 1, 120, 'CNY',
			'manual', '倒退时间不应接受', '倒退时间测试依据',
			'local_user', $2, '历史余额管理员', $3
		)
	`, draft.Obligation.ID, actor.ID, confirmationAt)
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) || pgError.Code != "23514" {
		t.Fatalf("backdated remission error = %v", err)
	}

	secondApplied, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: "49000000-0000-4000-8000-000000000001", CashTransactionID: cash.Transaction.ID,
		ObligationID: draft.Obligation.ID, ExpectedObligationRevision: 2,
		Amount: "100", ManualSourceReference: "历史余额最终核销", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply final allocation: %v", err)
	}
	finalAllocationAt := storedHistoricalFactTime(t, ctx, environment, `SELECT applied_at FROM allocations WHERE id = $1`, secondApplied.Allocation.ID)
	assertHistoricalSnapshot(t, historicalAt(t, ctx, service, draft.Obligation.ID, finalAllocationAt), 2, "120", "100", "20", "0", 4, StatusOpen)

	closed, err := service.Close(ctx, CloseInput{
		CommandID: "4a000000-0000-4000-8000-000000000001", ObligationID: draft.Obligation.ID,
		ExpectedCurrentRevision: 2, ExpectedBalanceSequence: 4, Reason: "历史余额测试归零关闭", Actor: actor,
	})
	if err != nil {
		t.Fatalf("close obligation: %v", err)
	}
	closureAt := storedHistoricalFactTime(t, ctx, environment, `SELECT closed_at FROM obligation_closures WHERE id = $1`, closed.Closure.ID)
	closedSnapshot := historicalAt(t, ctx, service, draft.Obligation.ID, closureAt)
	assertHistoricalSnapshot(t, closedSnapshot, 2, "120", "100", "20", "0", 4, StatusClosed)
	if closedSnapshot.Closure == nil || closedSnapshot.Closure.ID != closed.Closure.ID ||
		closedSnapshot.Closure.SettlementBasis != ClosureBasisMixed || !closedSnapshot.Closure.ClosedAt.Equal(closureAt) {
		t.Fatalf("historical closure = %#v", closedSnapshot.Closure)
	}
	if closedSnapshot.LedgerOwner().PartyID != owner.ID || closedSnapshot.Counterparty().PartyID != counterparty.ID ||
		closedSnapshot.Title != "历史余额服务费" || closedSnapshot.DueOn != "2026-12-31" {
		t.Fatalf("historical snapshots = %#v", closedSnapshot)
	}
	if confirmed.Revision.Revision != 1 {
		t.Fatalf("confirmation result = %#v", confirmed)
	}
}

func TestPostgresRecordedTimeGuardRejectsBackdatedFactAfterLockWait(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "11111111-1111-4111-8111-111111111111", DisplayName: "时间门禁管理员",
	}
	owner, counterparty := createTestParties(t, ctx, environment, actor)
	service := NewService(NewPostgresRepository(environment.Pool))
	confirmed := createConfirmedAdjustmentObligation(t, ctx, service, actor, owner.ID, counterparty.ID, "100")
	confirmationAt := storedHistoricalFactTime(t, ctx, environment, `
		SELECT recorded_at FROM obligation_revisions WHERE obligation_id = $1 AND revision = 1
	`, confirmed.ID)
	laterAt := confirmationAt.Add(2 * time.Hour)
	earlierAt := confirmationAt.Add(time.Hour)

	laterTx, err := environment.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin later fact transaction: %v", err)
	}
	defer func() { _ = laterTx.Rollback(context.Background()) }()
	if _, err := laterTx.Exec(ctx, `
		INSERT INTO obligation_remissions (
			id, obligation_id, obligation_revision,
			amount, recorded_effective_amount, currency,
			source_kind, remission_reason, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		) VALUES (
			'd1000000-0000-4000-8000-000000000001', $1, 1,
			10, 100, 'CNY', 'manual', '先提交较晚记录时刻', '时间门禁并发依据一',
			$2, $3, $4, $5
		)
	`, confirmed.ID, actor.Kind, actor.ID, actor.DisplayName, laterAt); err != nil {
		t.Fatalf("insert later recorded fact: %v", err)
	}

	type waitingResult struct {
		backendPID int32
		err        error
	}
	backendPIDs := make(chan int32, 1)
	results := make(chan waitingResult, 1)
	go func() {
		connection, acquireErr := environment.Pool.Acquire(ctx)
		if acquireErr != nil {
			results <- waitingResult{err: acquireErr}
			return
		}
		defer connection.Release()
		var backendPID int32
		if queryErr := connection.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&backendPID); queryErr != nil {
			results <- waitingResult{err: queryErr}
			return
		}
		backendPIDs <- backendPID
		_, insertErr := connection.Exec(ctx, `
			INSERT INTO obligation_remissions (
				id, obligation_id, obligation_revision,
				amount, recorded_effective_amount, currency,
				source_kind, remission_reason, manual_source_reference,
				recorded_by_actor_kind, recorded_by_actor_id,
				recorded_by_actor_display_name, recorded_at
			) VALUES (
				'd1000000-0000-4000-8000-000000000002', $1, 1,
				10, 100, 'CNY', 'manual', '等待后尝试较早记录时刻', '时间门禁并发依据二',
				$2, $3, $4, $5
			)
		`, confirmed.ID, actor.Kind, actor.ID, actor.DisplayName, earlierAt)
		results <- waitingResult{backendPID: backendPID, err: insertErr}
	}()

	var waitingBackendPID int32
	select {
	case waitingBackendPID = <-backendPIDs:
	case result := <-results:
		t.Fatalf("backdated insert stopped before waiting: %v", result.err)
	case <-ctx.Done():
		t.Fatalf("wait for backdated insert backend: %v", ctx.Err())
	}
	waitForPostgresLock(t, ctx, environment, waitingBackendPID)
	if err := laterTx.Commit(ctx); err != nil {
		t.Fatalf("commit later recorded fact: %v", err)
	}

	result := <-results
	var pgError *pgconn.PgError
	if !errors.As(result.err, &pgError) || pgError.Code != "23514" {
		t.Fatalf("waiting backdated insert error = %v", result.err)
	}
	var count int
	var latest time.Time
	if err := environment.Pool.QueryRow(ctx, `
		SELECT count(*), max(recorded_at)
		FROM obligation_remissions
		WHERE obligation_id = $1
	`, confirmed.ID).Scan(&count, &latest); err != nil {
		t.Fatalf("inspect recorded-time concurrency facts: %v", err)
	}
	if count != 1 || !latest.Equal(laterAt) {
		t.Fatalf("recorded-time concurrency facts count=%d latest=%v want=%v", count, latest, laterAt)
	}
}

func historicalAt(t *testing.T, ctx context.Context, service *Service, obligationID string, asOf time.Time) HistoricalSnapshot {
	t.Helper()
	snapshot, err := service.HistoricalBalance(ctx, HistoricalBalanceInput{
		ObligationID: obligationID,
		AsOf:         asOf.Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("HistoricalBalance(%s) error = %v", asOf.Format(time.RFC3339Nano), err)
	}
	return snapshot
}

func storedHistoricalFactTime(
	t *testing.T,
	ctx context.Context,
	environment testdatabase.Environment,
	query string,
	args ...any,
) time.Time {
	t.Helper()
	var value time.Time
	if err := environment.Pool.QueryRow(ctx, query, args...).Scan(&value); err != nil {
		t.Fatalf("read historical fact time: %v", err)
	}
	return value.UTC()
}

func assertHistoricalSnapshot(
	t *testing.T,
	snapshot HistoricalSnapshot,
	revision int64,
	effective, allocated, remitted, outstanding string,
	sequence int64,
	status string,
) {
	t.Helper()
	if snapshot.Revision != revision || snapshot.EffectiveAmount != effective ||
		snapshot.NetAllocatedAmount != allocated || snapshot.RemittedAmount != remitted ||
		snapshot.OutstandingAmount != outstanding || snapshot.BalanceSequence != sequence || snapshot.Status != status {
		t.Fatalf("historical snapshot = %#v", snapshot)
	}
}
