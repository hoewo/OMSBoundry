//go:build integration

package ledgerrestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/evidence"
	"omsbizfoundry/contract-ledger/internal/ledgerexport"
	"omsbizfoundry/contract-ledger/internal/party"
)

func TestPostgresRestoreDraftIsAtomicAndIdempotent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := openIsolatedRestoreDatabase(t, ctx, databaseURL)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ledger := testDraftLedger()
	ledger.Evidence = []ledgerexport.EvidenceDocument{{
		ID: "88888888-8888-4888-8888-888888888888", TargetType: "contract", TargetID: testContractID,
		Title: "签署稿", Purpose: "恢复验证",
		Actor:     ledgerexport.Actor{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "源管理员"},
		CreatedAt: "2026-09-19T09:10:00Z",
		Versions: []ledgerexport.EvidenceVersion{{
			ID: "99999999-9999-4999-8999-999999999999", Version: 1,
			OriginalFilename: "contract.pdf", DetectedMediaType: "application/pdf",
			SizeBytes: 4, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ContentAvailable: true,
			Actor:            ledgerexport.Actor{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "源管理员"},
			RecordedAt:       "2026-09-19T09:10:00Z",
		}},
	}}
	evidenceCommandID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	ledger.Audit = append(ledger.Audit, ledgerexport.AuditEntry{
		ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Action: "contract_evidence_created",
		CommandID:  &evidenceCommandID,
		Actor:      ledgerexport.Actor{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "源管理员"},
		OccurredAt: "2026-09-19T09:10:00Z", Summary: "上传合同证据：签署稿 v1",
		Details: []byte(`{"document_id":"88888888-8888-4888-8888-888888888888","version_id":"99999999-9999-4999-8999-999999999999","version":1}`),
	})
	archive := testArchive(t, ledger, nil)
	repository := NewPostgresRepository(pool)
	service, _ := NewService(repository)
	service.now = func() time.Time { return time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC) }
	service.newID = func() (string, error) { return "cccccccc-cccc-4ccc-8ccc-cccccccccccc", nil }
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", DisplayName: "恢复管理员"}
	commandID := "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"

	result, err := service.Restore(ctx, Input{CommandID: commandID, Archive: archive, Actor: actor})
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if result.ContractID != testContractID || result.Status != "draft" || result.MissingEvidenceVersions != 1 || result.Replayed {
		t.Fatalf("result = %#v", result)
	}
	replayed, err := service.Restore(ctx, Input{CommandID: commandID, Archive: archive, Actor: actor})
	if err != nil || !replayed.Replayed {
		t.Fatalf("same command replay=%#v error=%v", replayed, err)
	}
	digestReplay, err := service.Restore(ctx, Input{
		CommandID: "ffffffff-ffff-4fff-8fff-ffffffffffff", Archive: archive,
		Actor: audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "12121212-1212-4212-8212-121212121212", DisplayName: "另一管理员"},
	})
	if err != nil || !digestReplay.Replayed {
		t.Fatalf("digest replay=%#v error=%v", digestReplay, err)
	}
	_, err = service.Restore(ctx, Input{
		CommandID: commandID, Archive: archive,
		Actor: audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "12121212-1212-4212-8212-121212121212", DisplayName: "另一管理员"},
	})
	var idempotency *IdempotencyConflictError
	if !errors.As(err, &idempotency) {
		t.Fatalf("changed actor error = %T %v", err, err)
	}

	var contentStatus string
	var provider, key *string
	if err := pool.QueryRow(ctx, `
		SELECT content_status, storage_provider, storage_key
		FROM contract_evidence_versions WHERE id = $1
	`, "99999999-9999-4999-8999-999999999999").Scan(&contentStatus, &provider, &key); err != nil {
		t.Fatalf("query evidence version: %v", err)
	}
	if contentStatus != "missing" || provider != nil || key != nil {
		t.Fatalf("content status=%q provider=%v key=%v", contentStatus, provider, key)
	}
	var restoreOperations, restoreAudits, contracts, evidenceVersions int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_restore_operations),
			(SELECT count(*) FROM audit_entries WHERE action = 'contract_ledger_restored'),
			(SELECT count(*) FROM contracts),
			(SELECT count(*) FROM contract_evidence_versions)
	`).Scan(&restoreOperations, &restoreAudits, &contracts, &evidenceVersions); err != nil {
		t.Fatal(err)
	}
	if restoreOperations != 1 || restoreAudits != 1 || contracts != 1 || evidenceVersions != 1 {
		t.Fatalf("operations=%d audits=%d contracts=%d versions=%d", restoreOperations, restoreAudits, contracts, evidenceVersions)
	}

	exported, found, err := ledgerexport.NewPostgresRepository(pool).Load(ctx, testContractID)
	if err != nil || !found {
		t.Fatalf("Load() found=%v error=%v", found, err)
	}
	if exported.Contract.Status != "draft" || len(exported.Evidence) != 1 || exported.Evidence[0].Versions[0].ContentAvailable {
		t.Fatalf("exported ledger = %#v", exported)
	}

	other := testDraftLedger()
	other.Contract.ID = "13131313-1313-4313-8313-131313131313"
	other.Contract.ContractNumber = "DRAFT-002"
	_, err = service.Restore(ctx, Input{
		CommandID: "14141414-1414-4414-8414-141414141414", Archive: testArchiveWithContractID(t, other), Actor: actor,
	})
	var nonEmpty *DestinationNotEmptyError
	if !errors.As(err, &nonEmpty) {
		t.Fatalf("non-empty error = %T %v", err, err)
	}
}

func TestPostgresRestoreCancelledDraftPreservesCancellation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := openIsolatedRestoreDatabase(t, ctx, databaseURL)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ledger := testDraftLedger()
	ledger.Contract.Status = "cancelled"
	ledger.Contract.DraftVersion = 2
	cancelCommand := "18181818-1818-4818-8818-181818181818"
	cancelActor := ledgerexport.Actor{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "源管理员"}
	ledger.Cancellation = &ledgerexport.DraftCancellation{
		CommandID: cancelCommand, ExpectedDraftVersion: 1, ResultingDraftVersion: 2,
		Reason: "测试取消恢复", Actor: cancelActor, CancelledAt: "2026-09-19T09:30:00Z",
	}
	ledger.Audit = append(ledger.Audit, ledgerexport.AuditEntry{
		ID: "19191919-1919-4919-8919-191919191919", Action: "contract_draft_cancelled",
		CommandID: &cancelCommand, Actor: cancelActor, OccurredAt: "2026-09-19T09:30:00Z",
		Summary: "取消合同草稿：版本 1 → 2", Details: []byte(`{"expected_draft_version":1,"resulting_draft_version":2}`),
	})
	service, _ := NewService(NewPostgresRepository(pool))
	result, err := service.Restore(ctx, Input{
		CommandID: "20202020-2020-4020-8020-202020202020", Archive: testArchive(t, ledger, nil),
		Actor: audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "21212121-2121-4121-8121-212121212121", DisplayName: "恢复管理员"},
	})
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if result.Status != "cancelled" || result.CurrentRevision != 0 {
		t.Fatalf("result = %#v", result)
	}
	var reason string
	var expected, resulting int64
	if err := pool.QueryRow(ctx, `
		SELECT cancellation_reason, expected_draft_version, resulting_draft_version
		FROM contract_draft_change_commands
		WHERE contract_id = $1 AND action = 'cancel'
	`, testContractID).Scan(&reason, &expected, &resulting); err != nil {
		t.Fatal(err)
	}
	if reason != "测试取消恢复" || expected != 1 || resulting != 2 {
		t.Fatalf("cancellation reason=%q expected=%d resulting=%d", reason, expected, resulting)
	}
}

func TestPostgresRestoreRoundTripsClosedFormalHistory(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	sourcePool := openIsolatedRestoreDatabase(t, ctx, databaseURL)
	targetPool := openIsolatedRestoreDatabase(t, ctx, databaseURL)
	if err := database.Migrate(ctx, sourcePool); err != nil {
		t.Fatalf("migrate source: %v", err)
	}
	if err := database.Migrate(ctx, targetPool); err != nil {
		t.Fatalf("migrate target: %v", err)
	}

	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "业务管理员"}
	partyService := party.NewService(party.NewPostgresRepository(sourcePool))
	firstParty, err := partyService.Create(ctx, party.CreateInput{
		Kind: "organization", LegalName: "甲方有限公司", RegistrationIdentifier: "91310000TESTA", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create first party: %v", err)
	}
	secondParty, err := partyService.Create(ctx, party.CreateInput{
		Kind: "organization", LegalName: "乙方有限公司", RegistrationIdentifier: "91310000TESTB", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create second party: %v", err)
	}

	contractService := contract.NewService(contract.NewPostgresRepository(sourcePool))
	draft, err := contractService.CreateDraft(ctx, contract.CreateDraftInput{
		CommandID: mustCommandID(t), NumberNamespace: "ROUNDTRIP", ContractNumber: "RT-001",
		Title: "恢复往返合同", Category: "技术服务", Currency: "CNY", TotalAmount: "100.25",
		SignedOn: "2026-09-01", EffectiveOn: "2026-09-02", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	firstLink, err := contractService.AddDraftParty(ctx, contract.AddDraftPartyInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, PartyID: firstParty.ID,
		Role: "采购方", ExpectedDraftVersion: 1, Actor: actor,
	})
	if err != nil {
		t.Fatalf("add first party: %v", err)
	}
	_, err = contractService.AddDraftParty(ctx, contract.AddDraftPartyInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, PartyID: secondParty.ID,
		Role: "服务方", ExpectedDraftVersion: firstLink.DraftVersion, Actor: actor,
	})
	if err != nil {
		t.Fatalf("add second party: %v", err)
	}
	if _, err := contractService.Activate(ctx, contract.ActivateContractInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, ExpectedDraftVersion: 3, Actor: actor,
	}); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if _, err := contractService.AppendChange(ctx, contract.AppendContractChangeInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, ExpectedRevision: 1,
		InstrumentNumber: "RT-SUP-001", Title: "金额补充协议", ChangeSummary: "增加服务范围",
		SignedOn: "2026-09-03", EffectiveOn: "2026-09-04", EffectiveOrder: 2,
		AmountEffect: "adjustment", AmountValue: "49.75", Actor: actor,
	}); err != nil {
		t.Fatalf("append supplement: %v", err)
	}
	if _, err := contractService.RecordSettlement(ctx, contract.RecordContractSettlementInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, ExpectedContractRevision: 2,
		ExpectedSettlementSequence: 0, Kind: "progress", StatementNumber: "RT-SET-001",
		Title: "阶段结算", Amount: "80", ConfirmedOn: "2026-09-05", Basis: "双方确认",
		ConfirmingPartyPositions: []int64{1, 2}, Actor: actor,
	}); err != nil {
		t.Fatalf("record progress settlement: %v", err)
	}
	if _, err := contractService.Terminate(ctx, contract.TerminateContractInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, ExpectedRevision: 2,
		InstrumentNumber: "RT-TERM-001", Title: "终止协议", TerminationReason: "服务完成后协商终止",
		SignedOn: "2026-09-06", EffectiveOn: "2026-09-07", EffectiveOrder: 3, Actor: actor,
	}); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	finalSettlement, err := contractService.RecordSettlement(ctx, contract.RecordContractSettlementInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, ExpectedContractRevision: 3,
		ExpectedSettlementSequence: 1, Kind: "final", StatementNumber: "RT-SET-002",
		Title: "最终结算", Amount: "150", ConfirmedOn: "2026-09-08", Basis: "最终确认",
		ConfirmingPartyPositions: []int64{1, 2}, Actor: actor,
	})
	if err != nil {
		t.Fatalf("record final settlement: %v", err)
	}
	closureResult, err := contractService.CloseContract(ctx, contract.CloseContractInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, ExpectedContractRevision: 3,
		ExpectedSettlementSequence: finalSettlement.Settlement.Sequence,
		SettlementDisposition:      "final_recorded", ClosedOn: "2026-09-09", Reason: "履约及最终结算完成", Actor: actor,
	})
	if err != nil {
		t.Fatalf("close contract: %v", err)
	}
	if _, err := contractService.CorrectContract(ctx, contract.CorrectContractInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, ExpectedStatus: "closed", ExpectedRevision: 3,
		InstrumentNumber: "RT-CORR-001", InstrumentTitle: "金额更正确认", CorrectionReason: "纠正录入差异",
		InstrumentSignedOn: "2026-09-10", InstrumentEffectiveOn: "2026-09-10", EffectiveOrder: 4,
		CorrectedTitle: "恢复往返合同（更正）", CorrectedCategory: "技术服务", CorrectedTotalAmount: "151.25",
		CorrectedSignedOn: "2026-09-01", CorrectedEffectiveOn: "2026-09-02", Actor: actor,
	}); err != nil {
		t.Fatalf("correct closed contract: %v", err)
	}

	sourceStore, err := evidence.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sourceEvidence, _ := evidence.NewService(evidence.NewPostgresRepository(sourcePool), sourceStore, 1<<20)
	upload, err := sourceEvidence.Create(ctx, evidence.CreateInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, TargetType: "closure",
		TargetID: closureResult.Closure.ID, Title: "关闭确认", Purpose: "关闭依据",
		OriginalFilename: "closure.txt", Actor: actor,
	}, bytes.NewBufferString("closed and verified"))
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	exportService, _ := ledgerexport.NewService(ledgerexport.NewPostgresRepository(sourcePool))
	artifact, err := exportService.Export(ctx, ledgerexport.Input{ContractID: draft.ID, Actor: actor})
	if err != nil {
		t.Fatalf("export source: %v", err)
	}
	sourceCandidate, err := ParseArchive(artifact.Content)
	if err != nil {
		t.Fatalf("parse source archive: %v", err)
	}
	if sourceCandidate.Ledger.Contract.Status != "closed" || len(sourceCandidate.Ledger.Revisions) != 4 ||
		len(sourceCandidate.Ledger.Instruments) != 4 || len(sourceCandidate.Ledger.Settlements) != 2 ||
		sourceCandidate.Ledger.Closure == nil || len(sourceCandidate.Ledger.Evidence) != 1 {
		t.Fatalf("source candidate shape = %#v", sourceCandidate.Ledger)
	}

	restoreService, _ := NewService(NewPostgresRepository(targetPool))
	result, err := restoreService.Restore(ctx, Input{
		CommandID: mustCommandID(t), Archive: artifact.Content,
		Actor: audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "45454545-4545-4454-8454-454545454545", DisplayName: "恢复管理员"},
	})
	if err != nil {
		t.Fatalf("restore target: %v", err)
	}
	if result.Status != "closed" || result.CurrentRevision != 4 || result.MissingEvidenceVersions != 1 {
		t.Fatalf("restore result = %#v", result)
	}
	targetLedger, found, err := ledgerexport.NewPostgresRepository(targetPool).Load(ctx, draft.ID)
	if err != nil || !found {
		t.Fatalf("load target found=%v error=%v", found, err)
	}
	if targetLedger.Contract.Title != sourceCandidate.Ledger.Contract.Title ||
		targetLedger.Contract.TotalAmount == nil || *targetLedger.Contract.TotalAmount != "151.250000" ||
		len(targetLedger.DraftParties) != 2 || len(targetLedger.Revisions) != 4 ||
		len(targetLedger.Instruments) != 4 || len(targetLedger.Settlements) != 2 ||
		targetLedger.Closure == nil || targetLedger.Closure.ID != closureResult.Closure.ID ||
		len(targetLedger.Evidence) != 1 || targetLedger.Evidence[0].Versions[0].ContentAvailable ||
		len(targetLedger.Audit) != len(sourceCandidate.Ledger.Audit)+1 {
		t.Fatalf("target ledger shape = %#v", targetLedger)
	}
	if _, err := targetPool.Exec(ctx, `UPDATE contract_instruments SET title = 'tampered' WHERE contract_id = $1`, draft.ID); err == nil {
		t.Fatal("restored immutable instrument accepted UPDATE")
	}

	targetStore, err := evidence.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	targetEvidence, _ := evidence.NewService(evidence.NewPostgresRepository(targetPool), targetStore, 1<<20)
	_, err = targetEvidence.Open(ctx, evidence.DownloadInput{
		ContractID: draft.ID, DocumentID: upload.Document.ID, VersionID: upload.Version.ID,
		Actor: actor,
	})
	var unavailable *evidence.UnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("restored missing evidence open error = %T %v", err, err)
	}
	var missingDownloadAudits int
	if err := targetPool.QueryRow(ctx, `
		SELECT count(*) FROM audit_entries
		WHERE object_id = $1 AND action = 'contract_evidence_downloaded'
	`, draft.ID).Scan(&missingDownloadAudits); err != nil {
		t.Fatal(err)
	}
	if missingDownloadAudits != 0 {
		t.Fatalf("missing evidence produced %d download audits", missingDownloadAudits)
	}
	added, err := targetEvidence.AddVersion(ctx, evidence.AddVersionInput{
		CommandID: mustCommandID(t), ContractID: draft.ID, DocumentID: upload.Document.ID,
		ExpectedLatestVersion: 1, OriginalFilename: "closure-v2.txt", Actor: actor,
	}, bytes.NewBufferString("locally reattached evidence"))
	if err != nil || added.Version.Version != 2 || !added.Version.ContentAvailable() {
		t.Fatalf("add post-restore version = %#v error=%v", added, err)
	}
}

func TestPostgresRestoreRollsBackLateAuditFailure(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := openIsolatedRestoreDatabase(t, ctx, databaseURL)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	candidate, err := ParseArchive(testArchive(t, testDraftLedger(), nil))
	if err != nil {
		t.Fatal(err)
	}
	extraCommand := "15151515-1515-4515-8515-151515151515"
	candidate.Ledger.Audit = append(candidate.Ledger.Audit, ledgerexport.AuditEntry{
		ID:     candidate.Ledger.Audit[0].ID,
		Action: "contract_ledger_exported", CommandID: &extraCommand,
		Actor: candidate.Ledger.Audit[0].Actor, OccurredAt: "2026-09-19T10:00:00Z",
		Summary: "制造末端审计主键冲突", Details: []byte(`{}`),
	})
	_, _, err = NewPostgresRepository(pool).Restore(ctx, Command{
		CommandID:  "16161616-1616-4616-8616-161616161616",
		AuditID:    "17171717-1717-4717-8717-171717171717",
		Candidate:  candidate,
		Actor:      audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "恢复管理员"},
		RestoredAt: time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("Restore() error = nil")
	}
	var contracts, parties, audits, operations int
	if queryErr := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contracts),
			(SELECT count(*) FROM parties),
			(SELECT count(*) FROM audit_entries),
			(SELECT count(*) FROM contract_restore_operations)
	`).Scan(&contracts, &parties, &audits, &operations); queryErr != nil {
		t.Fatal(queryErr)
	}
	if contracts != 0 || parties != 0 || audits != 0 || operations != 0 {
		t.Fatalf("partial restore remained: contracts=%d parties=%d audits=%d operations=%d", contracts, parties, audits, operations)
	}
}

