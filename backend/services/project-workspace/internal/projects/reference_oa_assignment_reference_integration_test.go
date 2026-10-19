//go:build integration

package projects

import (
	"context"
	"errors"
	personnel "omsbizfoundry/project-workspace/internal/referenceoa/productionpersonnel"
	"reflect"
	"testing"
)

func TestOAProductionReferencePG18FrozenBoundedReadAndTenantIsolation(t *testing.T) {
	env, s, a, p := oaFixture(t)
	ctx := context.Background()
	empty, err := s.OAProductionAssignmentReferences(ctx, a.TenantID, p.ID, "")
	if err != nil || empty.TenantID != a.TenantID || empty.ProjectID != p.ID || empty.ProjectRevision != p.Revision || empty.ProjectStatus != p.Status || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("unenrolled discovery=%+v %v", empty, err)
	}
	_, err = s.OAProductionAssignmentReferences(ctx, a.TenantID, p.ID, intPersonB)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("unenrolled exact lookup=%v", err)
	}
	saved, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, oaIntention(940, p.Revision), oaResolved())
	if err != nil {
		t.Fatal(err)
	}
	before := oaDigest(t, env)
	list, err := s.OAProductionAssignmentReferences(ctx, a.TenantID, p.ID, "")
	if err != nil || len(list.Items) != 3 || list.ProjectRevision != saved.ProjectRevision {
		t.Fatalf("list=%+v %v", list, err)
	}
	var participant OAProductionAssignmentReference
	for _, v := range list.Items {
		single, e := s.OAProductionAssignmentReferences(ctx, a.TenantID, p.ID, v.ID)
		if e != nil || len(single.Items) != 1 || !reflect.DeepEqual(single.Items[0], v) {
			t.Fatalf("single=%+v %v", single, e)
		}
		if v.Eligible || v.ResourceType != OAProductionAssignmentResource {
			t.Fatal("draft/new roles mislabeled", v)
		}
		if v.RoleKey == "participant" {
			participant = v
		}
	}
	_, err = s.OAProductionAssignmentReferences(ctx, intTenantB, p.ID, participant.ID)
	assertOAZeroWrite(t, env, before, err, ErrNotFound)
	_, err = s.OAProductionAssignmentReferences(ctx, a.TenantID, p.ID, intPersonB)
	assertOAZeroWrite(t, env, before, err, ErrNotFound)
	if oaDigest(t, env) != before {
		t.Fatal("GET wrote facts")
	}
	applicable, err := s.SetOAProductionParticipantApplicable(ctx, a, p.ID, RevisionCommand{CommandID: command(941), ExpectedRevision: saved.ProjectRevision}, true)
	if err != nil {
		t.Fatal(err)
	}
	single, err := s.OAProductionAssignmentReferences(ctx, a.TenantID, p.ID, participant.ID)
	if err != nil || !single.Items[0].ParticipantNotApplicable || single.Items[0].State != "active" || single.Items[0].Revision != participant.Revision {
		t.Fatal("NA rewrote reference", single, err)
	}
	end := personnel.Intention{Profile: personnel.Profile, CommandID: command(942), ExpectedProjectRevision: applicable.ProjectRevision, Operation: personnel.EndDiscipline, SourceDisciplineID: oaDiscipline, ParticipantOperation: personnel.Preserve}
	ended, err := s.ApplyOAProductionPersonnel(ctx, a, p.ID, end, personnel.Resolved{})
	if err != nil {
		t.Fatal(err)
	}
	for _, original := range list.Items {
		now, e := s.OAProductionAssignmentReferences(ctx, a.TenantID, p.ID, original.ID)
		if e != nil {
			t.Fatal(e)
		}
		v := now.Items[0]
		if original.RoleKey == "participant" {
			if v.State != "active" {
				t.Fatal("hidden participant ended")
			}
		} else if v.State != "ended" || v.Revision != ended.ProjectRevision || v.Eligible || v.Person != original.Person || !reflect.DeepEqual(v.Discipline, original.Discipline) {
			t.Fatal("ended snapshot lost", v)
		}
	}
	active, err := s.OAProductionAssignmentReferences(ctx, a.TenantID, p.ID, "")
	if err != nil || len(active.Items) != 1 {
		t.Fatal(active, err)
	}
	_, err = s.OAProductionAssignmentReferences(ctx, a.TenantID, "not-a-project", "")
	if !errors.Is(err, personnel.ErrInvalid) {
		t.Fatal(err)
	}
}
