//go:build integration

package projects

import (
	"context"
	"fmt"
	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/testdatabase"
	"reflect"
	"testing"
)

func TestPostgresCounterpartyNameSuggestionsHistoryTenantScopeLiteralBoundedAndZeroWrite(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	seed := func(seq int, tenant, identity string) Project {
		who := actor
		who.TenantID = tenant
		input := CreateInput{CommandID: command(seq), Name: "项目", ProjectNumber: fmt.Sprintf("NAMES-%d", seq), ProjectTypeKeys: []string{"design"}}
		if identity != "" {
			input.validatedClientParty = &PartySnapshot{PartyID: intPersonA, SourceRevision: 1, Kind: "organization", LegalName: identity}
		}
		p, err := store.Create(ctx, who, input)
		if err != nil {
			t.Fatal(err)
		}
		assignment, err := store.ApplyAssignment(ctx, who, p.ID, AssignmentCommand{CommandID: command(seq + 1), ExpectedRevision: p.Revision, Operation: "assign", SubjectKind: "person", SubjectID: intPersonA, SourceRevision: 1, RoleKey: RoleBusinessOwner, DisplaySnapshot: "经营负责人"})
		if err != nil {
			t.Fatal(err)
		}
		p, err = store.CompleteInitiation(ctx, who, p.ID, RevisionCommand{CommandID: command(seq + 2), ExpectedRevision: assignment.Project.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := seed(3100, intTenantA, "已验证甲方名称")
	b := seed(3120, intTenantA, "")
	other := seed(3140, intTenantB, "")
	change := func(p Project, tenant string, seq int, name string) Project {
		who := actor
		who.TenantID = tenant
		view, err := store.ChangeCounterpartyContext(ctx, who, p.ID, CounterpartyContextInput{CommandID: command(seq), ExpectedRevision: p.Revision, Reason: "名称", CounterpartyContact: CounterpartyContact{Name: name, ContactName: "不返回联系人", ContactPhone: "123456"}})
		if err != nil {
			t.Fatal(err)
		}
		p.Revision = view.ProjectRevision
		return p
	}
	a = change(a, intTenantA, 3103, "旧甲方")
	a = change(a, intTenantA, 3104, "新甲方")
	b = change(b, intTenantA, 3123, "同租户秘密甲方")
	_ = change(other, intTenantB, 3143, "异租户秘密甲方")
	var before string
	if err := env.Pool.QueryRow(ctx, `SELECT md5(string_agg(row_to_json(h)::text,',' ORDER BY id)) FROM project_history_events h`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	q := CounterpartyNamesQuery{ScopeKind: "resources", ResourceIDs: []string{a.ID}, Query: "甲方"}
	got, err := store.CounterpartyNameSuggestions(ctx, intTenantA, q)
	if err != nil || !reflect.DeepEqual(got.Names, []string{"已验证甲方名称", "新甲方", "旧甲方"}) {
		t.Fatalf("scoped names=%#v %v", got, err)
	}
	q.ResourceIDs = []string{}
	got, err = store.CounterpartyNameSuggestions(ctx, intTenantA, q)
	if err != nil || len(got.Names) != 0 {
		t.Fatal("empty scope leaked names")
	}
	q.ScopeKind = "tenant"
	got, err = store.CounterpartyNameSuggestions(ctx, intTenantA, q)
	if err != nil || len(got.Names) != 4 {
		t.Fatalf("tenant=%#v %v", got, err)
	}
	q.Limit = 2
	got, err = store.CounterpartyNameSuggestions(ctx, intTenantA, q)
	if err != nil || len(got.Names) != 2 {
		t.Fatal("unbounded results")
	}
	b = change(b, intTenantA, 3124, "特殊%_Alpha甲方")
	q.Limit = 10
	q.Query = "%_aLPHa"
	got, err = store.CounterpartyNameSuggestions(ctx, intTenantA, q)
	if err != nil || !reflect.DeepEqual(got.Names, []string{"特殊%_Alpha甲方"}) {
		t.Fatalf("literal=%#v %v", got, err)
	}
	// Check immutable history plus Project revisions/receipt and assignment counts.
	var after string
	if err := env.Pool.QueryRow(ctx, `SELECT md5(string_agg(row_to_json(h)::text,',' ORDER BY id)) FROM project_history_events h`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("fixture write not recorded")
	}
	q.Query = "甲方"
	counts := func() string {
		t.Helper()
		var out string
		if err := env.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM projects)||':'||(SELECT sum(revision) FROM projects)||':'||(SELECT count(*) FROM project_history_events)||':'||(SELECT count(*) FROM project_commands)||':'||(SELECT count(*) FROM project_assignments)`).Scan(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	beforeCounts := counts()
	if _, err := store.CounterpartyNameSuggestions(ctx, intTenantA, q); err != nil {
		t.Fatal(err)
	}
	var final string
	if err := env.Pool.QueryRow(ctx, `SELECT md5(string_agg(row_to_json(h)::text,',' ORDER BY id)) FROM project_history_events h`).Scan(&final); err != nil || final != after {
		t.Fatal("suggestion GET wrote history")
	}
	if counts() != beforeCounts {
		t.Fatal("suggestion GET changed counts or revisions")
	}
	current, _ := store.Get(ctx, intTenantA, b.ID, true)
	if current.Project.Revision != b.Revision {
		t.Fatal("suggestion GET changed revision")
	}
}
