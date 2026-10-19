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

func TestReferenceOAProductionMigration11To12PreservesLegacyAndDoesNotSelectOverlay(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	versions, err := migrationVersions()
	if err != nil || len(versions) < 12 {
		t.Fatalf("versions=%v %v", versions, err)
	}
	if _, err := env.Pool.Exec(ctx, `CREATE TABLE schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	for _, v := range versions[:11] {
		raw, err := migrationFiles.ReadFile("migrations/" + v)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.Pool.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
		if _, err := env.Pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, v); err != nil {
			t.Fatal(err)
		}
	}
	store, _ := projects.NewPostgresStore(env.Pool)
	actor := projects.Actor{ID: "33333333-3333-4333-8333-333333333333", TenantID: "11111111-1111-4111-8111-111111111111"}
	p, err := store.Create(ctx, actor, projects.CreateInput{CommandID: "a0000000-0000-4000-8000-000000000901", ProjectNumber: "LEGACY-11", Name: "旧资料", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	for i, role := range []string{projects.RoleBusinessOwner, projects.RoleProductionOwner, projects.RoleParticipant, projects.RoleCommercialContributor} {
		result, err := store.ApplyAssignment(ctx, actor, p.ID, projects.AssignmentCommand{CommandID: []string{"a0000000-0000-4000-8000-000000000902", "a0000000-0000-4000-8000-000000000903", "a0000000-0000-4000-8000-000000000904", "a0000000-0000-4000-8000-000000000905"}[i], ExpectedRevision: p.Revision, Operation: "assign", SubjectKind: "person", SubjectID: actor.ID, SourceRevision: 1, DisplaySnapshot: "原人员", RoleKey: role})
		if err != nil {
			t.Fatal(err)
		}
		p = result.Project
	}
	facts := func() map[string]string {
		result := map[string]string{}
		// Counterparty snapshots are already owned by immutable history, not
		// a fictional project_counterparty_context table.
		for _, table := range []string{"projects", "project_assignments", "project_resource_links", "project_commands", "project_history_events", "project_package_conversions", "project_completion_confirmations"} {
			var v string
			if err := env.Pool.QueryRow(ctx, `SELECT count(*)::text||':'||md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM `+pgx.Identifier{table}.Sanitize()+` t`).Scan(&v); err != nil {
				t.Fatal(err)
			}
			result[table] = v
		}
		return result
	}
	before := facts()
	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, env.Pool); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, facts()) {
			t.Fatal("migration changed legacy facts, identities, revisions, snapshots, audit or receipts")
		}
	}
	var n int
	if err := env.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM reference_oa_production_profiles)+(SELECT count(*) FROM reference_oa_production_assignments)+(SELECT count(*) FROM reference_oa_production_events)+(SELECT count(*) FROM reference_oa_production_discipline_heads)+(SELECT count(*) FROM reference_oa_production_discipline_versions)`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("auto-created facts=%d %v", n, err)
	}
	if v, err := RequireCurrentSchema(ctx, env.Pool); err != nil || v != versions[len(versions)-1] {
		t.Fatal(v, err)
	}
	t.Log("migration11_to12_idempotent=true four_old_roles_ids_revisions_snapshots_receipts_history_unchanged=true profiles_and_customer_facts_zero=true no_automatic_grants=true")
}
