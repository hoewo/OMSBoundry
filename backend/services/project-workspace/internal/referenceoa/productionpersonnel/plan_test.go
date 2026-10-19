package productionpersonnel

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

const tenant = "11111111-1111-4111-8111-111111111111"
const project = "22222222-2222-4222-8222-222222222222"
const command = "33333333-3333-4333-8333-333333333333"
const personA = "44444444-4444-4444-8444-444444444444"
const personB = "55555555-5555-4555-8555-555555555555"
const disciplineA = "66666666-6666-4666-8666-666666666666"
const disciplineB = "77777777-7777-4777-8777-777777777777"
const factA = "88888888-8888-4888-8888-888888888888"
const factB = "99999999-9999-4999-8999-999999999999"

func current() Current {
	return Current{Profile: Profile, TenantID: tenant, ProjectID: project, ProjectRevision: 4, ProjectStatus: "active", Active: []Assignment{}}
}
func refs(id string) []Ref { return []Ref{{ID: id, Revision: 1}} }
func sources() Resolved {
	return Resolved{People: map[Ref]ResolvedPerson{
		{ID: personA, Revision: 1}: {TenantID: tenant, State: "active", PersonSnapshot: PersonSnapshot{Ref: Ref{ID: personA, Revision: 1}, Name: "设计甲"}},
		{ID: personB, Revision: 1}: {TenantID: tenant, State: "active", PersonSnapshot: PersonSnapshot{Ref: Ref{ID: personB, Revision: 1}, Name: "复核乙"}},
	}, Discipline: &ResolvedDiscipline{TenantID: tenant, State: "active", DisciplineSnapshot: DisciplineSnapshot{Ref: Ref{ID: disciplineA, Revision: 1}, Name: "桥梁"}}}
}
func group() Intention {
	return Intention{Profile: Profile, CommandID: command, ExpectedProjectRevision: 4, Operation: ReplaceDiscipline, TargetDiscipline: &Ref{ID: disciplineA, Revision: 1}, Changes: []RoleChange{
		{Role: "designer", Operation: Replace, People: refs(personA)},
		{Role: "participant", Operation: Replace, People: refs(personA)},
		{Role: "reviewer", Operation: Replace, People: refs(personB)},
		{Role: "auditor", Operation: Replace, People: []Ref{}},
	}}
}
func fact(id, role, person string, discipline bool) Assignment {
	s := sources()
	p := s.People[Ref{ID: person, Revision: 1}].PersonSnapshot
	v := Assignment{ID: id, Role: role, Person: p, CreatedProjectRevision: 2}
	if discipline {
		v.Discipline = copyDiscipline(&s.Discipline.DisciplineSnapshot)
	}
	return v
}
func requireError(t *testing.T, c Current, in Intention, s Resolved, want error) {
	t.Helper()
	delta, err := Prepare(c, in, s)
	if !errors.Is(err, want) || !reflect.DeepEqual(delta, Delta{}) {
		t.Fatalf("delta=%+v err=%v want=%v", delta, err, want)
	}
}

