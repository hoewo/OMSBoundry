package referencebonus

import (
	"context"
	"encoding/json"
	"errors"
	core "omsbizfoundry/bonus-distribution/internal/bonus"
	"reflect"
	"testing"
)

type professionalFixture struct {
	*resolverFixture
	professionals map[string]Participant
}

func (f *professionalFixture) ResolveProfessionalParticipant(_ context.Context, _ string, _ core.ProjectSnapshot, _ core.PersonRef, a core.AssignmentRef) (Participant, error) {
	f.calls++
	if f.fail {
		return Participant{}, core.ErrReferenceUnavailable
	}
	return f.professionals[a.ID], nil
}
func (f *professionalFixture) ListProfessionalCandidates(context.Context, string, core.ProjectRef) ([]Participant, error) {
	v := []Participant{}
	for _, p := range f.professionals {
		v = append(v, p)
	}
	return v, nil
}
func professionalTestFixture() (core.Actor, *professionalFixture, CreateInput) {
	a, base, in := fixture()
	a.ResourceScopes = []core.ResourceScope{{Type: SideKind("production"), ID: in.ProjectRef.ID}}
	in.Scope = "production"
	in.Items = []ItemInput{}
	f := &professionalFixture{resolverFixture: base, professionals: map[string]Participant{}}
	f.values = map[string]core.ParticipantReference{}
	for i, role := range []string{"designer", "participant", "reviewer", "auditor", "approver", "estimator"} {
		id := testID(100 + i)
		v := Participant{AssignmentSource: ProfessionalSource, ParticipantReference: core.ParticipantReference{TenantID: a.TenantID, BonusType: "production", Active: true, Eligible: true, Person: core.PersonSnapshot{ID: testID(5), Revision: 2, DisplayName: "同一人员", Digest: testDigest()}, Assignment: core.AssignmentSnapshot{ID: id, Revision: 4, ProjectID: in.ProjectRef.ID, PersonID: testID(5), RoleKey: role, Digest: testDigest()}}}
		if i < 4 {
			v.Discipline = &DisciplineSnapshot{ID: testID(200), Revision: 1, DisplayName: "桥梁"}
		}
		f.professionals[id] = v
		in.Items = append(in.Items, ItemInput{AssignmentSource: ProfessionalSource, PersonRef: core.PersonRef{ID: testID(5), Revision: 2}, AssignmentRef: core.AssignmentRef{ID: id, Revision: 4}, Method: "fixed", FixedAmountMinor: "101"})
	}
	return a, f, in
}
func TestProfessionalExplicitProfileSixRolesFrozenReplayAndWithdrawal(t *testing.T) {
	a, f, in := professionalTestFixture()
	ctx := context.Background()
	m := newMemory()
	v2, _ := New(m, f)
	if _, err := v2.Create(ctx, a, in); !errors.Is(err, core.ErrReferenceInvalid) || m.commits != 0 || f.calls != 0 {
		t.Fatalf("v2 expanded: %v", err)
	}
	s, err := NewWithProfile(m, f, ProfessionalProfile)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := s.Candidates(ctx, a, in.ProjectRef, in.Scope)
	if err != nil || len(candidates) != 6 {
		t.Fatalf("candidates=%+v %v", candidates, err)
	}
	rows, err := s.Create(ctx, a, in)
	if err != nil || len(rows) != 6 || m.commits != 1 {
		t.Fatalf("record=%+v %v", rows, err)
	}
	for _, v := range rows {
		if v.AssignmentSource != ProfessionalSource || v.EffectiveProfile() != ProfessionalProfile || v.Person.ID != testID(5) || v.AmountMinor != 101 {
			t.Fatal(v)
		}
	}
	for _, p := range f.professionals {
		if p.Discipline != nil {
			p.Discipline.DisplayName = "源专业后改"
		}
	}
	f.fail = true
	before := f.calls
	again, err := s.Create(ctx, a, in)
	if err != nil || !reflect.DeepEqual(rows, again) || f.calls != before || m.commits != 1 {
		t.Fatal("replay dereferenced upstream", err)
	}
	v, err := s.Withdraw(ctx, a, rows[0].ID, WithdrawInput{CommandID: testID(300), ExpectedRevision: 1, Reason: "保留原专业与人员快照"})
	if err != nil || v.State != "withdrawn" || v.Discipline.DisplayName != "桥梁" {
		t.Fatalf("frozen withdrawal=%+v %v", v, err)
	}
	history, err := s.History(ctx, a, v.ID)
	if err != nil || len(history) != 2 {
		t.Fatalf("history=%+v %v", history, err)
	}
}
func TestProfessionalInvalidWholeBatchZeroWrite(t *testing.T) {
	for _, mutation := range []string{"wrong source", "business", "ended", "wrong person", "missing discipline", "global discipline", "source unavailable", "duplicate"} {
		t.Run(mutation, func(t *testing.T) {
			a, f, in := professionalTestFixture()
			m := newMemory()
			s, _ := NewWithProfile(m, f, ProfessionalProfile)
			p := f.professionals[in.Items[5].AssignmentRef.ID]
			switch mutation {
			case "wrong source":
				in.Items[5].AssignmentSource = "project"
			case "business":
				in.Scope = "business"
				a.ResourceScopes = []core.ResourceScope{{Type: SideKind("business"), ID: in.ProjectRef.ID}}
			case "ended":
				p.Active = false
			case "wrong person":
				p.Assignment.PersonID = testID(99)
			case "missing discipline":
				q := f.professionals[in.Items[0].AssignmentRef.ID]
				q.Discipline = nil
				f.professionals[q.Assignment.ID] = q
			case "global discipline":
				p.Discipline = &DisciplineSnapshot{ID: testID(200), Revision: 1, DisplayName: "桥梁"}
			case "source unavailable":
				f.fail = true
			case "duplicate":
				in.Items = append(in.Items, in.Items[0])
			}
			f.professionals[p.Assignment.ID] = p
			if _, err := s.Create(context.Background(), a, in); err == nil || m.commits != 0 || len(m.rows) != 0 {
				t.Fatalf("partial write %v", err)
			}
		})
	}
}
func TestProfessionalLegacyInputJSONAndCommandHashUnchanged(t *testing.T) {
	a, _, in := fixture()
	type legacyItem struct {
		PersonRef        core.PersonRef     `json:"person_ref"`
		AssignmentRef    core.AssignmentRef `json:"assignment_ref"`
		Method           string             `json:"method"`
		RatioBasisPoints int64              `json:"ratio_basis_points"`
		FixedAmountMinor string             `json:"fixed_amount_minor"`
	}
	legacy := struct {
		CommandID       string          `json:"command_id"`
		ProjectRef      core.ProjectRef `json:"project_ref"`
		Scope           string          `json:"scope"`
		BaseAmountMinor string          `json:"base_amount_minor"`
		OccurredOn      string          `json:"occurred_on"`
		Description     string          `json:"description"`
		Items           []legacyItem    `json:"items"`
	}{in.CommandID, in.ProjectRef, in.Scope, in.BaseAmountMinor, in.OccurredOn, in.Description, []legacyItem{}}
	for _, v := range in.Items {
		legacy.Items = append(legacy.Items, legacyItem{v.PersonRef, v.AssignmentRef, v.Method, v.RatioBasisPoints, v.FixedAmountMinor})
	}
	x, _ := json.Marshal(in)
	y, _ := json.Marshal(legacy)
	if string(x) != string(y) || hash(a, "record", "", in) != hash(a, "record", "", legacy) {
		t.Fatalf("legacy hash changed: %s %s", x, y)
	}
}
