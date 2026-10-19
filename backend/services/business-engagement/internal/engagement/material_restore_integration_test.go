//go:build integration

package engagement

import (
	"context"
	"net/url"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/business-engagement/internal/database"
	"omsbizfoundry/business-engagement/internal/testdatabase"

	"github.com/jackc/pgx/v5/pgxpool"
)

func materialDatabaseFacts(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, table := range []string{"schema_migrations", "business_engagements", "business_engagement_materials", "business_engagement_history_events", "business_engagement_commands"} {
		var value string
		if err := pool.QueryRow(context.Background(), `SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM `+table+` t`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		result[table] = value
	}
	return result
}

func ownedMaterialDatabaseURL(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	name := pool.Config().ConnConfig.Database
	if !strings.HasPrefix(name, "business_engagement_test_") {
		t.Fatal("backup source is not an owned integration database")
	}
	var current string
	if err := pool.QueryRow(context.Background(), `SELECT current_database()`).Scan(&current); err != nil || current != name {
		t.Fatal("backup source database does not match its live pool", err)
	}
	// pgx ConnString retains the original parsed URL even after Config.Database
	// is changed. Build the tooling URL for the verified live, owned database.
	value, err := url.Parse(testdatabase.RequireURL(t))
	if err != nil || (value.Scheme != "postgres" && value.Scheme != "postgresql") {
		t.Fatal("invalid owned database tooling URL", err)
	}
	value.Path, value.RawPath = "/"+name, ""
	return value.String()
}

func TestMaterialCollectionEmptyTargetPgRestoreAndContinue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s, source, actor, id := collectionFixture(t)
	firstInput := MaterialInput{CommandID: command(800), ExpectedRevision: 3, Operation: "append", RequirementKey: RequirementBidDocument, EvidenceFileID: testFile, EvidenceVersion: 1, EvidenceSourceRevision: 1}
	first, err := s.ApplyMaterial(ctx, actor, id, firstInput)
	if err != nil {
		t.Fatal(err)
	}
	secondInput := firstInput
	secondInput.CommandID, secondInput.ExpectedRevision, secondInput.EvidenceFileID = command(801), 4, testProject
	second, err := s.ApplyMaterial(ctx, actor, id, secondInput)
	if err != nil {
		t.Fatal(err)
	}
	award := firstInput
	award.CommandID, award.ExpectedRevision, award.RequirementKey, award.EvidenceFileID = command(802), 5, RequirementAwardDocument, intAwardFile
	ready, err := s.ApplyMaterial(ctx, actor, id, award)
	if err != nil || ready.Engagement.Status != StatusReady {
		t.Fatal("ready with two bid members", err)
	}
	remove := MaterialInput{CommandID: command(803), ExpectedRevision: 6, Operation: "remove_one", RequirementKey: RequirementBidDocument, MaterialReferenceID: currentMaterials(first, RequirementBidDocument)[0].ID}
	removed, err := s.ApplyMaterial(ctx, actor, id, remove)
	if err != nil || removed.Engagement.Status != StatusReady || len(removed.Materials) != 3 {
		t.Fatal("removing one must retain ready when another satisfies", err)
	}
	before := materialDatabaseFacts(t, source.pool)
	projection, err := s.ReferenceMaterials(ctx, testTenant, id)
	if err != nil {
		t.Fatal(err)
	}
	history, err := s.History(ctx, testTenant, id, 0, 200)
	if err != nil {
		t.Fatal(err)
	}
	target := testdatabase.New(t, testdatabase.RequireURL(t))
	sourceURL := ownedMaterialDatabaseURL(t, source.pool)
	targetURL := ownedMaterialDatabaseURL(t, target.Pool)
	if source.pool.Config().ConnConfig.Database == target.Name {
		t.Fatal("restore target must differ from source")
	}
	var targetTables int
	if err := target.Pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'`).Scan(&targetTables); err != nil || targetTables != 0 {
		t.Fatal("restore target is not empty", err)
	}
	archive := filepath.Join(t.TempDir(), "engagement.dump")
	// Real PostgreSQL tools, not a projection copy or in-memory reconstruction.
	if output, err := exec.CommandContext(ctx, "pg_dump", "--dbname", sourceURL, "--format=custom", "--no-owner", "--no-acl", "--file", archive).CombinedOutput(); err != nil {
		t.Fatalf("owned DB pg_dump failed: %v (diagnostic bytes=%d)", err, len(output))
	}
	if output, err := exec.CommandContext(ctx, "pg_restore", "--dbname", targetURL, "--exit-on-error", "--no-owner", "--no-acl", archive).CombinedOutput(); err != nil {
		t.Fatalf("empty target pg_restore failed: %v (diagnostic bytes=%d)", err, len(output))
	}
	if !reflect.DeepEqual(before, materialDatabaseFacts(t, target.Pool)) {
		t.Fatal("restore changed rows, IDs, snapshots, revisions or command hashes")
	}
	if err := database.Migrate(ctx, target.Pool); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, materialDatabaseFacts(t, target.Pool)) {
		t.Fatal("repeat migration changed restored facts")
	}
	restoredStore, _ := NewPostgresStore(target.Pool)
	restored, _ := NewService(restoredStore, stableReferences{})
	if got, err := restored.Get(ctx, testTenant, id); err != nil || !equalMaterialWire(t, got, removed) {
		t.Fatal("restored details", err)
	}
	if got, err := restored.ReferenceMaterials(ctx, testTenant, id); err != nil || !reflect.DeepEqual(got, projection) {
		t.Fatal("restored projection/digest", err)
	}
	if got, err := restored.History(ctx, testTenant, id, 0, 200); err != nil || !equalMaterialWire(t, got, history) {
		t.Fatal("restored history", err)
	}
	if got, err := restored.ApplyMaterial(ctx, actor, id, secondInput); err != nil || !equalMaterialWire(t, got, second) {
		t.Fatal("restored append receipt replay", err)
	}
	if got, err := restored.ApplyMaterial(ctx, actor, id, remove); err != nil || !equalMaterialWire(t, got, removed) {
		t.Fatal("restored remove receipt replay", err)
	}
	if !reflect.DeepEqual(before, materialDatabaseFacts(t, target.Pool)) {
		t.Fatal("restored replay wrote")
	}
	remove.CommandID, remove.ExpectedRevision, remove.MaterialReferenceID = command(804), 7, currentMaterials(removed, RequirementBidDocument)[0].ID
	continued, err := restored.ApplyMaterial(ctx, actor, id, remove)
	if err != nil || continued.Engagement.Revision != 8 || continued.Engagement.Status != StatusCollectingEvidence || len(continued.Materials) != 3 {
		t.Fatal("continue must only end last bid reference", err)
	}
	if !reflect.DeepEqual(before, materialDatabaseFacts(t, source.pool)) {
		t.Fatal("restore/continuation changed source DB")
	}
	t.Log("actual pg_dump/empty-target pg_restore preserved all five tables; old receipts replayed zero-write; restored owner continued at revision 8")
}
