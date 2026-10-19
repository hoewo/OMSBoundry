//go:build integration

package projects_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"omsbizfoundry/project-workspace/internal/database"
	"omsbizfoundry/project-workspace/internal/projects"
	"omsbizfoundry/project-workspace/internal/references"
	"omsbizfoundry/project-workspace/internal/testdatabase"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Actual Project PG18 and typed HTTP ports. Upstream and credential providers
// are explicit contract fixtures, NOT the complete formal IAM journey.
func TestRecordedMaterialProjectPostgresThreeSourcesAndEmptyRestore(t *testing.T) {
	for _, tool := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " required for empty restore")
		}
	}
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := projects.NewPostgresStore(environment.Pool)
	actor := projects.Actor{ID: materialActor, TenantID: materialTenant, Kind: "test_subject"}
	cmd := func(n int) string { return fmt.Sprintf("93000000-0000-4000-8000-%012d", n) }
	project, err := store.Create(ctx, actor, projects.CreateInput{CommandID: cmd(1), ProjectNumber: "C5-C6-MATERIAL-CONSUMER", Name: "三来源资料组合", ProjectTypeKeys: []string{"design"}})
	if err != nil {
		t.Fatal(err)
	}
	type source struct {
		service, kind, path, action, id, file, project, state string
		revision, fileVersion                                 int64
		httpStatus                                            int
		empty                                                 bool
		calls, fileCalls                                      int
	}
	values := []*source{
		{service: "approval-audit-results", kind: "approval_audit_result", path: "approval-audit-results", action: "approval-audit-result.read"},
		{service: "cost-outsourcing", kind: "cost_entry", path: "cost-entries", action: "cost-entry.read"},
		{service: "cost-outsourcing", kind: "outsourcing_engagement", path: "outsourcing-engagements", action: "outsourcing-engagement.read"},
	}
	for i, s := range values {
		s.id = fmt.Sprintf("34000000-0000-4000-8000-%012d", i+1)
		s.file = fmt.Sprintf("44000000-0000-4000-8000-%012d", i+1)
		s.project = project.ID
		s.state = "recorded"
		s.revision = 5
		s.fileVersion = 2
		s.httpStatus = 200
	}
	var mu sync.Mutex
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer integration-bound-capability" {
			w.WriteHeader(401)
			return
		}
		for _, s := range values {
			if r.URL.Path == "/api/v1/references/"+s.path+"/"+s.id+"/materials" {
				s.calls++
				if s.httpStatus != 200 {
					w.WriteHeader(s.httpStatus)
					return
				}
				items := fmt.Sprintf(`[{"relation_key":"资料","file_id":%q,"version":2,"evidence_source_revision":2,"display_name":"材料.pdf"}]`, s.file)
				if s.empty {
					items = "[]"
				}
				digest := sha256.Sum256([]byte(items))
				fmt.Fprintf(w, `{"schema_version":"1","source_application":%q,"id":%q,"tenant_id":%q,"project_id":%q,"resource_type":%q,"result_kind":"audit","status":%q,"revision":%d,"digest":%q,"items":%s}`, s.service, s.id, materialTenant, s.project, s.kind, s.state, s.revision, hex.EncodeToString(digest[:]), items)
				return
			}
			if r.URL.Path == "/v1/references/files/"+s.file {
				s.fileCalls++
				fmt.Fprintf(w, `{"schema_version":"1","source_application":"evidence-service","id":%q,"tenant_id":%q,"resource_type":"evidence-file","status":"available","revision":%d,"version":%d,"file_name":"材料.pdf","size_bytes":64,"media_type":"application/pdf","sha256":%q}`, s.file, materialTenant, s.fileVersion, s.fileVersion, strings.Repeat("a", 64))
				return
			}
		}
		t.Errorf("unexpected typed path %s", r.URL.Path)
		w.WriteHeader(404)
	}))
	defer upstream.Close()
	credentials := &materialCredentials{}
	validator, err := references.NewHTTPValidator(time.Second, references.Endpoint{}, map[string]references.Endpoint{
		"approval-audit-results": {BaseURL: upstream.URL, Audience: "approval-audit-results", Action: "approval-audit-result.read", Credentials: credentials},
		"cost-outsourcing":       {BaseURL: upstream.URL, Audience: "cost-outsourcing", Action: "cost-entry.read", Credentials: credentials},
		"evidence-service":       {BaseURL: upstream.URL, Audience: "evidence-service", Action: "evidence-service.metadata.read", Credentials: credentials},
	})
	if err != nil {
		t.Fatal(err)
	}
	service, _ := projects.NewService(store, validator)
	inputs := make([]projects.ResourceLinkCommand, 3)
	links := make([]projects.ResourceLinkResult, 3)
	for i, s := range values {
		revision := int64(5)
		inputs[i] = projects.ResourceLinkCommand{CommandID: cmd(i + 2), ExpectedRevision: int64(i + 1), Operation: "link", TargetService: s.service, ResourceType: s.kind, ResourceID: s.id, SourceRevision: &revision, RelationKey: "source-materials"}
		before := projectMaterialFacts(t, environment.Pool)
		mu.Lock()
		s.project = materialActor
		mu.Unlock()
		if _, err := service.ApplyResourceLink(ctx, actor, project.ID, inputs[i]); err == nil {
			t.Fatal("cross-project link accepted")
		}
		if projectMaterialFacts(t, environment.Pool) != before {
			t.Fatal("wrong source wrote Project")
		}
		mu.Lock()
		s.project = project.ID
		s.state = "withdrawn"
		mu.Unlock()
		if _, err := service.ApplyResourceLink(ctx, actor, project.ID, inputs[i]); err == nil {
			t.Fatal("new withdrawn link accepted")
		}
		if projectMaterialFacts(t, environment.Pool) != before {
			t.Fatal("withdrawn source wrote Project")
		}
		mu.Lock()
		s.state = "recorded"
		mu.Unlock()
		links[i], err = service.ApplyResourceLink(ctx, actor, project.ID, inputs[i])
		if err != nil || links[i].Project.Revision != int64(i+2) {
			t.Fatalf("link %s %v", s.kind, err)
		}
	}
	before := projectMaterialFacts(t, environment.Pool)
	mu.Lock()
	values[0].httpStatus = 503
	calls := values[0].calls
	mu.Unlock()
	replay, err := service.ApplyResourceLink(ctx, actor, project.ID, inputs[0])
	mu.Lock()
	afterCalls := values[0].calls
	values[0].httpStatus = 200
	mu.Unlock()
	if err != nil || replay.ResourceLink.ID != links[0].ResourceLink.ID || calls != afterCalls || projectMaterialFacts(t, environment.Pool) != before {
		t.Fatal("offline command replay changed facts or called source")
	}
	assertRead := func(s *source, want string) {
		t.Helper()
		result, err := service.Materials(ctx, materialTenant, project.ID)
		if err != nil || len(result.Segments) != 3 || result.ProjectRevision != 4 {
			t.Fatalf("collection=%#v err=%v", result, err)
		}
		found := false
		for _, segment := range result.Segments {
			expected := projects.MaterialSegmentAvailable
			if segment.ResourceID == s.id {
				expected = want
				found = true
				if want == projects.MaterialSegmentInvalid && segment.ErrorCode != "source_withdrawn" {
					t.Fatal("withdrawn explanation missing")
				}
			}
			if segment.State != expected {
				t.Fatalf("source %s state=%s want=%s", segment.ResourceID, segment.State, expected)
			}
		}
		if !found || projectMaterialFacts(t, environment.Pool) != before {
			t.Fatal("GET wrote facts or lost source")
		}
	}
	for _, s := range values {
		assertRead(s, projects.MaterialSegmentAvailable)
		mu.Lock()
		s.httpStatus = 503
		mu.Unlock()
		assertRead(s, projects.MaterialSegmentUnavailable)
		mu.Lock()
		s.httpStatus = 403
		mu.Unlock()
		assertRead(s, projects.MaterialSegmentForbidden)
		mu.Lock()
		s.httpStatus = 200
		s.revision = 6
		mu.Unlock()
		assertRead(s, projects.MaterialSegmentRevisionChanged)
		mu.Lock()
		s.revision = 5
		s.fileVersion = 3
		mu.Unlock()
		assertRead(s, projects.MaterialSegmentRevisionChanged)
		mu.Lock()
		s.fileVersion = 2
		s.empty = true
		mu.Unlock()
		assertRead(s, projects.MaterialSegmentEmpty)
		mu.Lock()
		s.revision = 6
		mu.Unlock()
		assertRead(s, projects.MaterialSegmentRevisionChanged)
		mu.Lock()
		s.revision = 5
		s.empty = false
		s.state = "withdrawn"
		fileCalls := s.fileCalls
		mu.Unlock()
		assertRead(s, projects.MaterialSegmentInvalid)
		mu.Lock()
		unchanged := s.fileCalls == fileCalls
		s.state = "recorded"
		mu.Unlock()
		if !unchanged {
			t.Fatal("withdrawn source queried Evidence")
		}
		assertRead(s, projects.MaterialSegmentAvailable)
	}
	credentials.mu.Lock()
	for _, call := range credentials.calls {
		valid := false
		for _, s := range values {
			if call.audience == s.service && call.action == s.action && call.resourceType == s.kind && call.resourceID == s.id {
				valid = true
			}
			if call.audience == "evidence-service" && call.action == "evidence-service.metadata.read" && call.resourceType == "file" && call.resourceID == s.file {
				valid = true
			}
		}
		if !valid {
			t.Fatalf("expanded credential %#v", call)
		}
	}
	credentials.mu.Unlock()
	checkpoint := os.Getenv("RECORDED_MATERIAL_CHECKPOINT_DIR")
	if checkpoint == "" {
		checkpoint = t.TempDir()
	}
	if err := os.MkdirAll(checkpoint, 0700); err != nil {
		t.Fatal(err)
	}
	dump := filepath.Join(checkpoint, environment.Name+".dump")
	if output, err := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-acl", "--file="+dump, "--dbname="+environment.URL).CombinedOutput(); err != nil {
		t.Fatalf("dump failed: %s", output)
	}
	if err := os.Chmod(dump, 0600); err != nil {
		t.Fatal(err)
	}
	sourceSchema, schemaErr := database.RequireCurrentSchema(ctx, environment.Pool)
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	restored := testdatabase.New(t, testdatabase.RequireURL(t))
	if output, err := exec.CommandContext(ctx, "pg_restore", "--exit-on-error", "--no-owner", "--no-acl", "--dbname="+restored.URL, dump).CombinedOutput(); err != nil {
		t.Fatalf("empty restore failed: %s", output)
	}
	if current, err := database.RequireCurrentSchema(ctx, restored.Pool); err != nil || current != sourceSchema {
		t.Fatalf("schema=%q %v", current, err)
	}
	if projectMaterialFacts(t, restored.Pool) != before {
		t.Fatal("empty restore changed canonical facts")
	}
	restoredStore, _ := projects.NewPostgresStore(restored.Pool)
	details, err := restoredStore.Get(ctx, materialTenant, project.ID, true)
	if err != nil || details.Project.Revision != 4 || len(details.ResourceLinks) != 3 {
		t.Fatal("restored identity/revision/links lost")
	}
	for _, link := range links {
		found := false
		for _, r := range details.ResourceLinks {
			if r.ID == link.ResourceLink.ID {
				found = true
			}
		}
		if !found {
			t.Fatal("restored stable Link ID missing")
		}
	}
	restoredService, _ := projects.NewService(restoredStore, validator)
	if _, err := restoredService.ApplyResourceLink(ctx, actor, project.ID, projects.ResourceLinkCommand{CommandID: cmd(9), ExpectedRevision: 4, Operation: "unlink", LinkID: links[0].ResourceLink.ID}); err != nil {
		t.Fatal(err)
	}
	result, err := restoredService.Materials(ctx, materialTenant, project.ID)
	if err != nil || result.ProjectRevision != 5 || len(result.Segments) != 2 {
		t.Fatal("restored append/unlinked absence failed")
	}
	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	t.Logf("three_sources=true GET_owned_digest_unchanged=true canonical_before_restore=%s dump_sha256=%s restored_revision=4 continued_revision=5", before, hex.EncodeToString(digest[:]))
}
