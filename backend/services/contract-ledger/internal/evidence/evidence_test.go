package evidence

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
)

const (
	testCommandID  = "11111111-1111-4111-8111-111111111111"
	testContractID = "22222222-2222-4222-8222-222222222222"
	testActorID    = "33333333-3333-4333-8333-333333333333"
)

var testActor = audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "证据测试员"}

func TestServiceCreatesVersionsReplaysAndVerifiesDownloads(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	repository := newMemoryRepository(testContractID)
	service, err := NewService(repository, store, 1024)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC) }

	input := CreateInput{
		CommandID: testCommandID, ContractID: testContractID,
		TargetType: " CONTRACT ", TargetID: strings.ToUpper(testContractID),
		Title: " 原合同扫描件 ", Purpose: " 双方签署依据 ",
		OriginalFilename: `../../signed-contract.pdf`, Actor: testActor,
	}
	firstContent := []byte("%PDF-1.7\nfirst-version")
	created, err := service.Create(ctx, input, bytes.NewReader(firstContent))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Replayed || created.Document.Title != "原合同扫描件" || created.Document.Purpose != "双方签署依据" ||
		created.Version.Version != 1 || created.Version.OriginalFilename != "signed-contract.pdf" ||
		created.Version.SizeBytes != int64(len(firstContent)) || created.Version.DetectedMediaType != "application/pdf" {
		t.Fatalf("created evidence = %#v", created)
	}

	replayed, err := service.Create(ctx, input, bytes.NewReader(firstContent))
	if err != nil {
		t.Fatalf("replayed Create() error = %v", err)
	}
	if !replayed.Replayed || replayed.Document.ID != created.Document.ID || replayed.Version.ID != created.Version.ID {
		t.Fatalf("replayed evidence = %#v", replayed)
	}
	if count := countStoredObjects(t, root); count != 1 {
		t.Fatalf("stored object count after replay = %d, want 1", count)
	}

	_, err = service.Create(ctx, input, bytes.NewReader([]byte("different content")))
	var idempotency *IdempotencyConflictError
	if !errors.As(err, &idempotency) {
		t.Fatalf("changed replay error = %v", err)
	}
	if count := countStoredObjects(t, root); count != 1 {
		t.Fatalf("stored object count after conflict = %d, want 1", count)
	}

	secondContent := []byte("%PDF-1.7\nsecond-version")
	added, err := service.AddVersion(ctx, AddVersionInput{
		CommandID:  "44444444-4444-4444-8444-444444444444",
		ContractID: testContractID, DocumentID: created.Document.ID,
		ExpectedLatestVersion: 1, OriginalFilename: "signed-contract-v2.pdf", Actor: testActor,
	}, bytes.NewReader(secondContent))
	if err != nil {
		t.Fatalf("AddVersion() error = %v", err)
	}
	if added.Version.Version != 2 || added.Version.OriginalFilename != "signed-contract-v2.pdf" {
		t.Fatalf("added evidence version = %#v", added)
	}

	_, err = service.AddVersion(ctx, AddVersionInput{
		CommandID:  "55555555-5555-4555-8555-555555555555",
		ContractID: testContractID, DocumentID: created.Document.ID,
		ExpectedLatestVersion: 1, OriginalFilename: "stale.pdf", Actor: testActor,
	}, bytes.NewReader([]byte("stale")))
	var versionConflict *VersionConflictError
	if !errors.As(err, &versionConflict) || versionConflict.Actual != 2 {
		t.Fatalf("stale version error = %v", err)
	}
	if count := countStoredObjects(t, root); count != 2 {
		t.Fatalf("stored object count after stale version = %d, want 2", count)
	}

	download, err := service.Open(ctx, DownloadInput{
		ContractID: testContractID, DocumentID: created.Document.ID,
		VersionID: added.Version.ID, Actor: testActor,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer download.Content.Close()
	got, err := io.ReadAll(download.Content)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if !bytes.Equal(got, secondContent) || len(repository.downloads) != 1 || repository.downloads[0].Actor != testActor {
		t.Fatalf("download bytes=%q audits=%#v", got, repository.downloads)
	}

	objectPath, err := store.objectPath(added.Version.StorageKey, false)
	if err != nil {
		t.Fatalf("objectPath() error = %v", err)
	}
	if err := os.WriteFile(objectPath, []byte("tampered"), 0o600); err != nil {
		t.Fatalf("tamper evidence object: %v", err)
	}
	_, err = service.Open(ctx, DownloadInput{
		ContractID: testContractID, DocumentID: created.Document.ID,
		VersionID: added.Version.ID, Actor: testActor,
	})
	var integrity *IntegrityError
	if !errors.As(err, &integrity) {
		t.Fatalf("tampered Open() error = %v", err)
	}
	if len(repository.downloads) != 1 {
		t.Fatalf("tampered download wrote %d audits, want 1", len(repository.downloads))
	}
}

func TestServiceValidatesEvidenceInputsBeforeStorage(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	repository := newMemoryRepository(testContractID)
	service, err := NewService(repository, store, 1024)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	valid := CreateInput{
		CommandID: testCommandID, ContractID: testContractID,
		TargetType: string(TargetContract), TargetID: testContractID,
		Title: "证据", Purpose: "依据", OriginalFilename: "file.pdf", Actor: testActor,
	}
	tests := []struct {
		name   string
		mutate func(*CreateInput)
		field  string
	}{
		{name: "command", mutate: func(v *CreateInput) { v.CommandID = "bad" }, field: "command_id"},
		{name: "contract", mutate: func(v *CreateInput) { v.ContractID = "bad" }, field: "contract_id"},
		{name: "target type", mutate: func(v *CreateInput) { v.TargetType = "external" }, field: "target_type"},
		{name: "target", mutate: func(v *CreateInput) { v.TargetID = "bad" }, field: "target_id"},
		{name: "title", mutate: func(v *CreateInput) { v.Title = " " }, field: "title"},
		{name: "purpose", mutate: func(v *CreateInput) { v.Purpose = " " }, field: "purpose"},
		{name: "filename", mutate: func(v *CreateInput) { v.OriginalFilename = "bad\nname.pdf" }, field: "file"},
		{name: "actor", mutate: func(v *CreateInput) { v.Actor = audit.ActorRef{} }, field: "actor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			_, err := service.Create(context.Background(), input, bytes.NewReader([]byte("data")))
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("Create() error = %v", err)
			}
		})
	}

	wrongContractTarget := valid
	wrongContractTarget.TargetID = "66666666-6666-4666-8666-666666666666"
	_, err = service.Create(context.Background(), wrongContractTarget, bytes.NewReader([]byte("data")))
	var target *TargetConflictError
	if !errors.As(err, &target) {
		t.Fatalf("wrong contract target error = %v", err)
	}
	if count := countStoredObjects(t, store.Root()); count != 0 {
		t.Fatalf("invalid inputs stored %d objects", count)
	}
}

