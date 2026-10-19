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
	"omsbizfoundry/bonus-distribution/internal/auth"
	"omsbizfoundry/bonus-distribution/internal/bonus"
	"omsbizfoundry/bonus-distribution/internal/database"
	"omsbizfoundry/bonus-distribution/internal/overlays/referenceoa"
	"omsbizfoundry/bonus-distribution/internal/references"
)

const (
	httpTenant      = "11111111-1111-4111-8111-111111111111"
	httpTenantB     = "11111111-1111-4111-8111-111111111112"
	httpSubject     = "22222222-2222-4222-8222-222222222222"
	httpProject     = "33333333-3333-4333-8333-333333333333"
	httpProjectB    = "33333333-3333-4333-8333-333333333334"
	httpSettlement  = "44444444-4444-4444-8444-444444444444"
	httpReceiptSet  = "44444444-4444-4444-8444-444444444445"
	httpTransaction = "55555555-5555-4555-8555-555555555555"
	httpAllocation  = "55555555-5555-4555-8555-555555555556"
	httpPerson      = "66666666-6666-4666-8666-666666666666"
	httpAssignment  = "77777777-7777-4777-8777-777777777777"
	httpCommand     = "88888888-8888-4888-8888-888888888881"
	httpCommand2    = "88888888-8888-4888-8888-888888888882"
	httpCommand3    = "88888888-8888-4888-8888-888888888883"
	httpCommand4    = "88888888-8888-4888-8888-888888888884"
	httpKeyID       = "99999999-9999-4999-8999-999999999991"
	httpTokenID     = "99999999-9999-4999-8999-999999999992"
)

type httpCredentials struct {
	mu            sync.Mutex
	calls         []string
	invalidations int
}

func (c *httpCredentials) Credential(_ context.Context, audience, action, resourceType, resourceID string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, strings.Join([]string{audience, action, resourceType, resourceID}, ":"))
	return "exact-reference-token", nil
}

func (c *httpCredentials) Invalidate(string, string, string, string) {
	c.mu.Lock()
	c.invalidations++
	c.mu.Unlock()
}

type referenceMode struct {
	mu                    sync.RWMutex
	healthy, validProject bool
	calls                 int
}

func (m *referenceMode) set(healthy, validProject bool) {
	m.mu.Lock()
	m.healthy, m.validProject = healthy, validProject
	m.mu.Unlock()
}

func (m *referenceMode) values() (bool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.healthy, m.validProject
}

func (m *referenceMode) count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calls
}

