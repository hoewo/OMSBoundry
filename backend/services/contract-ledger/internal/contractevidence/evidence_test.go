package contractevidence

import (
	"context"
	"errors"
	"testing"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/referenceoa"
)

const (
	testTenant   = "11111111-1111-4111-8111-111111111111"
	testContract = "22222222-2222-4222-8222-222222222222"
	testFile     = "33333333-3333-4333-8333-333333333333"
	testCommand  = "44444444-4444-4444-8444-444444444444"
)

type repositoryStub struct {
	input ReplaceInput
	hash  []byte
	now   time.Time
	value Result
}

func (s *repositoryStub) Current(context.Context, string, string) (Snapshot, bool, error) {
	return Snapshot{}, false, nil
}

func (s *repositoryStub) Replace(_ context.Context, input ReplaceInput, hash []byte, now time.Time) (Result, error) {
	s.input, s.hash, s.now = input, append([]byte(nil), hash...), now
	return s.value, nil
}

type validatorStub struct {
	value referenceoa.FileReference
	err   error
	calls int
}

func (s *validatorStub) ValidateFile(context.Context, string, string, int) (referenceoa.FileReference, error) {
	s.calls++
	return s.value, s.err
}

func TestReplaceValidatesAndPersistsExactEvidenceReference(t *testing.T) {
	repository := &repositoryStub{value: Result{Snapshot: Snapshot{ContractID: testContract, Revision: 1}}}
	validator := &validatorStub{value: referenceoa.FileReference{ID: testFile, FileID: testFile, TenantID: testTenant, Version: 2, Revision: 2, Status: "available", Name: "合同扫描件.pdf"}}
	service, err := NewService(repository, validator)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	result, err := service.Replace(context.Background(), ReplaceInput{
		CommandID: testCommand, ExpectedRevision: 0, Reason: "  补充合同扫描件  ",
		EvidenceRefs: []Ref{{FileID: testFile, Version: 2, Purpose: "contract_document"}},
		Actor:        audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "55555555-5555-4555-8555-555555555555", DisplayName: " 会计 "},
		TenantID:     testTenant, ContractID: testContract,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Snapshot.Revision != 1 || validator.calls != 1 || len(repository.hash) != 32 || !repository.now.Equal(now) || repository.input.Reason != "补充合同扫描件" || repository.input.EvidenceRefs[0].DisplayName != "合同扫描件.pdf" {
		t.Fatalf("result=%#v input=%#v calls=%d hash=%d", result, repository.input, validator.calls, len(repository.hash))
	}
}

func TestReplaceRejectsInvalidPurposeBeforeEvidenceLookup(t *testing.T) {
	repository := &repositoryStub{}
	validator := &validatorStub{}
	service, _ := NewService(repository, validator)
	_, err := service.Replace(context.Background(), ReplaceInput{
		CommandID: testCommand, Reason: "附件", EvidenceRefs: []Ref{{FileID: testFile, Version: 1, Purpose: "tax_rule"}},
		Actor: audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "55555555-5555-4555-8555-555555555555"}, TenantID: testTenant, ContractID: testContract,
	})
	if !errors.Is(err, ErrInvalid) || validator.calls != 0 || len(repository.hash) != 0 {
		t.Fatalf("err=%v calls=%d hash=%x", err, validator.calls, repository.hash)
	}
}

func TestReplaceMapsEvidenceOutageWithoutPersisting(t *testing.T) {
	repository := &repositoryStub{}
	validator := &validatorStub{err: referenceoa.ErrEvidenceUnavailable}
	service, _ := NewService(repository, validator)
	_, err := service.Replace(context.Background(), ReplaceInput{
		CommandID: testCommand, Reason: "附件", EvidenceRefs: []Ref{{FileID: testFile, Version: 1, Purpose: "supporting_document"}},
		Actor: audit.ActorRef{Kind: audit.ActorKindIAMSubject, ID: "55555555-5555-4555-8555-555555555555"}, TenantID: testTenant, ContractID: testContract,
	})
	if !errors.Is(err, ErrEvidenceUnavailable) || validator.calls != 1 || len(repository.hash) != 0 {
		t.Fatalf("err=%v calls=%d hash=%x", err, validator.calls, repository.hash)
	}
}