func mustCommandID(t *testing.T) string {
	t.Helper()
	value, err := contract.NewCommandID()
	if err != nil {
		t.Fatalf("generate command ID: %v", err)
	}
	return value
}

func testArchiveWithContractID(t *testing.T, ledger ledgerexport.Ledger) []byte {
	t.Helper()
	// The unit helper uses the canonical test ID in the manifest, so use a temporary
	// manifest-compatible ID only to exercise the database non-empty check.
	ledger.Contract.ID = testContractID
	ledger.Contract.ContractNumber = "DRAFT-CHANGED"
	return testArchive(t, ledger, nil)
}

func openIsolatedRestoreDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	baseConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	adminConfig := baseConfig.Copy()
	adminConfig.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("open administration database: %v", err)
	}
	defer adminPool.Close()
	databaseName := fmt.Sprintf("contract_ledger_restore_%d", time.Now().UnixNano())
	quotedName := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminPool.Exec(ctx, `CREATE DATABASE `+quotedName); err != nil {
		t.Fatalf("create isolated database: %v", err)
	}
	testConfig := baseConfig.Copy()
	testConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		t.Fatalf("open isolated database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cleanupConfig := baseConfig.Copy()
		cleanupConfig.ConnConfig.Database = "postgres"
		cleanupPool, cleanupErr := pgxpool.NewWithConfig(cleanupCtx, cleanupConfig)
		if cleanupErr != nil {
			t.Errorf("open cleanup database: %v", cleanupErr)
			return
		}
		defer cleanupPool.Close()
		if _, cleanupErr := cleanupPool.Exec(cleanupCtx, `DROP DATABASE `+quotedName+` WITH (FORCE)`); cleanupErr != nil {
			t.Errorf("drop isolated database %q: %v", databaseName, cleanupErr)
		}
	})
	return pool
}
