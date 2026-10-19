//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/approval-audit-results/internal/auth"
	"omsbizfoundry/approval-audit-results/internal/database"
	"omsbizfoundry/approval-audit-results/internal/references"
	"omsbizfoundry/approval-audit-results/internal/results"
)

const (
	httpTenant   = "11111111-1111-4111-8111-111111111111"
	httpTenantB  = "11111111-1111-4111-8111-111111111112"
	httpSubject  = "22222222-2222-4222-8222-222222222222"
	httpProject  = "33333333-3333-4333-8333-333333333333"
	httpProjectB = "33333333-3333-4333-8333-333333333334"
	httpContract = "44444444-4444-4444-8444-444444444444"
	httpEvidence = "55555555-5555-4555-8555-555555555555"
	httpCommand  = "66666666-6666-4666-8666-666666666666"
	httpCommand2 = "66666666-6666-4666-8666-666666666667"
	httpCommand3 = "66666666-6666-4666-8666-666666666668"
	httpCommand4 = "66666666-6666-4666-8666-666666666669"
	httpKeyID    = "77777777-7777-4777-8777-777777777777"
	httpTokenID  = "88888888-8888-4888-8888-888888888888"
)

type httpCredentials struct {
	mu    sync.Mutex
	calls []string
}

func (c *httpCredentials) Credential(_ context.Context, audience, action, resourceType, resourceID string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, strings.Join([]string{audience, action, resourceType, resourceID}, ":"))
	return "exact-reference-token", nil
}
func (*httpCredentials) Invalidate(string, string, string, string) {}

type referenceMode struct {
	mu              sync.RWMutex
	healthy         bool
	evidenceHealthy bool
	linkedRevision  int64
}

func (m *referenceMode) set(healthy bool, revision int64) {
	m.mu.Lock()
	m.healthy, m.evidenceHealthy, m.linkedRevision = healthy, healthy, revision
	m.mu.Unlock()
}
func (m *referenceMode) setEvidence(healthy bool) {
	m.mu.Lock()
	m.evidenceHealthy = healthy
	m.mu.Unlock()
}
func (m *referenceMode) values() (bool, bool, int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy, m.evidenceHealthy, m.linkedRevision
}

