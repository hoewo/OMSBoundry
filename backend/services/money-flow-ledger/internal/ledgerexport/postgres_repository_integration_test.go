//go:build integration

package ledgerexport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/internaltransfer"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/party"
	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestPostgresOwnerLedgerExportIncludesCompleteFactsAndExcludesOtherOwner(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := exportTestActor()
	commands := &exportCommandSequence{}
	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	obligationService := obligation.NewService(obligation.NewPostgresRepository(environment.Pool))
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	allocationService := allocation.NewService(allocation.NewPostgresRepository(environment.Pool))
	transferService := internaltransfer.NewService(internaltransfer.NewPostgresRepository(environment.Pool))

	owner := createExportParty(t, ctx, partyService, commands, actor, "=本方,主体\n原文", "OWNER-A")
	counterparty := createExportParty(t, ctx, partyService, commands, actor, "往来方", "COUNTERPARTY-A")
	otherOwner := createExportParty(t, ctx, partyService, commands, actor, "另一本方主体", "OWNER-B")
	otherCounterparty := createExportParty(t, ctx, partyService, commands, actor, "无关往来方", "COUNTERPARTY-B")

	_ = createExportObligation(t, ctx, obligationService, commands, actor,
		"receivable", owner.ID, counterparty.ID, "草稿义务", "7.000001", false)
	openObligation := createExportObligation(t, ctx, obligationService, commands, actor,
		"receivable", owner.ID, counterparty.ID, "=调整义务,含逗号\n换行", "100.1", true)
	adjusted, err := obligationService.AdjustAmount(ctx, obligation.AdjustAmountInput{
		CommandID: commands.Next(), ObligationID: openObligation.ID, ExpectedCurrentRevision: 1,
		Type: "increase", Amount: "20.2", Reason: "调额依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("adjust export obligation: %v", err)
	}
	if _, err := obligationService.RecordRemission(ctx, obligation.RecordRemissionInput{
		CommandID: commands.Next(), ObligationID: openObligation.ID, ExpectedCurrentRevision: adjusted.Obligation.CurrentRevision,
		Amount: "10.000001", Reason: "减免依据", ManualSourceReference: "减免文件", Actor: actor,
	}); err != nil {
		t.Fatalf("remit export obligation: %v", err)
	}
	if _, err := obligationService.RecordCashExpectation(ctx, obligation.RecordCashExpectationInput{
		CommandID: commands.Next(), ObligationID: openObligation.ID,
		ExpectedCurrentRevision: adjusted.Obligation.CurrentRevision,
		Action:                  "set", ExpectedOn: "2026-10-20", Reason: "首次预计回款", Actor: actor,
	}); err != nil {
		t.Fatalf("set export cash expectation: %v", err)
	}
	if _, err := obligationService.RecordCashExpectation(ctx, obligation.RecordCashExpectationInput{
		CommandID: commands.Next(), ObligationID: openObligation.ID,
		ExpectedCurrentRevision: adjusted.Obligation.CurrentRevision,
		Action:                  "set", ExpectedOn: "2026-10-25", Reason: "客户更新日期", Actor: actor,
	}); err != nil {
		t.Fatalf("update export cash expectation: %v", err)
	}

	receipt := createExportCash(t, ctx, cashService, commands, actor,
		"receipt", owner.ID, counterparty.ID, "收款", "100.2")
	applied, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: commands.Next(), CashTransactionID: receipt.ID, ObligationID: openObligation.ID,
		ExpectedObligationRevision: adjusted.Obligation.CurrentRevision, Amount: "20.000001",
		ManualSourceReference: "核销依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply export allocation: %v", err)
	}
	if _, err := allocationService.Reverse(ctx, allocation.ReverseInput{
		CommandID: commands.Next(), AllocationID: applied.Allocation.ID, Reason: "错误核销", Actor: actor,
	}); err != nil {
		t.Fatalf("reverse export allocation: %v", err)
	}

	reversedCash := createExportCash(t, ctx, cashService, commands, actor,
		"receipt", owner.ID, counterparty.ID, "错误收款", "9.9")
	if _, err := cashService.Reverse(ctx, cashtransaction.ReverseInput{
		CommandID: commands.Next(), CashTransactionID: reversedCash.ID, Reason: "错误流水", Actor: actor,
	}); err != nil {
		t.Fatalf("reverse export cash: %v", err)
	}

	closedObligation := createExportObligation(t, ctx, obligationService, commands, actor,
		"payable", owner.ID, counterparty.ID, "已关闭义务", "30", true)
	if _, err := obligationService.RecordRemission(ctx, obligation.RecordRemissionInput{
		CommandID: commands.Next(), ObligationID: closedObligation.ID, ExpectedCurrentRevision: 1,
		Amount: "30", Reason: "全部减免", ManualSourceReference: "关闭前减免", Actor: actor,
	}); err != nil {
		t.Fatalf("remit closed export obligation: %v", err)
	}
	if _, err := obligationService.Close(ctx, obligation.CloseInput{
		CommandID: commands.Next(), ObligationID: closedObligation.ID,
		ExpectedCurrentRevision: 1, ExpectedBalanceSequence: 1, Reason: "余额归零后关闭", Actor: actor,
	}); err != nil {
		t.Fatalf("close export obligation: %v", err)
	}

	if _, err := transferService.Create(ctx, internaltransfer.CreateInput{
		CommandID: commands.Next(), LedgerOwnerPartyID: owner.ID, Description: "内部调拨",
		Amount: "88.000001", Currency: "CNY", OccurredOn: "2026-09-20",
		SourceAccountReference: "基本户", DestinationAccountReference: "备用金",
		ManualSourceReference: "内部调拨依据", ExternalTransferReference: "MOVE-A", Actor: actor,
	}); err != nil {
		t.Fatalf("create export internal transfer: %v", err)
	}

	_ = createExportObligation(t, ctx, obligationService, commands, actor,
		"receivable", otherOwner.ID, otherCounterparty.ID, "无关义务", "999", true)
	_ = createExportCash(t, ctx, cashService, commands, actor,
		"receipt", otherOwner.ID, otherCounterparty.ID, "无关收款", "999")
	if _, err := transferService.Create(ctx, internaltransfer.CreateInput{
		CommandID: commands.Next(), LedgerOwnerPartyID: otherOwner.ID, Description: "无关转账",
		Amount: "999", Currency: "CNY", OccurredOn: "2026-09-20",
		SourceAccountReference: "其他一", DestinationAccountReference: "其他二",
		ManualSourceReference: "无关依据", Actor: actor,
	}); err != nil {
		t.Fatalf("create other owner transfer: %v", err)
	}

	beforeBusiness := readExportBusinessCounts(t, ctx, environment.Pool)
	service, err := NewService(NewPostgresRepository(environment.Pool))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	first, err := service.Export(ctx, Input{LedgerOwnerPartyID: owner.ID, Actor: actor})
	if err != nil {
		t.Fatalf("first Export() error = %v", err)
	}
	assertExportArchive(t, first, owner.ID, otherOwner.ID, otherCounterparty.ID, map[string]int{
		"parties.csv":                       2,
		"obligations.csv":                   3,
		"obligation_revisions.csv":          3,
		"obligation_amount_adjustments.csv": 1,
		"obligation_remissions.csv":         2,
		"obligation_closures.csv":           1,
		"obligation_cash_expectations.csv":  2,
		"cash_transactions.csv":             2,
		"cash_transaction_reversals.csv":    1,
		"allocations.csv":                   1,
		"allocation_reversals.csv":          1,
		"internal_transfers.csv":            1,
	})
	firstFiles := readArchive(t, first.Content)
	if strings.Contains(string(firstFiles["audit_entries.csv"]), ActionMoneyFlowLedgerExported) {
		t.Fatal("current export audit leaked into its own snapshot")
	}
	obligationRows := readCSV(t, firstFiles["obligations.csv"])
	if !csvContains(obligationRows, "=调整义务,含逗号\n换行") || !csvContains(obligationRows, "7.000001") {
		t.Fatalf("obligation text/precision missing: %#v", obligationRows)
	}

	second, err := service.Export(ctx, Input{LedgerOwnerPartyID: owner.ID, Actor: actor})
	if err != nil {
		t.Fatalf("second Export() error = %v", err)
	}
	secondAuditRows := readCSV(t, readArchive(t, second.Content)["audit_entries.csv"])
	if countCSVValue(secondAuditRows, ActionMoneyFlowLedgerExported) != 1 {
		t.Fatalf("second export prior audit count = %d", countCSVValue(secondAuditRows, ActionMoneyFlowLedgerExported))
	}
	afterBusiness := readExportBusinessCounts(t, ctx, environment.Pool)
	if beforeBusiness != afterBusiness {
		t.Fatalf("export changed business facts: before=%v after=%v", beforeBusiness, afterBusiness)
	}
	var exportAudits int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_entries
		WHERE object_type = 'ledger_export' AND object_id = $1 AND action = $2
	`, owner.ID, ActionMoneyFlowLedgerExported).Scan(&exportAudits); err != nil {
		t.Fatalf("count export audits: %v", err)
	}
	if exportAudits != 2 {
		t.Fatalf("export audit count = %d", exportAudits)
	}
}

func TestPostgresOwnerLedgerExportUsesOneRepeatableReadSnapshot(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := exportTestActor()
	commands := &exportCommandSequence{}
	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	owner := createExportParty(t, ctx, partyService, commands, actor, "快照本方", "SNAPSHOT-OWNER")
	counterparty := createExportParty(t, ctx, partyService, commands, actor, "快照往来方", "SNAPSHOT-COUNTERPARTY")

	repository := NewPostgresRepository(environment.Pool)
	snapshotStarted := make(chan struct{})
	releaseSnapshot := make(chan struct{})
	repository.afterTableLoaded = func(filename string) {
		if filename == "obligations.csv" {
			close(snapshotStarted)
			<-releaseSnapshot
		}
	}
	type loadResult struct {
		snapshot Snapshot
		found    bool
		err      error
	}
	resultChannel := make(chan loadResult, 1)
	go func() {
		snapshot, found, err := repository.Load(ctx, owner.ID)
		resultChannel <- loadResult{snapshot: snapshot, found: found, err: err}
	}()
	<-snapshotStarted
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	created := createExportCash(t, ctx, cashService, commands, actor,
		"receipt", owner.ID, counterparty.ID, "快照后收款", "12.34")
	close(releaseSnapshot)
	result := <-resultChannel
	if result.err != nil || !result.found {
		t.Fatalf("Load() found=%t error=%v", result.found, result.err)
	}
	if got := len(findExportTable(t, result.snapshot, "cash_transactions.csv").Rows); got != 0 {
		t.Fatalf("snapshot included later cash rows = %d", got)
	}
	repository.afterTableLoaded = nil
	next, found, err := repository.Load(ctx, owner.ID)
	if err != nil || !found {
		t.Fatalf("second Load() found=%t error=%v", found, err)
	}
	cashRows := findExportTable(t, next, "cash_transactions.csv").Rows
	if len(cashRows) != 1 || cashRows[0][0] != created.ID {
		t.Fatalf("next snapshot cash rows = %#v", cashRows)
	}
}

func TestPostgresOwnerLedgerExportRejectsCrossOwnerAllocationCorruption(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}
	actor := exportTestActor()
	commands := &exportCommandSequence{}
	partyService := party.NewService(party.NewPostgresRepository(environment.Pool))
	ownerA := createExportParty(t, ctx, partyService, commands, actor, "损坏账簿 A", "BROKEN-A")
	ownerB := createExportParty(t, ctx, partyService, commands, actor, "损坏账簿 B", "BROKEN-B")
	counterparty := createExportParty(t, ctx, partyService, commands, actor, "损坏测试往来方", "BROKEN-C")
	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(environment.Pool))
	obligationService := obligation.NewService(obligation.NewPostgresRepository(environment.Pool))
	cash := createExportCash(t, ctx, cashService, commands, actor,
		"receipt", ownerA.ID, counterparty.ID, "A 收款", "10")
	foreignObligation := createExportObligation(t, ctx, obligationService, commands, actor,
		"receivable", ownerB.ID, counterparty.ID, "B 应收", "10", true)

	tx, err := environment.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin corruption transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	for _, trigger := range []string{"allocations_validate_insert", "allocations_validate_remission_insert"} {
		if _, err := tx.Exec(ctx, "ALTER TABLE allocations DISABLE TRIGGER "+trigger); err != nil {
			t.Fatalf("disable %s: %v", trigger, err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id, applied_by_actor_display_name, applied_at
		) VALUES (
			'99999999-9999-4999-8999-999999999999', $1, $2, 1,
			1, 'CNY', 'manual', '故意损坏的跨主体核销',
			$3, $4, $5, clock_timestamp()
		)
	`, cash.ID, foreignObligation.ID, actor.Kind, actor.ID, actor.DisplayName); err != nil {
		t.Fatalf("insert cross-owner allocation: %v", err)
	}
	for _, trigger := range []string{"allocations_validate_insert", "allocations_validate_remission_insert"} {
		if _, err := tx.Exec(ctx, "ALTER TABLE allocations ENABLE TRIGGER "+trigger); err != nil {
			t.Fatalf("enable %s: %v", trigger, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit corruption transaction: %v", err)
	}

	service, _ := NewService(NewPostgresRepository(environment.Pool))
	artifact, err := service.Export(ctx, Input{LedgerOwnerPartyID: ownerA.ID, Actor: actor})
	var invariant *SnapshotInvariantError
	if !errors.As(err, &invariant) || !strings.Contains(invariant.Error(), "跨越本方账簿主体") {
		t.Fatalf("Export() artifact=%#v error=%v", artifact, err)
	}
	if len(artifact.Content) != 0 {
		t.Fatalf("corrupt export returned %d bytes", len(artifact.Content))
	}
	var exportAudits int
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_entries WHERE action = $1`, ActionMoneyFlowLedgerExported).Scan(&exportAudits); err != nil {
		t.Fatalf("count corrupt export audits: %v", err)
	}
	if exportAudits != 0 {
		t.Fatalf("corrupt export audit count = %d", exportAudits)
	}
}

type exportCommandSequence struct{ value int }

func (s *exportCommandSequence) Next() string {
	s.value++
	return fmt.Sprintf("70000000-0000-4000-8000-%012x", s.value)
}

func exportTestActor() audit.ActorRef {
	return audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "60000000-0000-4000-8000-000000000001", DisplayName: "台账导出管理员",
	}
}

func createExportParty(
	t *testing.T,
	ctx context.Context,
	service *party.Service,
	commands *exportCommandSequence,
	actor audit.ActorRef,
	name string,
	registration string,
) party.Party {
	t.Helper()
	result, err := service.Create(ctx, party.CreateInput{
		CommandID: commands.Next(), Kind: "organization", LegalName: name,
		RegistrationIdentifier: registration, Actor: actor,
	})
	if err != nil {
		t.Fatalf("create export party %q: %v", name, err)
	}
	return result.Party
}

func createExportObligation(
	t *testing.T,
	ctx context.Context,
	service *obligation.Service,
	commands *exportCommandSequence,
	actor audit.ActorRef,
	direction string,
	ownerID string,
	counterpartyID string,
	title string,
	amount string,
	confirm bool,
) obligation.Obligation {
	t.Helper()
	created, err := service.CreateDraft(ctx, obligation.CreateDraftInput{
		CommandID: commands.Next(), Direction: direction, LedgerOwnerPartyID: ownerID,
		CounterpartyPartyID: counterpartyID, Title: title, OriginalAmount: amount,
		Currency: "CNY", DueOn: "2026-12-31", ManualSourceReference: "导出测试义务依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create export obligation %q: %v", title, err)
	}
	if !confirm {
		return created.Obligation
	}
	confirmed, err := service.Confirm(ctx, obligation.ConfirmInput{
		CommandID: commands.Next(), ObligationID: created.Obligation.ID,
		ExpectedDraftVersion: created.Obligation.DraftVersion, Actor: actor,
	})
	if err != nil {
		t.Fatalf("confirm export obligation %q: %v", title, err)
	}
	return confirmed.Obligation
}

func createExportCash(
	t *testing.T,
	ctx context.Context,
	service *cashtransaction.Service,
	commands *exportCommandSequence,
	actor audit.ActorRef,
	typeValue string,
	ownerID string,
	counterpartyID string,
	description string,
	amount string,
) cashtransaction.Transaction {
	t.Helper()
	created, err := service.Create(ctx, cashtransaction.CreateInput{
		CommandID: commands.Next(), Type: typeValue, LedgerOwnerPartyID: ownerID,
		CounterpartyPartyID: counterpartyID, Description: description, Amount: amount,
		Currency: "CNY", OccurredOn: "2026-09-20", AccountReference: "测试账户",
		ManualSourceReference: "导出测试资金依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create export cash %q: %v", description, err)
	}
	return created.Transaction
}

type exportBusinessCounts struct {
	Parties             int
	Obligations         int
	Revisions           int
	Adjustments         int
	Remissions          int
	Closures            int
	CashExpectations    int
	Cash                int
	CashReversals       int
	Allocations         int
	AllocationReversals int
	Transfers           int
}

func readExportBusinessCounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool) exportBusinessCounts {
	t.Helper()
	var value exportBusinessCounts
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM parties),
			(SELECT count(*) FROM obligations),
			(SELECT count(*) FROM obligation_revisions),
			(SELECT count(*) FROM obligation_amount_adjustments),
			(SELECT count(*) FROM obligation_remissions),
			(SELECT count(*) FROM obligation_closures),
			(SELECT count(*) FROM obligation_cash_expectations),
			(SELECT count(*) FROM cash_transactions),
			(SELECT count(*) FROM cash_transaction_reversals),
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_reversals),
			(SELECT count(*) FROM internal_transfers)
	`).Scan(
		&value.Parties, &value.Obligations, &value.Revisions, &value.Adjustments,
		&value.Remissions, &value.Closures, &value.CashExpectations, &value.Cash, &value.CashReversals,
		&value.Allocations, &value.AllocationReversals, &value.Transfers,
	); err != nil {
		t.Fatalf("read export business counts: %v", err)
	}
	return value
}

