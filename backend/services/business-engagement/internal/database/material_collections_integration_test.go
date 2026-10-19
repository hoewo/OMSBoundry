//go:build integration

package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"omsbizfoundry/business-engagement/internal/engagement"
	"omsbizfoundry/business-engagement/internal/testdatabase"

	"github.com/jackc/pgx/v5/pgxpool"
)

func legacyReceiptEqual(t *testing.T, left, right engagement.Details) bool {
	t.Helper()
	a, err := json.Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(right)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(a, b)
}

type collectionReferences struct{}

func (collectionReferences) ValidateProject(_ context.Context, tenant, id string, revision int64) (engagement.ProjectReference, error) {
	return engagement.ProjectReference{ID: id, TenantID: tenant, ResourceType: "project", Status: "active", Revision: revision}, nil
}
func (collectionReferences) ValidateEvidence(_ context.Context, tenant, id string, version int, revision int64) (engagement.EvidenceReference, error) {
	return engagement.EvidenceReference{FileID: id, TenantID: tenant, Version: version, Revision: revision, DisplayName: "材料.pdf"}, nil
}

func collectionFacts(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, table := range []string{"business_engagements", "business_engagement_materials", "business_engagement_history_events", "business_engagement_commands"} {
		var value string
		if err := pool.QueryRow(context.Background(), `SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM `+table+` t`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		result[table] = value
	}
	return result
}

func TestMaterialCollectionsMigrationPreservesLegacyRowsReceiptsAndReplay(t *testing.T) {
	ctx := context.Background()
	env := testdatabase.New(t, testdatabase.RequireURL(t))
	old, err := migrationFiles.ReadFile("migrations/000001_business_engagement.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = env.Pool.Exec(ctx, string(old)); err != nil {
		t.Fatal(err)
	}
	if _, err = env.Pool.Exec(ctx, `CREATE TABLE schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT clock_timestamp()); INSERT INTO schema_migrations(version) VALUES('000001_business_engagement.sql')`); err != nil {
		t.Fatal(err)
	}
	const tenant = "11111111-1111-4111-8111-111111111111"
	const subject = "22222222-2222-4222-8222-222222222222"
	const project = "33333333-3333-4333-8333-333333333333"
	const oldFile = "44444444-4444-4444-8444-444444444444"
	const material = "55555555-5555-4555-8555-555555555555"
	const putCommand = "66666666-6666-4666-8666-666666666666"
	store, _ := engagement.NewPostgresStore(env.Pool)
	service, _ := engagement.NewService(store, collectionReferences{})
	actor := engagement.Actor{ID: subject, TenantID: tenant, Kind: "test_subject"}
	created, err := service.Create(ctx, actor, engagement.CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-000000000001", ProjectID: project, ProjectSourceRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	id := created.Engagement.ID
	if _, err = service.SetApplicability(ctx, actor, id, engagement.ApplicabilityInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-000000000002", ExpectedRevision: 1, Applicable: true}); err != nil {
		t.Fatal(err)
	}
	method, err := service.SelectMethod(ctx, actor, id, engagement.MethodInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-000000000003", ExpectedRevision: 2, Method: engagement.MethodComparison})
	if err != nil {
		t.Fatal(err)
	}
	// Encode the actual legacy single-member contract without the additive field.
	input := engagement.MaterialInput{CommandID: putCommand, ExpectedRevision: 3, Operation: "put", RequirementKey: engagement.RequirementBidDocument, EvidenceFileID: oldFile, EvidenceVersion: 1, EvidenceSourceRevision: 1}
	legacyBody := struct {
		CommandID              string `json:"command_id"`
		ExpectedRevision       int64  `json:"expected_revision"`
		Operation              string `json:"operation"`
		RequirementKey         string `json:"requirement_key"`
		EvidenceFileID         string `json:"evidence_file_id"`
		EvidenceVersion        int    `json:"evidence_version"`
		EvidenceSourceRevision int64  `json:"evidence_source_revision"`
	}{putCommand, 3, "put", engagement.RequirementBidDocument, oldFile, 1, 1}
	body, _ := json.Marshal(legacyBody)
	sum := sha256.Sum256(body)
	now := time.Now().UTC().Truncate(time.Microsecond)
	method.Engagement.Revision = 4
	method.Engagement.UpdatedAt = now
	ref := engagement.MaterialReference{ID: material, TenantID: tenant, EngagementID: id, RequirementKey: engagement.RequirementBidDocument, EvidenceFileID: oldFile, EvidenceVersion: 1, EvidenceSourceRevision: 1, DisplaySnapshot: "旧材料.pdf", CreatedEngagementRevision: 4, CreatedAt: now}
	legacy := engagement.BuildDetails(method.Engagement, []engagement.MaterialReference{ref})
	for i := range legacy.Requirements {
		legacy.Requirements[i].MaterialReferences = nil
	}
	response, _ := json.Marshal(legacy)
	tx, err := env.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE business_engagements SET revision=4,updated_at=$2 WHERE id=$1`, id, now); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO business_engagement_materials(id,tenant_id,engagement_id,requirement_key,evidence_file_id,evidence_version,evidence_source_revision,display_snapshot,created_engagement_revision,created_at) VALUES($1,$2,$3,'bid_document',$4,1,1,'旧材料.pdf',4,$5)`, material, tenant, id, oldFile, now); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO business_engagement_history_events(id,tenant_id,engagement_id,engagement_revision,event_sequence,event_type,command_id,actor_subject,occurred_at,payload) VALUES('77777777-7777-4777-8777-777777777777',$1,$2,4,4,'material_put',$3,$4,$5,'{}')`, tenant, id, putCommand, subject, now); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO business_engagement_commands(tenant_id,command_id,request_hash,response_json,created_at) VALUES($1,$2,$3,$4,$5)`, tenant, putCommand, hex.EncodeToString(sum[:]), response, now); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	before := collectionFacts(t, env.Pool)
	if _, err := RequireCurrentSchema(ctx, env.Pool); err == nil {
		t.Fatal("new binary accepted old schema")
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := Migrate(ctx, env.Pool); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, collectionFacts(t, env.Pool)) {
			t.Fatal("migration changed old business rows/receipts")
		}
	}
	if version, err := RequireCurrentSchema(ctx, env.Pool); err != nil || version != "000002_material_collections.sql" {
		t.Fatal(version, err)
	}
	replay, err := service.ApplyMaterial(ctx, actor, id, input)
	if err != nil || !legacyReceiptEqual(t, legacy, replay) {
		t.Fatalf("legacy replay=%#v err=%v", replay, err)
	}
	if !reflect.DeepEqual(before, collectionFacts(t, env.Pool)) {
		t.Fatal("old replay wrote")
	}
	input.CommandID, input.ExpectedRevision, input.Operation, input.EvidenceFileID = "aaaaaaaa-aaaa-4aaa-8aaa-000000000005", 4, "append", project
	appended, err := service.ApplyMaterial(ctx, actor, id, input)
	if err != nil || len(appended.Requirements[1].MaterialReferences) != 2 || appended.Materials[0].SupersededAt != nil {
		t.Fatalf("append after upgrade=%#v err=%v", appended, err)
	}
	input.CommandID, input.ExpectedRevision, input.Operation, input.EvidenceFileID = putCommand, 3, "put", oldFile
	if replay, err := service.ApplyMaterial(ctx, actor, id, input); err != nil || !legacyReceiptEqual(t, legacy, replay) {
		t.Fatal("legacy replay changed after append", err)
	}
	if _, err := env.Pool.Exec(ctx, `UPDATE business_engagement_commands SET request_hash=request_hash WHERE command_id=$1`, putCommand); err == nil {
		t.Fatal("immutable receipt guard lost")
	}
	if _, err := env.Pool.Exec(ctx, `UPDATE business_engagement_history_events SET payload=payload WHERE command_id=$1`, putCommand); err == nil {
		t.Fatal("immutable history guard lost")
	}
}
