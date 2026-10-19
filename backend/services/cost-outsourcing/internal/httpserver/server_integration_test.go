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
	"omsbizfoundry/cost-outsourcing/internal/auth"
	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	"omsbizfoundry/cost-outsourcing/internal/database"
	"omsbizfoundry/cost-outsourcing/internal/references"
)

const (
	httpTenant   = "11111111-1111-4111-8111-111111111111"
	httpTenantB  = "11111111-1111-4111-8111-111111111112"
	httpSubject  = "22222222-2222-4222-8222-222222222222"
	httpProject  = "33333333-3333-4333-8333-333333333333"
	httpProjectB = "33333333-3333-4333-8333-333333333334"
	httpParty    = "44444444-4444-4444-8444-444444444444"
	httpContract = "55555555-5555-4555-8555-555555555555"
	httpEvidence = "66666666-6666-4666-8666-666666666666"
	httpCommand  = "77777777-7777-4777-8777-777777777777"
	httpCommand2 = "77777777-7777-4777-8777-777777777778"
	httpCommand3 = "77777777-7777-4777-8777-777777777779"
	httpKeyID    = "88888888-8888-4888-8888-888888888888"
	httpTokenID  = "99999999-9999-4999-8999-999999999999"
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
	mu                      sync.RWMutex
	healthy, providerLinked bool
}

func (m *referenceMode) set(healthy, linked bool) {
	m.mu.Lock()
	m.healthy, m.providerLinked = healthy, linked
	m.mu.Unlock()
}
func (m *referenceMode) values() (bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy, m.providerLinked
}