func TestProtectedHTTPWithIAMExactReferencesFailureRecoveryAndRedaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := httpIntegrationPool(t, ctx)
	defer cleanup()
	mode := &referenceMode{}
	referenceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthy, evidenceHealthy, linkedRevision := mode.values()
		if !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.Header.Get("Authorization") != "Bearer exact-reference-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/references/projects/" + httpProject:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"P-C5-001","display_summary":"C5 验证项目","status":"active","revision":7,"resource_links":[{"target_service":"contract-ledger","resource_type":"contract","resource_id":%q,"source_revision":%d,"relation_key":"primary-contract"}]}`, httpProject, httpTenant, httpContract, linkedRevision)
		case "/api/v1/references/contracts/" + httpContract:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"contract-ledger","id":%q,"tenant_id":%q,"resource_type":"contract","display_summary":"设计合同","status":"active","revision":4,"category":"design_fee","currency":"CNY","current_effective_amount":"123.45"}`, httpContract, httpTenant)
		case "/v1/references/files/" + httpEvidence + "/versions/2":
			if !evidenceHealthy {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = fmt.Fprintf(w, `{"id":%q,"file_id":%q,"tenant_id":%q,"resource_type":"evidence-file","status":"available","version":2,"revision":2,"display_name":"批复.pdf","sha256":"%s"}`, httpEvidence, httpEvidence, httpTenant, strings.Repeat("a", 64))
		default:
			http.NotFound(w, r)
		}
	}))
	defer referenceServer.Close()

	credentials := &httpCredentials{}
	endpoint := func(audience, action string) references.Endpoint {
		return references.Endpoint{BaseURL: referenceServer.URL, Audience: audience, Action: action, Credentials: credentials}
	}
	resolver, err := references.NewHTTPResolver(time.Second, endpoint("project-workspace", "project-workspace.read"), endpoint("contract-ledger", "contract-ledger.reference.read"), endpoint("evidence-service", "evidence-service.view"))
	if err != nil {
		t.Fatal(err)
	}
	store, _ := results.NewPostgresStore(pool)
	service, _ := results.NewService(store, resolver)
	provider, privateKey := integrationIAMProvider(t)
	server, err := New(service, provider)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	body := recordBody(httpCommand, true, false)
	writeToken := integrationToken(t, privateKey, "approval-audit-results", httpTenant, []string{auth.ActionCreate, auth.ActionEvidenceManage}, "project", httpProject)
	mode.set(false, 4)
	response := doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", writeToken, body)
	assertProblem(t, response, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE")
	assertFactCounts(t, pool, 0, 0, 0, 0)

	mode.set(true, 4)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", writeToken, body)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("record status=%d body=%s", response.StatusCode, readBody(response))
	}
	var created resultView
	decodeResponse(t, response, &created)
	if created.ID == "" || created.Revision != 1 || created.TotalAmountMinor != nil || created.Source.TotalAmountMinor != nil || len(created.Evidence) != 1 {
		t.Fatalf("created=%#v", created)
	}
	assertFactCounts(t, pool, 1, 1, 1, 1)

	mode.set(false, 4)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", writeToken, body)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("replay status=%d body=%s", response.StatusCode, readBody(response))
	}
	var replay resultView
	decodeResponse(t, response, &replay)
	if replay.ID != created.ID {
		t.Fatalf("replay id=%s want=%s", replay.ID, created.ID)
	}
	assertFactCounts(t, pool, 1, 1, 1, 1)
	conflictingReplay := bytes.Replace(body, []byte("批复文书"), []byte("不同批复文书"), 1)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", writeToken, conflictingReplay)
	assertProblem(t, response, http.StatusConflict, "COMMAND_PAYLOAD_CONFLICT")
	assertFactCounts(t, pool, 1, 1, 1, 1)

	readToken := integrationToken(t, privateKey, "approval-audit-results", httpTenant, []string{auth.ActionRead}, "project", httpProject)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/approval-audit-results/"+created.ID, readToken, nil)
	var redacted resultView
	decodeResponse(t, response, &redacted)
	if response.StatusCode != http.StatusOK || redacted.TotalAmountMinor != nil || redacted.Source.TotalAmountMinor != nil {
		t.Fatalf("redacted status=%d value=%#v", response.StatusCode, redacted)
	}
	amountToken := integrationToken(t, privateKey, "approval-audit-results", httpTenant, []string{auth.ActionRead, auth.ActionAmountRead}, "project", httpProject)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/approval-audit-results/"+created.ID, amountToken, nil)
	var visible resultView
	decodeResponse(t, response, &visible)
	if response.StatusCode != http.StatusOK || visible.TotalAmountMinor == nil || *visible.TotalAmountMinor != 12345 || visible.Source.TotalAmountMinor == nil || *visible.Source.TotalAmountMinor != 12345 {
		t.Fatalf("visible status=%d value=%#v", response.StatusCode, visible)
	}

	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", "", body)
	assertProblem(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")
	wrongAudience := integrationToken(t, privateKey, "billing-invoice", httpTenant, []string{auth.ActionCreate}, "project", httpProject)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", wrongAudience, body)
	assertProblem(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")
	wrongTenant := integrationToken(t, privateKey, "approval-audit-results", httpTenantB, []string{auth.ActionRead}, "project", httpProject)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/approval-audit-results/"+created.ID, wrongTenant, nil)
	assertProblem(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")
	adjacentRole := integrationToken(t, privateKey, "approval-audit-results", httpTenant, []string{auth.ActionRead}, "project", httpProject)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", adjacentRole, body)
	assertProblem(t, response, http.StatusForbidden, "CAPABILITY_DENIED")
	projectB := integrationToken(t, privateKey, "approval-audit-results", httpTenant, []string{auth.ActionCreate, auth.ActionEvidenceManage}, "project", httpProjectB)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", projectB, body)
	assertProblem(t, response, http.StatusForbidden, "SCOPE_DENIED")
	missingEvidencePermission := integrationToken(t, privateKey, "approval-audit-results", httpTenant, []string{auth.ActionCreate}, "project", httpProject)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", missingEvidencePermission, recordBody(httpCommand2, true, false))
	assertProblem(t, response, http.StatusForbidden, "CAPABILITY_DENIED")
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", missingEvidencePermission, recordBody(httpCommand2, false, true))
	assertProblem(t, response, http.StatusForbidden, "CAPABILITY_DENIED")
	assertFactCounts(t, pool, 1, 1, 1, 1)

	mode.set(true, 3)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", writeToken, recordBody(httpCommand2, false, false))
	assertProblem(t, response, http.StatusUnprocessableEntity, "REFERENCE_INVALID")
	assertFactCounts(t, pool, 1, 1, 1, 1)

	mode.set(true, 4)
	correctToken := integrationToken(t, privateKey, "approval-audit-results", httpTenant, []string{auth.ActionCorrect, auth.ActionEvidenceManage}, "project", httpProject)
	correction := []byte(fmt.Sprintf(`{"command_id":%q,"expected_revision":9,"reason":"错误旧版本","currency":"CNY","source_ref":{"application":"contract-ledger","resource_type":"contract","resource_id":%q,"revision":4},"external_confirmed_by":"批复文书","external_confirmed_at":"2026-09-28T08:00:00Z"}`, httpCommand3, httpContract))
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results/"+created.ID+"/corrections", correctToken, correction)
	assertProblem(t, response, http.StatusConflict, "REVISION_CONFLICT")
	assertFactCounts(t, pool, 1, 1, 1, 1)

	auditBody := []byte(fmt.Sprintf(`{"command_id":%q,"expected_revision":0,"project_ref":{"id":%q,"revision":7},"kind":"audit","currency":"CNY","source_ref":{"application":"approval-audit-results","resource_type":"approval-result","resource_id":%q,"revision":1},"evidence_refs":[{"file_id":%q,"version":2,"slot":"audit_document"}],"external_confirmed_by":"审计文书","external_confirmed_at":"2026-09-28T09:00:00Z"}`, httpCommand4, httpProject, created.ID, httpEvidence))
	mode.setEvidence(false)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", writeToken, auditBody)
	assertProblem(t, response, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE")
	assertFactCounts(t, pool, 1, 1, 1, 1)
	mode.setEvidence(true)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", writeToken, auditBody)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("audit recovery status=%d body=%s", response.StatusCode, readBody(response))
	}
	var auditCreated resultView
	decodeResponse(t, response, &auditCreated)
	if auditCreated.Kind != results.KindAudit || auditCreated.Source.ResourceID != created.ID || auditCreated.Source.Revision != 1 {
		t.Fatalf("audit=%#v", auditCreated)
	}
	assertFactCounts(t, pool, 2, 2, 2, 2)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/approval-audit-results?project_id="+httpProject, readToken, nil)
	var listed struct {
		ProjectID string       `json:"project_id"`
		Items     []resultView `json:"items"`
	}
	decodeResponse(t, response, &listed)
	if response.StatusCode != http.StatusOK || listed.ProjectID != httpProject || len(listed.Items) != 2 {
		t.Fatalf("list status=%d value=%#v", response.StatusCode, listed)
	}
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/references/projects/"+httpProject+"/approval-audit-results", readToken, nil)
	var projectReferences results.ProjectResultReferenceList
	decodeResponse(t, response, &projectReferences)
	if response.StatusCode != http.StatusOK || projectReferences.SchemaVersion != "1" || projectReferences.SourceApplication != "approval-audit-results" || len(projectReferences.Items) != 2 {
		t.Fatalf("references status=%d value=%#v", response.StatusCode, projectReferences)
	}
	for _, item := range projectReferences.Items {
		if item.ProjectID != httpProject || item.ResourceType != "approval_audit_result" || item.Digest == "" || item.DisplaySummary == "" {
			t.Fatalf("invalid reference=%#v", item)
		}
	}
	mode.setEvidence(false)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/approval-audit-results", writeToken, auditBody)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("audit replay status=%d body=%s", response.StatusCode, readBody(response))
	}
	assertFactCounts(t, pool, 2, 2, 2, 2)

	credentials.mu.Lock()
	joinedCalls := strings.Join(credentials.calls, "\n")
	credentials.mu.Unlock()
	for _, want := range []string{"project-workspace:project-workspace.read:project:" + httpProject, "contract-ledger:contract-ledger.reference.read:contract:" + httpContract, "evidence-service:evidence-service.view:file:" + httpEvidence} {
		if !strings.Contains(joinedCalls, want) {
			t.Fatalf("credential calls missing %s: %s", want, joinedCalls)
		}
	}
}

func recordBody(command string, evidence, explicitAmount bool) []byte {
	evidenceJSON := ""
	if evidence {
		evidenceJSON = fmt.Sprintf(`,"evidence_refs":[{"file_id":%q,"version":2,"slot":"approval_document"}]`, httpEvidence)
	}
	amountJSON := ""
	if explicitAmount {
		amountJSON = `,"amount_components":[{"kind":"design_fee","amount_minor":12345},{"kind":"survey_fee","amount_minor":0},{"kind":"consultation_fee","amount_minor":0}]`
	}
	return []byte(fmt.Sprintf(`{"command_id":%q,"expected_revision":0,"project_ref":{"id":%q,"revision":7},"kind":"approval","currency":"CNY","source_ref":{"application":"contract-ledger","resource_type":"contract","resource_id":%q,"revision":4}%s%s,"external_confirmed_by":"批复文书","external_confirmed_at":"2026-09-28T08:00:00Z"}`, command, httpProject, httpContract, amountJSON, evidenceJSON))
}

func httpIntegrationPool(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(ctx, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "approval_audit_http_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool, func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	}
}

func integrationIAMProvider(t *testing.T) (*auth.IAMProvider, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"keys": []map[string]any{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": httpKeyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	path := filepath.Join(t.TempDir(), "keys.json")
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := auth.NewIAMProvider("https://iam.results.integration.test", httpTenant, path)
	if err != nil {
		t.Fatal(err)
	}
	return provider, privateKey
}

func integrationToken(t *testing.T, privateKey ed25519.PrivateKey, audience, tenant string, actions []string, resourceType, resourceID string) string {
	t.Helper()
	now := time.Now().UTC()
	header, _ := json.Marshal(map[string]any{"alg": "EdDSA", "typ": "JWT", "kid": httpKeyID})
	claims, _ := json.Marshal(map[string]any{"ver": "1", "iss": "https://iam.results.integration.test", "sub": httpSubject, "tid": tenant, "aud": audience, "name": "C5 集成操作者", "actions": actions, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": resourceType, "id": resourceID}}}, "iat": now.Add(-time.Second).Unix(), "nbf": now.Add(-time.Second).Unix(), "exp": now.Add(time.Minute).Unix(), "auth_time": now.Add(-time.Second).Unix(), "jti": httpTokenID})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(unsigned)))
}

func doHTTP(t *testing.T, method, endpoint, token string, body []byte) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func assertProblem(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	body := readBody(response)
	if response.StatusCode != status || !strings.Contains(body, `"code":"`+code+`"`) {
		t.Fatalf("status=%d body=%s want=%d/%s", response.StatusCode, body, status, code)
	}
}
func readBody(response *http.Response) string {
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	return string(raw)
}
func decodeResponse(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatal(err)
	}
}

func assertFactCounts(t *testing.T, pool *pgxpool.Pool, current, revisions, commands, audit int) {
	t.Helper()
	for _, item := range []struct {
		table string
		want  int
	}{{"approval_audit_results", current}, {"approval_audit_result_revisions", revisions}, {"approval_audit_result_commands", commands}, {"approval_audit_result_audit_events", audit}} {
		var got int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM `+item.table).Scan(&got); err != nil || got != item.want {
			t.Fatalf("%s count=%d want=%d err=%v", item.table, got, item.want, err)
		}
	}
}
