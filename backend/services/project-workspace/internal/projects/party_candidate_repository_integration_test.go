//go:build integration

package projects

import (
	"context"
	"errors"
	"testing"

	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

func TestPostgresPartyCandidateRegistrationIsIdempotentAndTenantScoped(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	actor := Actor{ID: intActorA, TenantID: intTenantA}
	input := PartyCandidateInput{CommandID: command(900), PartyID: intContract, SourceRevision: 3, RoleKey: PartyCandidateRoleContractor}

	created, err := store.RegisterPartyCandidate(ctx, actor, input)
	if err != nil || created.PartyID != intContract || created.RoleKey != PartyCandidateRoleContractor || created.SourceRevision != 3 {
		t.Fatalf("created=%#v error=%v", created, err)
	}
	replayed, err := store.RegisterPartyCandidate(ctx, actor, input)
	if err != nil || replayed != created {
		t.Fatalf("replayed=%#v error=%v", replayed, err)
	}
	conflict := input
	conflict.RoleKey = PartyCandidateRoleClient
	if _, err := store.RegisterPartyCandidate(ctx, actor, conflict); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("command conflict=%v", err)
	}
	secondCommand := input
	secondCommand.CommandID = command(901)
	secondCommand.SourceRevision = 4
	again, err := store.RegisterPartyCandidate(ctx, actor, secondCommand)
	if err != nil || again != created {
		t.Fatalf("second registration=%#v error=%v", again, err)
	}
	items, err := store.ListPartyCandidates(ctx, intTenantA)
	if err != nil || len(items) != 1 || items[0] != created {
		t.Fatalf("items=%#v error=%v", items, err)
	}
	other, err := store.ListPartyCandidates(ctx, intTenantB)
	if err != nil || len(other) != 0 {
		t.Fatalf("other tenant items=%#v error=%v", other, err)
	}
	if _, err := environment.Pool.Exec(ctx, `DELETE FROM project_party_candidates WHERE tenant_id=$1 AND party_id=$2`, intTenantA, intContract); err == nil {
		t.Fatal("immutable candidate registration was deleted")
	}
}
