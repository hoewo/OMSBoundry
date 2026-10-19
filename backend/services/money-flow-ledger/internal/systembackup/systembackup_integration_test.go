//go:build integration

package systembackup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/money-flow-ledger/internal/allocation"
	"omsbizfoundry/money-flow-ledger/internal/audit"
	"omsbizfoundry/money-flow-ledger/internal/cashtransaction"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/internaltransfer"
	"omsbizfoundry/money-flow-ledger/internal/obligation"
	"omsbizfoundry/money-flow-ledger/internal/party"
)

func TestFullBackupRestorePreservesCompleteMoneyFlowAndInvalidatesSessions(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	admin, config := openBackupTestAdmin(t, ctx, databaseURL)
	defer admin.Close(ctx)
	names := []string{
		"mf_backup_source_" + randomHex(t, 4),
		"mf_backup_target_" + randomHex(t, 4),
		"mf_backup_nonempty_" + randomHex(t, 4),
		"mf_backup_failure_" + randomHex(t, 4),
	}
	for _, name := range names {
		createBackupTestDatabase(t, ctx, admin, name)
	}
	defer func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		for index := len(names) - 1; index >= 0; index-- {
			dropBackupTestDatabase(t, dropCtx, admin, names[index])
		}
	}()
	sourceName, targetName, nonemptyName, failureName := names[0], names[1], names[2], names[3]

	sourcePool := openBackupTestPool(t, ctx, config, sourceName)
	defer sourcePool.Close()
	if err := database.Migrate(ctx, sourcePool); err != nil {
		t.Fatalf("migrate source: %v", err)
	}
	fixture := createCompleteBackupFixture(t, ctx, sourcePool)
	var sourceSessions int64
	if err := sourcePool.QueryRow(ctx, `SELECT count(*) FROM local_sessions`).Scan(&sourceSessions); err != nil || sourceSessions < 1 {
		t.Fatalf("source sessions=%d error=%v", sourceSessions, err)
	}
	var failedAttempts int
	if err := sourcePool.QueryRow(ctx, `SELECT failed_login_attempts FROM local_users WHERE username = 'backup-admin'`).Scan(&failedAttempts); err != nil || failedAttempts != 1 {
		t.Fatalf("source failed attempts=%d error=%v", failedAttempts, err)
	}

	dumpBinary, restoreBinary := backupClientBinaries(t)
	sourceClient, err := NewCommandClient(containerDatabaseURL(config, sourceName), dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	wrongCreateClient, err := NewCommandClient(containerDatabaseURL(config, targetName), dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	wrongOutput := filepath.Join(t.TempDir(), "wrong-client-must-not-exist.zip")
	if _, err := Create(ctx, CreateOptions{Pool: sourcePool, Client: wrongCreateClient, OutputPath: wrongOutput}); err == nil || !strings.Contains(err.Error(), "targets database") {
		t.Fatalf("Create(wrong client database) error = %v", err)
	}
	if _, err := os.Stat(wrongOutput); !os.IsNotExist(err) {
		t.Fatalf("wrong-client create left final output: %v", err)
	}
	archivePath := filepath.Join(t.TempDir(), "money-flow-ledger.zip")
	manifest, err := Create(ctx, CreateOptions{
		Pool: sourcePool, Client: sourceClient, OutputPath: archivePath,
		Now: func() time.Time { return time.Date(2026, 9, 20, 3, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := Create(ctx, CreateOptions{Pool: sourcePool, Client: sourceClient, OutputPath: archivePath}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Create(existing output) error = %v", err)
	}
	if manifest.Format != Format || manifest.Database.SchemaVersion != "000016_local_user_iam_subject_links.sql" {
		t.Fatalf("backup manifest identity = %#v", manifest)
	}
	if tableRows(t, manifest.Database.Tables, "local_users") != 1 ||
		tableRows(t, manifest.Database.Tables, "parties") != 2 ||
		tableRows(t, manifest.Database.Tables, "obligation_closures") != 1 ||
		tableRows(t, manifest.Database.Tables, "cash_transaction_reversals") != 1 ||
		tableRows(t, manifest.Database.Tables, "allocation_reversals") != 1 ||
		tableRows(t, manifest.Database.Tables, "internal_transfers") != 1 {
		t.Fatalf("backup manifest representative counts = %#v", manifest.Database.Tables)
	}
	for _, table := range manifest.Database.Tables {
		if table.Name == "local_sessions" {
			t.Fatal("manifest contains local_sessions count")
		}
	}
	archive, entries, decoded, err := openAndValidateArchive(archivePath)
	if err != nil {
		t.Fatalf("validate completed archive: %v", err)
	}
	archive.Close()
	if len(entries) != 2 || !tableCountsEqual(decoded.Database.Tables, manifest.Database.Tables) {
		t.Fatalf("decoded archive manifest = %#v entries=%d", decoded, len(entries))
	}
	beforeSource := querySourceFingerprint(t, ctx, sourcePool)

	targetPool := openBackupTestPool(t, ctx, config, targetName)
	defer targetPool.Close()
	targetClient, err := NewCommandClient(containerDatabaseURL(config, targetName), dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(ctx, RestoreOptions{Pool: targetPool, Client: sourceClient, InputPath: archivePath}); err == nil || !strings.Contains(err.Error(), "targets database") {
		t.Fatalf("Restore(wrong client database) error = %v", err)
	}
	restored, err := Restore(ctx, RestoreOptions{Pool: targetPool, Client: targetClient, InputPath: archivePath})
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if !tableCountsEqual(restored.Database.Tables, manifest.Database.Tables) {
		t.Fatalf("restored manifest table counts changed")
	}
	var targetSessions int64
	if err := targetPool.QueryRow(ctx, `SELECT count(*) FROM local_sessions`).Scan(&targetSessions); err != nil || targetSessions != 0 {
		t.Fatalf("restored sessions=%d error=%v", targetSessions, err)
	}
	if err := targetPool.QueryRow(ctx, `SELECT failed_login_attempts FROM local_users WHERE username = 'backup-admin'`).Scan(&failedAttempts); err != nil || failedAttempts != 1 {
		t.Fatalf("restored failed attempts=%d error=%v", failedAttempts, err)
	}
	if after := querySourceFingerprint(t, ctx, targetPool); after != beforeSource {
		t.Fatalf("restored fingerprint=%v, want %v", after, beforeSource)
	}
	if err := database.Migrate(ctx, targetPool); err != nil {
		t.Fatalf("replay migrations after restore: %v", err)
	}
	if after := querySourceFingerprint(t, ctx, targetPool); after != beforeSource {
		t.Fatalf("migration replay changed restored fingerprint=%v, want %v", after, beforeSource)
	}

	restoredIdentity, err := identity.NewService(identity.NewPostgresRepository(targetPool), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := restoredIdentity.ResolveSession(ctx, fixture.sessionToken); err != nil || found {
		t.Fatalf("restored old session found=%v error=%v", found, err)
	}
	login, err := restoredIdentity.Login(ctx, "backup-admin", "backup-password-123")
	if err != nil {
		t.Fatalf("login with restored password: %v", err)
	}
	if login.Principal.UserID != fixture.actor.ID {
		t.Fatalf("restored principal=%#v actor=%#v", login.Principal, fixture.actor)
	}

	restoredPartyService := party.NewService(party.NewPostgresRepository(targetPool))
	replayed, err := restoredPartyService.Create(ctx, fixture.replayPartyInput)
	if err != nil || !replayed.Replayed || replayed.Party.ID != fixture.ownerID {
		t.Fatalf("restored command replay=%#v error=%v", replayed, err)
	}
	restoredObligationService := obligation.NewService(obligation.NewPostgresRepository(targetPool))
	details, found, err := restoredObligationService.GetDetails(ctx, fixture.obligationID)
	if err != nil || !found {
		t.Fatalf("restored obligation found=%v error=%v", found, err)
	}
	if details.Obligation.Status != obligation.StatusClosed || details.Obligation.OutstandingAmount != "0" ||
		len(details.Revisions) != 2 || len(details.Adjustments) != 1 || len(details.Remissions) != 1 || details.Closure == nil {
		t.Fatalf("restored obligation details=%#v", details)
	}
	restoredCashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(targetPool))
	cash, found, err := restoredCashService.Get(ctx, fixture.reversedCashID)
	if err != nil || !found || cash.Status != cashtransaction.StatusReversed || cash.Reversal == nil {
		t.Fatalf("restored reversed cash=%#v found=%v error=%v", cash, found, err)
	}
	restoredAllocationService := allocation.NewService(allocation.NewPostgresRepository(targetPool))
	allocated, found, err := restoredAllocationService.Get(ctx, fixture.reversedAllocationID)
	if err != nil || !found || allocated.Status != allocation.StatusReversed || allocated.Reversal == nil {
		t.Fatalf("restored reversed allocation=%#v found=%v error=%v", allocated, found, err)
	}
	restoredTransferService := internaltransfer.NewService(internaltransfer.NewPostgresRepository(targetPool))
	transfer, found, err := restoredTransferService.Get(ctx, fixture.transferID)
	if err != nil || !found || transfer.Amount != "88.000001" {
		t.Fatalf("restored transfer=%#v found=%v error=%v", transfer, found, err)
	}
	newParty, err := restoredPartyService.Create(ctx, party.CreateInput{
		CommandID: "80000000-0000-4000-8000-000000000099", Kind: "organization",
		LegalName: "恢复后新增主体", RegistrationIdentifier: "RESTORED-WRITE", Actor: login.Principal.Actor(),
	})
	if err != nil || newParty.Replayed {
		t.Fatalf("write after restore=%#v error=%v", newParty, err)
	}

	if _, err := Restore(ctx, RestoreOptions{Pool: targetPool, Client: targetClient, InputPath: archivePath}); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("Restore(non-empty target) error = %v", err)
	}
	nonemptyPool := openBackupTestPool(t, ctx, config, nonemptyName)
	defer nonemptyPool.Close()
	if err := database.Migrate(ctx, nonemptyPool); err != nil {
		t.Fatalf("migrate non-empty target: %v", err)
	}
	nonemptyClient, err := NewCommandClient(containerDatabaseURL(config, nonemptyName), dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(ctx, RestoreOptions{Pool: nonemptyPool, Client: nonemptyClient, InputPath: archivePath}); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("Restore(migrated target) error = %v", err)
	}

	failurePool := openBackupTestPool(t, ctx, config, failureName)
	defer failurePool.Close()
	invalidDump := []byte("not a PostgreSQL custom archive")
	invalidDumpPath := filepath.Join(t.TempDir(), "invalid.dump")
	writeTestFile(t, invalidDumpPath, invalidDump)
	invalidManifest := manifest
	invalidManifest.Database.DumpBytes = int64(len(invalidDump))
	invalidManifest.Database.DumpSHA256 = digest(invalidDump)
	invalidArchivePath := filepath.Join(t.TempDir(), "invalid-backup.zip")
	if err := writeArchive(invalidArchivePath, invalidDumpPath, invalidManifest); err != nil {
		t.Fatalf("write invalid restore fixture: %v", err)
	}
	failureClient, err := NewCommandClient(containerDatabaseURL(config, failureName), dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(ctx, RestoreOptions{Pool: failurePool, Client: failureClient, InputPath: invalidArchivePath}); err == nil || !strings.Contains(err.Error(), "pg_restore") {
		t.Fatalf("Restore(invalid dump) error = %v", err)
	}
	if relations := userRelationCount(t, ctx, failurePool); relations != 0 {
		t.Fatalf("failed restore relations=%d", relations)
	}
	if afterSource := querySourceFingerprint(t, ctx, sourcePool); afterSource != beforeSource {
		t.Fatalf("backup changed source fingerprint=%v, want %v", afterSource, beforeSource)
	}
}

func TestBackupSnapshotKeepsDumpAndCountsAtOneMVCCPoint(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, config := openBackupTestAdmin(t, ctx, databaseURL)
	defer admin.Close(ctx)
	sourceName := "mf_snapshot_source_" + randomHex(t, 4)
	targetName := "mf_snapshot_target_" + randomHex(t, 4)
	createBackupTestDatabase(t, ctx, admin, sourceName)
	createBackupTestDatabase(t, ctx, admin, targetName)
	defer func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer dropCancel()
		dropBackupTestDatabase(t, dropCtx, admin, targetName)
		dropBackupTestDatabase(t, dropCtx, admin, sourceName)
	}()
	sourcePool := openBackupTestPool(t, ctx, config, sourceName)
	defer sourcePool.Close()
	if err := database.Migrate(ctx, sourcePool); err != nil {
		t.Fatalf("migrate snapshot source: %v", err)
	}
	service := party.NewService(party.NewPostgresRepository(sourcePool))
	actor := audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: "90000000-0000-4000-8000-000000000001", DisplayName: "快照测试管理员"}
	if _, err := service.Create(ctx, party.CreateInput{
		CommandID: "90000000-0000-4000-8000-000000000002", Kind: "organization",
		LegalName: "快照前主体", RegistrationIdentifier: "BEFORE", Actor: actor,
	}); err != nil {
		t.Fatalf("create before-snapshot party: %v", err)
	}
	metadata, snapshot, err := readSnapshotMetadata(ctx, sourcePool)
	if err != nil {
		t.Fatalf("readSnapshotMetadata() error = %v", err)
	}
	defer snapshot.Rollback(context.Background())
	if rows := tableRows(t, metadata.tables, "parties"); rows != 1 {
		t.Fatalf("snapshot parties=%d", rows)
	}
	if _, err := service.Create(ctx, party.CreateInput{
		CommandID: "90000000-0000-4000-8000-000000000003", Kind: "organization",
		LegalName: "快照后主体", RegistrationIdentifier: "AFTER", Actor: actor,
	}); err != nil {
		t.Fatalf("create after-snapshot party: %v", err)
	}
	dumpBinary, restoreBinary := backupClientBinaries(t)
	sourceClient, err := NewCommandClient(containerDatabaseURL(config, sourceName), dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	dumpPath := filepath.Join(t.TempDir(), "snapshot.dump")
	dumpFile, err := os.OpenFile(dumpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceClient.Dump(ctx, metadata.snapshotID, metadata.postgresqlVersion, dumpFile); err != nil {
		dumpFile.Close()
		t.Fatalf("snapshot Dump() error = %v", err)
	}
	if err := dumpFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Commit(ctx); err != nil {
		t.Fatalf("commit snapshot: %v", err)
	}
	targetPool := openBackupTestPool(t, ctx, config, targetName)
	defer targetPool.Close()
	targetClient, err := NewCommandClient(containerDatabaseURL(config, targetName), dumpBinary, restoreBinary)
	if err != nil {
		t.Fatal(err)
	}
	var targetVersion string
	if err := targetPool.QueryRow(ctx, `SELECT current_setting('server_version')`).Scan(&targetVersion); err != nil {
		t.Fatal(err)
	}
	if err := targetClient.Restore(ctx, dumpPath, metadata.postgresqlVersion, targetVersion); err != nil {
		t.Fatalf("snapshot Restore() error = %v", err)
	}
	var restoredParties int64
	var afterParties int64
	if err := targetPool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE registration_identifier = 'AFTER') FROM parties
	`).Scan(&restoredParties, &afterParties); err != nil {
		t.Fatal(err)
	}
	if restoredParties != 1 || afterParties != 0 {
		t.Fatalf("snapshot restored parties=%d after=%d", restoredParties, afterParties)
	}
	var sourceParties int64
	if err := sourcePool.QueryRow(ctx, `SELECT count(*) FROM parties`).Scan(&sourceParties); err != nil || sourceParties != 2 {
		t.Fatalf("source parties=%d error=%v", sourceParties, err)
	}
}

type backupFixture struct {
	actor                audit.ActorRef
	sessionToken         string
	ownerID              string
	obligationID         string
	reversedCashID       string
	reversedAllocationID string
	transferID           string
	replayPartyInput     party.CreateInput
}

func createCompleteBackupFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) backupFixture {
	t.Helper()
	identityService, err := identity.NewService(identity.NewPostgresRepository(pool), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if created, err := identityService.EnsureBootstrap(ctx, identity.BootstrapInput{
		Username: "backup-admin", DisplayName: "备份管理员", Password: "backup-password-123",
	}); err != nil || !created {
		t.Fatalf("bootstrap identity created=%v error=%v", created, err)
	}
	login, err := identityService.Login(ctx, "backup-admin", "backup-password-123")
	if err != nil {
		t.Fatalf("login source identity: %v", err)
	}
	if _, err := identityService.Login(ctx, "backup-admin", "wrong-password"); err == nil {
		t.Fatal("bad source login unexpectedly succeeded")
	}
	actor := login.Principal.Actor()
	commands := &backupCommandSequence{}
	partyService := party.NewService(party.NewPostgresRepository(pool))
	ownerInput := party.CreateInput{
		CommandID: commands.Next(), Kind: "organization", LegalName: "备份本方主体",
		RegistrationIdentifier: "BACKUP-OWNER", Actor: actor,
	}
	ownerResult, err := partyService.Create(ctx, ownerInput)
	if err != nil {
		t.Fatalf("create backup owner: %v", err)
	}
	counterparty := createBackupParty(t, ctx, partyService, commands, actor, "备份往来方", "BACKUP-COUNTERPARTY")

	obligationService := obligation.NewService(obligation.NewPostgresRepository(pool))
	createdObligation, err := obligationService.CreateDraft(ctx, obligation.CreateDraftInput{
		CommandID: commands.Next(), Direction: "receivable", LedgerOwnerPartyID: ownerResult.Party.ID,
		CounterpartyPartyID: counterparty.ID, Title: "备份恢复义务", OriginalAmount: "100.100001",
		Currency: "CNY", DueOn: "2026-12-31", ManualSourceReference: "备份义务依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create backup obligation: %v", err)
	}
	confirmed, err := obligationService.Confirm(ctx, obligation.ConfirmInput{
		CommandID: commands.Next(), ObligationID: createdObligation.Obligation.ID,
		ExpectedDraftVersion: createdObligation.Obligation.DraftVersion, Actor: actor,
	})
	if err != nil {
		t.Fatalf("confirm backup obligation: %v", err)
	}
	adjusted, err := obligationService.AdjustAmount(ctx, obligation.AdjustAmountInput{
		CommandID: commands.Next(), ObligationID: confirmed.Obligation.ID,
		ExpectedCurrentRevision: confirmed.Obligation.CurrentRevision,
		Type:                    "increase", Amount: "19.899999", Reason: "备份调额依据", Actor: actor,
	})
	if err != nil {
		t.Fatalf("adjust backup obligation: %v", err)
	}
	if adjusted.Obligation.EffectiveAmount != "120" {
		t.Fatalf("adjusted effective amount=%s", adjusted.Obligation.EffectiveAmount)
	}
	if _, err := obligationService.RecordRemission(ctx, obligation.RecordRemissionInput{
		CommandID: commands.Next(), ObligationID: confirmed.Obligation.ID,
		ExpectedCurrentRevision: adjusted.Obligation.CurrentRevision,
		Amount:                  "20", Reason: "备份减免依据", ManualSourceReference: "BACKUP-REMISSION", Actor: actor,
	}); err != nil {
		t.Fatalf("record backup remission: %v", err)
	}

	cashService := cashtransaction.NewService(cashtransaction.NewPostgresRepository(pool))
	reversedCash := createBackupCash(t, ctx, cashService, commands, actor, ownerResult.Party.ID, counterparty.ID, "待冲销收款", "50", "BACKUP-CASH-1")
	allocationService := allocation.NewService(allocation.NewPostgresRepository(pool))
	reversedAllocation, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: commands.Next(), CashTransactionID: reversedCash.ID, ObligationID: confirmed.Obligation.ID,
		ExpectedObligationRevision: adjusted.Obligation.CurrentRevision, Amount: "50",
		ManualSourceReference: "备份核销一", Actor: actor,
	})
	if err != nil {
		t.Fatalf("apply backup allocation for reversal: %v", err)
	}
	if _, err := allocationService.Reverse(ctx, allocation.ReverseInput{
		CommandID: commands.Next(), AllocationID: reversedAllocation.Allocation.ID, Reason: "备份反向核销", Actor: actor,
	}); err != nil {
		t.Fatalf("reverse backup allocation: %v", err)
	}
	if _, err := cashService.Reverse(ctx, cashtransaction.ReverseInput{
		CommandID: commands.Next(), CashTransactionID: reversedCash.ID, Reason: "备份冲销资金", Actor: actor,
	}); err != nil {
		t.Fatalf("reverse backup cash: %v", err)
	}
	settlingCash := createBackupCash(t, ctx, cashService, commands, actor, ownerResult.Party.ID, counterparty.ID, "结清收款", "100", "BACKUP-CASH-2")
	if _, err := allocationService.Apply(ctx, allocation.ApplyInput{
		CommandID: commands.Next(), CashTransactionID: settlingCash.ID, ObligationID: confirmed.Obligation.ID,
		ExpectedObligationRevision: adjusted.Obligation.CurrentRevision, Amount: "100",
		ManualSourceReference: "备份结清核销", Actor: actor,
	}); err != nil {
		t.Fatalf("apply backup settling allocation: %v", err)
	}
	current, found, err := obligationService.Get(ctx, confirmed.Obligation.ID)
	if err != nil || !found || current.OutstandingAmount != "0" {
		t.Fatalf("get zero backup obligation=%#v found=%v error=%v", current, found, err)
	}
	if _, err := obligationService.Close(ctx, obligation.CloseInput{
		CommandID: commands.Next(), ObligationID: current.ID,
		ExpectedCurrentRevision: current.CurrentRevision, ExpectedBalanceSequence: current.BalanceSequence,
		Reason: "备份恢复关闭", Actor: actor,
	}); err != nil {
		t.Fatalf("close backup obligation: %v", err)
	}

	transferService := internaltransfer.NewService(internaltransfer.NewPostgresRepository(pool))
	transfer, err := transferService.Create(ctx, internaltransfer.CreateInput{
		CommandID: commands.Next(), LedgerOwnerPartyID: ownerResult.Party.ID, Description: "备份内部调拨",
		Amount: "88.000001", Currency: "CNY", OccurredOn: "2026-09-20",
		SourceAccountReference: "基本户", DestinationAccountReference: "备用金",
		ManualSourceReference: "备份转账依据", ExternalTransferReference: "BACKUP-MOVE", Actor: actor,
	})
	if err != nil {
		t.Fatalf("create backup internal transfer: %v", err)
	}
	return backupFixture{
		actor: actor, sessionToken: login.Token, ownerID: ownerResult.Party.ID,
		obligationID: confirmed.Obligation.ID, reversedCashID: reversedCash.ID,
		reversedAllocationID: reversedAllocation.Allocation.ID, transferID: transfer.Transfer.ID,
		replayPartyInput: ownerInput,
	}
}

func createBackupParty(t *testing.T, ctx context.Context, service *party.Service, commands *backupCommandSequence, actor audit.ActorRef, name, registration string) party.Party {
	t.Helper()
	result, err := service.Create(ctx, party.CreateInput{
		CommandID: commands.Next(), Kind: "organization", LegalName: name,
		RegistrationIdentifier: registration, Actor: actor,
	})
	if err != nil {
		t.Fatalf("create backup party %q: %v", name, err)
	}
	return result.Party
}

func createBackupCash(t *testing.T, ctx context.Context, service *cashtransaction.Service, commands *backupCommandSequence, actor audit.ActorRef, ownerID, counterpartyID, description, amount, externalReference string) cashtransaction.Transaction {
	t.Helper()
	result, err := service.Create(ctx, cashtransaction.CreateInput{
		CommandID: commands.Next(), Type: "receipt", LedgerOwnerPartyID: ownerID,
		CounterpartyPartyID: counterpartyID, Description: description, Amount: amount,
		Currency: "CNY", OccurredOn: "2026-09-20", AccountReference: "备份测试账户",
		ManualSourceReference: "备份资金依据", ExternalTransactionReference: externalReference, Actor: actor,
	})
	if err != nil {
		t.Fatalf("create backup cash %q: %v", description, err)
	}
	return result.Transaction
}

type backupCommandSequence struct{ value int }

func (sequence *backupCommandSequence) Next() string {
	sequence.value++
	return fmt.Sprintf("80000000-0000-4000-8000-%012x", sequence.value)
}

type sourceFingerprint struct {
	Users               int64
	Parties             int64
	Obligations         int64
	Revisions           int64
	Adjustments         int64
	Remissions          int64
	Closures            int64
	Cash                int64
	CashReversals       int64
	Allocations         int64
	AllocationReversals int64
	Transfers           int64
	Commands            int64
	Audits              int64
}

func querySourceFingerprint(t *testing.T, ctx context.Context, pool *pgxpool.Pool) sourceFingerprint {
	t.Helper()
	var value sourceFingerprint
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM local_users),
			(SELECT count(*) FROM parties),
			(SELECT count(*) FROM obligations),
			(SELECT count(*) FROM obligation_revisions),
			(SELECT count(*) FROM obligation_amount_adjustments),
			(SELECT count(*) FROM obligation_remissions),
			(SELECT count(*) FROM obligation_closures),
			(SELECT count(*) FROM cash_transactions),
			(SELECT count(*) FROM cash_transaction_reversals),
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_reversals),
			(SELECT count(*) FROM internal_transfers),
			(SELECT count(*) FROM party_create_commands) +
			(SELECT count(*) FROM obligation_create_commands) +
			(SELECT count(*) FROM obligation_confirmation_commands) +
			(SELECT count(*) FROM obligation_amount_adjustment_commands) +
			(SELECT count(*) FROM obligation_remission_commands) +
			(SELECT count(*) FROM obligation_closure_commands) +
			(SELECT count(*) FROM cash_transaction_create_commands) +
			(SELECT count(*) FROM cash_transaction_reversal_create_commands) +
			(SELECT count(*) FROM allocation_apply_commands) +
			(SELECT count(*) FROM allocation_reversal_create_commands) +
			(SELECT count(*) FROM internal_transfer_create_commands),
			(SELECT count(*) FROM audit_entries)
	`).Scan(
		&value.Users, &value.Parties, &value.Obligations, &value.Revisions,
		&value.Adjustments, &value.Remissions, &value.Closures, &value.Cash,
		&value.CashReversals, &value.Allocations, &value.AllocationReversals,
		&value.Transfers, &value.Commands, &value.Audits,
	); err != nil {
		t.Fatalf("query source fingerprint: %v", err)
	}
	return value
}

func tableRows(t *testing.T, tables []TableCount, name string) int64 {
	t.Helper()
	for _, table := range tables {
		if table.Name == name {
			return table.Rows
		}
	}
	t.Fatalf("manifest table %q not found", name)
	return 0
}

func userRelationCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int64 {
	t.Helper()
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname !~ '^pg_toast'
		  AND c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')
	`).Scan(&count); err != nil {
		t.Fatalf("count user relations: %v", err)
	}
	return count
}

func backupClientBinaries(t *testing.T) (string, string) {
	t.Helper()
	dumpBinary, err := filepath.Abs("../../ops/local-pg-dump")
	if err != nil {
		t.Fatal(err)
	}
	restoreBinary, err := filepath.Abs("../../ops/local-pg-restore")
	if err != nil {
		t.Fatal(err)
	}
	return dumpBinary, restoreBinary
}

func openBackupTestAdmin(t *testing.T, ctx context.Context, databaseURL string) (*pgx.Conn, *pgx.ConnConfig) {
	t.Helper()
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	adminConfig := config.Copy()
	adminConfig.Database = "postgres"
	admin, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	return admin, config
}

func createBackupTestDatabase(t *testing.T, ctx context.Context, admin *pgx.Conn, name string) {
	t.Helper()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE template0"); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}
}

func dropBackupTestDatabase(t *testing.T, ctx context.Context, admin *pgx.Conn, name string) {
	t.Helper()
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
		t.Errorf("drop database %s: %v", name, err)
	}
}

func openBackupTestPool(t *testing.T, ctx context.Context, config *pgx.ConnConfig, name string) *pgxpool.Pool {
	t.Helper()
	poolConfig, err := pgxpool.ParseConfig(config.ConnString())
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	return pool
}

func containerDatabaseURL(config *pgx.ConnConfig, databaseName string) string {
	value := &url.URL{
		Scheme: "postgres", Host: "127.0.0.1:5432", Path: "/" + databaseName,
		User: url.UserPassword(config.User, config.Password),
	}
	query := value.Query()
	query.Set("sslmode", "disable")
	value.RawQuery = query.Encode()
	return value.String()
}

func randomHex(t *testing.T, bytes int) string {
	t.Helper()
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}
