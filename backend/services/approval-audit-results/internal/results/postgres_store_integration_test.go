//go:build integration

package results

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/approval-audit-results/internal/database"
)

func TestPostgresExactSourceLifecycleHistoryIsolationAndImmutability(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor, project, resolver := integrationFixture()
	service, _ := NewService(store, resolver)

	approvalInput := approvalRecordInput(project, id(5))
	approval, err := service.Record(ctx, actor, approvalInput)
	if err != nil {
		t.Fatal(err)
	}
	if approval.Revision != 1 || approval.State != StateRecorded || approval.TotalAmountMinor != 125 {
		t.Fatalf("unexpected approval: %#v", approval)
	}
	replayed, err := service.Record(ctx, actor, approvalInput)
	if err != nil || replayed.ID != approval.ID || replayed.Revision != 1 {
		t.Fatalf("bad replay: %#v %v", replayed, err)
	}
	conflict := approvalInput
	conflict.ExternalConfirmedBy = "different payload"
	if _, err = service.Record(ctx, actor, conflict); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("command conflict expected, got %v", err)
	}

	auditInput := RecordInput{
		CommandID: id(6), ProjectRef: ProjectRef{ID: project.ID, Revision: project.Revision}, Kind: KindAudit, Currency: "CNY",
		SourceRef:           SourceRef{Application: "approval-audit-results", ResourceType: "approval-result", ResourceID: approval.ID, Revision: 1},
		ExternalConfirmedBy: "审计结果单 A-1", ExternalConfirmedAt: time.Date(2026, 9, 28, 11, 0, 0, 0, time.UTC),
	}
	audit, err := service.Record(ctx, actor, auditInput)
	if err != nil {
		t.Fatal(err)
	}
	if audit.TotalAmountMinor != 125 || audit.Source.ResourceID != approval.ID || audit.Source.Revision != 1 {
		t.Fatalf("audit lost exact approval source: %#v", audit)
	}

	correctApproval := CorrectInput{
		CommandID: id(7), ExpectedRevision: 1, Reason: "书面审批更正", Currency: "CNY", AmountComponents: standardComponents(150, 20, 5),
		SourceRef: approvalInput.SourceRef, OverrideReason: "审批单明确调增设计费", ExternalConfirmedBy: "OA 审批更正单", ExternalConfirmedAt: time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
	}
	approval2, err := service.Correct(ctx, actor, approval.ID, correctApproval)
	if err != nil {
		t.Fatal(err)
	}
	if approval2.Revision != 2 || approval2.TotalAmountMinor != 175 {
		t.Fatalf("unexpected approval correction: %#v", approval2)
	}
	unchangedAudit, err := service.Get(ctx, actor.TenantID, audit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchangedAudit.TotalAmountMinor != 125 || unchangedAudit.Source.Revision != 1 {
		t.Fatalf("audit was coupled to later approval: %#v", unchangedAudit)
	}

	correctAudit := CorrectInput{
		CommandID: id(8), ExpectedRevision: 1, Reason: "补录审计编号", Currency: "CNY",
		SourceRef: auditInput.SourceRef, ExternalConfirmedBy: "审计结果单 A-1（补录编号）", ExternalConfirmedAt: auditInput.ExternalConfirmedAt,
	}
	audit2, err := service.Correct(ctx, actor, audit.ID, correctAudit)
	if err != nil {
		t.Fatal(err)
	}
	if audit2.Revision != 2 || audit2.TotalAmountMinor != 125 {
		t.Fatalf("unexpected audit correction: %#v", audit2)
	}
	stale := correctAudit
	stale.CommandID = id(9)
	stale.ExternalConfirmedBy = "过期写入"
	if _, err = service.Correct(ctx, actor, audit.ID, stale); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale correction expected, got %v", err)
	}

	withdrawn, err := service.Withdraw(ctx, actor, audit.ID, WithdrawInput{CommandID: id(10), ExpectedRevision: 2, Reason: "审计机构撤回结果"})
	if err != nil {
		t.Fatal(err)
	}
	if withdrawn.State != StateWithdrawn || withdrawn.Revision != 3 {
		t.Fatalf("unexpected withdrawal: %#v", withdrawn)
	}
	auditInput.CommandID = id(11)
	auditInput.ExpectedRevision = 3
	auditInput.ExternalConfirmedBy = "重新出具的审计结果单 A-2"
	reregistered, err := service.Record(ctx, actor, auditInput)
	if err != nil {
		t.Fatal(err)
	}
	if reregistered.ID != audit.ID || reregistered.Revision != 4 || reregistered.State != StateRecorded {
		t.Fatalf("unexpected re-registration: %#v", reregistered)
	}

	history, err := service.History(ctx, actor.TenantID, audit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 4 || history[0].EventKind != EventRecorded || history[1].EventKind != EventCorrected || history[2].EventKind != EventWithdrawn || history[3].EventKind != EventReregister {
		t.Fatalf("unexpected append-only history: %#v", history)
	}
	if history[0].Snapshot.TotalAmountMinor != 125 || history[0].Snapshot.Source.Revision != 1 {
		t.Fatalf("original audit revision was not preserved: %#v", history[0])
	}
	if _, err = service.Get(ctx, id(12), audit.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant read must look not found: %v", err)
	}

	if _, err = pool.Exec(ctx, `UPDATE approval_audit_result_revisions SET state='withdrawn' WHERE tenant_id=$1 AND result_id=$2 AND revision=1`, actor.TenantID, audit.ID); err == nil {
		t.Fatal("revision update must be rejected")
	}
	if _, err = pool.Exec(ctx, `UPDATE approval_audit_results SET total_amount_minor=1 WHERE tenant_id=$1 AND id=$2`, actor.TenantID, audit.ID); err == nil {
		t.Fatal("current projection without matching revision must be rejected")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM approval_audit_results WHERE tenant_id=$1 AND id=$2`, actor.TenantID, audit.ID); err == nil {
		t.Fatal("physical delete must be rejected")
	}

	var current, revisions, commands, audits int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM approval_audit_results),(SELECT count(*) FROM approval_audit_result_revisions),(SELECT count(*) FROM approval_audit_result_commands),(SELECT count(*) FROM approval_audit_result_audit_events)`).Scan(&current, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if current != 2 || revisions != 6 || commands != 6 || audits != 6 {
		t.Fatalf("failed writes changed facts: %d/%d/%d/%d", current, revisions, commands, audits)
	}
}

func TestPostgresDependencyFailureLeavesNoFacts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor, project, resolver := integrationFixture()
	resolver.failEvidence = ErrReferenceUnavailable
	service, _ := NewService(store, resolver)
	input := approvalRecordInput(project, id(5))
	input.EvidenceRefs = []EvidenceRef{{FileID: id(13), Version: 1, Slot: "approval_document"}}
	if _, err := service.Record(ctx, actor, input); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatalf("dependency failure expected, got %v", err)
	}
	var current, revisions, commands, audits int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM approval_audit_results),(SELECT count(*) FROM approval_audit_result_revisions),(SELECT count(*) FROM approval_audit_result_commands),(SELECT count(*) FROM approval_audit_result_audit_events)`).Scan(&current, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if current != 0 || revisions != 0 || commands != 0 || audits != 0 {
		t.Fatalf("dependency failure created partial facts: %d/%d/%d/%d", current, revisions, commands, audits)
	}
}

func TestConcurrentRecordReplayProducesOneResult(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor, project, resolver := integrationFixture()
	service, _ := NewService(store, resolver)
	input := approvalRecordInput(project, id(5))
	const workers = 8
	ids := make(chan string, workers)
	errs := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			value, err := service.Record(ctx, actor, input)
			if err != nil {
				errs <- err
				return
			}
			ids <- value.ID
		}()
	}
	group.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	first, count := "", 0
	for value := range ids {
		count++
		if first == "" {
			first = value
		} else if value != first {
			t.Fatalf("different result ids: %s %s", first, value)
		}
	}
	if count != workers {
		t.Fatalf("got %d results, want %d", count, workers)
	}
	var current, revisions, commands, audits int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM approval_audit_results),(SELECT count(*) FROM approval_audit_result_revisions),(SELECT count(*) FROM approval_audit_result_commands),(SELECT count(*) FROM approval_audit_result_audit_events)`).Scan(&current, &revisions, &commands, &audits); err != nil {
		t.Fatal(err)
	}
	if current != 1 || revisions != 1 || commands != 1 || audits != 1 {
		t.Fatalf("expected one committed fact, got %d/%d/%d/%d", current, revisions, commands, audits)
	}
}

