//go:build integration

package projects

import (
	"context"
	"errors"
	personnel "omsbizfoundry/project-workspace/internal/referenceoa/productionpersonnel"
	"reflect"
	"sync"
	"testing"
)

func TestOADisciplineNativePG18VersionReplayArchiveAndFrozenAssignment(t *testing.T) {
	env, s, a, p := oaFixture(t)
	ctx := context.Background()
	in := OADisciplineCommand{CommandID: command(870), Operation: "create", Name: "道路", Description: "原专业目录"}
	d, err := s.ApplyOADiscipline(ctx, a, in)
	if err != nil || d.Revision != 1 {
		t.Fatal(d, err)
	}
	before := oaDigest(t, env)
	replay, err := s.ApplyOADiscipline(ctx, a, in)
	if err != nil || !reflect.DeepEqual(d, replay) || before != oaDigest(t, env) {
		t.Fatal("catalogue replay changed")
	}
	create := oaIntention(871, p.Revision)
	create.TargetDiscipline = &personnel.Ref{ID: d.ID, Revision: 1}
	resolved := oaResolved()
	resolved.Discipline = nil
	first, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, create, resolved)
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := s.ApplyOADiscipline(ctx, a, OADisciplineCommand{CommandID: command(872), Operation: "update", ID: d.ID, ExpectedRevision: 1, Name: "道路新名", Description: "修订"})
	if err != nil || renamed.Revision != 2 {
		t.Fatal(renamed, err)
	}
	view, _ := s.OAProductionPersonnel(ctx, a.TenantID, p.ID, false)
	for _, f := range view.Facts {
		if f.Discipline.Name != "道路" || f.Discipline.Revision != 1 {
			t.Fatal("old discipline snapshot silently rewritten")
		}
	}
	stale := create
	stale.CommandID = command(873)
	stale.ExpectedProjectRevision = first.ProjectRevision
	before = oaDigest(t, env)
	_, err = s.ApplyOAProductionPersonnel(ctx, a, p.ID, stale, resolved)
	assertOAZeroWrite(t, env, before, err, personnel.ErrSource)
	archived, err := s.ApplyOADiscipline(ctx, a, OADisciplineCommand{CommandID: command(874), Operation: "archive", ID: d.ID, ExpectedRevision: 2})
	if err != nil || archived.State != "archived" {
		t.Fatal(archived, err)
	}
	list, err := s.OADisciplines(ctx, a.TenantID, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range list {
		if v.ID == d.ID {
			t.Fatal("archived candidate remained active")
		}
	}
	before = oaDigest(t, env)
	stale.CommandID = command(875)
	stale.TargetDiscipline.Revision = 3
	_, err = s.ApplyOAProductionPersonnel(ctx, a, p.ID, stale, resolved)
	assertOAZeroWrite(t, env, before, err, personnel.ErrSource)
	end := personnel.Intention{Profile: personnel.Profile, CommandID: command(876), ExpectedProjectRevision: first.ProjectRevision, Operation: personnel.EndDiscipline, SourceDisciplineID: d.ID, ParticipantOperation: personnel.End}
	if _, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, end, personnel.Resolved{}); err != nil {
		t.Fatal("archived catalogue prevented explicit assignment ending", err)
	}
	restored, err := s.ApplyOADiscipline(ctx, a, OADisciplineCommand{CommandID: command(877), Operation: "reactivate", ID: d.ID, ExpectedRevision: 3})
	if err != nil || restored.ID != d.ID || restored.Revision != 4 {
		t.Fatal(restored, err)
	}
	before = oaDigest(t, env)
	_, err = s.ApplyOADiscipline(ctx, a, OADisciplineCommand{CommandID: command(878), Operation: "create", Name: "道路新名"})
	assertOAZeroWrite(t, env, before, err, ErrOADisciplineNameConflict)
	if _, err := env.Pool.Exec(ctx, `UPDATE reference_oa_production_discipline_versions SET name='静默改名'`); err == nil || oaDigest(t, env) != before {
		t.Fatal("catalogue historical versions mutable")
	}
	t.Log("catalogue_native_commands=true version_append=true old_assignment_name_and_revision_frozen=true archive_hides_no_fact_delete=true ending_archived_group_allowed=true reactivate_same_id_new_revision=true same_name_rejected=true offline_replay=true")
}
func TestOADisciplineNativePG18ConcurrentSameNameCreatesOne(t *testing.T) {
	env, s, a, _ := oaFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = s.ApplyOADiscipline(ctx, a, OADisciplineCommand{CommandID: command(880 + i), Operation: "create", Name: "隧道"})
		}(i)
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrOADisciplineNameConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(errs)
	}
	var versions, heads int
	if err := env.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM reference_oa_production_discipline_versions WHERE name='隧道'),(SELECT count(*) FROM reference_oa_production_discipline_heads WHERE name_normalized='隧道')`).Scan(&versions, &heads); err != nil || versions != 1 || heads != 1 {
		t.Fatal(versions, heads, err)
	}
	t.Log("same_active_name_concurrency_one_winner=true rejected_version_command_rolled_back=true")
}
