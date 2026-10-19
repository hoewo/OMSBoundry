//go:build integration

package projects_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/projects"
	"omsbizfoundry/project-workspace/internal/references"
	"omsbizfoundry/project-workspace/internal/testdatabase"
)

const materialTenant = "11111111-1111-4111-8111-111111111111"
const materialActor = "22222222-2222-4222-8222-222222222222"
const materialWork = "33333333-3333-4333-8333-333333333333"
const materialFile = "44444444-4444-4444-8444-444444444444"

type materialCredentialCall struct{ audience, action, resourceType, resourceID string }
type materialCredentials struct {
	mu    sync.Mutex
	calls []materialCredentialCall
}

func (s *materialCredentials) Credential(_ context.Context, audience, action, resourceType, resourceID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, materialCredentialCall{audience, action, resourceType, resourceID})
	return "integration-bound-capability", nil
}
func (*materialCredentials) Invalidate(string, string, string, string) {}

func projectMaterialFacts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, table)
	}
	rows.Close()
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	facts := make([][2]string, 0, len(tables))
	for _, table := range tables {
		var canonical string
		query := `SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM ` + pgx.Identifier{"public", table}.Sanitize() + ` t`
		if err := pool.QueryRow(context.Background(), query).Scan(&canonical); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, [2]string{table, canonical})
	}
	raw, _ := json.Marshal(facts)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// Real Project PG18 + typed HTTP ports; upstream bodies and credentials are