func TestClosedCustomerRepresentationRejectsOtherProfilesAndBaseOwner(t *testing.T) {
	in := group()
	in.Profile = "generic-member-platform-v1"
	requireError(t, current(), in, sources(), ErrInvalid)
	in = group()
	in.Changes[0].Role = "production_owner"
	requireError(t, current(), in, sources(), ErrInvalid)
}
func TestCompleteIntentionRetainsMultiRolePersonWithoutSharingFactID(t *testing.T) {
	delta, err := Prepare(current(), group(), sources())
	if err != nil || len(delta.Add) != 3 || len(delta.EndIDs) != 0 {
		t.Fatalf("%+v %v", delta, err)
	}
	count := 0
	for _, f := range delta.Add {
		if f.Person.ID == personA {
			count++
		}
		if f.Discipline.ID != disciplineA || f.Person.Revision != 1 {
			t.Fatal(f)
		}
	}
	if count != 2 {
		t.Fatal("same person different roles collapsed")
	}
}
func TestPreserveOmissionAndExplicitClearAreDistinct(t *testing.T) {
	c := current()
	c.Active = []Assignment{fact(factA, "designer", personA, true)}
	in := group()
	in.Changes[0].People = nil
	requireError(t, c, in, sources(), ErrInvalid)
	in = group()
	in.Changes = in.Changes[:3]
	requireError(t, c, in, sources(), ErrInvalid)
	in = group()
	in.Changes[1].Role = "designer"
	requireError(t, c, in, sources(), ErrInvalid)
	in = group()
	for i := range in.Changes {
		in.Changes[i].People = []Ref{}
	}
	delta, err := Prepare(c, in, sources())
	if err != nil || len(delta.Add) != 0 || !reflect.DeepEqual(delta.EndIDs, []string{factA}) {
		t.Fatalf("%+v %v", delta, err)
	}
}
func TestUnchangedFactsKeepIdentityAndSnapshots(t *testing.T) {
	c := current()
	c.Active = []Assignment{fact(factA, "designer", personA, true), fact(factB, "reviewer", personB, true)}
	in := group()
	in.Changes[1].People = []Ref{}
	before, _ := json.Marshal(c)
	delta, err := Prepare(c, in, sources())
	after, _ := json.Marshal(c)
	if err != nil || len(delta.Add) != 0 || len(delta.EndIDs) != 0 || string(before) != string(after) {
		t.Fatalf("%+v %v mutated=%v", delta, err, string(before) != string(after))
	}
}
func TestNotApplicablePreservesOnSaveEndAndRestoration(t *testing.T) {
	c := current()
	c.ParticipantNotApplicable = true
	c.Active = []Assignment{fact(factA, "designer", personA, true), fact(factB, "participant", personA, true)}
	in := group()
	requireError(t, c, in, sources(), ErrState)
	in.Changes[1] = RoleChange{Role: "participant", Operation: Preserve}
	in.Changes[2].People = []Ref{}
	delta, err := Prepare(c, in, sources())
	if err != nil || len(delta.EndIDs) != 0 || len(delta.Add) != 0 {
		t.Fatalf("%+v %v", delta, err)
	}
	in = Intention{Profile: Profile, CommandID: command, ExpectedProjectRevision: 4, Operation: EndDiscipline, SourceDisciplineID: disciplineA, ParticipantOperation: Preserve}
	delta, err = Prepare(c, in, Resolved{})
	if err != nil || !reflect.DeepEqual(delta.EndIDs, []string{factA}) {
		t.Fatalf("%+v %v", delta, err)
	}
	in.ParticipantOperation = End
	requireError(t, c, in, Resolved{}, ErrState)
	c.ParticipantNotApplicable = false
	delta, err = Prepare(c, in, Resolved{})
	if err != nil || !reflect.DeepEqual(delta.EndIDs, []string{factA, factB}) {
		t.Fatalf("%+v %v", delta, err)
	}
}
func TestChangingDisciplineEndsOldCreatesNewAndCannotOverwriteAnotherGroup(t *testing.T) {
	c := current()
	c.Active = []Assignment{fact(factA, "designer", personA, true)}
	in := group()
	in.SourceDisciplineID = disciplineA
	in.TargetDiscipline = &Ref{ID: disciplineB, Revision: 2}
	s := sources()
	s.Discipline = &ResolvedDiscipline{TenantID: tenant, State: "active", DisciplineSnapshot: DisciplineSnapshot{Ref: *in.TargetDiscipline, Name: "道路"}}
	delta, err := Prepare(c, in, s)
	if err != nil || !reflect.DeepEqual(delta.EndIDs, []string{factA}) || len(delta.Add) != 3 {
		t.Fatalf("%+v %v", delta, err)
	}
	for _, a := range delta.Add {
		if a.Discipline.ID != disciplineB || a.Discipline.Revision != 2 {
			t.Fatal(a)
		}
	}
	b := fact(factB, "designer", personB, true)
	b.Discipline = &DisciplineSnapshot{Ref: *in.TargetDiscipline, Name: "道路"}
	c.Active = append(c.Active, b)
	requireError(t, c, in, s, ErrTargetOccupied)
	c.Active = []Assignment{}
	requireError(t, c, in, s, ErrState)
}
func TestGlobalReviewersNeverTouchProfessionalFacts(t *testing.T) {
	c := current()
	c.Active = []Assignment{fact(factA, "designer", personA, true), fact(factB, "approver", personA, false)}
	in := Intention{Profile: Profile, CommandID: command, ExpectedProjectRevision: 4, Operation: ReplaceReviewers, Changes: []RoleChange{{Role: "approver", Operation: Replace, People: refs(personB)}, {Role: "estimator", Operation: Replace, People: refs(personB)}}}
	delta, err := Prepare(c, in, sources())
	if err != nil || !reflect.DeepEqual(delta.EndIDs, []string{factB}) || len(delta.Add) != 2 {
		t.Fatalf("%+v %v", delta, err)
	}
	for _, a := range delta.Add {
		if a.Discipline != nil || a.Person.ID != personB {
			t.Fatal(a)
		}
	}
	in.TargetDiscipline = &Ref{ID: disciplineA, Revision: 1}
	requireError(t, c, in, sources(), ErrInvalid)
}
func TestSourcesFailClosedAndNeverReturnPartialDelta(t *testing.T) {
	for _, kind := range []string{"missing", "wrong_tenant", "inactive", "wrong_revision", "wrong_id", "same_revision_changed_name", "wrong_discipline_tenant", "inactive_discipline", "wrong_discipline_revision", "same_discipline_revision_changed_name"} {
		t.Run(kind, func(t *testing.T) {
			c := current()
			c.Active = []Assignment{fact(factA, "designer", personA, true)}
			s := sources()
			key := Ref{ID: personB, Revision: 1}
			v := s.People[key]
			switch kind {
			case "missing":
				delete(s.People, key)
			case "wrong_tenant":
				v.TenantID = project
			case "inactive":
				v.State = "inactive"
			case "wrong_revision":
				v.Revision = 2
			case "wrong_id":
				v.ID = personA
			case "same_revision_changed_name":
				key = Ref{ID: personA, Revision: 1}
				v = s.People[key]
				v.Name = "原地改名"
			case "wrong_discipline_tenant":
				s.Discipline.TenantID = project
			case "inactive_discipline":
				s.Discipline.State = "archived"
			case "wrong_discipline_revision":
				s.Discipline.Revision = 9
			case "same_discipline_revision_changed_name":
				s.Discipline.Name = "原地改专业名"
			}
			if kind != "missing" {
				s.People[key] = v
			}
			requireError(t, c, group(), s, ErrSource)
		})
	}
}
func TestSingleRoleAndActiveSnapshotIntegrity(t *testing.T) {
	for _, kind := range []string{"duplicate_person", "two_reviewers", "malformed_person", "missing_snapshot", "duplicate_fact_id", "unknown_role", "no_professional_discipline", "global_with_discipline", "duplicate_active_position", "different_single_people", "future_created_revision", "inconsistent_person_snapshot"} {
		t.Run(kind, func(t *testing.T) {
			c := current()
			in := group()
			switch kind {
			case "duplicate_person":
				in.Changes[0].People = append(refs(personA), Ref{ID: personA, Revision: 2})
			case "two_reviewers":
				in.Changes[2].People = append(refs(personB), Ref{ID: personA, Revision: 1})
			case "malformed_person":
				in.Changes[0].People = []Ref{{ID: "an-iam-account-name", Revision: 1}}
			case "missing_snapshot":
				c.Active = nil
			case "duplicate_fact_id":
				c.Active = []Assignment{fact(factA, "designer", personA, true), fact(factA, "participant", personA, true)}
			case "unknown_role":
				c.Active = []Assignment{fact(factA, "business_owner", personA, false)}
			case "no_professional_discipline":
				c.Active = []Assignment{fact(factA, "designer", personA, false)}
			case "global_with_discipline":
				c.Active = []Assignment{fact(factA, "approver", personA, true)}
			case "duplicate_active_position":
				c.Active = []Assignment{fact(factA, "designer", personA, true), fact(factB, "designer", personA, true)}
			case "different_single_people":
				c.Active = []Assignment{fact(factA, "reviewer", personA, true), fact(factB, "reviewer", personB, true)}
			case "future_created_revision":
				a := fact(factA, "designer", personA, true)
				a.CreatedProjectRevision = 5
				c.Active = []Assignment{a}
			case "inconsistent_person_snapshot":
				a, b := fact(factA, "designer", personA, true), fact(factB, "participant", personA, true)
				b.Person.Name = "同版本不同姓名"
				c.Active = []Assignment{a, b}
			}
			requireError(t, c, in, sources(), ErrInvalid)
		})
	}
}
func TestRevisionStateAndCanonicalCopies(t *testing.T) {
	c := current()
	in := group()
	in.ExpectedProjectRevision = 3
	requireError(t, c, in, sources(), ErrStale)
	c.ProjectStatus = "cancelled"
	requireError(t, c, group(), sources(), ErrState)
	in = group()
	in.Changes[0].People = append(refs(personB), Ref{ID: personA, Revision: 1})
	before, _ := json.Marshal(in)
	normalized, err := Normalize(in)
	after, _ := json.Marshal(in)
	if err != nil || string(before) != string(after) {
		t.Fatalf("%v mutation=%v", err, string(before) != string(after))
	}
	if normalized.Changes[0].Role != "auditor" {
		t.Fatal("role order is not canonical")
	}
	normalized.TargetDiscipline.Revision = 9
	if in.TargetDiscipline.Revision != 1 {
		t.Fatal("normalizer aliases target reference")
	}
	delta, err := Prepare(current(), group(), sources())
	if err != nil {
		t.Fatal(err)
	}
	delta.Add[0].Discipline.Name = "调用者修改"
	if delta.Add[1].Discipline.Name != "桥梁" {
		t.Fatal("new facts alias mutable discipline snapshots")
	}
}
