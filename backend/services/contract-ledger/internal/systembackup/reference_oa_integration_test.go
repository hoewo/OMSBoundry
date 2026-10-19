//go:build integration

package systembackup

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/party"
	"omsbizfoundry/contract-ledger/internal/referenceoa"
)

type backupMaterialValidator struct{}

func (backupMaterialValidator) ValidateFile(_ context.Context, tenant, id string, version int) (referenceoa.FileReference, error) {
	return referenceoa.FileReference{Schema: "1", Source: "evidence-service", ResourceType: "evidence-file", ID: id, FileID: id, TenantID: tenant, Version: version, Revision: version, Status: "available", Name: "恢复引用.pdf", Size: 5, MediaType: "application/pdf", SHA256: strings.Repeat("a", 64)}, nil
}

// Actual application backup/restore, not a template DB clone or manual table copy.
// Requires PG18 client binaries on PATH in the explicitly isolated runner.
func TestReferenceOAFullBackupRestoreAndOriginalCommandContinuation(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" || os.Getenv("OA_REFERENCE_BACKUP_PG_TOOLS") != "system" {
		t.Skip("isolated PG URL and explicit system PG clients are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 260*time.Second)
	defer cancel()
	admin, config := openBackupTestAdmin(t, ctx, os.Getenv("TEST_DATABASE_URL"))
	defer admin.Close(ctx)
	sourceName, targetName := "cl_backup_oa_source_"+randomHex(t, 4), "cl_backup_oa_target_"+randomHex(t, 4)
	createBackupTestDatabase(t, ctx, admin, sourceName)
	createBackupTestDatabase(t, ctx, admin, targetName)
	defer func() {
		clean, done := context.WithTimeout(context.Background(), 20*time.Second)
		defer done()
		dropBackupTestDatabase(t, clean, admin, targetName)
		dropBackupTestDatabase(t, clean, admin, sourceName)
	}()
	source := openBackupTestPool(t, ctx, config, sourceName)
	defer source.Close()
	if err := database.Migrate(ctx, source); err != nil {
		t.Fatal(err)
	}
	actor := audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "33333333-3333-4333-8333-333333333333", DisplayName: "备份测试操作者"}
	command := func() string {
		id, err := contract.NewCommandID()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	core := contract.NewService(contract.NewPostgresRepository(source))
	draft, err := core.CreateDraft(ctx, contract.CreateDraftInput{CommandID: command(), NumberNamespace: "oa-backup", ContractNumber: "OA-BK-001", Title: "客户草稿恢复", Currency: "CNY", TotalAmount: "9007199254740994.00", SignedOn: "2026-10-04", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	zero := int64(0)
	in := referenceoa.SaveInput{CommandID: command(), ContractID: draft.ID, TenantID: "11111111-1111-4111-8111-111111111111", ProjectID: "22222222-2222-4222-8222-222222222222", ChainType: "client_to_qualification", ExpectedDraftVersion: draft.DraftVersion, Fees: referenceoa.Fees{Design: "9007199254740993.01", Survey: "0.09", Consultation: "0.90"}, RateBPS: &zero, SettlementFollows: true, Actor: actor}
	store := referenceoa.NewStore(source)
	if _, err = store.Save(ctx, in); err != nil {
		t.Fatal(err)
	}
	in.CommandID = command()
	in.ExpectedRevision = 1
	in.Amendments = []referenceoa.Amendment{{ID: command(), Number: "BK-SUP-001", SignedOn: "2026-10-05", ChangeType: "adjustment", Fees: referenceoa.Fees{Design: "-0.01", Survey: "0.00", Consultation: "0.00"}, RateBPS: &zero}}
	saved, err := store.Save(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	formal, err := core.CreateDraft(ctx, contract.CreateDraftInput{CommandID: command(), NumberNamespace: "oa-backup", ContractNumber: "OA-BK-002", Title: "正式协议恢复", Currency: "CNY", TotalAmount: "100.00", SignedOn: "2026-10-04", EffectiveOn: "2026-10-04", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	formalIn := referenceoa.SaveInput{CommandID: command(), ContractID: formal.ID, TenantID: in.TenantID, ProjectID: in.ProjectID, ChainType: "qualification_to_our_company", ExpectedDraftVersion: 1, Fees: referenceoa.Fees{"100.00", "0.00", "0.00"}, RateBPS: &zero, SettlementFollows: true, Actor: actor, Amendments: []referenceoa.Amendment{{ID: command(), Number: "FORMAL-001", SignedOn: "2026-10-05", ChangeType: "adjustment", Fees: referenceoa.Fees{"0.01", "0.00", "0.00"}}}}
	if _, err = store.Save(ctx, formalIn); err != nil {
		t.Fatal(err)
	}
	parties := party.NewService(party.NewPostgresRepository(source))
	for i, name := range []string{"恢复甲方", "恢复乙方"} {
		p, e := parties.Create(ctx, party.CreateInput{Kind: "organization", LegalName: name, Actor: actor})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = core.AddDraftParty(ctx, contract.AddDraftPartyInput{CommandID: command(), ContractID: formal.ID, ExpectedDraftVersion: int64(i + 1), PartyID: p.ID, Role: []string{"client", "provider"}[i], Actor: actor}); e != nil {
			t.Fatal(e)
		}
	}
	if _, err = core.Activate(ctx, contract.ActivateContractInput{CommandID: command(), ContractID: formal.ID, ExpectedDraftVersion: 3, Actor: actor}); err != nil {
		t.Fatal(err)
	}
	confirmationInput := referenceoa.ConfirmInput{CommandID: command(), ContractID: formal.ID, TenantID: in.TenantID, AmendmentID: formalIn.Amendments[0].ID, ExpectedOverlayRevision: 1, ExpectedContractRevision: 1, EffectiveOn: "2026-10-05", Amendment: formalIn.Amendments[0], Actor: actor}
	confirmation, err := store.Confirm(ctx, confirmationInput)
	if err != nil || confirmation.ContractRevision != 2 {
		t.Fatal("source formal confirmation", err)
	}
	// This is the Contract reference backup layer, not Evidence byte backup.
	store.SetEvidenceValidator(backupMaterialValidator{})
	materialInput := referenceoa.MaterialInput{CommandID: command(), ContractID: formal.ID, TenantID: in.TenantID, Operation: "append", Kind: "amendment", AmendmentID: formalIn.Amendments[0].ID, FileID: command(), Version: 2, Actor: actor}
	materialSaved, err := store.MutateMaterial(ctx, materialInput)
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewCommandClient(referenceOABackupDatabaseURL(t, sourceName), "pg_dump", "pg_restore")
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "reference-oa-full.zip")
	legacyEvidenceRoot := t.TempDir()
	// The existing full-backup format requires an objects directory even when
	// there are no legacy embedded files. This test adds no file ownership.
	if err = os.Mkdir(filepath.Join(legacyEvidenceRoot, "objects"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = Create(ctx, CreateOptions{Pool: source, Client: client, EvidenceRoot: legacyEvidenceRoot, OutputPath: archive})
	if err != nil {
		t.Fatal(err)
	}
	target := openBackupTestPool(t, ctx, config, targetName)
	defer target.Close()
	targetClient, err := NewCommandClient(referenceOABackupDatabaseURL(t, targetName), "pg_dump", "pg_restore")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Restore(ctx, RestoreOptions{Pool: target, Client: targetClient, EvidenceRoot: filepath.Join(t.TempDir(), "empty-evidence"), InputPath: archive}); err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, target); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"contracts", "contract_draft_change_commands", "reference_oa_contract_drafts", "reference_oa_contract_draft_commands", "reference_oa_amendment_confirmations", "reference_oa_materials", "reference_oa_material_commands", "contract_instruments", "contract_change_commands", "contract_revisions", "contract_revision_parties", "audit_entries"} {
		query := `SELECT coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'') FROM public."` + table + `" t`
		var before, after string
		if err = source.QueryRow(ctx, query).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if err = target.QueryRow(ctx, query).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Fatalf("restored %s differs", table)
		}
	}
	restoredStore := referenceoa.NewStore(target)
	materialReplay, err := restoredStore.MutateMaterial(ctx, materialInput)
	if err != nil || !materialReplay.Replayed || !reflect.DeepEqual(materialReplay.Materials, materialSaved.Materials) {
		t.Fatal("restored material replay lost or consulted offline Evidence", err)
	}
	materialInput.CommandID = command()
	materialInput.ExpectedRevision = 1
	materialInput.Operation = "remove"
	materialInput.MaterialID = materialSaved.Materials.Items[0].ID
	materialInput.FileID = ""
	materialInput.Version = 0
	removed, err := restoredStore.MutateMaterial(ctx, materialInput)
	if err != nil || removed.Materials.Revision != 2 || removed.Materials.Items[0].RemovedAt == nil {
		t.Fatal("restored material removal lost history", err)
	}
	if _, err = target.Exec(ctx, `DELETE FROM reference_oa_material_commands`); err == nil {
		t.Fatal("material history deletion accepted after restore")
	}
	value, err := restoredStore.Read(ctx, in.TenantID, draft.ID)
	if err != nil || !reflect.DeepEqual(value, &saved.Draft) {
		t.Fatalf("restored typed draft differs: %v", err)
	}
	replay, err := restoredStore.Save(ctx, in)
	if err != nil || !replay.Replayed || !reflect.DeepEqual(replay.Draft, saved.Draft) {
		t.Fatalf("original command continuation: %v", err)
	}
	var count int
	if err = target.QueryRow(ctx, `SELECT count(*) FROM reference_oa_contract_draft_commands WHERE contract_id=$1`, draft.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("replay appended history: count=%d err=%v", count, err)
	}
	in.CommandID = command()
	in.ExpectedRevision = 2
	in.Amendments[0].Cancelled = true
	next, err := restoredStore.Save(ctx, in)
	if err != nil || next.Draft.Revision != 3 || !next.Draft.Amendments[0].Cancelled {
		t.Fatalf("restored new revision: %v", err)
	}
	if _, err = target.Exec(ctx, `DELETE FROM reference_oa_contract_draft_commands`); err == nil {
		t.Fatal("history deletion accepted after restore")
	}
	confirmedReplay, err := restoredStore.Confirm(ctx, confirmationInput)
	if err != nil || !confirmedReplay.Replayed || confirmedReplay.InstrumentID != confirmation.InstrumentID {
		t.Fatal("restored formal command continuation", err)
	}
	formalIn.CommandID = command()
	formalIn.ExpectedRevision = 1
	formalIn.ExpectedDraftVersion = 3
	formalIn.ExpectedContractRevision = 2
	formalIn.Amendments = append(formalIn.Amendments, referenceoa.Amendment{ID: command(), Number: "FORMAL-002", SignedOn: "2026-10-06", ChangeType: "no_amount_change", Fees: referenceoa.Fees{"0.00", "0.00", "0.00"}})
	if _, err = restoredStore.Save(ctx, formalIn); err != nil {
		t.Fatal("restored active intention continuation", err)
	}
	confirmationInput.CommandID = command()
	confirmationInput.AmendmentID = formalIn.Amendments[1].ID
	confirmationInput.Amendment = formalIn.Amendments[1]
	confirmationInput.ExpectedOverlayRevision = 2
	confirmationInput.ExpectedContractRevision = 2
	confirmationInput.EffectiveOn = "2026-10-06"
	if result, e := restoredStore.Confirm(ctx, confirmationInput); e != nil || result.ContractRevision != 3 || result.EffectiveAmount != "100.01" {
		t.Fatal("restored new formal revision", e)
	}
	if _, err = target.Exec(ctx, `DELETE FROM reference_oa_amendment_confirmations`); err == nil {
		t.Fatal("formal mapping history deletion accepted after restore")
	}
	if err = source.QueryRow(ctx, `SELECT count(*) FROM reference_oa_amendment_confirmations`).Scan(&count); err != nil || count != 1 {
		t.Fatal("formal continuation changed source", err)
	}
	assertReferenceOASourceUnchanged(t, ctx, source, store, in.TenantID, draft.ID, saved.Draft)
}

func referenceOABackupDatabaseURL(t *testing.T, database string) string {
	t.Helper()
	// This runner has separate app/client and PG containers. Preserve the
	// explicitly injected network endpoint, credentials and TLS settings.
	value, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if err != nil || (value.Scheme != "postgres" && value.Scheme != "postgresql") || value.Host == "" {
		t.Fatal("isolated backup test requires a PostgreSQL URL")
	}
	value.Path = "/" + database
	value.RawPath = ""
	return value.String()
}

func assertReferenceOASourceUnchanged(t *testing.T, ctx context.Context, pool *pgxpool.Pool, store *referenceoa.Store, tenant, id string, want referenceoa.Draft) {
	t.Helper()
	value, err := store.Read(ctx, tenant, id)
	if err != nil || !reflect.DeepEqual(value, &want) {
		t.Fatalf("restore wrote source: %v", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM reference_oa_contract_draft_commands WHERE contract_id=$1`, id).Scan(&count); err != nil || count != 2 {
		t.Fatal("source history changed")
	}
}
