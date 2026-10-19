//go:build integration

package database

import (
	"context"
	"github.com/jackc/pgx/v5"
	"omsbizfoundry/project-workspace/internal/projects"
	"omsbizfoundry/project-workspace/internal/testdatabase"
	"reflect"
	"testing"
)

func TestCommercialContributorMigration10To11PreservesAllLegacyFacts(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	versions, err := migrationVersions()
	if err != nil || len(versions) < 11 {
		t.Fatalf("versions=%v err=%v", versions, err)
	}
	if _, err := env.Pool.Exec(ctx, `CREATE TABLE schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	for _, v := range versions[:10] {
		script, err := migrationFiles.ReadFile("migrations/" + v)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.Pool.Exec(ctx, string(script)); err != nil {
			t.Fatal(err)
		}
		if _, err := env.Pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, v); err != nil {
			t.Fatal(err)
		}
	}
	actor := projects.Actor{TenantID: "11111111-1111-4111-8111-111111111111", ID: "22222222-2222-4222-8222-222222222222"}
	store, _ := projects.NewPostgresStore(env.Pool)
	p, err := store.Create(ctx, actor, projects.CreateInput{CommandID: "a0000000-0000-4000-8000-000000000701", ProjectNumber: "LEGACY-10-COMMERCIAL", Name: "旧事实", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	for i, role := range []string{projects.RoleBusinessOwner, projects.RoleProductionOwner, projects.RoleParticipant} {
		id := []string{"a0000000-0000-4000-8000-000000000702", "a0000000-0000-4000-8000-000000000703", "a0000000-0000-4000-8000-000000000704"}[i]
		result, err := store.ApplyAssignment(ctx, actor, p.ID, projects.AssignmentCommand{CommandID: id, ExpectedRevision: p.Revision, Operation: "assign", SubjectKind: "person", SubjectID: actor.ID, SourceRevision: 1, DisplaySnapshot: "旧人员", RoleKey: role})
		if err != nil {
			t.Fatal(err)
		}
		p = result.Project
	}
	facts := func() map[string]string {
		t.Helper()
		values := map[string]string{}
		for _, table := range []string{"projects", "project_assignments", "project_resource_links", "project_commands", "project_history_events", "project_package_conversions", "project_completion_confirmations", "project_counterparty_context"} {
			var exists bool
			if err := env.Pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if !exists {
				continue
			}
			var v string
			query := `SELECT count(*)::text||':'||md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM ` + pgx.Identifier{table}.Sanitize() + ` t`
			if err := env.Pool.QueryRow(ctx, query).Scan(&v); err != nil {
				t.Fatal(err)
			}
			values[table] = v
		}
		return values
	}
	before := facts()
	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, env.Pool); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, facts()) {
			t.Fatal("legacy row, revision, command or history changed")
		}
	}
	if v, err := RequireCurrentSchema(ctx, env.Pool); err != nil || v != versions[len(versions)-1] {
		t.Fatalf("schema=%s err=%v", v, err)
	}
	next, err := store.ApplyAssignment(ctx, actor, p.ID, projects.AssignmentCommand{CommandID: "a0000000-0000-4000-8000-000000000705", ExpectedRevision: p.Revision, Operation: "assign", SubjectKind: "person", SubjectID: actor.ID, SourceRevision: 1, DisplaySnapshot: "商业参与", RoleKey: projects.RoleCommercialContributor})
	if err != nil || next.Project.Revision != p.Revision+1 {
		t.Fatalf("new role=%+v err=%v", next, err)
	}
	t.Log("schema10_to11_idempotent=true all_old_role_rows_commands_history_unchanged=true new_person_role_append=true")
}
