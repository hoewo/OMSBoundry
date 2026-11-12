package recognition

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	contract "omsbizfoundry/invoice-recognition-contract"
)

const (
	tenantID   = "11111111-1111-4111-8111-111111111111"
	subjectID  = "22222222-2222-4222-8222-222222222222"
	commandID  = "33333333-3333-4333-8333-333333333333"
	fileID     = "44444444-4444-4444-8444-444444444444"
	proposalID = "55555555-5555-4555-8555-555555555555"
)

type evidenceStub struct{ body []byte }

func (stub evidenceStub) Open(context.Context, string, contract.EvidenceReference) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(stub.body)), nil
}

type providerStub struct {
	called bool
	result ProviderResult
}

func (stub *providerStub) Recognize(context.Context, Document) (ProviderResult, error) {
	stub.called = true
	return stub.result, nil
}

type repositoryStub struct {
	proposal contract.Proposal
}

func (stub *repositoryStub) ReplayProposal(context.Context, string, string, string) (contract.Proposal, bool, error) {
	return contract.Proposal{}, false, nil
}
func (stub *repositoryStub) ReplayConfirmation(context.Context, string, string, string) (contract.Confirmation, bool, error) {
	return contract.Confirmation{}, false, nil
}
func (stub *repositoryStub) ReplayDiscard(context.Context, string, string, string) (contract.Proposal, bool, error) {
	return contract.Proposal{}, false, nil
}

func (stub *repositoryStub) Create(_ context.Context, _ Actor, proposal contract.Proposal, hash string) (contract.Proposal, error) {
	if len(hash) != 64 {
		return contract.Proposal{}, fmt.Errorf("invalid intent hash")
	}
	stub.proposal = proposal
	return proposal, nil
}

func (stub *repositoryStub) Get(_ context.Context, tenant, id string) (contract.Proposal, error) {
	if tenant != stub.proposal.TenantID || id != stub.proposal.ID {
		return contract.Proposal{}, fmt.Errorf("not found")
	}
	return stub.proposal, nil
}

func (stub *repositoryStub) List(_ context.Context, tenant string) ([]contract.Proposal, error) {
	if tenant != stub.proposal.TenantID {
		return nil, fmt.Errorf("not found")
	}
	return []contract.Proposal{stub.proposal}, nil
}

func (stub *repositoryStub) Confirm(_ context.Context, _ Actor, _ string, proposal contract.Proposal, confirmation contract.Confirmation, hash string) (contract.Confirmation, error) {
	if len(hash) != 64 || proposal.ID != confirmation.ProposalID {
		return contract.Confirmation{}, fmt.Errorf("invalid confirmation")
	}
	stub.proposal.Status = "confirmed"
	return confirmation, nil
}
func (stub *repositoryStub) Discard(_ context.Context, actor Actor, _ string, proposal contract.Proposal, reason string, now time.Time, hash string) (contract.Proposal, error) {
	if len(hash) != 64 || proposal.ID != stub.proposal.ID {
		return contract.Proposal{}, fmt.Errorf("invalid discard")
	}
	stub.proposal.Status = "discarded"
	stub.proposal.Revision++
	stub.proposal.DiscardReason = reason
	stub.proposal.DiscardedBy = actor.SubjectID
	stub.proposal.DiscardedAt = &now
	return stub.proposal, nil
}

func amount(value int64) *int64 { return &value }

func serviceFixture(body []byte) (*Service, *providerStub, *repositoryStub) {
	provider := &providerStub{result: ProviderResult{
		ProviderKey: "fixture-ocr", ProviderModel: "invoice-v1", ProviderRevision: "2026-11-05",
		Candidate: contract.Candidate{InvoiceCode: " 011001900111 ", InvoiceNumber: "12345678", Currency: "cny"},
	}}
	repo := &repositoryStub{}
	now := func() time.Time { return time.Date(2026, 11, 5, 15, 0, 0, 0, time.UTC) }
	return New(evidenceStub{body: body}, provider, repo, func() string { return proposalID }, now), provider, repo
}

func evidence(body []byte) contract.EvidenceReference {
	return contract.EvidenceReference{
		FileID: fileID, Version: 2,
		ContentSHA256: "sha256:484dc89eadf0bacf23776c2b317d3a43ba203cdcf95ae0592e790a94606e6ec5",
		MIMEType:      "application/pdf", SizeBytes: int64(len(body)),
	}
}

