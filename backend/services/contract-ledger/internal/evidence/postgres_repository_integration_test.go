//go:build integration

package evidence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/party"
)

func TestPostgresRepositoryContractEvidence(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}

	newID := func() string {
		t.Helper()
		value, err := contract.NewCommandID()
		if err != nil {
			t.Fatalf("NewCommandID() error = %v", err)
		}
		return value
	}
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: newID(), DisplayName: "证据集成测试员"}
	suffix := strings.ToUpper(newID()[28:])
	contractService := contract.NewService(contract.NewPostgresRepository(pool))
	partyService := party.NewService(party.NewPostgresRepository(pool))
	contractIDs := make([]string, 0, 2)
	partyIDs := make([]string, 0, 2)
	defer func() { cleanupEvidenceFixtures(t, pool, contractIDs, partyIDs) }()

	firstParty, err := partyService.Create(ctx, party.CreateInput{
		Actor: actor, Kind: "organization", LegalName: "证据集成主体甲", RegistrationIdentifier: "EVIDENCE-A-" + suffix,
	})
	if err != nil {
		t.Fatalf("create first party: %v", err)
	}
	secondParty, err := partyService.Create(ctx, party.CreateInput{
		Actor: actor, Kind: "organization", LegalName: "证据集成主体乙", RegistrationIdentifier: "EVIDENCE-B-" + suffix,
	})
	if err != nil {
		t.Fatalf("create second party: %v", err)
	}
	partyIDs = append(partyIDs, firstParty.ID, secondParty.ID)

	createDraft := func(label string) contract.Contract {
		t.Helper()
		created, err := contractService.CreateDraft(ctx, contract.CreateDraftInput{
			CommandID: newID(), NumberNamespace: "evidence-integration",
			ContractNumber: label + "-" + suffix, Title: label + "证据合同",
			Currency: "CNY", TotalAmount: "1000", SignedOn: "2026-09-19", EffectiveOn: "2026-10-01", Actor: actor,
		})
		if err != nil {
			t.Fatalf("CreateDraft(%s) error = %v", label, err)
		}
		contractIDs = append(contractIDs, created.ID)
		return created
	}
	addPartiesAndActivate := func(created contract.Contract) {
		t.Helper()
		for index, partyID := range []string{firstParty.ID, secondParty.ID} {
			if _, err := contractService.AddDraftParty(ctx, contract.AddDraftPartyInput{
				CommandID: newID(), ContractID: created.ID, PartyID: partyID,
				Role: []string{"委托方", "受托方"}[index], ExpectedDraftVersion: int64(index + 1), Actor: actor,
			}); err != nil {
				t.Fatalf("AddDraftParty(%d) error = %v", index, err)
			}
		}
		if _, err := contractService.Activate(ctx, contract.ActivateContractInput{
			CommandID: newID(), ContractID: created.ID, ExpectedDraftVersion: 3, Actor: actor,
		}); err != nil {
			t.Fatalf("Activate() error = %v", err)
		}
	}

	storageRoot := t.TempDir()
	store, err := NewLocalStore(storageRoot)
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	repository := NewPostgresRepository(pool)
	service, err := NewService(repository, store, 1<<20)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	upload := func(contractID, targetType, targetID, title, filename, body string) UploadResult {
		t.Helper()
		result, err := service.Create(ctx, CreateInput{
			CommandID: newID(), ContractID: contractID, TargetType: targetType, TargetID: targetID,
			Title: title, Purpose: "验证各生命周期和本地文件完整性", OriginalFilename: filename, Actor: actor,
		}, strings.NewReader(body))
		if err != nil {
			t.Fatalf("Create evidence %s error = %v", title, err)
		}
		return result
	}

	primary := createDraft("PRIMARY")
	draftUploadCommandID := newID()
	draftContent := "draft evidence content"
	draftInput := CreateInput{
		CommandID: draftUploadCommandID, ContractID: primary.ID,
		TargetType: string(TargetContract), TargetID: primary.ID,
		Title: "草稿原始材料", Purpose: "草稿阶段上传且不得推进草稿版本",
		OriginalFilename: "draft.txt", Actor: actor,
	}
	draftEvidence, err := service.Create(ctx, draftInput, strings.NewReader(draftContent))
	if err != nil {
		t.Fatalf("Create(draft evidence) error = %v", err)
	}
	draftDetails, err := contractService.GetDraft(ctx, primary.ID)
	if err != nil || draftDetails.Contract.DraftVersion != 1 {
		t.Fatalf("draft evidence changed draft version: %#v error=%v", draftDetails.Contract, err)
	}
	replayed, err := service.Create(ctx, draftInput, strings.NewReader(draftContent))
	if err != nil || !replayed.Replayed || replayed.Version.ID != draftEvidence.Version.ID {
		t.Fatalf("replayed draft evidence = %#v error=%v", replayed, err)
	}
	_, err = service.Create(ctx, draftInput, strings.NewReader("changed content"))
	var idempotency *IdempotencyConflictError
	if !errors.As(err, &idempotency) {
		t.Fatalf("changed upload replay error = %v", err)
	}

	addPartiesAndActivate(primary)
	activeDetails, err := contractService.GetDraft(ctx, primary.ID)
	if err != nil || len(activeDetails.Instruments) != 1 {
		t.Fatalf("active details instruments=%d error=%v", len(activeDetails.Instruments), err)
	}
	upload(primary.ID, string(TargetInstrument), activeDetails.Instruments[0].ID, "原合同签署件", "original.pdf", "%PDF-1.7 original")

	settlement, err := contractService.RecordSettlement(ctx, contract.RecordContractSettlementInput{
		CommandID: newID(), ContractID: primary.ID, ExpectedContractRevision: 1, ExpectedSettlementSequence: 0,
		Kind: "progress", StatementNumber: "EVIDENCE-SET-" + suffix, Title: "证据阶段结算",
		Amount: "800", ConfirmedOn: "2026-10-04", Basis: "证据目标集成测试",
		ConfirmingPartyPositions: []int64{1, 2}, Actor: actor,
	})
	if err != nil {
		t.Fatalf("RecordSettlement() error = %v", err)
	}
	if _, err := contractService.Terminate(ctx, contract.TerminateContractInput{
		CommandID: newID(), ContractID: primary.ID, ExpectedRevision: 1,
		InstrumentNumber: "EVIDENCE-TERM-" + suffix, Title: "证据终止文书", TerminationReason: "验证 terminated 状态上传",
		SignedOn: "2026-10-05", EffectiveOn: "2026-10-05", EffectiveOrder: 2, Actor: actor,
	}); err != nil {
		t.Fatalf("Terminate() error = %v", err)
	}
	upload(primary.ID, string(TargetSettlement), settlement.Settlement.ID, "阶段结算确认件", "settlement.pdf", "%PDF-1.7 settlement")

	closed, err := contractService.CloseContract(ctx, contract.CloseContractInput{
		CommandID: newID(), ContractID: primary.ID, ExpectedContractRevision: 2, ExpectedSettlementSequence: 1,
		SettlementDisposition: "not_required", ClosedOn: "2026-10-06", Reason: "证据集成测试完成后关闭", Actor: actor,
	})
	if err != nil {
		t.Fatalf("CloseContract() error = %v", err)
	}
	upload(primary.ID, string(TargetClosure), closed.Closure.ID, "关闭依据文件", "closure.txt", "closure evidence")

	cancelled := createDraft("CANCELLED")
	if _, err := contractService.CancelDraft(ctx, contract.CancelDraftInput{
		CommandID: newID(), ContractID: cancelled.ID, ExpectedDraftVersion: 1,
		Reason: "验证 cancelled 状态证据", Actor: actor,
	}); err != nil {
		t.Fatalf("CancelDraft() error = %v", err)
	}
	upload(cancelled.ID, string(TargetContract), cancelled.ID, "取消依据", "cancelled.txt", "cancelled evidence")

	secondVersion, err := service.AddVersion(ctx, AddVersionInput{
		CommandID: newID(), ContractID: primary.ID, DocumentID: draftEvidence.Document.ID,
		ExpectedLatestVersion: 1, OriginalFilename: "draft-v2.txt", Actor: actor,
	}, strings.NewReader("draft evidence version two"))
	if err != nil || secondVersion.Version.Version != 2 {
		t.Fatalf("AddVersion() = %#v error=%v", secondVersion, err)
	}
	_, err = service.AddVersion(ctx, AddVersionInput{
		CommandID: newID(), ContractID: primary.ID, DocumentID: draftEvidence.Document.ID,
		ExpectedLatestVersion: 1, OriginalFilename: "stale.txt", Actor: actor,
	}, strings.NewReader("stale version"))
	var versionConflict *VersionConflictError
	if !errors.As(err, &versionConflict) || versionConflict.Actual != 2 {
		t.Fatalf("stale AddVersion() error = %v", err)
	}

	type concurrentResult struct {
		result UploadResult
		err    error
	}
	start := make(chan struct{})
	results := make(chan concurrentResult, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			result, err := service.AddVersion(context.Background(), AddVersionInput{
				CommandID: newID(), ContractID: primary.ID, DocumentID: draftEvidence.Document.ID,
				ExpectedLatestVersion: 2, OriginalFilename: fmt.Sprintf("concurrent-%d.txt", index), Actor: actor,
			}, strings.NewReader(fmt.Sprintf("concurrent evidence %d", index)))
			results <- concurrentResult{result: result, err: err}
		}(index)
	}
	close(start)
	wait.Wait()
	close(results)
	var thirdVersion Version
	successes, conflicts := 0, 0
	for result := range results {
		if result.err == nil {
			successes++
			thirdVersion = result.result.Version
			continue
		}
		if errors.As(result.err, &versionConflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent version error = %v", result.err)
	}
	if successes != 1 || conflicts != 1 || thirdVersion.Version != 3 {
		t.Fatalf("concurrent versions successes=%d conflicts=%d version=%#v", successes, conflicts, thirdVersion)
	}

	documents, err := service.List(ctx, primary.ID)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(documents) != 4 || documents[0].ID != draftEvidence.Document.ID || documents[0].Latest != 3 || len(documents[0].Versions) != 3 {
		t.Fatalf("primary evidence documents = %#v", documents)
	}
	download, err := service.Open(ctx, DownloadInput{
		ContractID: primary.ID, DocumentID: draftEvidence.Document.ID, VersionID: thirdVersion.ID, Actor: actor,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	downloaded, err := io.ReadAll(download.Content)
	_ = download.Content.Close()
	if err != nil || !strings.HasPrefix(string(downloaded), "concurrent evidence") {
		t.Fatalf("downloaded evidence = %q error=%v", downloaded, err)
	}

	_, err = service.Create(ctx, CreateInput{
		CommandID: newID(), ContractID: primary.ID,
		TargetType: string(TargetInstrument), TargetID: newID(),
		Title: "错误目标", Purpose: "必须拒绝", OriginalFilename: "wrong.txt", Actor: actor,
	}, strings.NewReader("must be cleaned"))
	var targetConflict *TargetConflictError
	if !errors.As(err, &targetConflict) {
		t.Fatalf("wrong target error = %v", err)
	}

	var existingAuditID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM audit_entries WHERE object_id = $1 LIMIT 1`, primary.ID).Scan(&existingAuditID); err != nil {
		t.Fatalf("read existing audit ID: %v", err)
	}
	rollbackDocumentID, rollbackVersionID := newID(), newID()
	rollbackService, err := NewService(repository, store, 1<<20)
	if err != nil {
		t.Fatalf("NewService(rollback) error = %v", err)
	}
	generated := []string{rollbackDocumentID, rollbackVersionID, existingAuditID}
	rollbackService.newID = func() (string, error) {
		value := generated[0]
		generated = generated[1:]
		return value, nil
	}
	objectsBeforeRollback := countStoredObjects(t, storageRoot)
	rollbackCommandID := newID()
	_, err = rollbackService.Create(ctx, CreateInput{
		CommandID: rollbackCommandID, ContractID: primary.ID,
		TargetType: string(TargetContract), TargetID: primary.ID,
		Title: "审计失败回滚", Purpose: "数据库与文件同时回滚", OriginalFilename: "rollback.txt", Actor: actor,
	}, strings.NewReader("rollback evidence"))
	if err == nil {
		t.Fatal("Create() with duplicate audit ID error = nil")
	}
	var rollbackFactCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_evidence_documents WHERE id = $1) +
			(SELECT count(*) FROM contract_evidence_versions WHERE id = $2) +
			(SELECT count(*) FROM contract_evidence_commands WHERE command_id = $3)
	`, rollbackDocumentID, rollbackVersionID, rollbackCommandID).Scan(&rollbackFactCount); err != nil {
		t.Fatalf("count rolled back evidence facts: %v", err)
	}
	if rollbackFactCount != 0 || countStoredObjects(t, storageRoot) != objectsBeforeRollback {
		t.Fatalf("rollback facts=%d objects=%d/%d", rollbackFactCount, countStoredObjects(t, storageRoot), objectsBeforeRollback)
	}

	var documentCount, versionCount, commandCount, createdAuditCount, versionAuditCount, downloadAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_evidence_documents WHERE contract_id = $1),
			(SELECT count(*) FROM contract_evidence_versions v JOIN contract_evidence_documents d ON d.id = v.document_id WHERE d.contract_id = $1),
			(SELECT count(*) FROM contract_evidence_commands WHERE contract_id = $1),
			(SELECT count(*) FROM audit_entries WHERE object_id = $1 AND action = 'contract_evidence_created'),
			(SELECT count(*) FROM audit_entries WHERE object_id = $1 AND action = 'contract_evidence_version_added'),
			(SELECT count(*) FROM audit_entries WHERE object_id = $1 AND action = 'contract_evidence_downloaded')
	`, primary.ID).Scan(&documentCount, &versionCount, &commandCount, &createdAuditCount, &versionAuditCount, &downloadAuditCount); err != nil {
		t.Fatalf("count primary evidence facts: %v", err)
	}
	if documentCount != 4 || versionCount != 6 || commandCount != 6 || createdAuditCount != 4 || versionAuditCount != 2 || downloadAuditCount != 1 {
		t.Fatalf("evidence facts documents=%d versions=%d commands=%d audits=%d/%d/%d", documentCount, versionCount, commandCount, createdAuditCount, versionAuditCount, downloadAuditCount)
	}
	if countStoredObjects(t, storageRoot) != 7 {
		t.Fatalf("stored object count = %d, want 7", countStoredObjects(t, storageRoot))
	}

	assertImmutableEvidenceFact(t, pool, `UPDATE contract_evidence_documents SET title = title WHERE id = $1`, draftEvidence.Document.ID)
	assertImmutableEvidenceFact(t, pool, `DELETE FROM contract_evidence_versions WHERE id = $1`, draftEvidence.Version.ID)
	assertImmutableEvidenceFact(t, pool, `UPDATE contract_evidence_commands SET occurred_at = occurred_at WHERE command_id = $1`, draftUploadCommandID)
	assertImmutableEvidenceFact(t, pool, `DELETE FROM audit_entries WHERE object_id = $1 AND action = 'contract_evidence_created'`, primary.ID)
}

func assertImmutableEvidenceFact(t *testing.T, pool *pgxpool.Pool, statement string, argument any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), statement, argument)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
		t.Fatalf("immutable evidence statement %q error = %v", statement, err)
	}
}

func cleanupEvidenceFixtures(t *testing.T, pool *pgxpool.Pool, contractIDs, partyIDs []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	connection, err := pool.Acquire(ctx)
	if err != nil {
		t.Errorf("acquire evidence cleanup connection: %v", err)
		return
	}
	defer connection.Release()
	if _, err := connection.Exec(ctx, `SET session_replication_role = replica`); err != nil {
		t.Errorf("disable evidence cleanup triggers: %v", err)
		return
	}
	defer func() { _, _ = connection.Exec(context.Background(), `SET session_replication_role = origin`) }()
	for _, contractID := range contractIDs {
		for _, statement := range []string{
			`DELETE FROM audit_entries WHERE object_id = $1`,
			`DELETE FROM contract_evidence_commands WHERE contract_id = $1`,
			`DELETE FROM contract_evidence_versions WHERE document_id IN (SELECT id FROM contract_evidence_documents WHERE contract_id = $1)`,
			`DELETE FROM contract_evidence_documents WHERE contract_id = $1`,
			`DELETE FROM contract_correction_commands WHERE contract_id = $1`,
			`DELETE FROM contract_closure_commands WHERE contract_id = $1`,
			`DELETE FROM contract_closures WHERE contract_id = $1`,
			`DELETE FROM contract_settlement_parties WHERE settlement_id IN (SELECT id FROM contract_settlements WHERE contract_id = $1)`,
			`DELETE FROM contract_settlement_commands WHERE contract_id = $1`,
			`DELETE FROM contract_settlements WHERE contract_id = $1`,
			`DELETE FROM contract_audit_entries WHERE contract_id = $1`,
			`DELETE FROM contract_termination_commands WHERE contract_id = $1`,
			`DELETE FROM contract_change_commands WHERE contract_id = $1`,
			`DELETE FROM contract_revision_parties WHERE contract_id = $1`,
			`DELETE FROM contract_revisions WHERE contract_id = $1`,
			`DELETE FROM contract_instruments WHERE contract_id = $1`,
			`DELETE FROM contract_activation_commands WHERE contract_id = $1`,
			`DELETE FROM contract_draft_party_commands WHERE contract_id = $1`,
			`DELETE FROM contract_draft_parties WHERE contract_id = $1`,
			`DELETE FROM contract_draft_change_commands WHERE contract_id = $1`,
			`DELETE FROM contracts WHERE id = $1`,
		} {
			if _, err := connection.Exec(ctx, statement, contractID); err != nil {
				t.Errorf("evidence cleanup contract %q: %v", contractID, err)
			}
		}
	}
	for _, partyID := range partyIDs {
		_, _ = connection.Exec(ctx, `DELETE FROM audit_entries WHERE object_id = $1`, partyID)
		_, _ = connection.Exec(ctx, `DELETE FROM parties WHERE id = $1`, partyID)
	}
}

func TestEvidenceBinaryReaderIsNotSilentlyTruncated(t *testing.T) {
	content := bytes.Repeat([]byte{0, 1, 2, 3, 255}, 1024)
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	stored, err := store.Put(context.Background(), "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	opened, err := store.Open(context.Background(), stored.StorageKey)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer opened.Close()
	actual, err := io.ReadAll(opened)
	if err != nil || !bytes.Equal(actual, content) {
		t.Fatalf("binary evidence length=%d/%d error=%v", len(actual), len(content), err)
	}
}
