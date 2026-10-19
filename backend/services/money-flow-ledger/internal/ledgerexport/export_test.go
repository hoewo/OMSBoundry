package ledgerexport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const testOwnerID = "11111111-1111-4111-8111-111111111111"

type repositoryStub struct {
	snapshot    Snapshot
	found       bool
	loadErr     error
	auditErr    error
	loadedOwner string
	audits      []audit.Entry
}

func (r *repositoryStub) Load(_ context.Context, ownerID string) (Snapshot, bool, error) {
	r.loadedOwner = ownerID
	return r.snapshot, r.found, r.loadErr
}

func (r *repositoryStub) RecordExport(_ context.Context, entry audit.Entry) error {
	if r.auditErr != nil {
		return r.auditErr
	}
	r.audits = append(r.audits, entry)
	return nil
}

func TestExportBuildsVerifiableFixedCSVArchiveAndRecordsAudit(t *testing.T) {
	snapshot := emptyTestSnapshot()
	formulaText := "=HYPERLINK(\"https://invalid.example\"), quoted\nline"
	snapshot.Owner.LegalName = formulaText
	snapshot.Tables[0].Rows = [][]string{{
		testOwnerID, "organization", formulaText, "REG-1", "1",
		"2026-09-20T00:00:00.000000Z", "2026-09-20T00:00:00.000000Z",
	}}
	snapshot.Tables[1].Rows = [][]string{rowWithValues(len(tableSpecs[1].Header), map[int]string{
		0: "22222222-2222-4222-8222-222222222222", 1: "receivable", 2: "0.1, \"quoted\"\nline", 3: testOwnerID,
		14: "0.100000", 15: "CNY", 19: "open", 20: "1", 21: "1",
	})}

	repository := &repositoryStub{snapshot: snapshot, found: true}
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	generatedAt := time.Date(2026, 9, 20, 1, 2, 3, 456000000, time.UTC)
	service.now = func() time.Time { return generatedAt }
	ids := []string{
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
	}
	service.newID = func() (string, error) {
		value := ids[0]
		ids = ids[1:]
		return value, nil
	}

	artifact, err := service.Export(context.Background(), Input{
		LedgerOwnerPartyID: "  " + strings.ToUpper(testOwnerID) + " ",
		Actor: audit.ActorRef{
			Kind: audit.ActorKindLocalUser, ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", DisplayName: "导出管理员",
		},
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if artifact.Filename != "money-flow-ledger-"+testOwnerID+".zip" || artifact.GeneratedAt != generatedAt {
		t.Fatalf("artifact metadata = %#v", artifact)
	}
	if repository.loadedOwner != testOwnerID {
		t.Fatalf("loaded owner = %q", repository.loadedOwner)
	}

	archiveFiles := readArchive(t, artifact.Content)
	if len(archiveFiles) != len(tableSpecs)+1 {
		t.Fatalf("archive file count = %d", len(archiveFiles))
	}
	manifestBytes, ok := archiveFiles[manifestFilename]
	if !ok {
		t.Fatal("manifest.json missing")
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.FormatID != FormatID || manifest.FormatVersion != FormatVersion ||
		manifest.GeneratedAt != generatedAt.Format(time.RFC3339Nano) ||
		manifest.LedgerOwner.ID != testOwnerID || manifest.SourceSchemaVersion != "000016_local_user_iam_subject_links.sql" {
		t.Fatalf("manifest = %#v", manifest)
	}
	if len(manifest.Files) != len(tableSpecs) || !strings.Contains(manifest.SpreadsheetFormulaPolicy, "import every column as text") {
		t.Fatalf("manifest files/policy = %#v", manifest)
	}
	for index, file := range manifest.Files {
		spec := tableSpecs[index]
		content, ok := archiveFiles[spec.Filename]
		if !ok {
			t.Fatalf("archive missing %s", spec.Filename)
		}
		digest := sha256.Sum256(content)
		if file.Filename != spec.Filename || file.Bytes != len(content) || file.SHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("manifest file %s = %#v", spec.Filename, file)
		}
		records := readCSV(t, content)
		if !reflect.DeepEqual(records[0], spec.Header) {
			t.Fatalf("%s header = %#v", spec.Filename, records[0])
		}
		if file.RecordCount != len(records)-1 {
			t.Fatalf("%s record count = %d, parsed = %d", spec.Filename, file.RecordCount, len(records)-1)
		}
	}
	partyRecords := readCSV(t, archiveFiles["parties.csv"])
	if partyRecords[1][2] != formulaText {
		t.Fatalf("formula-like text changed = %q", partyRecords[1][2])
	}
	obligationRecords := readCSV(t, archiveFiles["obligations.csv"])
	if obligationRecords[1][2] != "0.1, \"quoted\"\nline" || obligationRecords[1][14] != "0.100000" {
		t.Fatalf("obligation CSV did not round-trip = %#v", obligationRecords[1])
	}
	for _, filename := range []string{"obligation_revisions.csv", "cash_transactions.csv", "audit_entries.csv"} {
		if got := len(readCSV(t, archiveFiles[filename])); got != 1 {
			t.Fatalf("empty %s record count including header = %d", filename, got)
		}
	}

	if len(repository.audits) != 1 {
		t.Fatalf("audit count = %d", len(repository.audits))
	}
	entry := repository.audits[0]
	if entry.ID != "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" || entry.CommandID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" ||
		entry.ObjectType != audit.ObjectLedgerExport || entry.ObjectID != testOwnerID || entry.Action != ActionMoneyFlowLedgerExported ||
		entry.Actor.DisplayName != "导出管理员" || entry.OccurredAt != generatedAt {
		t.Fatalf("audit = %#v", entry)
	}
	if entry.Details["archive_bytes"] != len(artifact.Content) || entry.Details["source_schema_version"] != snapshot.SchemaVersion {
		t.Fatalf("audit details = %#v", entry.Details)
	}
}

func TestExportReturnsNoArtifactWhenAuditFails(t *testing.T) {
	repository := &repositoryStub{
		snapshot: emptyTestSnapshot(), found: true, auditErr: errors.New("audit unavailable"),
	}
	service, _ := NewService(repository)
	service.newID = sequenceIDs()

	artifact, err := service.Export(context.Background(), Input{LedgerOwnerPartyID: testOwnerID, Actor: testActor()})
	if err == nil || !strings.Contains(err.Error(), "record money flow ledger export audit") {
		t.Fatalf("Export() artifact=%#v error=%v", artifact, err)
	}
	if len(artifact.Content) != 0 {
		t.Fatalf("artifact returned after audit failure: %d bytes", len(artifact.Content))
	}
}

func TestExportValidatesInputNotFoundAndReadFailure(t *testing.T) {
	repository := &repositoryStub{snapshot: emptyTestSnapshot(), found: true}
	service, _ := NewService(repository)

	_, err := service.Export(context.Background(), Input{LedgerOwnerPartyID: "not-a-uuid", Actor: testActor()})
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Field != "ledger_owner_party_id" {
		t.Fatalf("invalid owner error = %v", err)
	}
	_, err = service.Export(context.Background(), Input{LedgerOwnerPartyID: testOwnerID})
	if !errors.As(err, &validation) || validation.Field != "actor" {
		t.Fatalf("invalid actor error = %v", err)
	}

	repository.found = false
	_, err = service.Export(context.Background(), Input{LedgerOwnerPartyID: testOwnerID, Actor: testActor()})
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("not found error = %v", err)
	}
	repository.found = true
	repository.loadErr = errors.New("database unavailable")
	_, err = service.Export(context.Background(), Input{LedgerOwnerPartyID: testOwnerID, Actor: testActor()})
	if err == nil || !strings.Contains(err.Error(), "load money flow ledger export") {
		t.Fatalf("read error = %v", err)
	}
}

func TestExportRejectsCorruptSnapshotShape(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{name: "owner mismatch", mutate: func(value *Snapshot) { value.Owner.ID = "22222222-2222-4222-8222-222222222222" }},
		{name: "schema missing", mutate: func(value *Snapshot) { value.SchemaVersion = "" }},
		{name: "table missing", mutate: func(value *Snapshot) { value.Tables = value.Tables[:len(value.Tables)-1] }},
		{name: "table duplicate", mutate: func(value *Snapshot) { value.Tables[1].Filename = value.Tables[0].Filename }},
		{name: "unknown table", mutate: func(value *Snapshot) { value.Tables[0].Filename = "../escape.csv" }},
		{name: "wrong column count", mutate: func(value *Snapshot) { value.Tables[0].Rows = [][]string{{"only-one"}} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := emptyTestSnapshot()
			test.mutate(&snapshot)
			repository := &repositoryStub{snapshot: snapshot, found: true}
			service, _ := NewService(repository)
			_, err := service.Export(context.Background(), Input{LedgerOwnerPartyID: testOwnerID, Actor: testActor()})
			var invariant *SnapshotInvariantError
			if !errors.As(err, &invariant) {
				t.Fatalf("error = %v", err)
			}
			if len(repository.audits) != 0 {
				t.Fatal("corrupt snapshot recorded export audit")
			}
		})
	}
}

