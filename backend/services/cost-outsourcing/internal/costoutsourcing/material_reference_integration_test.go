//go:build integration

package costoutsourcing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type materialNoUpstream struct{ ReferenceResolver }

func materialOwnedFacts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	facts := make([][2]string, 0, 10)
	for _, table := range []string{"cost_entries", "cost_entry_revisions", "outsourcing_engagements", "outsourcing_engagement_revisions", "outsourcing_assessment_revisions", "cost_outsourcing_commands", "cost_outsourcing_audit_events", "payable_suggestions", "payable_suggestion_revisions", "schema_migrations"} {
		var value string
		query := `SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM ` + pgx.Identifier{table}.Sanitize() + ` t`
		if err := pool.QueryRow(context.Background(), query).Scan(&value); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, [2]string{table, value})
	}
	raw, _ := json.Marshal(facts)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
func TestCostMaterialPostgresBothAggregatesCurrentHistoryAndZeroWrites(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := integrationPool(t, ctx)
	defer cleanup()
	store, _ := NewPostgresStore(pool)
	actor, resolver := integrationFixture()
	writer, _ := NewService(store, resolver)
	reader, _ := NewService(store, &materialNoUpstream{})
	cost, err := writer.RecordCost(ctx, actor, RecordCostInput{CommandID: id(50), ProjectRef: ProjectRef{ID: resolver.project.ID, Revision: 7}, Category: CategoryOther, OccurredOn: "2026-10-01", Currency: "CNY", AmountMinor: 125, Description: "synthetic cost", EvidenceRefs: []EvidenceRef{resolver.evidence.EvidenceRef}})
	if err != nil {
		t.Fatal(err)
	}
	engagement, err := writer.RecordEngagement(ctx, actor, RecordEngagementInput{CommandID: id(51), ProjectRef: ProjectRef{ID: resolver.project.ID, Revision: 7}, ProviderRef: PartyRef{ID: resolver.party.ID, Revision: 5}, Scope: "synthetic outsourcing", Currency: "CNY", AmountMinor: 500, EvidenceRefs: []EvidenceRef{resolver.evidence.EvidenceRef}})
	if err != nil {
		t.Fatal(err)
	}
	read := func(kind string, revision, version int64, state string) MaterialReferenceProjection {
		t.Helper()
		before := materialOwnedFacts(t, pool)
		readActor := actor
		readActor.ProjectIDs = nil
		var projection MaterialReferenceProjection
		var err error
		if kind == "cost_entry" {
			readActor.Actions = []string{ActionCostRead}
			readActor.ResourceScopes = []ResourceScope{{Type: kind, ID: cost.ID}}
			value, getErr := reader.GetCost(ctx, readActor, cost.ID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			projection, err = CostMaterialReference(value)
		} else {
			readActor.Actions = []string{ActionEngageRead}
			readActor.ResourceScopes = []ResourceScope{{Type: kind, ID: engagement.ID}}
			value, getErr := reader.GetEngagement(ctx, readActor, engagement.ID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			projection, err = EngagementMaterialReference(value)
		}
		if err != nil || projection.ResourceType != kind || projection.Revision != revision || projection.Status != state || len(projection.Items) != 1 || projection.Items[0].Version != version || projection.Items[0].EvidenceSourceRevision != version {
			t.Fatalf("projection=%#v err=%v", projection, err)
		}
		if materialOwnedFacts(t, pool) != before {
			t.Fatal("source read changed one of ten owned tables")
		}
		return projection
	}
	read("cost_entry", 1, 2, StateRecorded)
	read("outsourcing_engagement", 1, 2, StateRecorded)
	resolver.evidence.Version = 3
	resolver.evidence.DisplayName = "revised-receipt.pdf"
	if _, err := writer.CorrectCost(ctx, actor, cost.ID, CorrectCostInput{CommandID: id(52), ExpectedRevision: 1, Reason: "replace evidence", Category: cost.Category, OccurredOn: cost.OccurredOn, Currency: cost.Currency, AmountMinor: cost.AmountMinor, Description: cost.Description, EvidenceRefs: []EvidenceRef{resolver.evidence.EvidenceRef}}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.CorrectEngagement(ctx, actor, engagement.ID, CorrectEngagementInput{CommandID: id(53), ExpectedRevision: 1, Reason: "replace evidence", ProviderRef: PartyRef{ID: engagement.Provider.ID, Revision: engagement.Provider.Revision}, Scope: engagement.Scope, Currency: engagement.Currency, AmountMinor: engagement.AmountMinor, EvidenceRefs: []EvidenceRef{resolver.evidence.EvidenceRef}}); err != nil {
		t.Fatal(err)
	}
	secondCost := read("cost_entry", 2, 3, StateRecorded)
	secondEngagement := read("outsourcing_engagement", 2, 3, StateRecorded)
	costHistory, err := writer.CostHistory(ctx, actor, cost.ID)
	if err != nil || len(costHistory) != 2 || costHistory[0].Snapshot.Evidence[0].Version != 2 {
		t.Fatal("cost v2 history lost")
	}
	engageHistory, err := writer.EngagementHistory(ctx, actor, engagement.ID)
	if err != nil || len(engageHistory) != 2 || engageHistory[0].Snapshot.Evidence[0].Version != 2 {
		t.Fatal("engagement v2 history lost")
	}
	if _, err := writer.WithdrawCost(ctx, actor, cost.ID, WithdrawInput{CommandID: id(54), ExpectedRevision: 2, Reason: "synthetic withdrawal"}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WithdrawEngagement(ctx, actor, engagement.ID, WithdrawInput{CommandID: id(55), ExpectedRevision: 2, Reason: "synthetic withdrawal"}); err != nil {
		t.Fatal(err)
	}
	if read("cost_entry", 3, 3, StateWithdrawn).Digest != secondCost.Digest || read("outsourcing_engagement", 3, 3, StateWithdrawn).Digest != secondEngagement.Digest {
		t.Fatal("withdrawal removed frozen file references")
	}
	before := materialOwnedFacts(t, pool)
	wrong := actor
	wrong.TenantID = id(98)
	if _, err := reader.GetCost(ctx, wrong, cost.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant err=%v", err)
	}
	if materialOwnedFacts(t, pool) != before {
		t.Fatal("denied read changed facts")
	}
	t.Log("PG18 both aggregates replace v2→v3 and preserve v2 history; withdrawn v3 references remain; ten table read digests unchanged; exact resource read only; upstream resolver unavailable")
}