func TestProtectedHTTPWithIAMExactScopesReferencesFailureRecoveryAndRedaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := httpIntegrationPool(t, ctx)
	defer cleanup()
	mode := &referenceMode{}
	referenceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthy, providerLinked := mode.values()
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
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","project_number":"P-C6-001","display_summary":"C6 验证项目","status":"active","revision":7,"resource_links":[{"target_service":"contract-ledger","resource_type":"contract","resource_id":%q,"source_revision":9}]}`, httpProject, httpTenant, httpContract)
		case "/api/v1/references/parties/" + httpParty:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"party-directory","id":%q,"tenant_id":%q,"kind":"organization","legal_name":"协作单位甲","status":"active","revision":5}`, httpParty, httpTenant)
		case "/api/v1/references/contracts/" + httpContract:
			parties := `[]`
			if providerLinked {
				parties = fmt.Sprintf(`[%q]`, httpParty)
			}
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"contract-ledger","id":%q,"tenant_id":%q,"resource_type":"contract","display_summary":"测量外委合同","status":"active","revision":9,"currency":"CNY","current_effective_amount":"5000.00","party_ids":%s}`, httpContract, httpTenant, parties)
		case "/v1/references/files/" + httpEvidence + "/versions/2":
			_, _ = fmt.Fprintf(w, `{"id":%q,"file_id":%q,"tenant_id":%q,"resource_type":"evidence-file","status":"available","version":2,"revision":2,"display_name":"凭证.pdf","sha256":"%s"}`, httpEvidence, httpEvidence, httpTenant, strings.Repeat("a", 64))
		default:
			http.NotFound(w, r)
		}
	}))
	defer referenceServer.Close()
	credentials := &httpCredentials{}
	endpoint := func(audience, action string) references.Endpoint {
		return references.Endpoint{BaseURL: referenceServer.URL, Audience: audience, Action: action, Credentials: credentials}
	}
	resolver, err := references.NewHTTPResolver(time.Second, endpoint("project-workspace", "project-workspace.read"), endpoint("party-directory", "party-directory.reference.read"), endpoint("contract-ledger", "contract-ledger.reference.read"), endpoint("evidence-service", "evidence-service.metadata.read"))
	if err != nil {
		t.Fatal(err)
	}
	store, _ := costoutsourcing.NewPostgresStore(pool)
	service, _ := costoutsourcing.NewService(store, resolver)
	provider, privateKey := integrationIAMProvider(t)
	server, err := New(service, provider)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	writeActions := []string{auth.ActionCostCreate, auth.ActionCostAmountRead, auth.ActionCostEvidenceManage}
	writeToken := integrationToken(t, privateKey, "cost-outsourcing", httpTenant, writeActions, "project", httpProject)
	costBody := []byte(fmt.Sprintf(`{"command_id":%q,"project_ref":{"id":%q,"revision":7},"category":"vehicle","occurred_on":"2026-09-28","currency":"CNY","amount_minor":12345,"distance_meters":6789,"description":"项目现场车辆成本","evidence_refs":[{"file_id":%q,"version":2,"slot":"receipt"}]}`, httpCommand, httpProject, httpEvidence))
	mode.set(false, true)
	response := doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/cost-entries", writeToken, costBody)
	assertProblem(t, response, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE")
	assertCounts(t, pool, 0, 0, 0, 0)

	mode.set(true, true)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/cost-entries", writeToken, costBody)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("record status=%d body=%s", response.StatusCode, readBody(response))
	}
	var created costView
	decodeResponse(t, response, &created)
	if created.ID == "" || created.AmountMinor == nil || *created.AmountMinor != 12345 || len(created.Evidence) != 1 {
		t.Fatalf("created=%#v", created)
	}
	assertCounts(t, pool, 1, 1, 1, 1)

	mode.set(false, true)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/cost-entries", writeToken, costBody)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("replay status=%d body=%s", response.StatusCode, readBody(response))
	}
	var replay costView
	decodeResponse(t, response, &replay)
	if replay.ID != created.ID {
		t.Fatalf("replay id=%s want=%s", replay.ID, created.ID)
	}
	assertCounts(t, pool, 1, 1, 1, 1)
	conflict := bytes.Replace(costBody, []byte("项目现场车辆成本"), []byte("不同成本"), 1)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/cost-entries", writeToken, conflict)
	assertProblem(t, response, http.StatusConflict, "COMMAND_PAYLOAD_CONFLICT")

	readToken := integrationToken(t, privateKey, "cost-outsourcing", httpTenant, []string{auth.ActionCostRead}, "project", httpProject)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/cost-entries/"+created.ID, readToken, nil)
	var redacted costView
	decodeResponse(t, response, &redacted)
	if response.StatusCode != http.StatusOK || redacted.AmountMinor != nil || redacted.Currency != nil {
		t.Fatalf("redacted status=%d value=%#v", response.StatusCode, redacted)
	}
	exactToken := integrationToken(t, privateKey, "cost-outsourcing", httpTenant, []string{auth.ActionCostRead, auth.ActionCostAmountRead}, "cost_entry", created.ID)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/cost-entries/"+created.ID, exactToken, nil)
	var exact costView
	decodeResponse(t, response, &exact)
	if response.StatusCode != http.StatusOK || exact.AmountMinor == nil || *exact.AmountMinor != 12345 {
		t.Fatalf("exact status=%d value=%#v", response.StatusCode, exact)
	}
	adjacentToken := integrationToken(t, privateKey, "cost-outsourcing", httpTenant, []string{auth.ActionCostRead}, "cost_entry", httpProjectB)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/cost-entries/"+created.ID, adjacentToken, nil)
	assertProblem(t, response, http.StatusForbidden, "SCOPE_DENIED")

	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/cost-entries", "", costBody)
	assertProblem(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")
	wrongAudience := integrationToken(t, privateKey, "approval-audit-results", httpTenant, writeActions, "project", httpProject)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/cost-entries", wrongAudience, costBody)
	assertProblem(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")
	wrongTenant := integrationToken(t, privateKey, "cost-outsourcing", httpTenantB, []string{auth.ActionCostRead}, "project", httpProject)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/cost-entries/"+created.ID, wrongTenant, nil)
	assertProblem(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")
	adjacentRole := integrationToken(t, privateKey, "cost-outsourcing", httpTenant, []string{auth.ActionEngagementRead}, "project", httpProject)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/cost-entries", adjacentRole, costBody)
	assertProblem(t, response, http.StatusForbidden, "CAPABILITY_DENIED")
	missingAmount := integrationToken(t, privateKey, "cost-outsourcing", httpTenant, []string{auth.ActionCostCreate, auth.ActionCostEvidenceManage}, "project", httpProject)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/cost-entries", missingAmount, costBody)
	assertProblem(t, response, http.StatusForbidden, "CAPABILITY_DENIED")
	missingEvidence := integrationToken(t, privateKey, "cost-outsourcing", httpTenant, []string{auth.ActionCostCreate, auth.ActionCostAmountRead}, "project", httpProject)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/cost-entries", missingEvidence, bytes.Replace(costBody, []byte(httpCommand), []byte(httpCommand2), 1))
	assertProblem(t, response, http.StatusForbidden, "CAPABILITY_DENIED")

	mode.set(true, false)
	engagementActions := []string{auth.ActionEngagementCreate, auth.ActionEngagementAmountRead, auth.ActionEngagementEvidenceManage}
	engagementToken := integrationToken(t, privateKey, "cost-outsourcing", httpTenant, engagementActions, "project", httpProject)
	engagementBody := []byte(fmt.Sprintf(`{"command_id":%q,"project_ref":{"id":%q,"revision":7},"provider_ref":{"id":%q,"revision":5},"scope":"桥梁测量协作","currency":"CNY","amount_minor":500000,"contract_ref":{"application":"contract-ledger","resource_type":"contract","resource_id":%q,"revision":9},"evidence_refs":[{"file_id":%q,"version":2,"slot":"contract"}]}`, httpCommand2, httpProject, httpParty, httpContract, httpEvidence))
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/outsourcing-engagements", engagementToken, engagementBody)
	assertProblem(t, response, http.StatusUnprocessableEntity, "REFERENCE_INVALID")
	assertEngagementCounts(t, pool, 0, 0)
	mode.set(true, true)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/outsourcing-engagements", engagementToken, engagementBody)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("engagement status=%d body=%s", response.StatusCode, readBody(response))
	}
	var engagement engagementView
	decodeResponse(t, response, &engagement)
	if engagement.ID == "" || engagement.Contract == nil || engagement.Contract.PartyID != httpParty || engagement.Contract.AmountMinor == nil || *engagement.Contract.AmountMinor != 500000 {
		t.Fatalf("engagement=%#v", engagement)
	}
	assertEngagementCounts(t, pool, 1, 1)

	correctToken := integrationToken(t, privateKey, "cost-outsourcing", httpTenant, []string{auth.ActionCostCorrect, auth.ActionCostAmountRead, auth.ActionCostEvidenceManage}, "cost_entry", created.ID)
	correction := []byte(fmt.Sprintf(`{"command_id":%q,"expected_revision":9,"reason":"错误旧版本","category":"vehicle","occurred_on":"2026-09-28","currency":"CNY","amount_minor":12445,"distance_meters":6789,"description":"补录停车费","evidence_refs":[{"file_id":%q,"version":2,"slot":"receipt"}]}`, httpCommand3, httpEvidence))
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/cost-entries/"+created.ID+"/corrections", correctToken, correction)
	assertProblem(t, response, http.StatusConflict, "REVISION_CONFLICT")
	assertCounts(t, pool, 1, 1, 2, 2)

	credentials.mu.Lock()
	joinedCalls := strings.Join(credentials.calls, "\n")
	credentials.mu.Unlock()
	for _, wanted := range []string{"project-workspace:project-workspace.read:project:" + httpProject, "party-directory:party-directory.reference.read:party:" + httpParty, "contract-ledger:contract-ledger.reference.read:contract:" + httpContract, "evidence-service:evidence-service.metadata.read:file:" + httpEvidence} {
		if !strings.Contains(joinedCalls, wanted) {
			t.Fatalf("missing exact workload call %s in %s", wanted, joinedCalls)
		}
	}
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
	claims, _ := json.Marshal(map[string]any{"ver": "1", "iss": "https://iam.example.test", "sub": httpSubject, "tid": tenant, "aud": audience, "name": "C6 tester", "actions": actions, "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": resourceType, "id": resourceID}}}, "iat": now, "nbf": now - 1, "exp": now + 120, "auth_time": now - 1, "jti": httpTokenID})
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

func assertCounts(t *testing.T, pool *pgxpool.Pool, current, revisions, commands, audits int) {
	t.Helper()
	var a, b, c, d int
	if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM cost_entries),(SELECT count(*) FROM cost_entry_revisions),(SELECT count(*) FROM cost_outsourcing_commands),(SELECT count(*) FROM cost_outsourcing_audit_events)`).Scan(&a, &b, &c, &d); err != nil {
		t.Fatal(err)
	}
	if a != current || b != revisions || c != commands || d != audits {
		t.Fatalf("cost counts=%d/%d/%d/%d want=%d/%d/%d/%d", a, b, c, d, current, revisions, commands, audits)
	}
}
func assertEngagementCounts(t *testing.T, pool *pgxpool.Pool, current, revisions int) {
	t.Helper()
	var a, b int
	if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM outsourcing_engagements),(SELECT count(*) FROM outsourcing_engagement_revisions)`).Scan(&a, &b); err != nil {
		t.Fatal(err)
	}
	if a != current || b != revisions {
		t.Fatalf("engagement counts=%d/%d want=%d/%d", a, b, current, revisions)
	}
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
	schema := "cost_http_test_" + hex.EncodeToString(random[:])
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