// explicit test providers, not evidence of the full Nginx/Gateway/IAM journey.
func TestProductionMaterialProjectPostgresHTTPCompositionAndEmptyRestore(t *testing.T) {
	for _, binary := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Skip(binary + " is required for the explicit restore test")
		}
	}
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := projects.NewPostgresStore(environment.Pool)
	actor := projects.Actor{ID: materialActor, TenantID: materialTenant, Kind: "test_subject"}
	cmd := func(n int) string { return fmt.Sprintf("90000000-0000-4000-8000-%012d", n) }
	project, err := store.Create(ctx, actor, projects.CreateInput{CommandID: cmd(1), ProjectNumber: "C4-MATERIAL-CONSUMER", Name: "资料组合隔离验证", ProjectTypeKeys: []string{"consultation"}})
	if err != nil {
		t.Fatal(err)
	}
	var upstreamMu sync.Mutex
	sourceProject, sourceRevision, sourceStatus, fileVersion := project.ID, int64(5), http.StatusOK, 2
	itemsEmpty, sourceCalls := false, 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamMu.Lock()
		defer upstreamMu.Unlock()
		if r.Header.Get("Authorization") != "Bearer integration-bound-capability" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/api/v1/references/production-work-items/" + materialWork + "/materials":
			sourceCalls++
			if sourceStatus != http.StatusOK {
				w.WriteHeader(sourceStatus)
				return
			}
			// Field order below is the canonical v1 wire order.
			rawItems := `[{"relation_key":"report.primary-output","file_id":"` + materialFile + `","version":2,"evidence_source_revision":2,"display_name":"成果.pdf"}]`
			if itemsEmpty {
				rawItems = "[]"
			}
			digest := sha256.Sum256([]byte(rawItems))
			fmt.Fprintf(w, `{"schema_version":"1","source_application":"production-operations","id":%q,"tenant_id":%q,"project_id":%q,"resource_type":"production-work-item","status":"in-progress","revision":%d,"digest":%q,"items":%s}`, materialWork, materialTenant, sourceProject, sourceRevision, hex.EncodeToString(digest[:]), rawItems)
		case "/v1/references/files/" + materialFile:
			fmt.Fprintf(w, `{"schema_version":"1","source_application":"evidence-service","id":%q,"tenant_id":%q,"resource_type":"evidence-file","status":"available","revision":%d,"version":%d,"file_name":"成果.pdf","size_bytes":64,"media_type":"application/pdf","sha256":%q}`, materialFile, materialTenant, fileVersion, fileVersion, strings.Repeat("a", 64))
		default:
			t.Errorf("unexpected typed reference path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	credentials := &materialCredentials{}
	validator, err := references.NewHTTPValidator(time.Second, references.Endpoint{}, map[string]references.Endpoint{
		"production-operations": {BaseURL: upstream.URL, Audience: "production-operations", Action: "production-work-item.reference.read", Credentials: credentials},
		"evidence-service":      {BaseURL: upstream.URL, Audience: "evidence-service", Action: "evidence-service.metadata.read", Credentials: credentials},
	})
	if err != nil {
		t.Fatal(err)
	}
	service, _ := projects.NewService(store, validator)
	revision := int64(5)
	input := projects.ResourceLinkCommand{CommandID: cmd(2), ExpectedRevision: 1, Operation: "link", TargetService: "production-operations", ResourceType: "production-work-item", ResourceID: materialWork, SourceRevision: &revision, RelationKey: "production-materials"}
	before := projectMaterialFacts(t, environment.Pool)
	upstreamMu.Lock()
	sourceProject = materialActor
	upstreamMu.Unlock()
	if _, err := service.ApplyResourceLink(ctx, actor, project.ID, input); err == nil {
		t.Fatal("cross-project link accepted")
	}
	if projectMaterialFacts(t, environment.Pool) != before {
		t.Fatal("wrong source changed Project facts")
	}
	upstreamMu.Lock()
	sourceProject = project.ID
	upstreamMu.Unlock()
	linked, err := service.ApplyResourceLink(ctx, actor, project.ID, input)
	if err != nil || linked.Project.Revision != 2 {
		t.Fatalf("link=%#v err=%v", linked, err)
	}
	before = projectMaterialFacts(t, environment.Pool)
	upstreamMu.Lock()
	sourceStatus = 503
	callsBeforeReplay := sourceCalls
	upstreamMu.Unlock()
	replayed, err := service.ApplyResourceLink(ctx, actor, project.ID, input)
	upstreamMu.Lock()
	callsAfterReplay := sourceCalls
	sourceStatus = 200
	upstreamMu.Unlock()
	if err != nil || replayed.ResourceLink.ID != linked.ResourceLink.ID || callsBeforeReplay != callsAfterReplay || projectMaterialFacts(t, environment.Pool) != before {
		t.Fatal("idempotent replay depended on source or changed facts")
	}
	assertRead := func(state string) {
		t.Helper()
		value, err := service.Materials(ctx, materialTenant, project.ID)
		if err != nil || len(value.Segments) != 1 || value.Segments[0].State != state || value.ProjectRevision != 2 {
			t.Fatalf("materials=%#v err=%v", value, err)
		}
		if projectMaterialFacts(t, environment.Pool) != before {
			t.Fatal("projection GET changed owned facts")
		}
	}
	assertRead(projects.MaterialSegmentAvailable)
	upstreamMu.Lock()
	sourceRevision = 6
	upstreamMu.Unlock()
	assertRead(projects.MaterialSegmentRevisionChanged)
	upstreamMu.Lock()
	sourceRevision = 5
	fileVersion = 3
	upstreamMu.Unlock()
	assertRead(projects.MaterialSegmentRevisionChanged)
	upstreamMu.Lock()
	fileVersion = 2
	sourceStatus = 503
	upstreamMu.Unlock()
	assertRead(projects.MaterialSegmentUnavailable)
	upstreamMu.Lock()
	sourceStatus = 403
	upstreamMu.Unlock()
	assertRead(projects.MaterialSegmentForbidden)
	upstreamMu.Lock()
	sourceStatus = 200
	itemsEmpty = true
	upstreamMu.Unlock()
	assertRead(projects.MaterialSegmentEmpty)
	upstreamMu.Lock()
	sourceRevision = 6
	upstreamMu.Unlock()
	assertRead(projects.MaterialSegmentRevisionChanged)
	upstreamMu.Lock()
	sourceRevision = 5
	itemsEmpty = false
	upstreamMu.Unlock()
	assertRead(projects.MaterialSegmentAvailable)
	credentials.mu.Lock()
	for _, call := range credentials.calls {
		if call.audience == "production-operations" {
			if call.action != "production-work-item.reference.read" || call.resourceType != "production-work-item" || call.resourceID != materialWork {
				t.Fatal("production credential expanded")
			}
		} else if call.audience != "evidence-service" || call.action != "evidence-service.metadata.read" || call.resourceType != "file" || call.resourceID != materialFile {
			t.Fatal("Evidence credential expanded")
		}
	}
	credentials.mu.Unlock()
	checkpoint := os.Getenv("PRODUCTION_MATERIAL_CHECKPOINT_DIR")
	if checkpoint == "" {
		checkpoint = t.TempDir()
	}
	if err := os.MkdirAll(checkpoint, 0700); err != nil {
		t.Fatal(err)
	}
	dump := filepath.Join(checkpoint, environment.Name+".dump")
	if output, err := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-acl", "--file="+dump, "--dbname="+environment.URL).CombinedOutput(); err != nil {
		t.Fatalf("owned dump failed: %s", output)
	}
	if err := os.Chmod(dump, 0600); err != nil {
		t.Fatal(err)
	}
	sourceSchema, schemaErr := database.RequireCurrentSchema(ctx, environment.Pool)
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	restored := testdatabase.New(t, testdatabase.RequireURL(t)) // Empty: do not migrate before pg_restore.
	if output, err := exec.CommandContext(ctx, "pg_restore", "--exit-on-error", "--no-owner", "--no-acl", "--dbname="+restored.URL, dump).CombinedOutput(); err != nil {
		t.Fatalf("empty restore failed: %s", output)
	}
	if current, err := database.RequireCurrentSchema(ctx, restored.Pool); err != nil || current != sourceSchema {
		t.Fatalf("restored schema=%q err=%v", current, err)
	}
	if projectMaterialFacts(t, restored.Pool) != before {
		t.Fatal("empty restore changed canonical Project facts")
	}
	restoredStore, _ := projects.NewPostgresStore(restored.Pool)
	details, err := restoredStore.Get(ctx, materialTenant, project.ID, true)
	if err != nil || details.Project.Revision != 2 || len(details.ResourceLinks) != 1 || details.ResourceLinks[0].ID != linked.ResourceLink.ID {
		t.Fatal("restore lost stable identity/revision/link")
	}
	if _, err := restoredStore.ApplyResourceLink(ctx, actor, project.ID, projects.ResourceLinkCommand{CommandID: cmd(3), ExpectedRevision: 2, Operation: "unlink", LinkID: linked.ResourceLink.ID}); err != nil {
		t.Fatal(err)
	}
	continued, _ := restoredStore.Get(ctx, materialTenant, project.ID, true)
	if continued.Project.Revision != 3 || continued.ResourceLinks[0].UnlinkedAt == nil {
		t.Fatal("restored database could not append unlink")
	}
	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	dumpSHA := sha256.Sum256(raw)
	t.Logf("canonical_before_restore=%s dump_sha256=%s restored_revision=2 continued_revision=3", before, hex.EncodeToString(dumpSHA[:]))
}
