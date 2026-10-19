package ledgerexport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const (
	testContractID = "11111111-1111-4111-8111-111111111111"
	testActorID    = "22222222-2222-4222-8222-222222222222"
)

type repositoryStub struct {
	ledger      Ledger
	found       bool
	loadErr     error
	recordErr   error
	recorded    audit.Entry
	loadCount   int
	recordCount int
}

func (r *repositoryStub) Load(context.Context, string) (Ledger, bool, error) {
	r.loadCount++
	return r.ledger, r.found, r.loadErr
}

func (r *repositoryStub) RecordExport(_ context.Context, entry audit.Entry) error {
	r.recordCount++
	r.recorded = entry
	return r.recordErr
}

func TestExportBuildsVerifiableTwoFileArchiveAndRecordsAudit(t *testing.T) {
	repository := &repositoryStub{found: true, ledger: completeTestLedger()}
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	fixedNow := time.Date(2026, 9, 19, 13, 14, 15, 123456000, time.UTC)
	service.now = func() time.Time { return fixedNow }
	ids := []string{
		"33333333-3333-4333-8333-333333333333",
		"44444444-4444-4444-8444-444444444444",
	}
	service.newID = func() (string, error) {
		value := ids[0]
		ids = ids[1:]
		return value, nil
	}

	artifact, err := service.Export(context.Background(), Input{
		ContractID: testContractID,
		Actor: audit.ActorRef{
			Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "导出管理员",
		},
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if artifact.Filename != "contract-"+testContractID+"-ledger.zip" {
		t.Fatalf("artifact filename = %q", artifact.Filename)
	}
	if !artifact.GeneratedAt.Equal(fixedNow) || repository.loadCount != 1 || repository.recordCount != 1 {
		t.Fatalf("artifact time=%v load=%d record=%d", artifact.GeneratedAt, repository.loadCount, repository.recordCount)
	}

	files := readArchive(t, artifact.Content)
	if len(files) != 2 {
		t.Fatalf("archive file count = %d, files=%v", len(files), mapKeys(files))
	}
	manifestBytes, manifestFound := files[manifestFilename]
	ledgerBytes, ledgerFound := files[ledgerFilename]
	if !manifestFound || !ledgerFound {
		t.Fatalf("archive files = %v", mapKeys(files))
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	digestBytes := sha256.Sum256(ledgerBytes)
	digest := hex.EncodeToString(digestBytes[:])
	if manifest.FormatID != FormatID || manifest.FormatVersion != FormatVersion ||
		manifest.ContractID != testContractID || manifest.LedgerFilename != ledgerFilename ||
		manifest.LedgerBytes != len(ledgerBytes) || manifest.LedgerSHA256 != digest ||
		artifact.LedgerSHA256 != digest || manifest.EvidenceContentIncluded {
		t.Fatalf("manifest = %#v, digest=%s", manifest, digest)
	}
	if bytes.Contains(ledgerBytes, []byte("storage_key")) || bytes.Contains(ledgerBytes, []byte("secret-storage-id")) {
		t.Fatalf("ledger leaked storage data: %s", ledgerBytes)
	}
	var ledger Ledger
	if err := json.Unmarshal(ledgerBytes, &ledger); err != nil {
		t.Fatalf("decode ledger: %v", err)
	}
	if ledger.Format != FormatID || ledger.Contract.ID != testContractID || len(ledger.Revisions) != 2 ||
		len(ledger.Revisions[0].Parties) != 1 || len(ledger.Relations) != 1 ||
		len(ledger.Settlements) != 1 || ledger.Closure == nil || len(ledger.Evidence) != 1 ||
		len(ledger.Evidence[0].Versions) != 1 || len(ledger.Audit) != 1 {
		t.Fatalf("ledger shape = %#v", ledger)
	}
	for _, entry := range ledger.Audit {
		if entry.Action == ActionContractLedgerExported {
			t.Fatal("current export audit appeared in its own snapshot")
		}
	}

	entry := repository.recorded
	if entry.ID != "44444444-4444-4444-8444-444444444444" ||
		entry.CommandID != "33333333-3333-4333-8333-333333333333" ||
		entry.Action != ActionContractLedgerExported || entry.ObjectID != testContractID ||
		entry.Actor.ID != testActorID || !entry.OccurredAt.Equal(fixedNow) {
		t.Fatalf("recorded audit = %#v", entry)
	}
	if entry.Details["ledger_sha256"] != digest || entry.Details["archive_bytes"] != len(artifact.Content) ||
		entry.Details["evidence_documents"] != 1 || entry.Details["evidence_versions"] != 1 {
		t.Fatalf("recorded audit details = %#v", entry.Details)
	}
}

func TestExportReturnsNoArtifactWhenAuditFails(t *testing.T) {
	repository := &repositoryStub{
		found: true, ledger: completeTestLedger(), recordErr: errors.New("audit unavailable"),
	}
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.newID = func() (string, error) { return "33333333-3333-4333-8333-333333333333", nil }

	artifact, err := service.Export(context.Background(), Input{
		ContractID: testContractID,
		Actor:      audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "管理员"},
	})
	if err == nil || len(artifact.Content) != 0 || artifact.Filename != "" {
		t.Fatalf("Export() artifact=%#v error=%v", artifact, err)
	}
}

func TestExportValidatesIdentityAndNotFound(t *testing.T) {
	repository := &repositoryStub{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = service.Export(context.Background(), Input{ContractID: "not-a-uuid"})
	var validation *ValidationError
	if !errors.As(err, &validation) || repository.loadCount != 0 {
		t.Fatalf("invalid contract error=%v load=%d", err, repository.loadCount)
	}

	_, err = service.Export(context.Background(), Input{
		ContractID: testContractID,
		Actor:      audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "管理员"},
	})
	var notFound *NotFoundError
	if !errors.As(err, &notFound) || repository.recordCount != 0 {
		t.Fatalf("missing contract error=%v record=%d", err, repository.recordCount)
	}
}

func TestExportAllowsEveryLifecycleStatusAndEncodesEmptyCollectionsAsArrays(t *testing.T) {
	for _, status := range []string{"draft", "cancelled", "active", "terminated", "closed"} {
		t.Run(status, func(t *testing.T) {
			repository := &repositoryStub{found: true, ledger: Ledger{Contract: ContractSnapshot{
				ID: testContractID, NumberNamespace: "TEST", ContractNumber: "C-EMPTY",
				Title: "空集合合同", Currency: "CNY", Status: status, DraftVersion: 1,
			}}}
			service, err := NewService(repository)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			service.newID = func() (string, error) { return "33333333-3333-4333-8333-333333333333", nil }
			artifact, err := service.Export(context.Background(), Input{
				ContractID: testContractID,
				Actor:      audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "管理员"},
			})
			if err != nil {
				t.Fatalf("Export() error = %v", err)
			}
			ledgerBytes := readArchive(t, artifact.Content)[ledgerFilename]
			for _, field := range []string{`"draft_parties": []`, `"revisions": []`, `"instruments": []`, `"relations": []`, `"settlements": []`, `"evidence": []`, `"audit": []`} {
				if !bytes.Contains(ledgerBytes, []byte(field)) {
					t.Fatalf("ledger for %s missing %s: %s", status, field, ledgerBytes)
				}
			}
		})
	}
}

func completeTestLedger() Ledger {
	category := "采购"
	amount := "1000.25"
	revision := int64(2)
	commandID := "55555555-5555-4555-8555-555555555555"
	actor := Actor{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "管理员"}
	party := PartySnapshot{
		Position: 1, SourcePartyLinkID: "66666666-6666-4666-8666-666666666666",
		SourcePartyID: "77777777-7777-4777-8777-777777777777", SourcePartyRevision: 1,
		PartyKind: "organization", LegalName: "甲方", Role: "采购方",
	}
	return Ledger{
		Contract: ContractSnapshot{
			ID: testContractID, NumberNamespace: "TEST", ContractNumber: "C-001", Title: "测试合同",
			Category: &category, Currency: "CNY", TotalAmount: &amount,
			Status: "closed", DraftVersion: 3, CurrentRevision: &revision,
			CreatedAt: "2026-09-19T08:00:00Z", UpdatedAt: "2026-09-19T09:00:00Z",
		},
		DraftParties: []DraftParty{{
			ID: party.SourcePartyLinkID, PartyID: party.SourcePartyID, PartyKind: party.PartyKind,
			PartyLegalName: party.LegalName, PartyRevision: 1, Role: party.Role,
			AddedAt: "2026-09-19T08:10:00Z",
		}},
		Revisions: []Revision{
			{Revision: 1, SourceInstrumentID: "88888888-8888-4888-8888-888888888888", NumberNamespace: "TEST", ContractNumber: "C-001", Title: "原合同", Currency: "CNY", TotalAmount: &amount, SignedOn: "2026-09-01", EffectiveOn: "2026-09-02", Actor: actor, RecordedAt: "2026-09-19T08:20:00Z", Parties: []PartySnapshot{party}},
			{Revision: 2, SourceInstrumentID: "99999999-9999-4999-8999-999999999999", NumberNamespace: "TEST", ContractNumber: "C-001", Title: "测试合同", Currency: "CNY", TotalAmount: &amount, SignedOn: "2026-09-01", EffectiveOn: "2026-09-02", Actor: actor, RecordedAt: "2026-09-19T08:30:00Z", Parties: []PartySnapshot{party}},
		},
		Instruments: []Instrument{{
			ID: "88888888-8888-4888-8888-888888888888", InstrumentType: "original_contract",
			InstrumentNumber: "C-001", Title: "原合同", ChangeSummary: "原合同激活",
			SignedOn: "2026-09-01", EffectiveOn: "2026-09-02", EffectiveOrder: 1,
			AmountEffect: "original", AmountValue: &amount, SourceCommandID: commandID,
			ResultingRevision: 1, Actor: actor, RecordedAt: "2026-09-19T08:20:00Z",
		}},
		Relations: []Relation{{
			ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Type: "references",
			Source: RelationContractSnapshot{ContractID: testContractID, Revision: 1, NumberNamespace: "TEST", ContractNumber: "C-001", Title: "原合同"},
			Target: RelationContractSnapshot{ContractID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Revision: 1, NumberNamespace: "TEST", ContractNumber: "C-002", Title: "关联合同"},
			Actor:  actor, CreatedAt: "2026-09-19T08:40:00Z",
		}},
		Settlements: []Settlement{{
			ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", SourceRevision: 1, Sequence: 1,
			Kind: "final", StatementNumber: "S-001", Title: "最终结算", Amount: "900", Currency: "CNY",
			ConfirmedOn: "2026-09-10", Basis: "双方确认", ConfirmingParties: []PartySnapshot{party},
			Actor: actor, RecordedAt: "2026-09-19T08:50:00Z",
		}},
		Closure: &Closure{
			ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", SourceRevision: 1,
			SourceSettlementSequence: 1, SettlementDisposition: "final_recorded",
			ClosedOn: "2026-09-11", Reason: "业务完成", Actor: actor, RecordedAt: "2026-09-19T09:00:00Z",
		},
		Evidence: []EvidenceDocument{{
			ID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", TargetType: "contract", TargetID: testContractID,
			Title: "合同扫描件", Purpose: "留档", Actor: actor, CreatedAt: "2026-09-19T09:10:00Z",
			Versions: []EvidenceVersion{{
				ID: "ffffffff-ffff-4fff-8fff-ffffffffffff", Version: 1, OriginalFilename: "contract.pdf",
				DetectedMediaType: "application/pdf", SizeBytes: 12, SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				Actor: actor, RecordedAt: "2026-09-19T09:10:00Z",
			}},
		}},
		Audit: []AuditEntry{{
			ID: "12121212-1212-4212-8212-121212121212", Action: "contract_activated", CommandID: &commandID,
			Actor: actor, OccurredAt: "2026-09-19T08:20:00Z", Summary: "激活合同", Details: json.RawMessage(`{"resulting_revision":1}`),
		}},
	}
}

func readArchive(t *testing.T, content []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		if _, duplicate := files[file.Name]; duplicate || file.FileInfo().IsDir() {
			t.Fatalf("unsafe archive entry %q", file.Name)
		}
		stream, err := file.Open()
		if err != nil {
			t.Fatalf("open archive entry %q: %v", file.Name, err)
		}
		data, err := io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			t.Fatalf("read archive entry %q: %v", file.Name, err)
		}
		files[file.Name] = data
	}
	return files
}

func mapKeys(values map[string][]byte) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
