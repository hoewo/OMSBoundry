//go:build integration

package database

import (
	"context"
	"testing"

	"omsbizfoundry/project-workspace/internal/projects"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

func TestInitialDispatchMigration9To10PreservesLegacyFactsAndActivationConstraints(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	versions, err := migrationVersions()
	if err != nil || len(versions) < 10 {
		t.Fatalf("versions=%v error=%v", versions, err)
	}
	if _, err := environment.Pool.Exec(ctx, `CREATE TABLE schema_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	for _, version := range versions[:9] {
		script, _ := migrationFiles.ReadFile("migrations/" + version)
		if _, err := environment.Pool.Exec(ctx, string(script)); err != nil {
			t.Fatal(err)
		}
		if _, err := environment.Pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			t.Fatal(err)
		}
	}
	store, _ := projects.NewPostgresStore(environment.Pool)
	actor := projects.Actor{TenantID: "11111111-1111-4111-8111-111111111111", ID: "11111111-1111-4111-8111-111111111112"}
	legacy, err := store.Create(ctx, actor, projects.CreateInput{CommandID: "a0000000-0000-4000-8000-000000000001", ProjectNumber: "LEGACY-9", Name: "原项目", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	facts := func() string {
		t.Helper()
		var value string
		if err := environment.Pool.QueryRow(ctx, `SELECT md5(coalesce((SELECT string_agg(to_jsonb(p)::text, E'\n' ORDER BY id) FROM projects p), '') || coalesce((SELECT string_agg(to_jsonb(c)::text, E'\n' ORDER BY command_id) FROM project_commands c), '') || coalesce((SELECT string_agg(to_jsonb(h)::text, E'\n' ORDER BY id) FROM project_history_events h), ''))`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	before := facts()
	for attempt := 0; attempt < 2; attempt++ {
		if err := Migrate(ctx, environment.Pool); err != nil || facts() != before {
			t.Fatalf("migration changed legacy facts: error=%v", err)
		}
	}
	if current, err := RequireCurrentSchema(ctx, environment.Pool); err != nil || current != versions[len(versions)-1] {
		t.Fatalf("current=%q error=%v", current, err)
	}
	if _, err := environment.Pool.Exec(ctx, `UPDATE projects SET project_number='',project_number_normalized='',status='active' WHERE id=$1`, legacy.ID); err == nil {
		t.Fatal("unnumbered ordinary project activated by database")
	}
	if _, err := environment.Pool.Exec(ctx, `UPDATE projects SET project_type_keys=ARRAY[]::text[],status='active' WHERE id=$1`, legacy.ID); err == nil {
		t.Fatal("untyped project activated by database")
	}
	if facts() != before {
		t.Fatal("rejected activation changed original facts")
	}
}
