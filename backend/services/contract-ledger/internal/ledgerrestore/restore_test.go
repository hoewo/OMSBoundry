package ledgerrestore

import (
	"context"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
)

type restoreRepositoryStub struct {
	record  Record
	created bool
	err     error
	command Command
	calls   int
}

func (r *restoreRepositoryStub) Restore(_ context.Context, command Command) (Record, bool, error) {
	r.calls++
	r.command = command
	return r.record, r.created, r.err
}

func TestRestoreValidatesThenDelegates(t *testing.T) {
	repository := &restoreRepositoryStub{created: true, record: Record{
		ContractID: testContractID, Status: "draft", MissingEvidenceVersions: 0,
	}}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 19, 11, 0, 0, 0, time.UTC) }
	service.newID = func() (string, error) { return "66666666-6666-4666-8666-666666666666", nil }
	result, err := service.Restore(context.Background(), Input{
		CommandID: "77777777-7777-4777-8777-777777777777",
		Archive:   testArchive(t, testDraftLedger(), nil),
		Actor:     audit.ActorRef{Kind: audit.ActorKindLocalUser, ID: testActorID, DisplayName: "恢复管理员"},
	})
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if repository.calls != 1 || result.ContractID != testContractID || result.Replayed ||
		repository.command.AuditID != "66666666-6666-4666-8666-666666666666" ||
		repository.command.Candidate.LedgerSHA256 == "" {
		t.Fatalf("calls=%d result=%#v command=%#v", repository.calls, result, repository.command)
	}
}

func TestRestoreRejectsInvalidInputBeforeRepository(t *testing.T) {
	repository := &restoreRepositoryStub{}
	service, _ := NewService(repository)
	_, err := service.Restore(context.Background(), Input{CommandID: "bad", Archive: []byte("bad")})
	var validation *ValidationError
	if !errors.As(err, &validation) || repository.calls != 0 {
		t.Fatalf("error=%v calls=%d", err, repository.calls)
	}
}
