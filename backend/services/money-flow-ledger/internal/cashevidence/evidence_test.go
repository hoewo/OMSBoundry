package cashevidence

import (
	"context"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

const (
	testTenant  = "11111111-1111-4111-8111-111111111111"
	testCash    = "22222222-2222-4222-8222-222222222222"
	testFile    = "33333333-3333-4333-8333-333333333333"
	testCommand = "44444444-4444-4444-8444-444444444444"
)

type repositoryStub struct {
	input ReplaceInput
	hash  []byte
	now   time.Time
	value Result
	err   error
}

func (s *repositoryStub) Current(context.Context, string) (Snapshot, bool, error) {
	return Snapshot{}, false, nil
}

func (s *repositoryStub) Replace(_ context.Context, input ReplaceInput, hash []byte, now time.Time) (Result, error) {
	s.input, s.hash, s.now = input, append([]byte(nil), hash...), now
	if s.err != nil {
		return Result{}, s.err
	}
	return s.value, nil
}

type validatorStub struct {
	calls int
	ref   FileReference
	err   error
}

func (s *validatorStub) Resolve(context.Context, string, Ref) (FileReference, error) {
	s.calls++
	return s.ref, s.err
}

func TestReplaceValidatesExactEvidenceAndPersistsNormalizedRevision(t *testing.T) {
	repository := &repositoryStub{value: Result{Snapshot: Snapshot{CashTransactionID: testCash, Revision: 1}}}
	validator := &validatorStub{ref: FileReference{TenantID: testTenant, FileID: testFile, Version: 2, Revision: 2, Status: "available", Name: "银行回单.pdf"}}
	service, err := NewService(repository, validator)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	result, err := service.Replace(context.Background(), ReplaceInput{
		CommandID:         testCommand,
		ExpectedRevision:  0,
		Reason:            "  上传实际回款凭证  ",
		EvidenceRefs:      []Ref{{FileID: testFile, Version: 2, Purpose: "receipt_proof"}},
		Actor:             audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "55555555-5555-4555-8555-555555555555", DisplayName: " 会计 "},
		TenantID:          testTenant,
		CashTransactionID: testCash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Snapshot.Revision != 1 || validator.calls != 1 || len(repository.hash) != 32 || !repository.now.Equal(now) {
		t.Fatalf("result=%#v validator_calls=%d hash=%d now=%s", result, validator.calls, len(repository.hash), repository.now)
	}
	if repository.input.Reason != "上传实际回款凭证" || repository.input.Actor.DisplayName != "会计" || len(repository.input.EvidenceRefs) != 1 || repository.input.EvidenceRefs[0].DisplayName != "银行回单.pdf" {
		t.Fatalf("normalized input = %#v", repository.input)
	}
}

func TestReplaceRejectsInvalidInputBeforeCallingEvidence(t *testing.T) {
	repository := &repositoryStub{}
	validator := &validatorStub{}
	service, _ := NewService(repository, validator)
	_, err := service.Replace(context.Background(), ReplaceInput{
		CommandID:         testCommand,
		Reason:            "附件",
		EvidenceRefs:      []Ref{{FileID: testFile, Version: 1, Purpose: "tax_decision"}},
		Actor:             audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "55555555-5555-4555-8555-555555555555"},
		TenantID:          testTenant,
		CashTransactionID: testCash,
	})
	if !errors.Is(err, ErrInvalid) || validator.calls != 0 || len(repository.hash) != 0 {
		t.Fatalf("err=%v validator_calls=%d hash=%x", err, validator.calls, repository.hash)
	}
}

func TestReplaceRejectsStaleEvidenceWithoutPersisting(t *testing.T) {
	repository := &repositoryStub{}
	validator := &validatorStub{ref: FileReference{TenantID: testTenant, FileID: testFile, Version: 2, Revision: 3, Status: "available", Name: "stale.pdf"}}
	service, _ := NewService(repository, validator)
	_, err := service.Replace(context.Background(), ReplaceInput{
		CommandID:         testCommand,
		Reason:            "附件",
		EvidenceRefs:      []Ref{{FileID: testFile, Version: 2, Purpose: "payment_proof"}},
		Actor:             audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "55555555-5555-4555-8555-555555555555"},
		TenantID:          testTenant,
		CashTransactionID: testCash,
	})
	if !errors.Is(err, ErrEvidenceInvalid) || validator.calls != 1 || len(repository.hash) != 0 {
		t.Fatalf("err=%v validator_calls=%d hash=%x", err, validator.calls, repository.hash)
	}
}
