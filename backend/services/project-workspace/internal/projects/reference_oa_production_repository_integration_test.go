//go:build integration

package projects

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"testing"

	"omsbizfoundry/project-workspace/internal/database"
	personnel "omsbizfoundry/project-workspace/internal/referenceoa/productionpersonnel"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

const oaDiscipline = "77777777-7777-4777-8777-777777777777"

func oaFixture(t *testing.T) (testdatabase.Environment, *PostgresStore, Actor, Project) {
	t.Helper()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx := context.Background()
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA, Kind: "test_subject"}
	// Fixed test catalogue source, stored in this owner's database. Native
	// catalogue commands have their own separate lifecycle/concurrency tests.
	if _, err := env.Pool.Exec(ctx, `INSERT INTO reference_oa_production_discipline_versions(tenant_id,id,revision,name,description,state,command_id,actor_subject,recorded_at) VALUES($1,$2,1,'桥梁','','active',$3,$4,clock_timestamp());`, actor.TenantID, oaDiscipline, command(800), actor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Pool.Exec(ctx, `INSERT INTO reference_oa_production_discipline_heads(tenant_id,id,revision,name_normalized,state) VALUES($1,$2,1,'桥梁','active')`, actor.TenantID, oaDiscipline); err != nil {
		t.Fatal(err)
	}
	p, err := store.Create(ctx, actor, CreateInput{CommandID: command(801), ProjectNumber: "OA-PRODUCTION-801", Name: "原生产分工存储", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	base, err := store.ApplyAssignment(ctx, actor, p.ID, AssignmentCommand{CommandID: command(802), ExpectedRevision: 1, Operation: "assign", RoleKey: RoleProductionOwner, SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 1, DisplaySnapshot: "旧生产负责人"})
	if err != nil {
		t.Fatal(err)
	}
	p = base.Project
	if _, err := store.OAProductionPersonnel(ctx, actor.TenantID, p.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("migration auto-enrolled profile: %v", err)
	}
	enrolled, err := store.EnrollOAProduction(ctx, actor, p.ID, RevisionCommand{CommandID: command(803), ExpectedRevision: p.Revision})
	if err != nil || enrolled.ProjectRevision != 3 {
		t.Fatalf("install=%+v %v", enrolled, err)
	}
	p.Revision = enrolled.ProjectRevision
	return env, store, actor, p
}
func oaIntention(n int, revision int64) personnel.Intention {
	return personnel.Intention{Profile: personnel.Profile, CommandID: command(n), ExpectedProjectRevision: revision, Operation: personnel.ReplaceDiscipline, TargetDiscipline: &personnel.Ref{ID: oaDiscipline, Revision: 1}, Changes: []personnel.RoleChange{
		{Role: "designer", Operation: personnel.Replace, People: []personnel.Ref{{ID: intPersonA, Revision: 1}}},
		{Role: "participant", Operation: personnel.Replace, People: []personnel.Ref{{ID: intPersonA, Revision: 1}}},
		{Role: "reviewer", Operation: personnel.Replace, People: []personnel.Ref{{ID: intPersonB, Revision: 1}}},
		{Role: "auditor", Operation: personnel.Replace, People: []personnel.Ref{}},
	}}
}
func oaResolved() personnel.Resolved {
	return personnel.Resolved{People: map[personnel.Ref]personnel.ResolvedPerson{
		{ID: intPersonA, Revision: 1}: {TenantID: intTenantA, State: "active", PersonSnapshot: personnel.PersonSnapshot{Ref: personnel.Ref{ID: intPersonA, Revision: 1}, Name: "设计甲"}},
		{ID: intPersonB, Revision: 1}: {TenantID: intTenantA, State: "active", PersonSnapshot: personnel.PersonSnapshot{Ref: personnel.Ref{ID: intPersonB, Revision: 1}, Name: "复核乙"}},
	}, Discipline: &personnel.ResolvedDiscipline{TenantID: intTenantA, State: "active", DisciplineSnapshot: personnel.DisciplineSnapshot{Ref: personnel.Ref{ID: oaDiscipline, Revision: 1}, Name: "桥梁"}}}
}
func oaDigest(t *testing.T, env testdatabase.Environment) string {
	t.Helper()
	var value string
	if err := env.Pool.QueryRow(context.Background(), `SELECT md5(
 coalesce((SELECT string_agg(to_jsonb(p)::text,E'\n' ORDER BY id) FROM projects p),'')||
 coalesce((SELECT string_agg(to_jsonb(a)::text,E'\n' ORDER BY id) FROM project_assignments a),'')||
 coalesce((SELECT string_agg(to_jsonb(c)::text,E'\n' ORDER BY command_id) FROM project_commands c),'')||
 coalesce((SELECT string_agg(to_jsonb(h)::text,E'\n' ORDER BY id) FROM project_history_events h),'')||
 coalesce((SELECT string_agg(to_jsonb(p)::text,E'\n' ORDER BY project_id) FROM reference_oa_production_profiles p),'')||
 coalesce((SELECT string_agg(to_jsonb(a)::text,E'\n' ORDER BY id) FROM reference_oa_production_assignments a),'')||
 coalesce((SELECT string_agg(to_jsonb(e)::text,E'\n' ORDER BY id) FROM reference_oa_production_events e),'')||
 coalesce((SELECT string_agg(to_jsonb(d)::text,E'\n' ORDER BY id,revision) FROM reference_oa_production_discipline_versions d),'')||
 coalesce((SELECT string_agg(to_jsonb(d)::text,E'\n' ORDER BY id) FROM reference_oa_production_discipline_heads d),''))`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
func assertOAZeroWrite(t *testing.T, env testdatabase.Environment, before string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) || oaDigest(t, env) != before {
		t.Fatalf("err=%v want=%v zero_write=%t", err, want, oaDigest(t, env) == before)
	}
}

func TestOAProductionNativePG18LifecycleReplayPreserveAndBaseIsolation(t *testing.T) {
	env, s, a, p := oaFixture(t)
	ctx := context.Background()
	base, _ := s.Get(ctx, a.TenantID, p.ID, true)
	in := oaIntention(804, p.Revision)
	added, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, in, oaResolved())
	if err != nil || !added.Changed || added.ProjectRevision != 4 || len(added.Added) != 3 {
		t.Fatalf("add=%+v %v", added, err)
	}
	before := oaDigest(t, env)
	replay, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, in, personnel.Resolved{})
	if err != nil || !reflect.DeepEqual(added, replay) || oaDigest(t, env) != before {
		t.Fatal("replay accessed sources, changed identity or wrote")
	}
	conflict := in
	conflict.Changes = append([]personnel.RoleChange(nil), in.Changes...)
	conflict.Changes[0].People = []personnel.Ref{}
	_, err = s.ApplyOAProductionPersonnel(ctx, a, p.ID, conflict, oaResolved())
	assertOAZeroWrite(t, env, before, err, ErrCommandConflict)
	noop := oaIntention(805, 4)
	unchanged, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, noop, oaResolved())
	if err != nil || unchanged.Changed || unchanged.ProjectRevision != 4 {
		t.Fatalf("no-op=%+v %v", unchanged, err)
	}
	view, _ := s.OAProductionPersonnel(ctx, a.TenantID, p.ID, false)
	for _, f := range view.Facts {
		found := false
		for _, old := range added.Added {
			found = found || f.ID == old.ID
		}
		if !found {
			t.Fatal("unchanged identity replaced")
		}
	}
	na, err := s.SetOAProductionParticipantApplicable(ctx, a, p.ID, RevisionCommand{CommandID: command(806), ExpectedRevision: 4}, true)
	if err != nil || na.ProjectRevision != 5 {
		t.Fatal(err)
	}
	end := personnel.Intention{Profile: personnel.Profile, CommandID: command(807), ExpectedProjectRevision: 5, Operation: personnel.EndDiscipline, SourceDisciplineID: oaDiscipline, ParticipantOperation: personnel.Preserve}
	ended, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, end, personnel.Resolved{})
	if err != nil || len(ended.EndIDs) != 2 || ended.ProjectRevision != 6 {
		t.Fatalf("end=%+v %v", ended, err)
	}
	active, _ := s.OAProductionPersonnel(ctx, a.TenantID, p.ID, false)
	if len(active.Facts) != 1 || active.Facts[0].Role != "participant" {
		t.Fatalf("hidden participant was ended: %+v", active)
	}
	participantID := active.Facts[0].ID
	if _, err := s.SetOAProductionParticipantApplicable(ctx, a, p.ID, RevisionCommand{CommandID: command(808), ExpectedRevision: 6}, false); err != nil {
		t.Fatal(err)
	}
	rejoined, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, oaIntention(809, 7), oaResolved())
	if err != nil || len(rejoined.Added) != 2 || rejoined.ProjectRevision != 8 {
		t.Fatalf("rejoin=%+v %v", rejoined, err)
	}
	all, _ := s.OAProductionPersonnel(ctx, a.TenantID, p.ID, true)
	if len(all.Facts) != 5 {
		t.Fatalf("history lost: %+v", all)
	}
	for _, f := range rejoined.Added {
		for _, old := range added.Added {
			if f.ID == old.ID {
				t.Fatal("rejoin reused ended ID")
			}
		}
	}
	active, _ = s.OAProductionPersonnel(ctx, a.TenantID, p.ID, false)
	for _, f := range active.Facts {
		if f.Role == "participant" && f.ID != participantID {
			t.Fatal("restored participant identity changed")
		}
	}
	old, _ := s.Get(ctx, a.TenantID, p.ID, true)
	if !reflect.DeepEqual(old.Assignments, base.Assignments) {
		t.Fatal("Base owner changed or customer facts leaked into old v1")
	}
	var events, commands int
	if err := env.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM reference_oa_production_events),(SELECT count(*) FROM project_commands)`).Scan(&events, &commands); err != nil {
		t.Fatal(err)
	}
	if events != 6 || commands != 9 {
		t.Fatalf("events=%d commands=%d", events, commands)
	}
	t.Log("native_project_storage=true people_and_discipline_test_resolvers=true changed_events=6 receipts=9 frozen_facts=5 active=3 ended=2 no_op_revision_unchanged=true replay_offline=true base_owner_and_v1_unchanged=true")
}

func TestOAProductionNativePG18FailureRollbackAndSameCommandRetry(t *testing.T) {
	env, s, a, p := oaFixture(t)
	ctx := context.Background()
	first, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, oaIntention(820, p.Revision), oaResolved())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Pool.Exec(ctx, `CREATE FUNCTION test_oa_fail_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'owned test failure after endings'; END $$; CREATE TRIGGER test_oa_fail_insert BEFORE INSERT ON reference_oa_production_assignments FOR EACH ROW EXECUTE FUNCTION test_oa_fail_insert()`); err != nil {
		t.Fatal(err)
	}
	in := oaIntention(821, first.ProjectRevision)
	in.Changes[0].People = []personnel.Ref{{ID: intPersonB, Revision: 1}}
	before := oaDigest(t, env)
	if _, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, in, oaResolved()); err == nil || oaDigest(t, env) != before {
		t.Fatalf("partial endings, revision, receipt or audit escaped rollback: %v", err)
	}
	if _, err := env.Pool.Exec(ctx, `DROP TRIGGER test_oa_fail_insert ON reference_oa_production_assignments; DROP FUNCTION test_oa_fail_insert()`); err != nil {
		t.Fatal(err)
	}
	retried, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, in, oaResolved())
	if err != nil || len(retried.EndIDs) != 1 || len(retried.Added) != 1 || retried.ProjectRevision != 5 {
		t.Fatalf("retry=%+v %v", retried, err)
	}
	t.Log("injected_after_end_before_add_rollback=true project_revision_command_audit_zero_partial=true same_command_retry=true")
}

func TestOAProductionNativePG18ScopeSourceStaleAndClosedProfile(t *testing.T) {
	env, s, a, p := oaFixture(t)
	ctx := context.Background()
	before := oaDigest(t, env)
	in := oaIntention(830, p.Revision)
	_, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, in, personnel.Resolved{})
	assertOAZeroWrite(t, env, before, err, personnel.ErrSource)
	wrong := a
	wrong.TenantID = intTenantB
	_, err = s.ApplyOAProductionPersonnel(ctx, wrong, p.ID, in, oaResolved())
	assertOAZeroWrite(t, env, before, err, ErrNotFound)
	_, err = s.OAProductionPersonnel(ctx, wrong.TenantID, p.ID, true)
	assertOAZeroWrite(t, env, before, err, ErrNotFound)
	sources := oaResolved()
	sources.Discipline.TenantID = intTenantB
	_, err = s.ApplyOAProductionPersonnel(ctx, a, p.ID, in, sources)
	assertOAZeroWrite(t, env, before, err, personnel.ErrSource)
	in.ExpectedProjectRevision--
	_, err = s.ApplyOAProductionPersonnel(ctx, a, p.ID, in, oaResolved())
	assertOAZeroWrite(t, env, before, err, personnel.ErrStale)
	in = oaIntention(831, p.Revision)
	in.Profile = "unselected-platform"
	_, err = s.ApplyOAProductionPersonnel(ctx, a, p.ID, in, oaResolved())
	assertOAZeroWrite(t, env, before, err, personnel.ErrInvalid)
	wrong = a
	wrong.ID = "admin"
	_, err = s.ApplyOAProductionPersonnel(ctx, wrong, p.ID, oaIntention(832, p.Revision), oaResolved())
	assertOAZeroWrite(t, env, before, err, personnel.ErrInvalid)
	t.Log("tenant_scope_not_found=true source_tenant_mismatch_zero_write=true stale_zero_write=true missing_resolver_zero_write=true unknown_profile_zero_write=true no_http_or_iam_claim=true")
}

func TestOAProductionNativePG18ConcurrentSameAndDifferentCommands(t *testing.T) {
	env, s, a, p := oaFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make([]OAProductionReceipt, 2)
	errs := make([]error, 2)
	in := oaIntention(840, p.Revision)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = s.ApplyOAProductionPersonnel(ctx, a, p.ID, in, oaResolved())
		}(i)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || !reflect.DeepEqual(results[0], results[1]) {
		t.Fatalf("same-command race: %v %+v", errs, results)
	}
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			next := oaIntention(841+i, 4)
			next.Changes[2].People = []personnel.Ref{}
			if i == 1 {
				next.Changes[2].People = []personnel.Ref{{ID: intPersonA, Revision: 1}}
			}
			results[i], errs[i] = s.ApplyOAProductionPersonnel(ctx, a, p.ID, next, oaResolved())
		}(i)
	}
	wg.Wait()
	winners, conflicts := 0, 0
	for _, err := range errs {
		if err == nil {
			winners++
		} else if errors.Is(err, personnel.ErrStale) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("different-command race winners=%d conflicts=%d", winners, conflicts)
	}
	var maximum int
	if err := env.Pool.QueryRow(ctx, `SELECT coalesce(max(n),0) FROM (SELECT count(*) n FROM reference_oa_production_assignments WHERE ended_at IS NULL AND role_key='reviewer' GROUP BY project_id,discipline_id) s`).Scan(&maximum); err != nil || maximum > 1 {
		t.Fatalf("single reviewer=%d %v", maximum, err)
	}
	t.Log("same_command_two_replies_one_commit=true different_command_one_winner_one_stale=true no_implicit_overwrite=true active_reviewer_unique=true")
}

func TestOAProductionNativePG18DatabaseGuardsRejectSnapshotAndHistoryMutation(t *testing.T) {
	env, s, a, p := oaFixture(t)
	ctx := context.Background()
	added, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, oaIntention(850, p.Revision), oaResolved())
	if err != nil {
		t.Fatal(err)
	}
	before := oaDigest(t, env)
	queries := []string{
		`UPDATE reference_oa_production_assignments SET person_name='silent rename'`,
		`DELETE FROM reference_oa_production_assignments`,
		`UPDATE reference_oa_production_assignments SET ended_at=clock_timestamp()`,
		`UPDATE reference_oa_production_events SET actor_subject='22222222-2222-4222-8222-222222222222'`,
		`DELETE FROM reference_oa_production_profiles`,
		`UPDATE reference_oa_production_profiles SET profile_key='generic-platform'`,
		`UPDATE project_commands SET response_json='{}'::jsonb`,
	}
	for _, query := range queries {
		if _, err := env.Pool.Exec(ctx, query); err == nil || oaDigest(t, env) != before {
			t.Fatalf("mutation accepted or wrote: %s %v", query, err)
		}
	}
	// Exercise SQL uniqueness independently of the planner and UI single Select.
	for _, role := range []string{"reviewer", "designer"} {
		if _, err := env.Pool.Exec(ctx, `INSERT INTO reference_oa_production_assignments(id,tenant_id,project_id,role_key,person_id,person_revision,person_name,discipline_id,discipline_revision,discipline_name,created_project_revision,started_at) SELECT $1,tenant_id,project_id,role_key,person_id,person_revision,person_name,discipline_id,discipline_revision,discipline_name,created_project_revision,started_at FROM reference_oa_production_assignments WHERE role_key=$2`, newUUID(), role); err == nil || oaDigest(t, env) != before {
			t.Fatalf("duplicate role accepted: %s %v", role, err)
		}
	}
	if len(added.Added) != 3 {
		t.Fatal(added)
	}
	t.Log("sql_frozen_snapshot_guard=true no_delete=true no_partial_end=true immutable_receipt_and_audit=true sql_role_uniqueness=true")
}

func TestOAProductionNativePG18ColdRestoreReplayAndContinue(t *testing.T) {
	container := os.Getenv("TEST_PG18_CONTAINER")
	if container == "" {
		t.Skip("TEST_PG18_CONTAINER is not set for isolated pg_dump/pg_restore")
	}
	env, s, a, p := oaFixture(t)
	ctx := context.Background()
	in := oaIntention(860, p.Revision)
	first, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, in, oaResolved())
	if err != nil {
		t.Fatal(err)
	}
	end := personnel.Intention{Profile: personnel.Profile, CommandID: command(861), ExpectedProjectRevision: first.ProjectRevision, Operation: personnel.EndDiscipline, SourceDisciplineID: oaDiscipline, ParticipantOperation: personnel.End}
	if _, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, end, personnel.Resolved{}); err != nil {
		t.Fatal(err)
	}
	before := oaDigest(t, env)
	view, _ := s.OAProductionPersonnel(ctx, a.TenantID, p.ID, true)
	dump, err := exec.CommandContext(ctx, "docker", "exec", container, "pg_dump", "-U", "postgres", "-Fc", env.Name).Output()
	if err != nil {
		t.Fatal(err)
	}
	// Empty target is another owned testdatabase; neither Main nor Suite restore.
	other := testdatabase.New(t, testdatabase.RequireURL(t))
	restore := exec.CommandContext(ctx, "docker", "exec", "-i", container, "pg_restore", "-U", "postgres", "--exit-on-error", "--no-owner", "--no-acl", "-d", other.Name)
	restore.Stdin = bytes.NewReader(dump)
	if output, err := restore.CombinedOutput(); err != nil {
		t.Fatalf("restore: %v %s", err, output)
	}
	if _, err := database.RequireCurrentSchema(ctx, other.Pool); err != nil {
		t.Fatal(err)
	}
	if oaDigest(t, other) != before {
		t.Fatal("restoration changed Project, Base, extension identity, lifecycle, receipts or audit")
	}
	recovered, _ := NewPostgresStore(other.Pool)
	got, _ := recovered.OAProductionPersonnel(ctx, a.TenantID, p.ID, true)
	if !reflect.DeepEqual(got, view) {
		t.Fatal("view changed after restore")
	}
	replay, err := recovered.ApplyOAProductionPersonnel(ctx, a, p.ID, in, personnel.Resolved{})
	if err != nil || !reflect.DeepEqual(replay, first) || oaDigest(t, other) != before {
		t.Fatal("restored offline replay wrote or changed receipt")
	}
	next, err := recovered.ApplyOAProductionPersonnel(ctx, a, p.ID, oaIntention(862, got.ProjectRevision), oaResolved())
	if err != nil || next.ProjectRevision != got.ProjectRevision+1 || len(next.Added) != 3 {
		t.Fatalf("continue=%+v %v", next, err)
	}
	digest := sha256.Sum256(dump)
	t.Logf("project_only_pg18_cold_empty_target_restore=true canonical_digest=%s dump_sha256=%s facts_and_audit_unchanged=true offline_replay_zero_write=true continued_new_identities=true suite_restore=false production_iam=false", before, hex.EncodeToString(digest[:]))
}

// Keep an explicit check that a fresh DB CHECK cannot be bypassed with SQL NULL.
func TestOAProductionNativePG18DatabaseRejectsPartialProfessionalSnapshot(t *testing.T) {
	env, _, a, p := oaFixture(t)
	ctx := context.Background()
	before := oaDigest(t, env)
	_, err := env.Pool.Exec(ctx, `INSERT INTO reference_oa_production_assignments(id,tenant_id,project_id,role_key,person_id,person_revision,person_name,discipline_id,discipline_revision,discipline_name,created_project_revision,started_at) VALUES($1,$2,$3,'designer',$4,1,'甲',$5,NULL,NULL,4,clock_timestamp())`, newUUID(), a.TenantID, p.ID, intPersonA, oaDiscipline)
	var pgErr interface{ SQLState() string }
	if err == nil || !errors.As(err, &pgErr) || pgErr.SQLState() != "23514" || oaDigest(t, env) != before {
		t.Fatalf("NULL bypass: %v", err)
	}
}
