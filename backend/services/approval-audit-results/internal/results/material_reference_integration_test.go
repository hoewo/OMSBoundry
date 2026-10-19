//go:build integration

package results

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// An accidental read-time call to an upstream resolver panics. Seed-time
// references are synthetic, while persistence and the read path are real.
type materialNoUpstream struct{ ReferenceResolver }

func materialOwnedFacts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	facts := make([][2]string, 0, 5)
	for _, table := range []string{"approval_audit_results", "approval_audit_result_revisions", "approval_audit_result_commands", "approval_audit_result_audit_events", "schema_migrations"} {
		var value string
		query := `SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM ` + pgx.Identifier{table}.Sanitize() + ` t`
		if err := pool.QueryRow(context.Background(), query).Scan(&value); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, [2]string{table, value})
	}
	raw, _ := json.Marshal(facts)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func TestResultsMaterialPostgresCurrentCorrectionWithdrawalAndZeroWrites(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor, project, resolver := integrationFixture()
	writer, _ := NewService(store, resolver)
	reader, _ := NewService(store, &materialNoUpstream{})
	input := approvalRecordInput(project, id(5))
	input.EvidenceRefs = []EvidenceRef{{FileID: id(13), Version: 1, Slot: "approval_document"}}
	created, err := writer.Record(ctx, actor, input)
	if err != nil {
		t.Fatal(err)
	}
	read := func(revision, version int64, state string) MaterialReferenceProjection {
		t.Helper()
		before := materialOwnedFacts(t, pool)
		value, err := reader.Get(ctx, actor.TenantID, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		projection, err := MaterialReference(value)
		if err != nil || projection.Revision != revision || projection.Status != state || len(projection.Items) != 1 || projection.Items[0].Version != version || projection.Items[0].EvidenceSourceRevision != version {
			t.Fatalf("projection=%#v err=%v", projection, err)
		}
		if materialOwnedFacts(t, pool) != before {
			t.Fatal("source read changed owned facts")
		}
		return projection
	}
	first := read(1, 1, StateRecorded)
	resolver.evidence[id(13)] = EvidenceReference{EvidenceSnapshot: EvidenceSnapshot{EvidenceRef: EvidenceRef{FileID: id(13), Version: 2, Slot: "approval_document"}, Digest: "sha256:file-v2", DisplayName: "approval-revised.pdf"}, TenantID: actor.TenantID, Status: "available"}
	correction := CorrectInput{CommandID: id(7), ExpectedRevision: 1, Reason: "替换批复文件", Currency: input.Currency, SourceRef: input.SourceRef, EvidenceRefs: []EvidenceRef{{FileID: id(13), Version: 2, Slot: "approval_document"}}, ExternalConfirmedBy: input.ExternalConfirmedBy, ExternalConfirmedAt: input.ExternalConfirmedAt}
	if _, err := writer.Correct(ctx, actor, created.ID, correction); err != nil {
		t.Fatal(err)
	}
	second := read(2, 2, StateRecorded)
	if first.Digest == second.Digest {
		t.Fatal("replaced file did not change material digest")
	}
	history, err := reader.History(ctx, actor.TenantID, created.ID)
	if err != nil || len(history) != 2 || history[0].Snapshot.Evidence[0].Version != 1 || history[1].Snapshot.Evidence[0].Version != 2 {
		t.Fatalf("history=%#v err=%v", history, err)
	}
	if _, err := writer.Withdraw(ctx, actor, created.ID, WithdrawInput{CommandID: id(10), ExpectedRevision: 2, Reason: "文书撤回"}); err != nil {
		t.Fatal(err)
	}
	withdrawn := read(3, 2, StateWithdrawn)
	if withdrawn.Digest != second.Digest {
		t.Fatal("withdrawal silently changed preserved file references")
	}
	before := materialOwnedFacts(t, pool)
	if _, err := reader.Get(ctx, id(12), created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant err=%v", err)
	}
	if materialOwnedFacts(t, pool) != before {
		t.Fatal("denied read changed facts")
	}
	t.Log("PG current v2/recorded then v2/withdrawn, v1 history preserved, five owned tables unchanged for each read, no upstream resolver calls")
}
