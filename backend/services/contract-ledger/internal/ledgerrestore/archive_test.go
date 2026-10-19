package ledgerrestore

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/ledgerexport"
)

const (
	testContractID = "11111111-1111-4111-8111-111111111111"
	testCommandID  = "22222222-2222-4222-8222-222222222222"
	testActorID    = "33333333-3333-4333-8333-333333333333"
)

func TestParseArchiveAcceptsSelfContainedDraft(t *testing.T) {
	archive := testArchive(t, testDraftLedger(), nil)
	candidate, err := ParseArchive(archive)
	if err != nil {
		t.Fatalf("ParseArchive() error = %v", err)
	}
	if candidate.Ledger.Contract.ID != testContractID || candidate.LedgerSHA256 == "" ||
		!candidate.SourceGeneratedAt.Equal(time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestParseArchiveRejectsUnsafeOrUnverifiableInput(t *testing.T) {
	tests := []struct {
		name    string
		archive func(*testing.T) []byte
		want    any
	}{
		{name: "not zip", archive: func(*testing.T) []byte { return []byte("not-a-zip") }, want: &ValidationError{}},
		{name: "extra file", archive: func(t *testing.T) []byte {
			return testArchive(t, testDraftLedger(), map[string][]byte{"extra.txt": []byte("x")})
		}, want: &ValidationError{}},
		{name: "traversal", archive: func(t *testing.T) []byte {
			return rawArchive(t, map[string][]byte{"../manifest.json": []byte("{}"), ledgerFilename: []byte("{}")})
		}, want: &ValidationError{}},
		{name: "digest mismatch", archive: func(t *testing.T) []byte {
			value := testArchive(t, testDraftLedger(), nil)
			files := unzipForTest(t, value)
			files[ledgerFilename] = append(files[ledgerFilename], ' ')
			return rawArchive(t, files)
		}, want: &ValidationError{}},
		{name: "unknown ledger field", archive: func(t *testing.T) []byte {
			ledger := testDraftLedger()
			ledgerBytes, _ := json.Marshal(ledger)
			ledgerBytes = bytes.Replace(ledgerBytes, []byte(`"format":`), []byte(`"unknown":true,"format":`), 1)
			return archiveFromLedgerBytes(t, ledgerBytes)
		}, want: &ValidationError{}},
		{name: "external relation", archive: func(t *testing.T) []byte {
			ledger := testDraftLedger()
			ledger.Relations = []ledgerexport.Relation{{ID: "44444444-4444-4444-8444-444444444444"}}
			return testArchive(t, ledger, nil)
		}, want: &UnsupportedRelationsError{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseArchive(test.archive(t))
			if err == nil {
				t.Fatal("ParseArchive() error = nil")
			}
			switch test.want.(type) {
			case *ValidationError:
				var target *ValidationError
				if !errors.As(err, &target) {
					t.Fatalf("error = %T %v", err, err)
				}
			case *UnsupportedRelationsError:
				var target *UnsupportedRelationsError
				if !errors.As(err, &target) {
					t.Fatalf("error = %T %v", err, err)
				}
			}
		})
	}
}

func TestParseArchiveRejectsBrokenLifecycleAndReferences(t *testing.T) {
	ledger := testDraftLedger()
	revision := int64(1)
	ledger.Contract.Status = "active"
	ledger.Contract.CurrentRevision = &revision
	_, err := ParseArchive(testArchive(t, ledger, nil))
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %T %v", err, err)
	}
}

func testDraftLedger() ledgerexport.Ledger {
	commandID := testCommandID
	return ledgerexport.Ledger{
		Format: ledgerexport.FormatID,
		Contract: ledgerexport.ContractSnapshot{
			ID: testContractID, NumberNamespace: "TEST", ContractNumber: "DRAFT-001",
			Title: "待恢复草稿", Currency: "CNY", Status: "draft", DraftVersion: 1,
			CreatedAt: "2026-09-19T09:00:00Z", UpdatedAt: "2026-09-19T09:00:00Z",
		},
		DraftParties: []ledgerexport.DraftParty{}, Revisions: []ledgerexport.Revision{},
		Instruments: []ledgerexport.Instrument{}, Relations: []ledgerexport.Relation{},
		Settlements: []ledgerexport.Settlement{}, Evidence: []ledgerexport.EvidenceDocument{},
		Audit: []ledgerexport.AuditEntry{{
			ID: "55555555-5555-4555-8555-555555555555", Action: "contract_draft_created",
			CommandID:  &commandID,
			Actor:      ledgerexport.Actor{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "源管理员"},
			OccurredAt: "2026-09-19T09:00:00Z", Summary: "创建合同草稿，草稿版本 1",
			Details: json.RawMessage(`{"resulting_draft_version":1}`),
		}},
	}
}

func testArchive(t *testing.T, ledger ledgerexport.Ledger, extras map[string][]byte) []byte {
	t.Helper()
	ledgerBytes, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipForTest(t, archiveFromLedgerBytes(t, ledgerBytes))
	for name, value := range extras {
		files[name] = value
	}
	return rawArchive(t, files)
}

func archiveFromLedgerBytes(t *testing.T, ledgerBytes []byte) []byte {
	t.Helper()
	digestBytes := sha256.Sum256(ledgerBytes)
	manifest := ledgerexport.Manifest{
		FormatID: ledgerexport.FormatID, FormatVersion: ledgerexport.FormatVersion,
		GeneratedAt: "2026-09-19T10:00:00Z", ContractID: testContractID,
		LedgerFilename: ledgerFilename, LedgerBytes: len(ledgerBytes),
		LedgerSHA256: hex.EncodeToString(digestBytes[:]), EvidenceContentIncluded: false,
		ExcludedContent: []string{"evidence binary content"},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return rawArchive(t, map[string][]byte{manifestFilename: manifestBytes, ledgerFilename: ledgerBytes})
}

func rawArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, value := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o600)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func unzipForTest(t *testing.T, content []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte)
	for _, file := range reader.File {
		opened, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buffer bytes.Buffer
		if _, err := buffer.ReadFrom(opened); err != nil {
			t.Fatal(err)
		}
		_ = opened.Close()
		files[file.Name] = buffer.Bytes()
	}
	return files
}
