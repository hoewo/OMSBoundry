//go:build integration

package database

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"omsbizfoundry/project-workspace/internal/projects"
	"omsbizfoundry/project-workspace/internal/testdatabase"
	"testing"
)

func TestRecordedMaterialMigration8To9KeepsFactsAndAllThreeLinks(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	versions, err := migrationVersions()
	if err != nil || len(versions) < 9 {
		t.Fatalf("versions=%v err=%v", versions, err)
	}
	if _, err := environment.Pool.Exec(ctx, `CREATE TABLE schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	for _, version := range versions[:8] {
		script, err := migrationFiles.ReadFile("migrations/" + version)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := environment.Pool.Exec(ctx, string(script)); err != nil {
			t.Fatal(err)
		}
		if _, err := environment.Pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			t.Fatal(err)
		}
	}
	store, _ := projects.NewPostgresStore(environment.Pool)
	actor := projects.Actor{ID: "11111111-1111-4111-8111-111111111112", TenantID: "11111111-1111-4111-8111-111111111111"}
	cmd := func(n int) string { return fmt.Sprintf("92000000-0000-4000-8000-%012d", n) }
	project, err := store.Create(ctx, actor, projects.CreateInput{CommandID: cmd(1), ProjectNumber: "SCHEMA-8-LEGACY", Name: "升级前项目", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := store.ApplyResourceLink(ctx, actor, project.ID, projects.ResourceLinkCommand{CommandID: cmd(2), ExpectedRevision: 1, Operation: "link", TargetService: "production-operations", ResourceType: "production-work-item", ResourceID: actor.ID, RelationKey: "production-materials"})
	if err != nil {
		t.Fatal(err)
	}
	facts := func() map[string]string {
		t.Helper()
		values := map[string]string{}
		for _, table := range []string{"projects", "project_assignments", "project_resource_links", "project_commands", "project_history_events", "project_package_conversions", "project_completion_confirmations"} {
			var value string
			if err := environment.Pool.QueryRow(ctx, `SELECT count(*)::text||':'||md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM `+pgx.Identifier{"public", table}.Sanitize()+` t`).Scan(&value); err != nil {
				t.Fatal(err)
			}
			values[table] = value
		}
		return values
	}
	before := facts()
	for i := 0; i < 2; i++ {
		var applied bool
		if err := environment.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, versions[8]).Scan(&applied); err != nil {
			t.Fatal(err)
		}
		if !applied {
			script, err := migrationFiles.ReadFile("migrations/" + versions[8])
			if err != nil {
				t.Fatal(err)
			}
			tx, err := environment.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, string(script)); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, versions[8]); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		}
		for table, value := range facts() {
			if before[table] != value {
				t.Fatalf("migration changed %s", table)
			}
		}
	}
	var current string
	if err := environment.Pool.QueryRow(ctx, `SELECT max(version) FROM schema_migrations`).Scan(&current); err != nil || current != versions[8] {
		t.Fatalf("current=%q err=%v", current, err)
	}
	details, err := store.Get(ctx, actor.TenantID, project.ID, true)
	if err != nil || len(details.ResourceLinks) != 1 || details.ResourceLinks[0].ID != legacy.ResourceLink.ID || details.Project.Revision != 2 {
		t.Fatal("legacy Production identity changed")
	}
	for i, typ := range []struct{ service, kind string }{{"approval-audit-results", "approval_audit_result"}, {"cost-outsourcing", "cost_entry"}, {"cost-outsourcing", "outsourcing_engagement"}} {
		if _, err := store.ApplyResourceLink(ctx, actor, project.ID, projects.ResourceLinkCommand{CommandID: cmd(i + 3), ExpectedRevision: int64(i + 2), Operation: "link", TargetService: typ.service, ResourceType: typ.kind, ResourceID: actor.ID, RelationKey: "source-materials"}); err != nil {
			t.Fatalf("canonical %s: %v", typ.kind, err)
		}
	}
	t.Log("schema8_to9_legacy_owned_facts_unchanged=true idempotent=true three_canonical_links_appended=true")
}
