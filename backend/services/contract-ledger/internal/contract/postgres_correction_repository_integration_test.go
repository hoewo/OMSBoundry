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

func TestPostgresRepositoryEffectiveContractCorrections(t *testing.T) {
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

	repository := contract.NewPostgresRepository(pool)
	service := contract.NewService(repository)
	partyService := party.NewService(party.NewPostgresRepository(pool))
	contractIDs := make([]string, 0, 8)
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
		Actor: integrationActor, Kind: "organization", LegalName: "更正测试主体甲",
		RegistrationIdentifier: "CORR-A-" + suffix,
	})
	if err != nil {
		t.Fatalf("create first party: %v", err)
	}
	secondParty, err := partyService.Create(ctx, party.CreateInput{
		Actor: integrationActor, Kind: "organization", LegalName: "更正测试主体乙",
		RegistrationIdentifier: "CORR-B-" + suffix,
	})
	if err != nil {
		t.Fatalf("create second party: %v", err)
	}
	partyIDs = append(partyIDs, firstParty.ID, secondParty.ID)

	createDraft := func(label string) contract.Contract {
		t.Helper()
		created, err := service.CreateDraft(ctx, contract.CreateDraftInput{
			CommandID: newID(), NumberNamespace: "correction-integration",
			ContractNumber: fmt.Sprintf("%s-%s", label, suffix), Title: label + "更正集成合同",
			Category: "更正前分类", Currency: "CNY", TotalAmount: "1000",
			SignedOn: "2026-09-19", EffectiveOn: "2026-10-01", Actor: integrationActor,
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
	correctionInput := func(value contract.Contract, status contract.Status, revision int64, label string) contract.CorrectContractInput {
		t.Helper()
		return contract.CorrectContractInput{
			CommandID: newID(), ContractID: value.ID, ExpectedStatus: string(status), ExpectedRevision: revision,
			InstrumentNumber: fmt.Sprintf("CORR-%s-%s", label, suffix), InstrumentTitle: label + "受控更正文书",
			CorrectionReason:   "核对原始材料后修正结构化录入错误",
			InstrumentSignedOn: "2026-10-02", InstrumentEffectiveOn: "2026-10-02", EffectiveOrder: revision + 1,
			CorrectedTitle: label + "更正后标题", CorrectedCategory: "更正后分类", CorrectedTotalAmount: "1200.125000",
			CorrectedSignedOn: "2026-09-20", CorrectedEffectiveOn: "2026-10-01", Actor: integrationActor,
		}
	}

	active := activate("ACTIVE")
	activeInput := correctionInput(active, contract.StatusActive, 1, "ACTIVE")
	corrected, err := service.CorrectContract(ctx, activeInput)
	if err != nil {
		t.Fatalf("CorrectContract(active) error = %v", err)
	}
	if corrected.Replayed || corrected.Revision != 2 || corrected.Status != contract.StatusActive || corrected.EffectiveAmount != "1200.125" {
		t.Fatalf("CorrectContract(active) = %#v", corrected)
	}
	details, err := service.GetDraft(ctx, active.ID)
	if err != nil {
		t.Fatalf("GetDraft(corrected active) error = %v", err)
	}
	if details.Contract.Status != contract.StatusActive || details.Contract.CurrentRevision != 2 ||
		details.Contract.Title != activeInput.CorrectedTitle || details.Contract.Category != activeInput.CorrectedCategory ||
		details.Contract.TotalAmount != "1200.125" || details.Contract.SignedOn != activeInput.CorrectedSignedOn ||
		details.Contract.EffectiveOn != activeInput.CorrectedEffectiveOn {
		t.Fatalf("corrected current contract = %#v", details.Contract)
	}
	if len(details.Revisions) != 2 || details.Revisions[0].Title != "ACTIVE更正集成合同" || details.Revisions[0].TotalAmount != "1000" ||
		details.Revisions[1].Title != activeInput.CorrectedTitle || details.Revisions[1].ContractNumber != details.Revisions[0].ContractNumber ||
		details.Revisions[1].Currency != details.Revisions[0].Currency {
		t.Fatalf("corrected revision history = %#v", details.Revisions)
	}
	if len(details.Instruments) != 2 || details.Instruments[1].InstrumentType != contract.InstrumentTypeCorrection ||
		details.Instruments[1].AmountEffect != contract.AmountEffectCorrected || details.Instruments[1].AmountValue != "1200.125" {
		t.Fatalf("correction instrument timeline = %#v", details.Instruments)
	}
	if len(details.PartySnapshots) != 2 || details.PartySnapshots[0].Revision != 2 ||
		details.PartySnapshots[0].SourcePartyID != firstParty.ID || details.PartySnapshots[1].SourcePartyID != secondParty.ID {
		t.Fatalf("current correction party snapshots = %#v", details.PartySnapshots)
	}
	var copiedSnapshotCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM contract_revision_parties current_snapshot
		JOIN contract_revision_parties prior_snapshot
			ON prior_snapshot.contract_id = current_snapshot.contract_id
			AND prior_snapshot.revision = 1
			AND prior_snapshot.position = current_snapshot.position
			AND prior_snapshot.source_party_link_id = current_snapshot.source_party_link_id
			AND prior_snapshot.source_party_id = current_snapshot.source_party_id
			AND prior_snapshot.source_party_revision = current_snapshot.source_party_revision
			AND prior_snapshot.party_kind = current_snapshot.party_kind
			AND prior_snapshot.legal_name = current_snapshot.legal_name
			AND prior_snapshot.registration_identifier IS NOT DISTINCT FROM current_snapshot.registration_identifier
			AND prior_snapshot.role = current_snapshot.role
		WHERE current_snapshot.contract_id = $1 AND current_snapshot.revision = 2
	`, active.ID).Scan(&copiedSnapshotCount); err != nil {
		t.Fatalf("verify copied party snapshots: %v", err)
	}
	if copiedSnapshotCount != 2 {
		t.Fatalf("copied party snapshot count = %d, want 2", copiedSnapshotCount)
	}
	page, err := service.Search(ctx, contract.SearchContractsInput{Keyword: activeInput.CorrectedTitle})
	if err != nil || len(page.Contracts) != 1 || page.Contracts[0].ID != active.ID || page.Contracts[0].Title != activeInput.CorrectedTitle {
		t.Fatalf("search corrected current title = %#v, error=%v", page, err)
	}
	oldPage, err := service.Search(ctx, contract.SearchContractsInput{Keyword: "ACTIVE更正集成合同"})
	if err != nil || len(oldPage.Contracts) != 0 {
		t.Fatalf("search old formal title = %#v, error=%v", oldPage, err)
	}

	replayed, err := service.CorrectContract(ctx, activeInput)
	if err != nil || !replayed.Replayed || replayed.InstrumentID != corrected.InstrumentID || replayed.Revision != 2 {
		t.Fatalf("replayed CorrectContract() = %#v, error=%v", replayed, err)
	}
	changedReplay := activeInput
	changedReplay.CorrectedCategory = "篡改重放分类"
	_, err = service.CorrectContract(ctx, changedReplay)
	var idempotencyConflict *contract.IdempotencyConflictError
	if !errors.As(err, &idempotencyConflict) {
		t.Fatalf("changed correction replay error = %v, want IdempotencyConflictError", err)
	}

	noChanges := activeInput
	noChanges.CommandID = newID()
	noChanges.ExpectedRevision = 2
	noChanges.EffectiveOrder = 3
	noChanges.InstrumentNumber = "CORR-NOOP-" + suffix
	noChanges.InstrumentEffectiveOn = "2026-10-03"
	_, err = service.CorrectContract(ctx, noChanges)
	var noChangesError *contract.ContractCorrectionNoChangesError
	if !errors.As(err, &noChangesError) {
		t.Fatalf("no-op correction error = %v, want ContractCorrectionNoChangesError", err)
	}
	stale := noChanges
	stale.CommandID = newID()
	stale.ExpectedRevision = 1
	stale.EffectiveOrder = 2
	stale.InstrumentNumber = "CORR-STALE-" + suffix
	stale.CorrectedTitle = "过期 revision 不得覆盖"
	_, err = service.CorrectContract(ctx, stale)
	var revisionConflict *contract.ContractRevisionConflictError
	if !errors.As(err, &revisionConflict) || revisionConflict.Actual != 2 {
		t.Fatalf("stale correction error = %v, want current revision 2", err)
	}
	wrongStatus := noChanges
	wrongStatus.CommandID = newID()
	wrongStatus.ExpectedStatus = string(contract.StatusClosed)
	wrongStatus.InstrumentNumber = "CORR-STATUS-" + suffix
	wrongStatus.CorrectedTitle = "错误状态不得覆盖"
	_, err = service.CorrectContract(ctx, wrongStatus)
	var stateConflict *contract.ContractCorrectionStateConflictError
	if !errors.As(err, &stateConflict) || stateConflict.Actual != contract.StatusActive {
		t.Fatalf("stale status correction error = %v, want active conflict", err)
	}
	backdated := noChanges
	backdated.CommandID = newID()
	backdated.InstrumentNumber = "CORR-BACKDATED-" + suffix
	backdated.InstrumentEffectiveOn = "2026-09-30"
	backdated.CorrectedTitle = "倒序日期不得写入"
	_, err = service.CorrectContract(ctx, backdated)
	var precondition *contract.ContractCorrectionPreconditionError
	if !errors.As(err, &precondition) || precondition.Field != "correction_instrument_effective_on" {
		t.Fatalf("backdated correction error = %v", err)
	}
	duplicateNumber := noChanges
	duplicateNumber.CommandID = newID()
	duplicateNumber.InstrumentNumber = strings.ToLower(activeInput.InstrumentNumber)
	duplicateNumber.CorrectedTitle = "重复文书编号不得写入"
	_, err = service.CorrectContract(ctx, duplicateNumber)
	var duplicateInstrument *contract.DuplicateInstrumentNumberError
	if !errors.As(err, &duplicateInstrument) {
		t.Fatalf("duplicate correction instrument error = %v", err)
	}

	terminated := activate("TERMINATED")
	if _, err := service.Terminate(ctx, contract.TerminateContractInput{
		CommandID: newID(), ContractID: terminated.ID, ExpectedRevision: 1,
		InstrumentNumber: "TERM-" + suffix, Title: "终止更正测试文书", TerminationReason: "进入终止状态",
		SignedOn: "2026-10-02", EffectiveOn: "2026-10-02", EffectiveOrder: 2, Actor: integrationActor,
	}); err != nil {
		t.Fatalf("terminate correction fixture: %v", err)
	}
	terminatedInput := correctionInput(terminated, contract.StatusTerminated, 2, "TERMINATED")
	terminatedInput.InstrumentEffectiveOn = "2026-10-03"
	terminatedInput.EffectiveOrder = 3
	terminatedResult, err := service.CorrectContract(ctx, terminatedInput)
	if err != nil || terminatedResult.Status != contract.StatusTerminated || terminatedResult.Revision != 3 {
		t.Fatalf("CorrectContract(terminated) = %#v, error=%v", terminatedResult, err)
	}

	closed := activate("CLOSED")
	closedResult, err := service.CloseContract(ctx, contract.CloseContractInput{
		CommandID: newID(), ContractID: closed.ID, ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		SettlementDisposition: string(contract.SettlementDispositionNotRequired), ClosedOn: "2026-10-03",
		Reason: "关闭后验证受控更正", Actor: integrationActor,
	})
	if err != nil {
		t.Fatalf("close correction fixture: %v", err)
	}
	closedInput := correctionInput(closed, contract.StatusClosed, 1, "CLOSED")
	closedInput.InstrumentEffectiveOn = "2026-10-04"
	closedCorrection, err := service.CorrectContract(ctx, closedInput)
	if err != nil || closedCorrection.Status != contract.StatusClosed || closedCorrection.Revision != 2 {
		t.Fatalf("CorrectContract(closed) = %#v, error=%v", closedCorrection, err)
	}
	closedDetails, err := service.GetDraft(ctx, closed.ID)
	if err != nil || closedDetails.Contract.Status != contract.StatusClosed || closedDetails.Contract.CurrentRevision != 2 ||
		closedDetails.Closure == nil || closedDetails.Closure.ID != closedResult.Closure.ID || closedDetails.Closure.SourceRevision != 1 {
		t.Fatalf("closed correction details = %#v, error=%v", closedDetails, err)
	}

	draft := createDraft("DRAFT")
	draftInput := correctionInput(draft, contract.StatusActive, 1, "DRAFT")
	_, err = service.CorrectContract(ctx, draftInput)
	if !errors.As(err, &stateConflict) || stateConflict.Actual != contract.StatusDraft {
		t.Fatalf("draft correction error = %v, want draft state conflict", err)
	}
	cancelled := createDraft("CANCELLED")
	if _, err := service.CancelDraft(ctx, contract.CancelDraftInput{
		CommandID: newID(), ContractID: cancelled.ID, ExpectedDraftVersion: 1,
		Reason: "取消状态不能创建正式更正", Actor: integrationActor,
	}); err != nil {
		t.Fatalf("cancel correction fixture: %v", err)
	}
	cancelledInput := correctionInput(cancelled, contract.StatusActive, 1, "CANCELLED")
	_, err = service.CorrectContract(ctx, cancelledInput)
	if !errors.As(err, &stateConflict) || stateConflict.Actual != contract.StatusCancelled {
		t.Fatalf("cancelled correction error = %v, want cancelled state conflict", err)
	}

	concurrent := activate("CONCURRENT")
	start := make(chan struct{})
	type outcome struct{ err error }
	outcomes := make(chan outcome, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		input := correctionInput(concurrent, contract.StatusActive, 1, "CONCURRENT")
		_, err := service.CorrectContract(context.Background(), input)
		outcomes <- outcome{err: err}
	}()
	go func() {
		defer wait.Done()
		<-start
		_, err := service.AppendChange(context.Background(), contract.AppendContractChangeInput{
			CommandID: newID(), ContractID: concurrent.ID, ExpectedRevision: 1,
			InstrumentNumber: "SUPP-CONCURRENT-" + suffix, Title: "并发补充协议", ChangeSummary: "与更正竞争 revision 2",
			SignedOn: "2026-10-02", EffectiveOn: "2026-10-02", EffectiveOrder: 2,
			AmountEffect: contract.AmountEffectUnchanged, Actor: integrationActor,
		})
		outcomes <- outcome{err: err}
	}()
	close(start)
	wait.Wait()
	close(outcomes)
	successes, conflicts := 0, 0
	for value := range outcomes {
		if value.err == nil {
			successes++
			continue
		}
		if errors.As(value.err, &revisionConflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent correction/change error = %v", value.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent correction/change successes=%d conflicts=%d", successes, conflicts)
	}

	auditFailure := activate("AUDIT-ROLLBACK")
	var duplicateAuditID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM audit_entries WHERE object_id = $1 ORDER BY occurred_at LIMIT 1`, auditFailure.ID).Scan(&duplicateAuditID); err != nil {
		t.Fatalf("read duplicate audit fixture: %v", err)
	}
	failingCommand := contract.ContractCorrectionCommand{
		CommandID: newID(), AuditID: duplicateAuditID, InstrumentID: newID(), ContractID: auditFailure.ID,
		ExpectedStatus: contract.StatusActive, ExpectedRevision: 1,
		InstrumentNumber: "CORR-AUDIT-" + suffix, InstrumentTitle: "审计失败整体回滚更正文书",
		CorrectionReason:   "故意复用审计主键验证事务原子性",
		InstrumentSignedOn: "2026-10-02", InstrumentEffectiveOn: "2026-10-02", EffectiveOrder: 2,
		CorrectedTitle: "不应持久化的更正标题", CorrectedCategory: "不应持久化",
		CorrectedTotalAmount: "1300", CorrectedSignedOn: "2026-09-19", CorrectedEffectiveOn: "2026-10-01",
		Actor: integrationActor, ResultingRevision: 2, OccurredAt: time.Now().UTC(),
	}
	if _, _, err := repository.CorrectContract(ctx, failingCommand); err == nil {
		t.Fatal("CorrectContract(audit failure) error = nil")
	}
	var rollbackRevision, rollbackFacts int
	if err := pool.QueryRow(ctx, `
		SELECT current_revision,
			(SELECT count(*) FROM contract_correction_commands WHERE contract_id = c.id) +
			(SELECT count(*) FROM contract_instruments WHERE contract_id = c.id AND instrument_type = 'correction_instrument') +
			(SELECT count(*) FROM contract_revisions WHERE contract_id = c.id AND revision > 1)
		FROM contracts c WHERE c.id = $1
	`, auditFailure.ID).Scan(&rollbackRevision, &rollbackFacts); err != nil {
		t.Fatalf("read correction audit rollback facts: %v", err)
	}
	if rollbackRevision != 1 || rollbackFacts != 0 {
		t.Fatalf("audit failure left current revision=%d partial facts=%d", rollbackRevision, rollbackFacts)
	}

	assertImmutableDatabaseFact(t, pool, `UPDATE contract_correction_commands SET correction_reason = correction_reason WHERE command_id = $1`, activeInput.CommandID)
	assertImmutableDatabaseFact(t, pool, `DELETE FROM contract_correction_commands WHERE command_id = $1`, activeInput.CommandID)
	assertImmutableDatabaseFact(t, pool, `UPDATE contract_instruments SET title = title WHERE id = $1`, corrected.InstrumentID)
	assertImmutableDatabaseFact(t, pool, `UPDATE contract_revisions SET title = title WHERE contract_id = $1 AND revision = 2`, active.ID)
	assertImmutableDatabaseFact(t, pool, `DELETE FROM audit_entries WHERE command_id = $1`, activeInput.CommandID)
}
