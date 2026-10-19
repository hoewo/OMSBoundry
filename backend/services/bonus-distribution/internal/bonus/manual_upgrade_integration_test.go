//go:build integration

package bonus_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/bonus-distribution/internal/bonus"
	"omsbizfoundry/bonus-distribution/internal/database"
	"omsbizfoundry/bonus-distribution/internal/overlays/referenceoa"
	rb "omsbizfoundry/bonus-distribution/internal/referencebonus"
)

type manualUpgradeResolver struct{ *resolverFixture }

func (r manualUpgradeResolver) ResolveManualParticipant(ctx context.Context, tenant string, p bonus.ProjectSnapshot, scope string, person bonus.PersonRef, assignment bonus.AssignmentRef) (bonus.ParticipantReference, error) {
	v, e := r.ResolveParticipant(ctx, tenant, p, scope, person, assignment)
	if strings.HasSuffix(v.Assignment.RoleKey, "manager") {
		v.Assignment.RoleKey = "business_owner"
	} else {
		v.Assignment.RoleKey = "commercial_contributor"
	}
	return v, e
}
func (r manualUpgradeResolver) ListManualCandidates(context.Context, string, bonus.ProjectRef, string) ([]bonus.ParticipantReference, error) {
	return nil, nil
}

func TestSchema2To3PreservesLegacyPoolSuggestionsAndCommandReplay(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx, true)
	defer cleanup()
	store, _ := bonus.NewPostgresStore(pool)
	actor, resolver := fixture()
	actor.Actions = append(actor.Actions, bonus.ActionSuggestionCreate, bonus.ActionSuggestionRead)
	service, _ := bonus.NewService(store, resolver, referenceoa.Evaluator{})
	draft, e := service.CreateBusinessDraft(ctx, actor, businessDraftInput(910, resolver))
	if e != nil {
		t.Fatal(e)
	}
	published, e := service.Publish(ctx, actor, draft.ID, bonus.PoolCommandInput{CommandID: id(911), ExpectedRevision: 1, Reason: "升级前已发布"})
	if e != nil {
		t.Fatal(e)
	}
	suggestion, e := service.ProposePayableSuggestion(ctx, actor, bonus.ProposePayableSuggestionInput{CommandID: id(912), AllocationID: published.Allocations[0].ID, AllocationRevision: published.Allocations[0].Revision, LedgerOwner: bonus.MoneyPartyRef{ID: id(80), Revision: 1, Kind: "organization", LegalName: "原设计单位"}, DueOn: "2026-10-31", Purpose: "升级前独立应付建议"})
	if e != nil || suggestion.State != bonus.SuggestionTargetUnavailable {
		t.Fatalf("suggestion=%+v %v", suggestion, e)
	}
	digest := legacyBonusDigest(t, pool)
	if e = database.Migrate(ctx, pool); e != nil {
		t.Fatal(e)
	}
	if _, e = database.RequireCurrentSchema(ctx, pool); e != nil {
		t.Fatal(e)
	}
	if legacyBonusDigest(t, pool) != digest {
		t.Fatal("migration rewrote legacy facts")
	}
	mstore, _ := rb.NewPostgresStore(pool)
	manual, _ := rb.New(mstore, manualUpgradeResolver{resolver})
	ma := actor
	ma.SubjectKind = "iam_subject"
	ma.TenantScope = false
	ma.ResourceScopes = []bonus.ResourceScope{{Type: "business_bonus_project", ID: resolver.project.ID}}
	ma.Actions = rb.Actions
	p := resolver.participants[bonus.TypeBusiness+":"+id(41)]
	rows, e := manual.Create(ctx, ma, rb.CreateInput{CommandID: id(913), ProjectRef: bonus.ProjectRef{ID: resolver.project.ID, Revision: 7}, Scope: "business", BaseAmountMinor: "101", OccurredOn: "2026-10-04", Items: []rb.ItemInput{{PersonRef: bonus.PersonRef{ID: p.Person.ID, Revision: p.Person.Revision}, AssignmentRef: bonus.AssignmentRef{ID: p.Assignment.ID, Revision: p.Assignment.Revision}, Method: "fixed", FixedAmountMinor: "202"}}})
	if e != nil || len(rows) != 1 {
		t.Fatalf("manual=%+v %v", rows, e)
	}
	if legacyBonusDigest(t, pool) != digest {
		t.Fatal("manual overlay changed confirmed-source Pool or Money suggestion")
	}
	resolver.failProject = bonus.ErrReferenceUnavailable
	replay, e := service.Publish(ctx, actor, draft.ID, bonus.PoolCommandInput{CommandID: id(911), ExpectedRevision: 1, Reason: "升级前已发布"})
	if e != nil || replay.ID != published.ID || replay.Revision != published.Revision {
		t.Fatalf("legacy replay=%+v %v", replay, e)
	}
	if legacyBonusDigest(t, pool) != digest {
		t.Fatal("legacy replay appended facts")
	}
}
func legacyBonusDigest(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	out := ""
	for _, table := range []string{"bonus_pools", "bonus_pool_revisions", "bonus_allocations", "bonus_allocation_revisions", "bonus_distribution_commands", "bonus_distribution_audit_events", "bonus_payable_suggestions", "bonus_payable_suggestion_revisions"} {
		var part string
		e := pool.QueryRow(context.Background(), `SELECT md5(COALESCE(string_agg(row_to_json(t)::text,',' ORDER BY row_to_json(t)::text),'')) FROM `+table+` t`).Scan(&part)
		if e != nil {
			t.Fatal(e)
		}
		out += part
	}
	return out
}