type integrationResolver struct {
	project      ProjectReference
	source       SourceReference
	evidence     map[string]EvidenceReference
	failEvidence error
}

func (r *integrationResolver) ResolveProject(context.Context, string, ProjectRef) (ProjectReference, error) {
	return r.project, nil
}
func (r *integrationResolver) ResolveSource(context.Context, string, ProjectRef, SourceRef) (SourceReference, error) {
	return r.source, nil
}
func (r *integrationResolver) ResolveEvidence(_ context.Context, _ string, ref EvidenceRef) (EvidenceReference, error) {
	if r.failEvidence != nil {
		return EvidenceReference{}, r.failEvidence
	}
	return r.evidence[ref.FileID], nil
}

func integrationFixture() (Actor, ProjectSnapshot, *integrationResolver) {
	actor := Actor{ID: id(1), TenantID: id(2), Kind: "user", CredentialID: "cred-1"}
	project := ProjectSnapshot{ID: id(3), Revision: 7, Code: "P-001", DisplayName: "道路设计项目"}
	components, _, _ := NormalizeComponents(standardComponents(100, 20, 5), false)
	ref := SourceRef{Application: "contract-ledger", ResourceType: "settlement", ResourceID: id(4), Revision: 9}
	resolver := &integrationResolver{
		project:  ProjectReference{ProjectSnapshot: project, TenantID: actor.TenantID, Status: "active"},
		source:   SourceReference{SourceSnapshot: SourceSnapshot{SourceRef: ref, State: "confirmed", ProjectID: project.ID, Currency: "CNY", AmountProfile: AmountProfileReferenceOAV1, AmountComponents: components, TotalAmountMinor: 125, Digest: "sha256:contract-rev-9", DisplaySummary: "结算 revision 9"}, TenantID: actor.TenantID, Eligible: true},
		evidence: map[string]EvidenceReference{id(13): {EvidenceSnapshot: EvidenceSnapshot{EvidenceRef: EvidenceRef{FileID: id(13), Version: 1, Slot: "approval_document"}, Digest: "sha256:file-v1", DisplayName: "approval.pdf"}, TenantID: actor.TenantID, Status: "available"}},
	}
	return actor, project, resolver
}

func approvalRecordInput(project ProjectSnapshot, commandID string) RecordInput {
	return RecordInput{CommandID: commandID, ProjectRef: ProjectRef{ID: project.ID, Revision: project.Revision}, Kind: KindApproval, Currency: "CNY", SourceRef: SourceRef{Application: "contract-ledger", ResourceType: "settlement", ResourceID: id(4), Revision: 9}, ExternalConfirmedBy: "OA 审批记录", ExternalConfirmedAt: time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)}
}

func integrationPool(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "approval_audit_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	}
	return pool, cleanup
}