func TestNewServiceRejectsNilRepository(t *testing.T) {
	if _, err := NewService(nil); err == nil {
		t.Fatal("NewService(nil) succeeded")
	}
}

func emptyTestSnapshot() Snapshot {
	tables := make([]Table, 0, len(tableSpecs))
	for _, spec := range tableSpecs {
		tables = append(tables, Table{Filename: spec.Filename, Rows: make([][]string, 0)})
	}
	return Snapshot{
		Owner: Owner{
			ID: testOwnerID, Kind: "organization", LegalName: "本方主体", RegistrationIdentifier: "REG-1", Revision: 1,
		},
		SchemaVersion: "000016_local_user_iam_subject_links.sql",
		Tables:        tables,
	}
}

func rowWithValues(length int, values map[int]string) []string {
	row := make([]string, length)
	for index, value := range values {
		row[index] = value
	}
	return row
}

func sequenceIDs() func() (string, error) {
	values := []string{
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
	}
	return func() (string, error) {
		value := values[0]
		values = values[1:]
		return value, nil
	}
}

func testActor() audit.ActorRef {
	return audit.ActorRef{
		Kind: audit.ActorKindLocalUser, ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", DisplayName: "导出管理员",
	}
}

func readArchive(t *testing.T, content []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("open ZIP: %v", err)
	}
	result := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || file.Mode()&0o777 != 0o600 {
			t.Fatalf("unsafe ZIP entry %q mode=%o", file.Name, file.Mode())
		}
		if _, exists := result[file.Name]; exists {
			t.Fatalf("duplicate ZIP entry %q", file.Name)
		}
		opened, err := file.Open()
		if err != nil {
			t.Fatalf("open %q: %v", file.Name, err)
		}
		value, err := io.ReadAll(opened)
		_ = opened.Close()
		if err != nil {
			t.Fatalf("read %q: %v", file.Name, err)
		}
		result[file.Name] = value
	}
	return result
}

func readCSV(t *testing.T, content []byte) [][]string {
	t.Helper()
	reader := csv.NewReader(bytes.NewReader(content))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	return records
}
