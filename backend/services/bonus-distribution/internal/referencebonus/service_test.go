package referencebonus

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	core "omsbizfoundry/bonus-distribution/internal/bonus"
)

func testID(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
func testDigest() string  { return "sha256:" + strings.Repeat("a", 64) }

type memoryStore struct {
	rows     map[string]Record
	commands map[string]struct {
		actor, hash string
		rows        []Record
	}
	histories map[string][]Revision
	commits   int
}

func newMemory() *memoryStore {
	return &memoryStore{rows: map[string]Record{}, commands: map[string]struct {
		actor, hash string
		rows        []Record
	}{}, histories: map[string][]Revision{}}
}
func (m *memoryStore) Get(_ context.Context, tenant, id string) (Record, error) {
	v, ok := m.rows[id]
	if !ok || v.TenantID != tenant {
		return Record{}, core.ErrNotFound
	}
	return v, nil
}
func (m *memoryStore) List(_ context.Context, tenant, project, scope string) ([]Record, error) {
	out := []Record{}
	for _, v := range m.rows {
		if v.TenantID == tenant && v.Project.ID == project && v.Scope == scope {
			out = append(out, v)
		}
	}
	return out, nil
}
func (m *memoryStore) History(_ context.Context, tenant, id string) ([]Revision, error) {
	if _, e := m.Get(context.Background(), tenant, id); e != nil {
		return nil, e
	}
	return m.histories[id], nil
}
func (m *memoryStore) Replay(_ context.Context, a core.Actor, cmd, h string) ([]Record, bool, error) {
	v, ok := m.commands[a.TenantID+cmd]
	if !ok {
		return nil, false, nil
	}
	if v.actor != a.ID || v.hash != h {
		return nil, false, core.ErrCommandConflict
	}
	return v.rows, true, nil
}
func (m *memoryStore) Commit(ctx context.Context, a core.Actor, cmd, h, op, reason string, expected int64, rows []Record) ([]Record, error) {
	if v, found, e := m.Replay(ctx, a, cmd, h); e != nil || found {
		return v, e
	}
	for _, v := range rows {
		if expected > 0 {
			old, e := m.Get(ctx, a.TenantID, v.ID)
			if e != nil {
				return nil, e
			}
			if old.Revision != expected {
				return nil, core.ErrRevisionConflict
			}
		}
	}
	for _, v := range rows {
		m.rows[v.ID] = v
		m.histories[v.ID] = append(m.histories[v.ID], Revision{v, cmd, op, reason})
	}
	m.commands[a.TenantID+cmd] = struct {
		actor, hash string
		rows        []Record
	}{a.ID, h, rows}
	m.commits++
	return rows, nil
}

type resolverFixture struct {
	project core.ProjectReference
	values  map[string]core.ParticipantReference
	fail    bool
	calls   int
}

func (f *resolverFixture) ResolveProject(context.Context, string, core.ProjectRef) (core.ProjectReference, error) {
	f.calls++
	if f.fail {
		return core.ProjectReference{}, core.ErrReferenceUnavailable
	}
	return f.project, nil
}
func (f *resolverFixture) ResolveManualParticipant(_ context.Context, _ string, _ core.ProjectSnapshot, _ string, _ core.PersonRef, a core.AssignmentRef) (core.ParticipantReference, error) {
	f.calls++
	if f.fail {
		return core.ParticipantReference{}, core.ErrReferenceUnavailable
	}
	return f.values[a.ID], nil
}
func (f *resolverFixture) ListManualCandidates(context.Context, string, core.ProjectRef, string) ([]core.ParticipantReference, error) {
	out := []core.ParticipantReference{}
	for _, v := range f.values {
		out = append(out, v)
	}
	return out, nil
}
func fixture() (core.Actor, *resolverFixture, CreateInput) {
	a := core.Actor{ID: testID(1), TenantID: testID(2), Credential: testID(3), SubjectKind: "iam_subject", Actions: append([]string{}, Actions...), ResourceScopes: []core.ResourceScope{{Type: SideKind("business"), ID: testID(4)}}}
	p := core.ProjectSnapshot{ID: testID(4), Revision: 18, Code: "OA-VERIFY", DisplayName: "验收项目", Digest: testDigest()}
	f := &resolverFixture{project: core.ProjectReference{ProjectSnapshot: p, TenantID: a.TenantID, State: "active"}, values: map[string]core.ParticipantReference{}}
	items := []ItemInput{}
	for i, role := range []string{"business_owner", "commercial_contributor"} {
		id := testID(6 + i)
		f.values[id] = core.ParticipantReference{TenantID: a.TenantID, BonusType: "business", Active: true, Eligible: true, Person: core.PersonSnapshot{ID: testID(5), Revision: 2, DisplayName: "同一人员", Digest: testDigest()}, Assignment: core.AssignmentSnapshot{ID: id, Revision: int64(i + 1), ProjectID: p.ID, PersonID: testID(5), RoleKey: role, Digest: testDigest()}}
		items = append(items, ItemInput{PersonRef: core.PersonRef{ID: testID(5), Revision: 2}, AssignmentRef: core.AssignmentRef{ID: id, Revision: int64(i + 1)}, Method: "ratio", RatioBasisPoints: 5000, FixedAmountMinor: "0"})
	}
	items[1].Method = "fixed"
	items[1].RatioBasisPoints = 0
	items[1].FixedAmountMinor = "7000"
	in := CreateInput{CommandID: testID(20), ProjectRef: core.ProjectRef{ID: p.ID, Revision: p.Revision}, Scope: "business", BaseAmountMinor: "1000", OccurredOn: "2026-10-04", Description: "混合分配", Items: items}
	return a, f, in
}
func TestManualMixedSamePersonDifferentAssignmentsAndFrozenReplay(t *testing.T) {
	a, f, in := fixture()
	m := newMemory()
	s, _ := New(m, f)
	ctx := context.Background()
	rows, e := s.Create(ctx, a, in)
	if e != nil || len(rows) != 2 || rows[0].AmountMinor != 500 || rows[1].AmountMinor != 7000 || rows[0].BatchID != rows[1].BatchID || rows[0].Person.ID != rows[1].Person.ID {
		t.Fatalf("mixed=%+v %v", rows, e)
	}
	calls := f.calls
	f.fail = true
	old, e := s.Create(ctx, a, in)
	if e != nil || !reflect.DeepEqual(old, rows) || f.calls != calls || m.commits != 1 {
		t.Fatalf("replay reread source or duplicated rows: %v", e)
	}
	bad := in
	bad.Description = "changed"
	if _, e = s.Create(ctx, a, bad); !errors.Is(e, core.ErrCommandConflict) {
		t.Fatalf("payload conflict=%v", e)
	}
	withdrawal := WithdrawInput{CommandID: testID(21), ExpectedRevision: 1, Reason: "录入错误，保留历史"}
	v, e := s.Withdraw(ctx, a, rows[0].ID, withdrawal)
	if e != nil || v.Revision != 2 || v.State != "withdrawn" || v.AmountMinor != 500 {
		t.Fatalf("withdraw=%+v %v", v, e)
	}
	v2, e := s.Withdraw(ctx, a, rows[0].ID, withdrawal)
	if e != nil || !reflect.DeepEqual(v, v2) || m.commits != 2 {
		t.Fatal("withdraw replay failed")
	}
	history, e := s.History(ctx, a, v.ID)
	if e != nil || len(history) != 2 || history[0].Snapshot.State != "recorded" || history[1].Reason != withdrawal.Reason {
		t.Fatalf("history=%+v %v", history, e)
	}
	if m.rows[rows[1].ID].State != "recorded" {
		t.Fatal("withdraw cascaded to batch neighbour")
	}
}
func TestManualInputMatrix(t *testing.T) {
	tests := []struct {
		name string
		edit func(*CreateInput)
	}{
		{"negative base", func(v *CreateInput) { v.BaseAmountMinor = "-1" }}, {"exponent", func(v *CreateInput) { v.BaseAmountMinor = "1e3" }}, {"leading zero", func(v *CreateInput) { v.BaseAmountMinor = "010" }}, {"decimal minor", func(v *CreateInput) { v.BaseAmountMinor = "10.0" }},
		{"overflow", func(v *CreateInput) { v.BaseAmountMinor = "9223372036854775808" }}, {"zero ratio base", func(v *CreateInput) { v.BaseAmountMinor = "0" }}, {"rounds to zero", func(v *CreateInput) { v.BaseAmountMinor = "1"; v.Items[0].RatioBasisPoints = 1 }},
		{"duplicate assignment", func(v *CreateInput) { v.Items[1].AssignmentRef = v.Items[0].AssignmentRef }}, {"wrong method", func(v *CreateInput) { v.Items[0].Method = "other" }}, {"ratio fixed input", func(v *CreateInput) { v.Items[0].FixedAmountMinor = "1" }}, {"fixed ratio", func(v *CreateInput) { v.Items[1].RatioBasisPoints = 1 }},
		{"zero fixed", func(v *CreateInput) { v.Items[1].FixedAmountMinor = "0" }}, {"ratio over 100", func(v *CreateInput) { v.Items[0].RatioBasisPoints = 10001 }}, {"total over 100", func(v *CreateInput) {
			v.Items[1].Method = "ratio"
			v.Items[1].FixedAmountMinor = "0"
			v.Items[1].RatioBasisPoints = 6000
		}},
		{"bad date", func(v *CreateInput) { v.OccurredOn = "2026-02-30" }}, {"year zero", func(v *CreateInput) { v.OccurredOn = "0000-01-01" }}, {"missing date", func(v *CreateInput) { v.OccurredOn = "" }}, {"no rows", func(v *CreateInput) { v.Items = nil }}, {"side", func(v *CreateInput) { v.Scope = "x" }},
		{"total overflow", func(v *CreateInput) {
			v.Items[0].Method = "fixed"
			v.Items[0].RatioBasisPoints = 0
			v.Items[0].FixedAmountMinor = "9223372036854775807"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, f, in := fixture()
			tt.edit(&in)
			m := newMemory()
			s, _ := New(m, f)
			if _, e := s.Create(context.Background(), a, in); e == nil {
				t.Fatal("invalid input accepted")
			}
			if m.commits != 0 || f.calls != 0 {
				t.Fatal("invalid input touched source/store")
			}
		})
	}
	_, _, in := fixture()
	in.BaseAmountMinor = "5"
	in.Items = in.Items[:1]
	in.Items[0].RatioBasisPoints = 1000
	_, _, values, e := normalize(in)
	if e != nil || values[0] != 1 {
		t.Fatalf("half-up=%v %v", values, e)
	}
	in.Items[0].Method = "fixed"
	in.Items[0].RatioBasisPoints = 0
	in.Items[0].FixedAmountMinor = "2"
	in.BaseAmountMinor = "0"
	if _, _, _, e = normalize(in); e != nil {
		t.Fatal("fixed-only zero base must work", e)
	}
}
func TestManualScopedAuthorizationAndReferenceFailureAtomicity(t *testing.T) {
	for _, scope := range []string{"project", "production_bonus_project", "bonus_pool"} {
		a, f, in := fixture()
		a.ResourceScopes = []core.ResourceScope{{Type: scope, ID: in.ProjectRef.ID}}
		m := newMemory()
		s, _ := New(m, f)
		if _, e := s.Create(context.Background(), a, in); !errors.Is(e, core.ErrForbidden) || m.commits != 0 || f.calls != 0 {
			t.Fatalf("scope %s widened", scope)
		}
	}
	for _, action := range []string{ActionRecord, ActionAmountRead} {
		a, f, in := fixture()
		filtered := []string{}
		for _, v := range a.Actions {
			if v != action {
				filtered = append(filtered, v)
			}
		}
		a.Actions = filtered
		m := newMemory()
		s, _ := New(m, f)
		if _, e := s.Create(context.Background(), a, in); !errors.Is(e, core.ErrForbidden) {
			t.Fatalf("missing %s=%v", action, e)
		}
	}
	for _, field := range []string{"tenant", "person", "project", "revision", "role", "active", "digest"} {
		t.Run(field, func(t *testing.T) {
			a, f, in := fixture()
			key := in.Items[1].AssignmentRef.ID
			v := f.values[key]
			switch field {
			case "tenant":
				v.TenantID = testID(99)
			case "person":
				v.Assignment.PersonID = testID(99)
			case "project":
				v.Assignment.ProjectID = testID(99)
			case "revision":
				v.Person.Revision++
			case "role":
				v.Assignment.RoleKey = "production_owner"
			case "active":
				v.Active = false
			case "digest":
				v.Person.Digest = "invalid"
			}
			f.values[key] = v
			m := newMemory()
			s, _ := New(m, f)
			if _, e := s.Create(context.Background(), a, in); !errors.Is(e, core.ErrReferenceInvalid) || m.commits != 0 {
				t.Fatal("partial invalid batch persisted", e)
			}
		})
	}
}