func TestLocalStoreEnforcesContentAndKeyBoundaries(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	ctx := context.Background()
	key := "77777777-7777-4777-8777-777777777777"
	content := []byte("hello evidence")
	stored, err := store.Put(ctx, key, bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if stored.StorageKey != key || stored.SizeBytes != int64(len(content)) || len(stored.SHA256) != 64 || stored.DetectedMediaType == "" {
		t.Fatalf("stored blob = %#v", stored)
	}
	opened, err := store.Open(ctx, key)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	got, _ := io.ReadAll(opened)
	_ = opened.Close()
	if !bytes.Equal(got, content) {
		t.Fatalf("opened content = %q", got)
	}

	_, err = store.Put(ctx, "../../escape", bytes.NewReader(content), 100)
	if err == nil {
		t.Fatal("Put() with unsafe key error = nil")
	}
	_, err = store.Put(ctx, "88888888-8888-4888-8888-888888888888", bytes.NewReader(nil), 100)
	var empty *EmptyFileError
	if !errors.As(err, &empty) {
		t.Fatalf("empty Put() error = %v", err)
	}
	_, err = store.Put(ctx, "99999999-9999-4999-8999-999999999999", bytes.NewReader([]byte("too large")), 3)
	var tooLarge *FileTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("large Put() error = %v", err)
	}
	if count := countStoredObjects(t, root); count != 1 {
		t.Fatalf("stored object count = %d, want 1", count)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if count := countStoredObjects(t, root); count != 0 {
		t.Fatalf("stored object count after delete = %d", count)
	}
	if _, err := NewLocalStore(string(filepath.Separator)); err == nil {
		t.Fatal("NewLocalStore(filesystem root) error = nil")
	}
}

func TestLocalStoreWritableCheckLeavesNoProbe(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CheckWritable(context.Background()); err != nil {
		t.Fatalf("CheckWritable() error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("writable check left entries: %#v", entries)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.CheckWritable(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("CheckWritable(cancelled) error = %v", err)
	}
}

func TestServiceCleansCandidateAfterRepositoryFailureCancelsRequest(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	repository := &cancelingRecordRepository{
		Repository: newMemoryRepository(testContractID),
		cancel:     cancel,
	}
	service, err := NewService(repository, store, 1024)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	_, err = service.Create(ctx, CreateInput{
		CommandID: testCommandID, ContractID: testContractID,
		TargetType: string(TargetContract), TargetID: testContractID,
		Title: "取消请求清理", Purpose: "验证数据库失败后仍执行补偿清理",
		OriginalFilename: "cancelled.txt", Actor: testActor,
	}, strings.NewReader("candidate content"))
	if err == nil {
		t.Fatal("Create() error = nil")
	}
	if count := countStoredObjects(t, root); count != 0 {
		t.Fatalf("stored object count after cancelled repository failure = %d", count)
	}
}

type cancelingRecordRepository struct {
	Repository
	cancel context.CancelFunc
}

func (r *cancelingRecordRepository) Record(context.Context, Command) (Command, Document, Version, bool, error) {
	r.cancel()
	return Command{}, Document{}, Version{}, false, errors.New("repository unavailable")
}

func countStoredObjects(t *testing.T, root string) int {
	t.Helper()
	count := 0
	err := filepath.Walk(filepath.Join(root, "objects"), func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			count++
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("walk stored objects: %v", err)
	}
	return count
}

type memoryRepository struct {
	contractID string
	commands   map[string]Command
	documents  map[string]Document
	downloads  []DownloadAudit
}

func newMemoryRepository(contractID string) *memoryRepository {
	return &memoryRepository{
		contractID: contractID,
		commands:   make(map[string]Command),
		documents:  make(map[string]Document),
	}
}

func (r *memoryRepository) Record(_ context.Context, command Command) (Command, Document, Version, bool, error) {
	if existing, ok := r.commands[command.CommandID]; ok {
		document := r.documents[existing.DocumentID]
		for _, version := range document.Versions {
			if version.ID == existing.VersionID {
				return existing, document, version, false, nil
			}
		}
	}
	if command.ContractID != r.contractID {
		return Command{}, Document{}, Version{}, false, &ContractNotFoundError{ContractID: command.ContractID}
	}
	var document Document
	if command.Action == CommandCreateDocument {
		document = Document{
			ID: command.DocumentID, ContractID: command.ContractID,
			TargetType: command.TargetType, TargetID: command.TargetID,
			Title: command.Title, Purpose: command.Purpose,
			Actor: command.Actor, CreatedAt: command.OccurredAt,
		}
	} else {
		var ok bool
		document, ok = r.documents[command.DocumentID]
		if !ok {
			return Command{}, Document{}, Version{}, false, &DocumentNotFoundError{DocumentID: command.DocumentID}
		}
		if document.Latest != command.ExpectedLatestVersion {
			return Command{}, Document{}, Version{}, false, &VersionConflictError{Expected: command.ExpectedLatestVersion, Actual: document.Latest}
		}
		command.TargetType = document.TargetType
		command.TargetID = document.TargetID
		command.Title = document.Title
		command.Purpose = document.Purpose
	}
	version := Version{
		ID: command.VersionID, DocumentID: command.DocumentID, Version: command.ResultingVersion,
		OriginalFilename: command.OriginalFilename, DetectedMediaType: command.DetectedMediaType,
		SizeBytes: command.SizeBytes, SHA256: command.SHA256,
		StorageProvider: command.StorageProvider, StorageKey: command.StorageKey,
		Actor: command.Actor, RecordedAt: command.OccurredAt,
	}
	document.Versions = append(document.Versions, version)
	document.Latest = version.Version
	r.documents[document.ID] = document
	r.commands[command.CommandID] = command
	return command, document, version, true, nil
}

func (r *memoryRepository) List(_ context.Context, contractID string) ([]Document, bool, error) {
	if contractID != r.contractID {
		return nil, false, nil
	}
	values := make([]Document, 0, len(r.documents))
	for _, document := range r.documents {
		values = append(values, document)
	}
	return values, true, nil
}

func (r *memoryRepository) FindVersion(_ context.Context, contractID, documentID, versionID string) (Document, Version, bool, error) {
	if contractID != r.contractID {
		return Document{}, Version{}, false, nil
	}
	document, ok := r.documents[documentID]
	if !ok {
		return Document{}, Version{}, false, nil
	}
	for _, version := range document.Versions {
		if version.ID == versionID {
			return document, version, true, nil
		}
	}
	return Document{}, Version{}, false, nil
}

func (r *memoryRepository) RecordDownload(_ context.Context, event DownloadAudit) error {
	r.downloads = append(r.downloads, event)
	return nil
}