func TestRecognitionRequiresExactEvidenceVersionAndHumanConfirmation(t *testing.T) {
	body := []byte("trusted invoice bytes")
	service, provider, repo := serviceFixture(body)
	proposal, err := service.Start(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, StartInput{CommandID: commandID, Evidence: evidence(body)})
	if err != nil {
		t.Fatal(err)
	}
	if !provider.called || proposal.Status != "needs_review" || proposal.Candidate.InvoiceCode != "011001900111" || repo.proposal.ID != proposalID {
		t.Fatalf("unexpected proposal: %#v", proposal)
	}
	candidate := proposal.Candidate
	candidate.AmountExcludingTaxMinor = amount(10000)
	candidate.TaxAmountMinor = amount(1300)
	candidate.TotalAmountMinor = amount(11300)
	confirmation, err := service.Confirm(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, ConfirmInput{
		CommandID: "66666666-6666-4666-8666-666666666666", ProposalID: proposal.ID, ExpectedRevision: 1, Candidate: candidate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(confirmation.DuplicateKeys) != 2 || confirmation.DuplicateKeys[1] != "invoice:011001900111:12345678" {
		t.Fatalf("unexpected confirmation: %#v", confirmation)
	}
}

func TestRecognitionRejectsDigestDriftBeforeCallingProvider(t *testing.T) {
	body := []byte("different bytes")
	service, provider, _ := serviceFixture(body)
	ref := evidence(body)
	ref.ContentSHA256 = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := service.Start(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, StartInput{CommandID: commandID, Evidence: ref}); err == nil {
		t.Fatal("digest drift accepted")
	}
	if provider.called {
		t.Fatal("provider called before exact Evidence validation")
	}
}

func TestConfirmationRejectsInconsistentAmount(t *testing.T) {
	body := []byte("trusted invoice bytes")
	service, _, _ := serviceFixture(body)
	proposal, err := service.Start(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, StartInput{CommandID: commandID, Evidence: evidence(body)})
	if err != nil {
		t.Fatal(err)
	}
	candidate := proposal.Candidate
	candidate.AmountExcludingTaxMinor = amount(10000)
	candidate.TaxAmountMinor = amount(1300)
	candidate.TotalAmountMinor = amount(11301)
	if _, err := service.Confirm(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, ConfirmInput{
		CommandID: "66666666-6666-4666-8666-666666666666", ProposalID: proposal.ID, ExpectedRevision: 1, Candidate: candidate,
	}); err == nil {
		t.Fatal("inconsistent amount accepted")
	}
}

func TestPendingProposalCanBeDiscardedWithoutErasingHistory(t *testing.T) {
	body := []byte("trusted invoice bytes")
	service, _, _ := serviceFixture(body)
	proposal, err := service.Start(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, StartInput{CommandID: commandID, Evidence: evidence(body)})
	if err != nil {
		t.Fatal(err)
	}
	discarded, err := service.Discard(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, DiscardInput{
		CommandID: "77777777-7777-4777-8777-777777777777", ProposalID: proposal.ID, ExpectedRevision: proposal.Revision, Reason: "重复上传",
	})
	if err != nil {
		t.Fatal(err)
	}
	if discarded.Status != "discarded" || discarded.Revision != 2 || discarded.DiscardReason != "重复上传" || discarded.DiscardedBy != subjectID || discarded.DiscardedAt == nil {
		t.Fatalf("unexpected discarded proposal: %#v", discarded)
	}
	if _, err = service.Confirm(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, ConfirmInput{
		CommandID: "88888888-8888-4888-8888-888888888888", ProposalID: proposal.ID, ExpectedRevision: discarded.Revision, Candidate: proposal.Candidate,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("discarded proposal confirmation returned %v", err)
	}
}

func TestConfirmedProposalCanBeDiscardedWithoutChangingConfirmation(t *testing.T) {
	body := []byte("trusted invoice bytes")
	service, _, repo := serviceFixture(body)
	proposal, err := service.Start(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, StartInput{CommandID: commandID, Evidence: evidence(body)})
	if err != nil {
		t.Fatal(err)
	}
	candidate := proposal.Candidate
	candidate.TotalAmountMinor = amount(11300)
	confirmation, err := service.Confirm(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, ConfirmInput{
		CommandID: "66666666-6666-4666-8666-666666666666", ProposalID: proposal.ID, ExpectedRevision: proposal.Revision, Candidate: candidate,
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.proposal.Revision = 2
	discarded, err := service.Discard(context.Background(), Actor{SubjectID: subjectID, TenantID: tenantID}, DiscardInput{
		CommandID: "77777777-7777-4777-8777-777777777778", ProposalID: proposal.ID, ExpectedRevision: 2, Reason: "重复确认，保留审计后删除",
	})
	if err != nil {
		t.Fatal(err)
	}
	if discarded.Status != "discarded" || discarded.Revision != 3 || discarded.DiscardedBy != subjectID || discarded.DiscardedAt == nil {
		t.Fatalf("unexpected discarded confirmation: %#v", discarded)
	}
	if confirmation.ProposalID != proposal.ID || confirmation.Candidate.TotalAmountMinor == nil || *confirmation.Candidate.TotalAmountMinor != 11300 {
		t.Fatalf("confirmation changed after discard: %#v", confirmation)
	}
}