func TestProtectedHTTPWithIAMTypedReferencesFailureRecoveryReplayAndRedaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := httpIntegrationPool(t, ctx)
	defer cleanup()
	mode := &referenceMode{}
	referenceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthy, validProject := mode.values()
		if !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.Header.Get("Authorization") != "Bearer exact-reference-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		projectID := httpProject
		if !validProject {
			projectID = httpProjectB
		}
		switch r.URL.Path {
		case "/api/v1/references/projects/" + httpProject:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"P-C7-001","display_summary":"C7 验证项目","status":"active","revision":7,"project_type_keys":["design"],"node_type":"project","resource_links":[{"target_service":"contract-ledger","resource_type":"settlement","resource_id":%q,"source_revision":1,"relation_key":"business_settlement"}]}`, httpProject, httpTenant, httpSettlement)
		case "/api/v1/references/projects/" + httpProject + "/assignments/" + httpAssignment:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"project_id":%q,"resource_type":"project-assignment","subject_kind":"person","subject_id":%q,"source_revision":5,"role_key":"participant","status":"active","revision":4,"effective_from":"2026-09-01T00:00:00Z","display_name":"奖金参与人"}`, httpAssignment, httpTenant, httpProject, httpPerson)
		case "/api/v1/references/persons/" + httpPerson:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"people-groups","id":%q,"tenant_id":%q,"resource_type":"person","display_name":"奖金参与人","status":"active","revision":5}`, httpPerson, httpTenant)
		case "/api/v1/references/settlements/" + httpSettlement:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"contract-ledger","id":%q,"tenant_id":%q,"resource_type":"settlement","display_summary":"经营结算","status":"recorded","revision":1,"category":"design","contract_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","currency":"CNY","current_effective_amount":"1065.00"}`, httpSettlement, httpTenant)
		case "/api/v1/references/receipt-sets/" + httpReceiptSet:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"money-flow-ledger","id":%q,"tenant_id":%q,"resource_type":"receipt-set","project_id":%q,"status":"confirmed","revision":1,"currency":"CNY","total_amount_minor":106500,"items":[{"transaction_id":%q,"transaction_revision":1,"allocation_id":%q,"allocation_revision":1,"amount_minor":106500,"currency":"CNY","status":"confirmed"}]}`, httpReceiptSet, httpTenant, projectID, httpTransaction, httpAllocation)
		default:
			http.NotFound(w, r)
		}
	}))
	defer referenceServer.Close()
	credentials := &httpCredentials{}
	endpoint := func(audience, action string) references.Endpoint {
		return references.Endpoint{BaseURL: referenceServer.URL, Audience: audience, Action: action, Credentials: credentials}
	}
	resolver, err := references.NewHTTPResolver(time.Second, endpoint("project-workspace", "project-workspace.read"), endpoint("people-groups", "people-groups.person.reference.read"), endpoint("contract-ledger", "contract-ledger.reference.read"), endpoint("money-flow-ledger", "money-flow-ledger.receipt-set.reference.read"))
	if err != nil {
		t.Fatal(err)
	}
	store, _ := bonus.NewPostgresStore(pool)
	service, _ := bonus.NewService(store, resolver, referenceoa.Evaluator{})
	provider, privateKey := integrationIAMProvider(t)
	server, err := New(service, provider)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	writeActions := []string{bonus.ActionBusinessCalculate, bonus.ActionBusinessPublish, bonus.ActionBusinessSupersede, bonus.ActionProductionCalculate, bonus.ActionAllocationManage, bonus.ActionAllocationAmountRead}
	writeToken := integrationToken(t, privateKey, "bonus-distribution", httpTenant, writeActions, "project", httpProject)
	businessBody := []byte(fmt.Sprintf(`{"command_id":%q,"project":{"id":%q,"revision":7},"source":{"settlement_id":%q,"revision":1},"policy":{"key":"reference-oa-bonus-v1","version":"1.0.0","rate_basis_points":1000,"rounding":"half_up"},"allocations":[{"person":{"id":%q,"revision":5},"assignment":{"id":%q,"revision":4},"ratio_basis_points":10000,"reason":"经营参与"}]}`, httpCommand, httpProject, httpSettlement, httpPerson, httpAssignment))

	mode.set(false, true)
	response := doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/bonus-pools/business-drafts", writeToken, businessBody)
	assertProblem(t, response, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE")
	assertCounts(t, pool, 0, 0, 0, 0, 0, 0)

	mode.set(true, true)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/bonus-pools/business-drafts", writeToken, businessBody)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.StatusCode, readBody(response))
	}
	var created poolView
	decodeResponse(t, response, &created)
	if created.ID == "" || created.AmountMinor == nil || *created.AmountMinor != 10_000 || len(created.Allocations) != 1 || created.Allocations[0].AmountMinor == nil || !containsString(created.AllowedActions, bonus.ActionBusinessPublish) {
		t.Fatalf("created=%#v", created)
	}
	assertCounts(t, pool, 1, 1, 1, 1, 1, 1)

	mode.set(false, true)
	callsBeforeReplay := mode.count()
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/bonus-pools/business-drafts", writeToken, businessBody)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("replay status=%d body=%s", response.StatusCode, readBody(response))
	}
	var replay poolView
	decodeResponse(t, response, &replay)
	if replay.ID != created.ID || mode.count() != callsBeforeReplay {
		t.Fatalf("successful replay called upstream or changed id")
	}
	assertCounts(t, pool, 1, 1, 1, 1, 1, 1)

	mode.set(true, true)
	readToken := integrationToken(t, privateKey, "bonus-distribution", httpTenant, []string{bonus.ActionPoolRead, bonus.ActionPoolHistory, bonus.ActionAllocationRead}, "project", httpProject)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/bonus-pools/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", readToken, nil)
	assertProblem(t, response, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	stalePublish := []byte(fmt.Sprintf(`{"command_id":%q,"expected_revision":9,"reason":"陈旧 revision"}`, httpCommand4))
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/bonus-pools/"+created.ID+"/publication", writeToken, stalePublish)
	assertProblem(t, response, http.StatusConflict, "REVISION_CONFLICT")
	assertCounts(t, pool, 1, 1, 1, 1, 1, 1)
	publishBody := []byte(fmt.Sprintf(`{"command_id":%q,"expected_revision":1,"reason":"确认经营奖金"}`, httpCommand2))
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/bonus-pools/"+created.ID+"/publication", writeToken, publishBody)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("publish status=%d body=%s", response.StatusCode, readBody(response))
	}
	var published poolView
	decodeResponse(t, response, &published)
	if published.State != bonus.StatePublished || published.Revision != 2 || !containsString(published.AllowedActions, bonus.ActionBusinessSupersede) {
		t.Fatalf("published=%#v", published)
	}

	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/bonus-pools/"+created.ID, readToken, nil)
	var redacted poolView
	decodeResponse(t, response, &redacted)
	if response.StatusCode != http.StatusOK || redacted.AmountMinor != nil || redacted.UnallocatedMinor != nil || redacted.Source.TotalAmountMinor != nil || redacted.Source.Items[0].AmountMinor != nil || redacted.Allocations[0].AmountMinor != nil {
		t.Fatalf("redaction failed: status=%d value=%#v", response.StatusCode, redacted)
	}
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/bonus-pools/"+created.ID+"/history", readToken, nil)
	var history struct {
		Items []poolRevisionView `json:"items"`
	}
	decodeResponse(t, response, &history)
	if response.StatusCode != http.StatusOK || len(history.Items) != 2 || history.Items[0].Snapshot.AmountMinor != nil || history.Items[1].Snapshot.Allocations[0].AmountMinor != nil {
		t.Fatalf("history redaction failed: %#v", history)
	}
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/bonus-allocations/"+created.Allocations[0].ID+"/history", readToken, nil)
	var allocationHistory struct {
		Items []allocationRevisionView `json:"items"`
	}
	decodeResponse(t, response, &allocationHistory)
	if response.StatusCode != http.StatusOK || len(allocationHistory.Items) != 2 || allocationHistory.Items[0].Snapshot.AmountMinor != nil {
		t.Fatalf("allocation history redaction failed: %#v", allocationHistory)
	}

	mode.set(true, false)
	productionBody := []byte(fmt.Sprintf(`{"command_id":%q,"project":{"id":%q,"revision":7},"source":{"receipt_set_id":%q,"revision":1},"policy":{"key":"reference-oa-bonus-v1","version":"1.0.0","rate_basis_points":1000,"rounding":"half_up"},"allocations":[{"person":{"id":%q,"revision":5},"assignment":{"id":%q,"revision":4},"ratio_basis_points":10000,"reason":"生产参与"}]}`, httpCommand3, httpProject, httpReceiptSet, httpPerson, httpAssignment))
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/bonus-pools/production-drafts", writeToken, productionBody)
	assertProblem(t, response, http.StatusUnprocessableEntity, "REFERENCE_OR_POLICY_INVALID")
	assertCounts(t, pool, 1, 2, 1, 2, 2, 2)
	mode.set(true, true)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/bonus-pools/production-drafts", writeToken, productionBody)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("production status=%d body=%s", response.StatusCode, readBody(response))
	}
	var production poolView
	decodeResponse(t, response, &production)
	if production.Source.Type != bonus.SourceMoneyReceiptSet || production.Source.Items[0].TransactionID != httpTransaction {
		t.Fatalf("production=%#v", production)
	}

	assertIAMDenials(t, httpServer.URL, privateKey, businessBody, created.ID)

	credentials.mu.Lock()
	joinedCalls := strings.Join(credentials.calls, "\n")
	credentials.mu.Unlock()
	for _, wanted := range []string{
		"project-workspace:project-workspace.read:project:" + httpProject,
		"people-groups:people-groups.person.reference.read:person:" + httpPerson,
		"contract-ledger:contract-ledger.reference.read:settlement:" + httpSettlement,
		"money-flow-ledger:money-flow-ledger.receipt-set.reference.read:receipt_set:" + httpReceiptSet,
	} {
		if !strings.Contains(joinedCalls, wanted) {
			t.Fatalf("missing exact workload call %s in %s", wanted, joinedCalls)
		}
	}
}

func assertIAMDenials(t *testing.T, baseURL string, privateKey ed25519.PrivateKey, body []byte, poolID string) {
	t.Helper()
	response := doHTTP(t, http.MethodPost, baseURL+"/api/v1/bonus-pools/business-drafts", "", body)
	assertProblem(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")
	wrongAudience := integrationToken(t, privateKey, "cost-outsourcing", httpTenant, []string{bonus.ActionBusinessCalculate, bonus.ActionAllocationManage, bonus.ActionAllocationAmountRead}, "project", httpProject)
	response = doHTTP(t, http.MethodPost, baseURL+"/api/v1/bonus-pools/business-drafts", wrongAudience, body)
	assertProblem(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")
	wrongTenant := integrationToken(t, privateKey, "bonus-distribution", httpTenantB, []string{bonus.ActionPoolRead}, "project", httpProject)
	response = doHTTP(t, http.MethodGet, baseURL+"/api/v1/bonus-pools/"+poolID, wrongTenant, nil)
	assertProblem(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")
	wrongProject := integrationToken(t, privateKey, "bonus-distribution", httpTenant, []string{bonus.ActionPoolRead}, "project", httpProjectB)
	response = doHTTP(t, http.MethodGet, baseURL+"/api/v1/bonus-pools/"+poolID, wrongProject, nil)
	assertProblem(t, response, http.StatusForbidden, "SCOPE_DENIED")
	adjacentRole := integrationToken(t, privateKey, "bonus-distribution", httpTenant, []string{bonus.ActionProductionCalculate, bonus.ActionAllocationManage, bonus.ActionAllocationAmountRead}, "project", httpProject)
	response = doHTTP(t, http.MethodPost, baseURL+"/api/v1/bonus-pools/business-drafts", adjacentRole, body)
	assertProblem(t, response, http.StatusForbidden, "CAPABILITY_DENIED")
	adjacentResource := integrationToken(t, privateKey, "bonus-distribution", httpTenant, []string{bonus.ActionPoolRead}, "bonus_pool", httpProjectB)
	response = doHTTP(t, http.MethodGet, baseURL+"/api/v1/bonus-pools/"+poolID, adjacentResource, nil)
	assertProblem(t, response, http.StatusForbidden, "SCOPE_DENIED")
}

func integrationIAMProvider(t *testing.T) (*auth.IAMProvider, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "keys.json")
	data, _ := json.Marshal(map[string]any{"keys": []map[string]any{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": httpKeyID, "x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active"}}})
	if err = os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := auth.NewIAMProvider("https://iam.example.test", httpTenant, path)
	if err != nil {
		t.Fatal(err)
	}
	return provider, privateKey
}

func integrationToken(t *testing.T, privateKey ed25519.PrivateKey, audience, tenant string, actions []string, resourceType, resourceID string) string {
	t.Helper()
	now := time.Now().UTC().Unix()
	header, _ := json.Marshal(map[string]any{"alg": "EdDSA", "typ": "JWT", "kid": httpKeyID})
	claims, _ := json.Marshal(map[string]any{"ver": "1", "iss": "https://iam.example.test", "sub": httpSubject, "tid": tenant, "aud": audience, "name": "C7 tester", "actions": actions, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": resourceType, "id": resourceID}}}, "iat": now, "nbf": now - 1, "exp": now + 120, "auth_time": now - 1, "jti": httpTokenID})
	encoded := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(encoded)))
}

func doHTTP(t *testing.T, method, url, token string, body []byte) *http.Response {
	t.Helper()
	request, _ := http.NewRequest(method, url, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decodeResponse(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatal(err)
	}
}

func readBody(response *http.Response) string {
	defer response.Body.Close()
	value, _ := io.ReadAll(response.Body)
	return string(value)
}

func assertProblem(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	defer response.Body.Close()
	var value struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(response.Body).Decode(&value)
	if response.StatusCode != status || value.Error.Code != code {
		t.Fatalf("status=%d code=%s want=%d/%s", response.StatusCode, value.Error.Code, status, code)
	}
}

func assertCounts(t *testing.T, pool *pgxpool.Pool, pools, poolRevisions, allocations, allocationRevisions, commands, audits int) {
	t.Helper()
	var values [6]int
	err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM bonus_pools),(SELECT count(*) FROM bonus_pool_revisions),(SELECT count(*) FROM bonus_allocations),(SELECT count(*) FROM bonus_allocation_revisions),(SELECT count(*) FROM bonus_distribution_commands),(SELECT count(*) FROM bonus_distribution_audit_events)`).Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5])
	if err != nil {
		t.Fatal(err)
	}
	want := [6]int{pools, poolRevisions, allocations, allocationRevisions, commands, audits}
	if values != want {
		t.Fatalf("counts=%v want=%v", values, want)
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func httpIntegrationPool(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "bonus_http_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(databaseURL)
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
