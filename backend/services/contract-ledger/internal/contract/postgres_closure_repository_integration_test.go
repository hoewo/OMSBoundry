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

	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/party"
)

func TestPostgresRepositoryOperationalClosures(t *testing.T) {
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
	contractIDs := make([]string, 0, 10)
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
		Actor: integrationActor, Kind: "organization", LegalName: "关闭测试主体甲", RegistrationIdentifier: "CLOSE-A-" + suffix,
	})
	if err != nil {
		t.Fatalf("create first party: %v", err)
	}
	secondParty, err := partyService.Create(ctx, party.CreateInput{
		Actor: integrationActor, Kind: "organization", LegalName: "关闭测试主体乙", RegistrationIdentifier: "CLOSE-B-" + suffix,
	})
	if err != nil {
		t.Fatalf("create second party: %v", err)
	}
	partyIDs = append(partyIDs, firstParty.ID, secondParty.ID)

	createDraft := func(label string) contract.Contract {
		t.Helper()
		created, err := service.CreateDraft(ctx, contract.CreateDraftInput{
			CommandID: newID(), NumberNamespace: "closure-integration",
			ContractNumber: fmt.Sprintf("%s-%s", label, suffix), Title: label + "关闭集成合同",
			Currency: "CNY", TotalAmount: "1000", SignedOn: "2026-09-19", EffectiveOn: "2026-10-01",
			Actor: integrationActor,
		})
		if err != nil {
			t.Fatalf("CreateDraft(%s) error = %v", label, err)
		}
		contractIDs = append(contractIDs, created.ID)
		return created
	}
	activate := func(label string) contract.Contract {
		t.Helper()
		created := createDraft(label)
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
	recordSettlement := func(value contract.RecordContractSettlementInput) contract.ContractSettlementResult {
		t.Helper()
		result, err := service.RecordSettlement(ctx, value)
		if err != nil {
			t.Fatalf("RecordSettlement(%s) error = %v", value.StatementNumber, err)
		}
		return result
	}

	active := activate("ACTIVE-NO-FINAL")
	tooEarly := contract.CloseContractInput{
		CommandID: newID(), ContractID: active.ID,
		ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		SettlementDisposition: "not_required", ClosedOn: "2026-09-30",
		Reason: "日期顺序测试", Actor: integrationActor,
	}
	_, err = service.CloseContract(ctx, tooEarly)
	var precondition *contract.ContractClosurePreconditionError
	if !errors.As(err, &precondition) || precondition.Field != "closed_on" {
		t.Fatalf("early closure error = %v", err)
	}
	closeInput := tooEarly
	closeInput.CommandID = newID()
	closeInput.ClosedOn = "2026-10-05"
	closeInput.Reason = "合同无需独立最终结算，常规维护结束"
	closed, err := service.CloseContract(ctx, closeInput)
	if err != nil {
		t.Fatalf("CloseContract(active not required) error = %v", err)
	}
	if closed.Replayed || closed.Closure.SourceRevision != 1 || closed.Closure.SourceSettlementSequence != 0 ||
		closed.Closure.SettlementDisposition != contract.SettlementDispositionNotRequired || closed.Closure.FinalSettlementID != "" {
		t.Fatalf("active closure = %#v", closed)
	}
	replayed, err := service.CloseContract(ctx, closeInput)
	if err != nil || !replayed.Replayed || replayed.Closure.ID != closed.Closure.ID {
		t.Fatalf("replayed CloseContract() = %#v, error=%v", replayed, err)
	}
	changedReplay := closeInput
	changedReplay.Reason = "改变关闭原因"
	_, err = service.CloseContract(ctx, changedReplay)
	var idempotencyConflict *contract.IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed closure replay error = %v", err)
	}
	secondClose := closeInput
	secondClose.CommandID = newID()
	_, err = service.CloseContract(ctx, secondClose)
	var closureState *contract.ContractClosureStateConflictError
	if !errors.As(err, &closureState) || closureState.Status != contract.StatusClosed {
		t.Fatalf("second closure error = %v", err)
	}
	activeDetails, err := service.GetDraft(ctx, active.ID)
	if err != nil {
		t.Fatalf("GetDraft(closed active) error = %v", err)
	}
	if activeDetails.Contract.Status != contract.StatusClosed || activeDetails.Contract.CurrentRevision != 1 ||
		activeDetails.Closure == nil || activeDetails.Closure.ID != closed.Closure.ID || len(activeDetails.Instruments) != 1 {
		t.Fatalf("closed active details = %#v", activeDetails)
	}
	assertImmutableDatabaseFact(t, pool, `UPDATE contract_closures SET reason = reason WHERE id = $1`, closed.Closure.ID)
	assertImmutableDatabaseFact(t, pool, `DELETE FROM contract_closure_commands WHERE command_id = $1`, closeInput.CommandID)
	assertImmutableDatabaseFact(t, pool, `UPDATE audit_entries SET summary = summary WHERE command_id = $1`, closeInput.CommandID)

	finalContract := activate("WITH-FINAL")
	finalSettlement := recordSettlement(contract.RecordContractSettlementInput{
		CommandID: newID(), ContractID: finalContract.ID,
		ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		Kind: "final", StatementNumber: "FINAL-" + suffix, Title: "最终结算",
		Amount: "950", ConfirmedOn: "2026-10-10", Basis: "双方确认最终结算",
		ConfirmingPartyPositions: []int64{1, 2}, Actor: integrationActor,
	})
	wrongDisposition := contract.CloseContractInput{
		CommandID: newID(), ContractID: finalContract.ID,
		ExpectedContractRevision: 1, ExpectedSettlementSequence: 1,
		SettlementDisposition: "not_required", ClosedOn: "2026-10-11",
		Reason: "错误结算处置", Actor: integrationActor,
	}
	_, err = service.CloseContract(ctx, wrongDisposition)
	var settlementConflict *contract.ContractClosureSettlementConflictError
	if !errors.As(err, &settlementConflict) {
		t.Fatalf("final with not_required error = %v", err)
	}
	staleSequence := wrongDisposition
	staleSequence.CommandID = newID()
	staleSequence.ExpectedSettlementSequence = 0
	staleSequence.SettlementDisposition = "final_recorded"
	_, err = service.CloseContract(ctx, staleSequence)
	var sequenceConflict *contract.ContractSettlementSequenceConflictError
	if !errors.As(err, &sequenceConflict) || sequenceConflict.Actual != 1 {
		t.Fatalf("stale closure settlement sequence error = %v", err)
	}
	staleRevision := wrongDisposition
	staleRevision.CommandID = newID()
	staleRevision.ExpectedContractRevision = 2
	staleRevision.SettlementDisposition = "final_recorded"
	_, err = service.CloseContract(ctx, staleRevision)
	var revisionConflict *contract.ContractRevisionConflictError
	if !errors.As(err, &revisionConflict) || revisionConflict.Actual != 1 {
		t.Fatalf("stale closure revision error = %v", err)
	}
	earlySettlementDate := wrongDisposition
	earlySettlementDate.CommandID = newID()
	earlySettlementDate.SettlementDisposition = "final_recorded"
	earlySettlementDate.ClosedOn = "2026-10-09"
	_, err = service.CloseContract(ctx, earlySettlementDate)
	if !errors.As(err, &precondition) || precondition.Field != "closed_on" {
		t.Fatalf("closure before final settlement error = %v", err)
	}
	finalClose := wrongDisposition
	finalClose.CommandID = newID()
	finalClose.SettlementDisposition = "final_recorded"
	finalClose.Reason = "最终结算后关闭"
	finalClosed, err := service.CloseContract(ctx, finalClose)
	if err != nil {
		t.Fatalf("CloseContract(final recorded) error = %v", err)
	}
	if finalClosed.Closure.FinalSettlementID != finalSettlement.Settlement.ID ||
		finalClosed.Closure.SettlementDisposition != contract.SettlementDispositionFinalRecorded ||
		finalClosed.Closure.SourceSettlementSequence != 1 {
		t.Fatalf("final recorded closure = %#v", finalClosed)
	}

	progressContract := activate("WITH-PROGRESS")
	recordSettlement(contract.RecordContractSettlementInput{
		CommandID: newID(), ContractID: progressContract.ID,
		ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		Kind: "progress", StatementNumber: "PROGRESS-" + suffix, Title: "阶段结算",
		Amount: "600", ConfirmedOn: "2026-10-10", Basis: "已有阶段结算但不需单独最终结算",
		ConfirmingPartyPositions: []int64{1}, Actor: integrationActor,
	})
	_, err = service.CloseContract(ctx, contract.CloseContractInput{
		CommandID: newID(), ContractID: progressContract.ID,
		ExpectedContractRevision: 1, ExpectedSettlementSequence: 1,
		SettlementDisposition: "final_recorded", ClosedOn: "2026-10-11",
		Reason: "错误声称最终结算", Actor: integrationActor,
	})
	if !errors.As(err, &settlementConflict) {
		t.Fatalf("progress with final_recorded error = %v", err)
	}
	progressClosed, err := service.CloseContract(ctx, contract.CloseContractInput{
		CommandID: newID(), ContractID: progressContract.ID,
		ExpectedContractRevision: 1, ExpectedSettlementSequence: 1,
		SettlementDisposition: "not_required", ClosedOn: "2026-10-11",
		Reason: "已有阶段结算且确认无需独立最终结算", Actor: integrationActor,
	})
	if err != nil || progressClosed.Closure.FinalSettlementID != "" || progressClosed.Closure.SourceSettlementSequence != 1 {
		t.Fatalf("progress not-required closure = %#v, error=%v", progressClosed, err)
	}

	terminatedContract := activate("TERMINATED")
	if _, err := service.Terminate(ctx, contract.TerminateContractInput{
		CommandID: newID(), ContractID: terminatedContract.ID, ExpectedRevision: 1,
		InstrumentNumber: "TERM-CLOSE-" + suffix, Title: "终止后关闭测试", TerminationReason: "终止后完成收尾",
		SignedOn: "2026-10-05", EffectiveOn: "2026-10-05", EffectiveOrder: 2, Actor: integrationActor,
	}); err != nil {
		t.Fatalf("Terminate(closure fixture) error = %v", err)
	}
	terminatedClosed, err := service.CloseContract(ctx, contract.CloseContractInput{
		CommandID: newID(), ContractID: terminatedContract.ID,
		ExpectedContractRevision: 2, ExpectedSettlementSequence: 0,
		SettlementDisposition: "not_required", ClosedOn: "2026-10-06",
		Reason: "终止合同无需独立最终结算", Actor: integrationActor,
	})
	if err != nil || terminatedClosed.Closure.SourceRevision != 2 {
		t.Fatalf("terminated closure = %#v, error=%v", terminatedClosed, err)
	}

	draft := createDraft("DRAFT")
	_, err = service.CloseContract(ctx, contract.CloseContractInput{
		CommandID: newID(), ContractID: draft.ID, ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		SettlementDisposition: "not_required", ClosedOn: "2026-10-01", Reason: "草稿必须拒绝", Actor: integrationActor,
	})
	if !errors.As(err, &closureState) || closureState.Status != contract.StatusDraft {
		t.Fatalf("draft closure error = %v", err)
	}
	cancelled := createDraft("CANCELLED")
	if _, err := service.CancelDraft(ctx, contract.CancelDraftInput{
		CommandID: newID(), ContractID: cancelled.ID, ExpectedDraftVersion: 1,
		Reason: "取消状态关闭拒绝测试", Actor: integrationActor,
	}); err != nil {
		t.Fatalf("CancelDraft(closure fixture) error = %v", err)
	}
	_, err = service.CloseContract(ctx, contract.CloseContractInput{
		CommandID: newID(), ContractID: cancelled.ID, ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		SettlementDisposition: "not_required", ClosedOn: "2026-10-01", Reason: "取消合同必须拒绝", Actor: integrationActor,
	})
	if !errors.As(err, &closureState) || closureState.Status != contract.StatusCancelled {
		t.Fatalf("cancelled closure error = %v", err)
	}

	concurrent := activate("CONCURRENT")
	closeCommandID, settlementCommandID := newID(), newID()
	type concurrentResult struct {
		kind string
		err  error
	}
	start := make(chan struct{})
	results := make(chan concurrentResult, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, err := service.CloseContract(context.Background(), contract.CloseContractInput{
			CommandID: closeCommandID, ContractID: concurrent.ID,
			ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
			SettlementDisposition: "not_required", ClosedOn: "2026-10-10",
			Reason: "并发关闭", Actor: integrationActor,
		})
		results <- concurrentResult{kind: "close", err: err}
	}()
	go func() {
		defer wait.Done()
		<-start
		_, err := service.RecordSettlement(context.Background(), contract.RecordContractSettlementInput{
			CommandID: settlementCommandID, ContractID: concurrent.ID,
			ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
			Kind: "progress", StatementNumber: "CONCURRENT-" + suffix, Title: "并发结算",
			Amount: "500", ConfirmedOn: "2026-10-09", Basis: "验证关闭与结算串行",
			ConfirmingPartyPositions: []int64{1, 2}, Actor: integrationActor,
		})
		results <- concurrentResult{kind: "settlement", err: err}
	}()
	close(start)
	wait.Wait()
	close(results)
	successes, expectedConflicts := 0, 0
	for result := range results {
		if result.err == nil {
			successes++
			continue
		}
		var settlementState *contract.ContractSettlementStateConflictError
		if errors.As(result.err, &settlementState) || errors.As(result.err, &sequenceConflict) {
			expectedConflicts++
			continue
		}
		t.Fatalf("unexpected %s concurrency error = %v", result.kind, result.err)
	}
	if successes != 1 || expectedConflicts != 1 {
		t.Fatalf("closure/settlement concurrency successes=%d conflicts=%d", successes, expectedConflicts)
	}

	auditFailure := activate("AUDIT-FAILURE")
	var existingAuditID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM audit_entries WHERE command_id = $1`, closeInput.CommandID).Scan(&existingAuditID); err != nil {
		t.Fatalf("read existing audit ID: %v", err)
	}
	repository := contract.NewPostgresRepository(pool)
	failedCommandID, failedClosureID := newID(), newID()
	_, _, _, err = repository.CloseContract(ctx, contract.ContractClosureCommand{
		CommandID: failedCommandID, AuditID: existingAuditID, ClosureID: failedClosureID,
		ContractID: auditFailure.ID, ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		SettlementDisposition: contract.SettlementDispositionNotRequired,
		ClosedOn:              "2026-10-10", Reason: "验证审计失败整体回滚",
		Actor: integrationActor, OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("CloseContract() with duplicate audit ID error = nil")
	}
	var failureStatus contract.Status
	var failedFactCount int
	if err := pool.QueryRow(ctx, `SELECT status FROM contracts WHERE id = $1`, auditFailure.ID).Scan(&failureStatus); err != nil {
		t.Fatalf("read audit failure contract status: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_closures WHERE contract_id = $1) +
			(SELECT count(*) FROM contract_closure_commands WHERE contract_id = $1)
	`, auditFailure.ID).Scan(&failedFactCount); err != nil {
		t.Fatalf("count rolled back closure facts: %v", err)
	}
	if failureStatus != contract.StatusActive || failedFactCount != 0 {
		t.Fatalf("audit failure status=%s facts=%d", failureStatus, failedFactCount)
	}

	var closureAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_entries
		WHERE object_id IN ($1, $2, $3, $4) AND action = 'contract_closed'
	`, active.ID, finalContract.ID, progressContract.ID, terminatedContract.ID).Scan(&closureAuditCount); err != nil {
		t.Fatalf("count closure audits: %v", err)
	}
	if closureAuditCount != 4 {
		t.Fatalf("closure audit count = %d, want 4", closureAuditCount)
	}
}
