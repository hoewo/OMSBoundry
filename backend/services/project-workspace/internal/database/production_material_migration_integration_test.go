//go:build integration

package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"omsbizfoundry/project-workspace/internal/projects"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

func TestProductionMaterialMigration7To8KeepsLegacyFactsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	versions, err := migrationVersions()
	if err != nil || len(versions) < 8 {
		t.Fatalf("versions=%v err=%v", versions, err)
	}
	if _, err := environment.Pool.Exec(ctx, `CREATE TABLE schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	for _, version := range versions[:7] {
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
	command := func(n int) string { return fmt.Sprintf("91000000-0000-4000-8000-%012d", n) }
	project, err := store.Create(ctx, actor, projects.CreateInput{CommandID: command(1), ProjectNumber: "SCHEMA-7-LEGACY", Name: "升级前项目", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	linked, err := store.ApplyResourceLink(ctx, actor, project.ID, projects.ResourceLinkCommand{CommandID: command(2), ExpectedRevision: 1, Operation: "link", TargetService: "design-operations", ResourceType: "design-work-item", ResourceID: actor.ID, RelationKey: "legacy-design"})
	if err != nil {
		t.Fatal(err)
	}
	facts := func() map[string]string {
		t.Helper()
		result := map[string]string{}
		for _, table := range []string{"projects", "project_assignments", "project_resource_links", "project_commands", "project_history_events", "project_package_conversions", "project_completion_confirmations"} {
			var canonical string
			query := `SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM ` + pgx.Identifier{"public", table}.Sanitize() + ` t`
			if err := environment.Pool.QueryRow(ctx, query).Scan(&canonical); err != nil {
				t.Fatal(err)
			}
			result[table] = canonical
		}
		return result
	}
	before := facts()
	for attempt := 0; attempt < 2; attempt++ {
		// Keep this historical regression specifically 7→8 even after later
		// forward migrations exist. The new current-schema path has its own test.
		var exists bool
		if err := environment.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, versions[7]).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			script, err := migrationFiles.ReadFile("migrations/" + versions[7])
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
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, versions[7]); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		}
		for table, digest := range facts() {
			if before[table] != digest {
				t.Fatalf("migration changed %s", table)
			}
		}
	}
	var current string
	if err := environment.Pool.QueryRow(ctx, `SELECT max(version) FROM schema_migrations`).Scan(&current); err != nil || current != versions[7] {
		t.Fatalf("current=%q err=%v", current, err)
	}
	details, err := store.Get(ctx, actor.TenantID, project.ID, true)
	if err != nil || len(details.ResourceLinks) != 1 || details.ResourceLinks[0].ID != linked.ResourceLink.ID || details.ResourceLinks[0].TargetService != "design-operations" || details.ResourceLinks[0].ResourceType != "design-work-item" {
		t.Fatal("legacy source was rewritten or guessed")
	}
	if _, err := store.ApplyResourceLink(ctx, actor, project.ID, projects.ResourceLinkCommand{CommandID: command(3), ExpectedRevision: 2, Operation: "link", TargetService: "production-operations", ResourceType: "production-work-item", ResourceID: actor.ID, RelationKey: "production-materials"}); err != nil {
		t.Fatalf("schema 8 rejects canonical link: %v", err)
	}
	t.Log("schema7_to_8_legacy_owned_facts_unchanged=true idempotent=true canonical_link_appended=true")
}
