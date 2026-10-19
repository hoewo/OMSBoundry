//go:build integration

package projects

import (
	"context"
	"errors"
	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/testdatabase"
	"reflect"
	"sync"
	"testing"
)

func TestPostgresCounterpartyContextIsLocalHistoricalAndReplayable(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	for index, node := range []string{NodeTypeProject, NodeTypePackage} {
		t.Run(node, func(t *testing.T) {
			seq := 2100 + index*30
			p, err := store.DispatchInitiation(ctx, actor, InitiationDispatchInput{CommandID: command(seq), NodeType: node, Name: "项目", BusinessOwner: InitiationOwnerReference{PersonID: intPersonA, SourceRevision: 1}, ownerSnapshot: "经营负责人"})
			if err != nil {
				t.Fatal(err)
			}
			number := "CONTEXT-001"
			if node == NodeTypePackage {
				number = ""
			}
			p, err = store.Update(ctx, actor, p.ID, UpdateInput{CreateInput: CreateInput{CommandID: command(seq + 1), Name: p.Name, ProjectNumber: number, ProjectTypeKeys: []string{"design"}}, ExpectedRevision: p.Revision})
			if err != nil {
				t.Fatal(err)
			}
			pending := CounterpartyContextInput{CommandID: command(seq + 2), ExpectedRevision: p.Revision, Reason: "资料录入", CounterpartyContact: CounterpartyContact{Name: "甲方"}}
			if _, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, pending); !errors.Is(err, ErrStateConflict) {
				t.Fatalf("pending=%v", err)
			}
			p, err = store.CompleteInitiation(ctx, actor, p.ID, RevisionCommand{CommandID: command(seq + 3), ExpectedRevision: p.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
			if err != nil {
				t.Fatal(err)
			}
			before, _ := store.Get(ctx, actor.TenantID, p.ID, true)
			empty, err := store.CounterpartyContext(ctx, actor.TenantID, p.ID)
			if err != nil || empty.Context != nil || empty.ProjectRevision != p.Revision {
				t.Fatalf("empty=%#v %v", empty, err)
			}
			in := CounterpartyContextInput{CommandID: command(seq + 4), ExpectedRevision: p.Revision, Reason: "录入本项目甲方", CounterpartyContact: CounterpartyContact{Name: "某局", ContactName: "李工", ContactPhone: "+86 (0571) 123-456"}}
			first, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, in)
			if err != nil || first.ProjectRevision != p.Revision+1 || first.Context.RecordedProjectRevision != first.ProjectRevision {
				t.Fatalf("first=%#v %v", first, err)
			}
			replay, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, in)
			if err != nil || !reflect.DeepEqual(first, replay) {
				t.Fatalf("replay=%#v %v", replay, err)
			}
			altered := in
			altered.Name = "另一个甲方"
			if _, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, altered); !errors.Is(err, ErrCommandConflict) {
				t.Fatalf("collision=%v", err)
			}
			in.CommandID = command(seq + 5)
			if _, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, in); !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("stale=%v", err)
			}
			if _, err := store.CounterpartyContext(ctx, intTenantB, p.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("tenant read=%v", err)
			}
			if _, err := store.ChangeCounterpartyContext(ctx, Actor{ID: intActorA, TenantID: intTenantB}, p.ID, in); !errors.Is(err, ErrNotFound) {
				t.Fatalf("tenant write=%v", err)
			}
			in.ExpectedRevision = first.ProjectRevision
			noop, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, in)
			if err != nil || noop.ProjectRevision != first.ProjectRevision {
				t.Fatalf("noop=%#v %v", noop, err)
			}
			in.CommandID = command(seq + 6)
			in.ContactName = ""
			in.ContactPhone = ""
			second, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, in)
			if err != nil || second.Context.ContactName != "" || second.ProjectRevision != first.ProjectRevision+1 {
				t.Fatalf("clear=%#v %v", second, err)
			}
			after, _ := store.Get(ctx, actor.TenantID, p.ID, true)
			if after.Project.ID != before.Project.ID || after.Project.Name != before.Project.Name || !reflect.DeepEqual(after.Assignments, before.Assignments) || !reflect.DeepEqual(after.Project.ClientParty, before.Project.ClientParty) || !reflect.DeepEqual(after.ResourceLinks, before.ResourceLinks) {
				t.Fatal("context changed unrelated facts")
			}
			history, _ := store.History(ctx, actor.TenantID, p.ID, "", 100)
			if len(history.Items) != 5 || history.Items[0].Payload["schema_version"] != CounterpartyContextSchema || history.Items[1].Payload["context"].(map[string]any)["contact_name"] != "李工" {
				t.Fatalf("history=%#v", history)
			}
			if _, err := env.Pool.Exec(ctx, `UPDATE project_history_events SET payload='{}' WHERE tenant_id=$1 AND project_id=$2`, actor.TenantID, p.ID); err == nil {
				t.Fatal("history mutable")
			}
			cancelled, err := store.Cancel(ctx, actor, p.ID, CancelCommand{RevisionCommand: RevisionCommand{CommandID: command(seq + 7), ExpectedRevision: second.ProjectRevision}, Reason: "终止"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, in); err != nil {
				t.Fatalf("cancel replay=%v", err)
			}
			in.CommandID = command(seq + 8)
			in.ExpectedRevision = cancelled.Revision
			if _, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, in); !errors.Is(err, ErrStateConflict) {
				t.Fatalf("cancel new=%v", err)
			}
		})
	}
}
func TestPostgresCounterpartyContextRollbackAndConcurrentRevision(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, env.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(env.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	p, err := store.DispatchInitiation(ctx, actor, InitiationDispatchInput{CommandID: command(2200), NodeType: NodeTypePackage, Name: "包", BusinessOwner: InitiationOwnerReference{PersonID: intPersonA, SourceRevision: 1}, ownerSnapshot: "经营"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = store.Update(ctx, actor, p.ID, UpdateInput{CreateInput: CreateInput{CommandID: command(2209), Name: p.Name, ProjectTypeKeys: []string{"design"}}, ExpectedRevision: p.Revision})
	if err != nil {
		t.Fatal(err)
	}
	p, err = store.CompleteInitiation(ctx, actor, p.ID, RevisionCommand{CommandID: command(2201), ExpectedRevision: p.Revision, InitiationProfile: InitiationProfileBusinessOwnerV1})
	if err != nil {
		t.Fatal(err)
	}
	in := CounterpartyContextInput{CommandID: command(2202), ExpectedRevision: p.Revision, Reason: "录入", CounterpartyContact: CounterpartyContact{Name: "甲方"}}
	_, err = env.Pool.Exec(ctx, `CREATE FUNCTION reject_context_probe() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected context failure'; END $$; CREATE TRIGGER reject_context_probe BEFORE INSERT ON project_history_events FOR EACH ROW EXECUTE FUNCTION reject_context_probe()`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, in); err == nil {
		t.Fatal("fault unexpectedly committed")
	}
	view, _ := store.CounterpartyContext(ctx, actor.TenantID, p.ID)
	if view.Context != nil || view.ProjectRevision != p.Revision {
		t.Fatal("partial context write")
	}
	var receipts int
	_ = env.Pool.QueryRow(ctx, `SELECT count(*) FROM project_commands WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, in.CommandID).Scan(&receipts)
	if receipts != 0 {
		t.Fatal("failed receipt stored")
	}
	if _, err := env.Pool.Exec(ctx, `DROP TRIGGER reject_context_probe ON project_history_events; DROP FUNCTION reject_context_probe()`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, in); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			v := in
			v.CommandID = command(2203 + index)
			v.ExpectedRevision = p.Revision + 1
			v.Name = []string{"甲方一", "甲方二"}[index]
			_, err := store.ChangeCounterpartyContext(ctx, actor, p.ID, v)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, ErrRevisionConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("concurrent=%d/%d", success, conflict)
	}
}
