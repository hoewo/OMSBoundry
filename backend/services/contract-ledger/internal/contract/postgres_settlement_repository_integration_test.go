//go:build integration

package contract_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/party"
)

func TestPostgresRepositoryContractSettlements(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}

	service := contract.NewService(contract.NewPostgresRepository(pool))
	partyService := party.NewService(party.NewPostgresRepository(pool))
	contractIDs := make([]string, 0, 5)
	partyIDs := make([]string, 0, 2)
	defer func() { cleanupActivationFixtures(t, pool, contractIDs, partyIDs) }()

	newID := func() string {
		t.Helper()
		value, err := contract.NewCommandID()
		if err != nil {
			t.Fatalf("NewCommandID() error = %v", err)
		}
		return value
	}
	suffix := strings.ToUpper(newID()[28:])
	firstParty, err := partyService.Create(ctx, party.CreateInput{
		Actor: integrationActor, Kind: "organization", LegalName: "结算确认主体甲", RegistrationIdentifier: "SET-A-" + suffix,
	})
	if err != nil {
		t.Fatalf("create first party: %v", err)
	}
	secondParty, err := partyService.Create(ctx, party.CreateInput{
		Actor: integrationActor, Kind: "organization", LegalName: "结算确认主体乙", RegistrationIdentifier: "SET-B-" + suffix,
	})
	if err != nil {
		t.Fatalf("create second party: %v", err)
	}
	partyIDs = append(partyIDs, firstParty.ID, secondParty.ID)

	createDraft := func(label, amount string) contract.Contract {
		t.Helper()
		created, err := service.CreateDraft(ctx, contract.CreateDraftInput{
			CommandID: newID(), NumberNamespace: "settlement-integration",
			ContractNumber: fmt.Sprintf("%s-%s", label, suffix), Title: label + "结算集成合同",
			Currency: "CNY", TotalAmount: amount, SignedOn: "2026-09-19", EffectiveOn: "2026-10-01",
			Actor: integrationActor,
		})
		if err != nil {
			t.Fatalf("CreateDraft(%s) error = %v", label, err)
		}
		contractIDs = append(contractIDs, created.ID)
		return created
	}
	activate := func(label, amount string) contract.Contract {
		t.Helper()
		created := createDraft(label, amount)
		if _, err := service.AddDraftParty(ctx, contract.AddDraftPartyInput{
			CommandID: newID(), ContractID: created.ID, PartyID: firstParty.ID,
			Role: "委托方", ExpectedDraftVersion: 1, Actor: integrationActor,
		}); err != nil {
			t.Fatalf("add first party to %s: %v", label, err)
		}
		if _, err := service.AddDraftParty(ctx, contract.AddDraftPartyInput{
			CommandID: newID(), ContractID: created.ID, PartyID: secondParty.ID,
			Role: "受托方", ExpectedDraftVersion: 2, Actor: integrationActor,
		}); err != nil {
			t.Fatalf("add second party to %s: %v", label, err)
		}
		if _, err := service.Activate(ctx, contract.ActivateContractInput{
			CommandID: newID(), ContractID: created.ID, ExpectedDraftVersion: 3, Actor: integrationActor,
		}); err != nil {
			t.Fatalf("activate %s: %v", label, err)
		}
		return created
	}

	primary := activate("PRIMARY", "")
	progressOne := contract.RecordContractSettlementInput{
		CommandID: newID(), ContractID: primary.ID,
		ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		Kind: "progress", StatementNumber: "SET-001-" + suffix, Title: "零金额阶段结算",
		Amount: "0.000000", ConfirmedOn: "2026-10-10", Basis: "仅主体甲确认的阶段口径",
		ConfirmingPartyPositions: []int64{1}, Actor: integrationActor,
	}
	first, err := service.RecordSettlement(ctx, progressOne)
	if err != nil {
		t.Fatalf("RecordSettlement(first progress) error = %v", err)
	}
	if first.Replayed || first.Settlement.Sequence != 1 || first.Settlement.Amount != "0" ||
		first.Settlement.Currency != "CNY" || len(first.Settlement.ConfirmingParties) != 1 ||
		first.Settlement.ConfirmingParties[0].LegalName != firstParty.LegalName {
		t.Fatalf("first settlement = %#v", first)
	}
	replayed, err := service.RecordSettlement(ctx, progressOne)
	if err != nil || !replayed.Replayed || replayed.Settlement.ID != first.Settlement.ID {
		t.Fatalf("replayed settlement = %#v, error=%v", replayed, err)
	}
	changedReplay := progressOne
	changedReplay.Amount = "1"
	_, err = service.RecordSettlement(ctx, changedReplay)
	var idempotencyConflict *contract.IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed settlement replay error = %v", err)
	}

	duplicate := progressOne
	duplicate.CommandID = newID()
	duplicate.ExpectedSettlementSequence = 1
	duplicate.StatementNumber = strings.ToLower(progressOne.StatementNumber)
	_, err = service.RecordSettlement(ctx, duplicate)
	var duplicateNumber *contract.DuplicateSettlementNumberError
	if !errors.As(err, &duplicateNumber) {
		t.Fatalf("case-insensitive duplicate settlement number error = %v", err)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE parties
		SET legal_name = '结算后主体甲当前名称', revision = revision + 1, updated_at = clock_timestamp()
		WHERE id = $1
	`, firstParty.ID); err != nil {
		t.Fatalf("update current party after settlement: %v", err)
	}
	change, err := service.AppendChange(ctx, contract.AppendContractChangeInput{
		CommandID: newID(), ContractID: primary.ID, ExpectedRevision: 1,
		InstrumentNumber: "SUP-SET-" + suffix, Title: "重述合同金额",
		ChangeSummary: "从待确定重述为确定金额", SignedOn: "2026-10-15", EffectiveOn: "2026-10-15",
		EffectiveOrder: 2, AmountEffect: contract.AmountEffectRestatement, AmountValue: "1500", Actor: integrationActor,
	})
	if err != nil || change.Revision != 2 || change.EffectiveAmount != "1500" {
		t.Fatalf("AppendChange(restatement) = %#v, error=%v", change, err)
	}
	staleRevision := progressOne
	staleRevision.CommandID = newID()
	staleRevision.StatementNumber = "STALE-" + suffix
	staleRevision.ExpectedSettlementSequence = 1
	_, err = service.RecordSettlement(ctx, staleRevision)
	var revisionConflict *contract.ContractRevisionConflictError
	if !errors.As(err, &revisionConflict) || revisionConflict.Actual != 2 {
		t.Fatalf("stale settlement revision error = %v", err)
	}

	badParty := progressOne
	badParty.CommandID = newID()
	badParty.ExpectedContractRevision = 2
	badParty.ExpectedSettlementSequence = 1
	badParty.StatementNumber = "BAD-PARTY-" + suffix
	badParty.ConfirmingPartyPositions = []int64{99}
	_, err = service.RecordSettlement(ctx, badParty)
	var partyError *contract.ContractSettlementPartyError
	if !errors.As(err, &partyError) {
		t.Fatalf("invalid settlement party error = %v", err)
	}

	progressTwo := contract.RecordContractSettlementInput{
		CommandID: newID(), ContractID: primary.ID,
		ExpectedContractRevision: 2, ExpectedSettlementSequence: 1,
		Kind: "progress", StatementNumber: "SET-002-" + suffix, Title: "高于当前金额的累计结算",
		Amount: "2000", ConfirmedOn: "2026-10-20", Basis: "双方确认差异另有业务原因",
		ConfirmingPartyPositions: []int64{2, 1}, Actor: integrationActor,
	}
	second, err := service.RecordSettlement(ctx, progressTwo)
	if err != nil || second.Settlement.Sequence != 2 || len(second.Settlement.ConfirmingParties) != 2 {
		t.Fatalf("second settlement = %#v, error=%v", second, err)
	}

	type concurrentResult struct {
		value contract.ContractSettlementResult
		err   error
	}
	start := make(chan struct{})
	results := make(chan concurrentResult, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			value, err := service.RecordSettlement(context.Background(), contract.RecordContractSettlementInput{
				CommandID: newID(), ContractID: primary.ID,
				ExpectedContractRevision: 2, ExpectedSettlementSequence: 2,
				Kind: "progress", StatementNumber: fmt.Sprintf("CONCURRENT-%d-%s", index, suffix),
				Title: "并发阶段结算", Amount: fmt.Sprintf("%d", 2100+index),
				ConfirmedOn: "2026-10-25", Basis: "并发顺序验证",
				ConfirmingPartyPositions: []int64{1, 2}, Actor: integrationActor,
			})
			results <- concurrentResult{value: value, err: err}
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for result := range results {
		if result.err == nil {
			successes++
			if result.value.Settlement.Sequence != 3 {
				t.Fatalf("concurrent settlement sequence = %d", result.value.Settlement.Sequence)
			}
			continue
		}
		var sequenceConflict *contract.ContractSettlementSequenceConflictError
		if errors.As(result.err, &sequenceConflict) && sequenceConflict.Actual == 3 {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent settlement error = %v", result.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent settlements successes=%d conflicts=%d", successes, conflicts)
	}

	terminated, err := service.Terminate(ctx, contract.TerminateContractInput{
		CommandID: newID(), ContractID: primary.ID, ExpectedRevision: 2,
		InstrumentNumber: "TERM-SET-" + suffix, Title: "结算测试终止文书", TerminationReason: "终止后登记最终结算",
		SignedOn: "2026-10-30", EffectiveOn: "2026-10-30", EffectiveOrder: 3, Actor: integrationActor,
	})
	if err != nil || terminated.Revision != 3 {
		t.Fatalf("Terminate() = %#v, error=%v", terminated, err)
	}
	finalInput := contract.RecordContractSettlementInput{
		CommandID: newID(), ContractID: primary.ID,
		ExpectedContractRevision: 3, ExpectedSettlementSequence: 3,
		Kind: "final", StatementNumber: "FINAL-" + suffix, Title: "最终结算",
		Amount: "1200.500000", ConfirmedOn: "2026-11-05", Basis: "双方确认最终累计结算金额",
		ConfirmingPartyPositions: []int64{1, 2}, Actor: integrationActor,
	}
	final, err := service.RecordSettlement(ctx, finalInput)
	if err != nil || final.Settlement.Sequence != 4 || final.Settlement.Kind != contract.SettlementKindFinal || final.Settlement.Amount != "1200.5" {
		t.Fatalf("final settlement = %#v, error=%v", final, err)
	}
	afterFinal := finalInput
	afterFinal.CommandID = newID()
	afterFinal.StatementNumber = "AFTER-FINAL-" + suffix
	afterFinal.ExpectedSettlementSequence = 4
	afterFinal.Kind = "progress"
	_, err = service.RecordSettlement(ctx, afterFinal)
	var finalized *contract.ContractSettlementFinalizedError
	if !errors.As(err, &finalized) {
		t.Fatalf("settlement after final error = %v", err)
	}

	details, err := service.GetDraft(ctx, primary.ID)
	if err != nil {
		t.Fatalf("GetDraft() after settlements: %v", err)
	}
	if len(details.Settlements) != 4 || details.Settlements[0].Amount != "0" ||
		details.Settlements[0].ConfirmingParties[0].LegalName != firstParty.LegalName ||
		details.Settlements[3].Kind != contract.SettlementKindFinal || details.Contract.TotalAmount != "1500" {
		t.Fatalf("settlement details = %#v, contract=%#v", details.Settlements, details.Contract)
	}
	var settlementAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_entries
		WHERE object_id = $1 AND action = 'contract_settlement_recorded'
	`, primary.ID).Scan(&settlementAuditCount); err != nil {
		t.Fatalf("count settlement audit entries: %v", err)
	}
	if settlementAuditCount != 4 {
		t.Fatalf("settlement audit count = %d, want 4", settlementAuditCount)
	}

	assertImmutableDatabaseFact(t, pool, `UPDATE contract_settlements SET title = title WHERE id = $1`, first.Settlement.ID)
	assertImmutableDatabaseFact(t, pool, `DELETE FROM contract_settlement_commands WHERE command_id = $1`, progressOne.CommandID)
	assertImmutableDatabaseFact(t, pool, `DELETE FROM contract_settlement_parties WHERE settlement_id = $1`, first.Settlement.ID)
	assertImmutableDatabaseFact(t, pool, `UPDATE audit_entries SET summary = summary WHERE command_id = $1`, progressOne.CommandID)

	draft := createDraft("DRAFT", "100")
	_, err = service.RecordSettlement(ctx, contract.RecordContractSettlementInput{
		CommandID: newID(), ContractID: draft.ID, ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		Kind: "progress", StatementNumber: "DRAFT-SET-" + suffix, Title: "草稿结算", Amount: "1",
		ConfirmedOn: "2026-10-01", Basis: "草稿必须拒绝", ConfirmingPartyPositions: []int64{1}, Actor: integrationActor,
	})
	var stateConflict *contract.ContractSettlementStateConflictError
	if !errors.As(err, &stateConflict) || stateConflict.Status != contract.StatusDraft {
		t.Fatalf("draft settlement error = %v", err)
	}
	cancelled := createDraft("CANCELLED", "100")
	if _, err := service.CancelDraft(ctx, contract.CancelDraftInput{
		CommandID: newID(), ContractID: cancelled.ID, ExpectedDraftVersion: 1,
		Reason: "取消状态结算拒绝测试", Actor: integrationActor,
	}); err != nil {
		t.Fatalf("CancelDraft(cancelled settlement fixture) error = %v", err)
	}
	_, err = service.RecordSettlement(ctx, contract.RecordContractSettlementInput{
		CommandID: newID(), ContractID: cancelled.ID, ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		Kind: "progress", StatementNumber: "CANCELLED-SET-" + suffix, Title: "取消合同结算", Amount: "1",
		ConfirmedOn: "2026-10-01", Basis: "取消状态必须拒绝", ConfirmingPartyPositions: []int64{1}, Actor: integrationActor,
	})
	if !errors.As(err, &stateConflict) || stateConflict.Status != contract.StatusCancelled {
		t.Fatalf("cancelled settlement error = %v", err)
	}

	closed := activate("CLOSED", "100")
	if _, err := pool.Exec(ctx, `UPDATE contracts SET status = 'closed' WHERE id = $1`, closed.ID); err != nil {
		t.Fatalf("mark fixture closed: %v", err)
	}
	_, err = service.RecordSettlement(ctx, contract.RecordContractSettlementInput{
		CommandID: newID(), ContractID: closed.ID, ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		Kind: "final", StatementNumber: "CLOSED-SET-" + suffix, Title: "关闭合同结算", Amount: "1",
		ConfirmedOn: "2026-10-01", Basis: "关闭状态必须拒绝", ConfirmingPartyPositions: []int64{1}, Actor: integrationActor,
	})
	if !errors.As(err, &stateConflict) || stateConflict.Status != contract.StatusClosed {
		t.Fatalf("closed settlement error = %v", err)
	}

	auditFailure := activate("AUDIT-FAILURE", "100")
	var existingAuditID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM audit_entries WHERE command_id = $1`, progressOne.CommandID).Scan(&existingAuditID); err != nil {
		t.Fatalf("read existing audit ID: %v", err)
	}
	repository := contract.NewPostgresRepository(pool)
	failedCommandID, failedSettlementID := newID(), newID()
	_, _, _, err = repository.RecordSettlement(ctx, contract.ContractSettlementCommand{
		CommandID: failedCommandID, AuditID: existingAuditID, SettlementID: failedSettlementID,
		ContractID: auditFailure.ID, ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		Kind: contract.SettlementKindProgress, StatementNumber: "AUDIT-FAIL-" + suffix,
		Title: "审计失败结算", Amount: "50", ConfirmedOn: "2026-10-10", Basis: "验证事务回滚",
		ConfirmingPartyPositions: []int64{1}, Actor: integrationActor,
		ResultingSettlementSequence: 1, OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("RecordSettlement() with duplicate audit ID error = nil")
	}
	var failedFactCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_settlements WHERE contract_id = $1) +
			(SELECT count(*) FROM contract_settlement_commands WHERE contract_id = $1) +
			(SELECT count(*) FROM contract_settlement_parties WHERE settlement_id = $2)
	`, auditFailure.ID, failedSettlementID).Scan(&failedFactCount); err != nil {
		t.Fatalf("count rolled back settlement facts: %v", err)
	}
	if failedFactCount != 0 {
		t.Fatalf("audit failure left %d settlement facts", failedFactCount)
	}
}

func settlementTableCounts(t *testing.T, pool *pgxpool.Pool, contractID string) (int, int, int) {
	t.Helper()
	var settlements, commands, parties int
	if err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM contract_settlements WHERE contract_id = $1),
			(SELECT count(*) FROM contract_settlement_commands WHERE contract_id = $1),
			(SELECT count(*) FROM contract_settlement_parties p JOIN contract_settlements s ON s.id = p.settlement_id WHERE s.contract_id = $1)
	`, contractID).Scan(&settlements, &commands, &parties); err != nil {
		t.Fatalf("count settlement tables: %v", err)
	}
	return settlements, commands, parties
}
