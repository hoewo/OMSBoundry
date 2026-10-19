//go:build integration

package design

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/design-operations/internal/database"
	"omsbizfoundry/design-operations/internal/testdatabase"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func stageFacts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	tables := []string{"schema_migrations", "design_work_items", "design_stages", "design_deliverables", "design_commands", "design_history_events", "production_stage_materials", "production_stage_material_endings", "production_stage_scores"}
	out := ""
	for _, table := range tables {
		var value string
		if err := pool.QueryRow(ctx, `SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),'[]'::jsonb)::text FROM `+table+` r`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		out += table + ":" + value + "\n"
	}
	return out
}
func TestStageMaterialPG18EmptyTargetRestoreAndContinuation(t *testing.T) {
	for _, bin := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skip(bin + " not installed")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, _, refs, actor, id, source := materialDBFixture(t)
	score := "0.12345678901234567890123456789"
	in := materialInput(370, 1, 201, &score)
	in.MainFiles = append(in.MainFiles, stageFile(202))
	in.ReviewSheets = []StageFileInput{stageFile(203)}
	saved, err := s.RegisterStageMaterials(ctx, actor, id, "scheme", in)
	if err != nil {
		t.Fatal(err)
	}
	before := stageFacts(t, source.Pool)
	dump := filepath.Join(t.TempDir(), "production.dump")
	if data, err := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-acl", "--file="+dump, source.URL).CombinedOutput(); err != nil {
		t.Fatalf("isolated pg_dump failed: %v %s", err, data)
	}
	target := testdatabase.New(t, testdatabase.RequireURL(t))
	var count int
	if err := target.Pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("target not empty", err)
	}
	if data, err := exec.CommandContext(ctx, "pg_restore", "--exit-on-error", "--no-owner", "--no-acl", "--dbname="+target.URL, dump).CombinedOutput(); err != nil {
		t.Fatalf("isolated pg_restore failed: %v %s", err, data)
	}
	if err := database.Migrate(ctx, target.Pool); err != nil {
		t.Fatal(err)
	}
	if after := stageFacts(t, target.Pool); after != before {
		t.Fatal("IDs/revisions/references/scores/history/receipts changed on restore")
	}
	store, _ := NewPostgresStore(target.Pool)
	restored, _ := NewServiceWithStageMaterials(store, refs, StageMaterialProfile)
	refs.unavailable = true
	replay, err := restored.RegisterStageMaterials(ctx, actor, id, "scheme", in)
	if err != nil || replay.MainFiles[0].ID != saved.MainFiles[0].ID || replay.WorkItem.Revision != saved.WorkItem.Revision {
		t.Fatal("restored original command", err)
	}
	if stageFacts(t, target.Pool) != before {
		t.Fatal("replay wrote facts")
	}
	refs.unavailable = false
	continued, err := restored.WithdrawStageMaterial(ctx, actor, id, "scheme", saved.MainFiles[0].ID, ReasonInput{RevisionInput: RevisionInput{CommandID: command(371), ExpectedRevision: 2}, Reason: "restore continuation"})
	if err != nil || continued.WorkItem.ID != id || continued.WorkItem.Revision != 3 || len(continued.MainFiles) != 1 || *continued.Score != score {
		t.Fatal("same-identity continuation", err)
	}
	if stageFacts(t, source.Pool) != before {
		t.Fatal("source changed by restore")
	}
	t.Log("PG18 empty target: nine-table facts identical; original receipt zero writes; same ID/revision continuation; source unchanged")
}