func assertExportArchive(t *testing.T, artifact Artifact, ownerID string, excludedIDs ...any) {
	t.Helper()
	expectedCounts, ok := excludedIDs[len(excludedIDs)-1].(map[string]int)
	if !ok {
		t.Fatal("expected counts argument missing")
	}
	excluded := make([]string, 0, len(excludedIDs)-1)
	for _, value := range excludedIDs[:len(excludedIDs)-1] {
		excluded = append(excluded, value.(string))
	}
	files := readArchive(t, artifact.Content)
	if len(files) != len(tableSpecs)+1 {
		t.Fatalf("archive files = %d", len(files))
	}
	for _, spec := range tableSpecs {
		rows := readCSV(t, files[spec.Filename])
		if expected, exists := expectedCounts[spec.Filename]; exists && len(rows)-1 != expected {
			t.Fatalf("%s count = %d, want %d", spec.Filename, len(rows)-1, expected)
		}
		for _, row := range rows {
			for _, cell := range row {
				for _, excludedID := range excluded {
					if cell == excludedID {
						t.Fatalf("%s leaked excluded ID %s", spec.Filename, excludedID)
					}
				}
			}
		}
	}
	if artifact.Manifest.LedgerOwner.ID != ownerID || artifact.Manifest.SourceSchemaVersion != "000026_internal_finance_payments.sql" {
		t.Fatalf("artifact manifest = %#v", artifact.Manifest)
	}
}

func csvContains(rows [][]string, expected string) bool {
	return countCSVValue(rows, expected) > 0
}

func countCSVValue(rows [][]string, expected string) int {
	count := 0
	for _, row := range rows {
		for _, cell := range row {
			if cell == expected {
				count++
			}
		}
	}
	return count
}

func findExportTable(t *testing.T, snapshot Snapshot, filename string) Table {
	t.Helper()
	for _, table := range snapshot.Tables {
		if table.Filename == filename {
			return table
		}
	}
	t.Fatalf("snapshot table %s not found", filename)
	return Table{}
}
