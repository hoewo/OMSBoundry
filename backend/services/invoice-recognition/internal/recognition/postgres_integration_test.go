//go:build integration

package recognition

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	contract "omsbizfoundry/invoice-recognition-contract"
	"omsbizfoundry/invoice-recognition/internal/database"
)

type integrationEvidence struct{ content []byte }

func (e integrationEvidence) Open(context.Context, string, contract.EvidenceReference) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(e.content)), nil
}

type integrationProvider struct{}

func (integrationProvider) Recognize(context.Context, Document) (ProviderResult, error) {
	before, tax, total := int64(10000), int64(600), int64(10600)
	return ProviderResult{ProviderKey: "integration", ProviderModel: "fixed", ProviderRevision: "v1", Candidate: contract.Candidate{InvoiceNumber: "INV-001", AmountExcludingTaxMinor: &before, TaxAmountMinor: &tax, TotalAmountMinor: &total, Currency: "CNY"}}, nil
}

func TestPostgresProposalConfirmationReplayAndImmutableHistory(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := invoiceIntegrationPool(t, ctx)
	defer cleanup()
	repository, _ := NewPostgresRepository(pool)
	content := []byte("trusted invoice bytes")
	digest := sha256.Sum256(content)
	now := time.Date(2026, 11, 6, 1, 2, 3, 0, time.UTC)
	proposalIDs := []string{"44444444-4444-4444-8444-444444444444", "66666666-6666-4666-8666-666666666666"}
	service := New(integrationEvidence{content: content}, integrationProvider{}, repository, func() string {
		id := proposalIDs[0]
		proposalIDs = proposalIDs[1:]
		return id
	}, func() time.Time { return now })
	actor := Actor{TenantID: "11111111-1111-4111-8111-111111111111", SubjectID: "22222222-2222-4222-8222-222222222222", DisplayName: "管理员", CredentialID: "33333333-3333-4333-8333-333333333333"}
	evidence := contract.EvidenceReference{FileID: "55555555-5555-4555-8555-555555555555", Version: 1, ContentSHA256: "sha256:" + hex.EncodeToString(digest[:]), MIMEType: "application/xml", SizeBytes: int64(len(content))}
	start := StartInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1", Evidence: evidence}
	proposal, err := service.Start(ctx, actor, start)
	if err != nil || proposal.Revision != 1 || proposal.Status != "needs_review" {
		t.Fatalf("proposal=%+v err=%v", proposal, err)
	}
	if replay, replayErr := service.Start(ctx, actor, start); replayErr != nil || replay.ID != proposal.ID {
		t.Fatalf("replay=%+v err=%v", replay, replayErr)
	}
	conflict := start
	conflict.Evidence.Version = 2
	if _, conflictErr := service.Start(ctx, actor, conflict); !errors.Is(conflictErr, ErrIdempotencyConflict) {
		t.Fatalf("expected command conflict, got %v", conflictErr)
	}
	corrected := proposal.Candidate
	corrected.InvoiceNumber = "INV-001-CORRECTED"
	confirmationInput := ConfirmInput{CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1", ProposalID: proposal.ID, ExpectedRevision: proposal.Revision, Candidate: corrected}
	confirmation, err := service.Confirm(ctx, actor, confirmationInput)
	if err != nil || confirmation.Candidate.InvoiceNumber != corrected.InvoiceNumber || len(confirmation.DuplicateKeys) != 2 {
		t.Fatalf("confirmation=%+v err=%v", confirmation, err)
	}
	confirmed, err := service.Get(ctx, actor.TenantID, proposal.ID)
	if err != nil || confirmed.Status != "confirmed" || confirmed.Revision != 2 || confirmed.Candidate.InvoiceNumber != corrected.InvoiceNumber {
		t.Fatalf("confirmed=%+v err=%v", confirmed, err)
	}
	if replay, replayErr := service.Confirm(ctx, actor, confirmationInput); replayErr != nil || replay.ProposalID != proposal.ID {
		t.Fatalf("confirmation replay=%+v err=%v", replay, replayErr)
	}
	if _, err = pool.Exec(ctx, `UPDATE invoice_recognition_proposal_revisions SET snapshot='{}' WHERE tenant_id=$1 AND proposal_id=$2 AND revision=1`, actor.TenantID, proposal.ID); err == nil {
		t.Fatal("proposal revision must be immutable")
	}

	discardEvidence := evidence
	discardEvidence.FileID = "77777777-7777-4777-8777-777777777777"
	discardedProposal, err := service.Start(ctx, actor, StartInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2", Evidence: discardEvidence})
	if err != nil {
		t.Fatal(err)
	}
	discardInput := DiscardInput{CommandID: "cccccccc-cccc-4ccc-8ccc-ccccccccccc1", ProposalID: discardedProposal.ID, ExpectedRevision: discardedProposal.Revision, Reason: "重复上传"}
	discarded, err := service.Discard(ctx, actor, discardInput)
	if err != nil || discarded.Status != "discarded" || discarded.DiscardReason != "重复上传" || discarded.DiscardedAt == nil || discarded.Revision != 2 {
		t.Fatalf("discarded=%+v err=%v", discarded, err)
	}
	if replay, replayErr := service.Discard(ctx, actor, discardInput); replayErr != nil || replay.ID != discarded.ID || replay.Revision != 2 {
		t.Fatalf("discard replay=%+v err=%v", replay, replayErr)
	}
	if _, confirmErr := service.Confirm(ctx, actor, ConfirmInput{CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb2", ProposalID: discarded.ID, ExpectedRevision: discarded.Revision, Candidate: discarded.Candidate}); !errors.Is(confirmErr, ErrConflict) {
		t.Fatalf("discarded proposal must not be confirmed, got %v", confirmErr)
	}
}

func invoiceIntegrationPool(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	_, _ = rand.Read(random[:])
	schema := "invoice_recognition_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	config, _ := pgxpool.ParseConfig(raw)
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool, func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	}
}
