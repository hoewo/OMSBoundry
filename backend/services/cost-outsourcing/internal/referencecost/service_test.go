package referencecost

import (
	"context"
	"errors"
	core "omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const tenant = "11111111-1111-4111-8111-111111111111"
const subject = "22222222-2222-4222-8222-222222222222"
const project = "33333333-3333-4333-8333-333333333333"
const file = "44444444-4444-4444-8444-444444444444"

type memoryStore struct {
	rows     map[string]Record
	history  map[string][]Revision
	commands map[string]struct {
		actor, hash string
		value       Record
	}
	reads int
}

func newMemory() *memoryStore {
	return &memoryStore{rows: map[string]Record{}, history: map[string][]Revision{}, commands: map[string]struct {
		actor, hash string
		value       Record
	}{}}
}
func (m *memoryStore) Get(_ context.Context, tenant, id string) (Record, error) {
	m.reads++
	v, ok := m.rows[tenant+id]
	if !ok {
		return Record{}, core.ErrNotFound
	}
	return v, nil
}
func (m *memoryStore) List(_ context.Context, t, p, s string) ([]Record, error) {
	m.reads++
	out := []Record{}
	for _, v := range m.rows {
		if v.TenantID == t && v.Project.ID == p && v.Scope == s {
			out = append(out, v)
		}
	}
	return out, nil
}
func (m *memoryStore) History(_ context.Context, t, id string) ([]Revision, error) {
	return m.history[t+id], nil
}
func (m *memoryStore) Replay(_ context.Context, a core.Actor, cmd, h string) (Record, bool, error) {
	v, ok := m.commands[a.TenantID+cmd]
	if !ok {
		return Record{}, false, nil
	}
	if v.hash != h || v.actor != a.ID {
		return Record{}, false, core.ErrCommandConflict
	}
	return v.value, true, nil
}
func (m *memoryStore) Commit(ctx context.Context, a core.Actor, cmd, h, op, why string, expected int64, v Record) (Record, error) {
	if old, ok, e := m.Replay(ctx, a, cmd, h); ok || e != nil {
		return old, e
	}
	if expected > 0 {
		old, e := m.Get(ctx, a.TenantID, v.ID)
		if e != nil {
			return Record{}, e
		}
		if old.State != "recorded" {
			return Record{}, core.ErrWithdrawn
		}
		if old.Revision != expected {
			return Record{}, core.ErrRevisionConflict
		}
	}
	m.rows[a.TenantID+v.ID] = v
	m.history[a.TenantID+v.ID] = append(m.history[a.TenantID+v.ID], Revision{v, cmd, op, why})
	m.commands[a.TenantID+cmd] = struct {
		actor, hash string
		value       Record
	}{a.ID, h, v}
	return v, nil
}

type resolver struct {
	mu          sync.Mutex
	calls       int
	unavailable bool
	badEvidence bool
}

func (r *resolver) ResolveProject(_ context.Context, t string, ref core.ProjectRef) (core.ProjectReference, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.unavailable {
		return core.ProjectReference{}, core.ErrReferenceUnavailable
	}
	return core.ProjectReference{TenantID: t, Status: "active", ProjectSnapshot: core.ProjectSnapshot{ID: ref.ID, Revision: ref.Revision, Code: "OA-1", DisplayName: "冻结项目"}}, nil
}
func (r *resolver) ResolveEvidence(_ context.Context, t string, ref core.EvidenceRef) (core.EvidenceReference, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.unavailable {
		return core.EvidenceReference{}, core.ErrReferenceUnavailable
	}
	v := core.EvidenceReference{TenantID: t, Status: "available", EvidenceSnapshot: core.EvidenceSnapshot{EvidenceRef: ref, Digest: "sha256:" + strings.Repeat("a", 64), DisplayName: "发票.png"}}
	if r.badEvidence {
		v.Digest = strings.Repeat("x", 71)
	}
	return v, nil
}
func actor() core.Actor {
	return core.Actor{ID: subject, TenantID: tenant, Kind: "iam_subject", CredentialID: file, TenantScope: true, Actions: []string{core.ActionCostCreate, core.ActionCostRead, core.ActionCostAmountRead, core.ActionCostCorrect, core.ActionCostWithdraw, core.ActionCostHistory, core.ActionCostEvidenceManage}}
}
func input(scope string) CreateInput {
	return CreateInput{CommandID: newID(), ProjectRef: core.ProjectRef{ID: project, Revision: 7}, Scope: scope, Fields: Fields{Category: "vehicle", AmountMinor: "9007199254740993", EvidenceRefs: []core.EvidenceRef{}}}
}
func TestExactOptionalFieldsAndSevenCategorySideConstraints(t *testing.T) {
	for _, side := range []string{"business", "production"} {
		for category := range categories {
			m, r := newMemory(), &resolver{}
			s, _ := New(m, r)
			in := input(side)
			in.Fields.Category = category
			v, e := s.Create(context.Background(), actor(), in)
			if side == "production" && (category == "bid_signup_fee" || category == "bid_service_fee") {
				if e == nil || len(m.rows) != 0 {
					t.Fatal("business-only cost escaped side")
				}
				continue
			}
			if e != nil || v.AmountMinor != 9007199254740993 || v.OccurredOn != nil || v.Description != nil || v.Project.Revision != 7 {
				t.Fatalf("category %s/%s: %#v %v", side, category, v, e)
			}
		}
	}
	for _, amount := range []string{"-1", "1.1", "01", "9223372036854775808", "1e3"} {
		m, r := newMemory(), &resolver{}
		s, _ := New(m, r)
		in := input("business")
		in.Fields.AmountMinor = amount
		if _, e := s.Create(context.Background(), actor(), in); e == nil || len(m.rows) != 0 || r.calls != 0 {
			t.Fatal("invalid amount reached dependencies")
		}
	}
}
func TestReplayPrecedesExternalFailureAndChangedPayloadConflicts(t *testing.T) {
	ctx := context.Background()
	m, r := newMemory(), &resolver{}
	s, _ := New(m, r)
	in := input("business")
	in.Fields.EvidenceRefs = []core.EvidenceRef{{FileID: file, Version: 1, Slot: "invoice"}}
	v, e := s.Create(ctx, actor(), in)
	if e != nil {
		t.Fatal(e)
	}
	r.unavailable = true
	calls := r.calls
	again, e := s.Create(ctx, actor(), in)
	if e != nil || again.ID != v.ID || r.calls != calls {
		t.Fatal("committed create not replayed independently")
	}
	in.Fields.AmountMinor = "1"
	if _, e = s.Create(ctx, actor(), in); !errors.Is(e, core.ErrCommandConflict) {
		t.Fatal(e)
	}
	if len(m.rows) != 1 || len(m.commands) != 1 {
		t.Fatal("duplicate facts")
	}
}
func TestAppendCorrectionWithdrawalAndReadScopeIsolation(t *testing.T) {
	ctx := context.Background()
	m, r := newMemory(), &resolver{}
	s, _ := New(m, r)
	a := actor()
	v, e := s.Create(ctx, a, input("business"))
	if e != nil {
		t.Fatal(e)
	}
	change := ChangeInput{CommandID: newID(), ExpectedRevision: 1, Reason: "追加停车费", Fields: Fields{Category: "vehicle", AmountMinor: "101", EvidenceRefs: []core.EvidenceRef{}}}
	v2, e := s.Correct(ctx, a, v.ID, change)
	if e != nil || v2.Revision != 2 || v2.Project != v.Project {
		t.Fatal(e)
	}
	change2 := change
	change2.CommandID = newID()
	if _, e = s.Correct(ctx, a, v.ID, change2); !errors.Is(e, core.ErrRevisionConflict) {
		t.Fatal("stale overwrite")
	}
	withdraw := WithdrawInput{CommandID: newID(), ExpectedRevision: 2, Reason: "留存后撤回"}
	v3, e := s.Withdraw(ctx, a, v.ID, withdraw)
	if e != nil || v3.AmountMinor != 101 || v3.State != "withdrawn" {
		t.Fatal(e)
	}
	again, e := s.Withdraw(ctx, a, v.ID, withdraw)
	if e != nil || again.Revision != 3 {
		t.Fatal("withdraw replay")
	}
	h, e := s.History(ctx, a, v.ID)
	if e != nil || len(h) != 3 || h[0].Record.AmountMinor != v.AmountMinor || h[1].Record.AmountMinor != 101 {
		t.Fatal("history overwritten")
	}
	legacy := a
	legacy.TenantScope = false
	legacy.ProjectIDs = []string{project}
	if _, e = s.Get(ctx, legacy, v.ID); !errors.Is(e, core.ErrForbidden) {
		t.Fatal("generic project grant expanded")
	}
	side := legacy
	side.ResourceScopes = []core.ResourceScope{{Type: SideKind("production"), ID: project}}
	if _, e = s.Get(ctx, side, v.ID); !errors.Is(e, core.ErrForbidden) {
		t.Fatal("production scope reads business cost")
	}
	side.ResourceScopes = []core.ResourceScope{{Type: SideKind("business"), ID: project}}
	rows, e := s.List(ctx, side, project, "business")
	if e != nil || len(rows) != 1 {
		t.Fatal(e)
	}
	exact := legacy
	exact.ResourceScopes = []core.ResourceScope{{Type: ResourceType, ID: v.ID}}
	if _, e = s.Get(ctx, exact, v.ID); e != nil {
		t.Fatal(e)
	}
	exact.ResourceScopes[0].ID = newID()
	rows, e = s.List(ctx, exact, project, "business")
	if e != nil || len(rows) != 0 {
		t.Fatal("ungranted record leaked")
	}
	bad := a
	bad.CredentialID = ""
	reads := m.reads
	if _, e = s.List(ctx, bad, project, "business"); !errors.Is(e, core.ErrForbidden) || m.reads != reads {
		t.Fatal("untrusted actor reached store")
	}
	if !reflect.DeepEqual(v3.Evidence, v2.Evidence) {
		t.Fatal("withdrawal erased files")
	}
}
func TestInvalidEvidenceAndMissingCapabilitiesNeverCommit(t *testing.T) {
	m, r := newMemory(), &resolver{badEvidence: true}
	s, _ := New(m, r)
	in := input("business")
	in.Fields.EvidenceRefs = []core.EvidenceRef{{FileID: file, Version: 1, Slot: "invoice"}}
	if _, e := s.Create(context.Background(), actor(), in); !errors.Is(e, core.ErrReferenceInvalid) || len(m.rows) != 0 {
		t.Fatal("digest not validated")
	}
	a := actor()
	a.Actions = []string{core.ActionCostCreate}
	if _, e := s.Create(context.Background(), a, in); !errors.Is(e, core.ErrForbidden) || len(m.rows) != 0 {
		t.Fatal("amount blind-write permitted")
	}
}
