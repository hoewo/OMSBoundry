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
	"omsbizfoundry/billing-invoice/internal/auth"
	"omsbizfoundry/billing-invoice/internal/database"
	"omsbizfoundry/billing-invoice/internal/invoice"
	"omsbizfoundry/billing-invoice/internal/references"
)

const (
	integrationIssuerParty    = "77777777-7777-4777-8777-777777777777"
	integrationRecipientParty = "88888888-8888-4888-8888-888888888888"
	integrationKeyID          = "99999999-9999-4999-8999-999999999999"
	integrationTokenID        = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	integrationTenantB        = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	integrationCommand2       = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	integrationCommand3       = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	integrationCommand4       = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	integrationCommand5       = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	integrationEvidence       = "12121212-1212-4121-8121-121212121212"
)

type integrationCredentials struct {
	mu    sync.Mutex
	calls []string
}

func (s *integrationCredentials) Credential(_ context.Context, audience, action, resourceType, resourceID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, strings.Join([]string{audience, action, resourceType, resourceID}, ":"))
	return "exact-reference-token", nil
}

func (*integrationCredentials) Invalidate(string, string, string, string) {}

type sourceMode struct {
	mu      sync.RWMutex
	healthy bool
	status  string
}

func (m *sourceMode) set(healthy bool, status string) {
	m.mu.Lock()
	m.healthy, m.status = healthy, status
	m.mu.Unlock()
}

func (m *sourceMode) values() (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy, m.status
}

func TestProtectedHTTPWithNativeIAMPostgresAndTypedResolver(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := httpIntegrationPool(t, ctx)
	defer cleanup()

	mode := &sourceMode{}
	sources := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthy, projectStatus := mode.values()
		if !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.Header.Get("Authorization") != "Bearer exact-reference-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/references/parties/" + integrationIssuerParty:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"party-directory","id":%q,"tenant_id":%q,"kind":"organization","legal_name":"开票方","status":"active","revision":1}`, integrationIssuerParty, tenantID)
		case "/api/v1/references/parties/" + integrationRecipientParty:
			_, _ = fmt.Fprintf(w, `{"schema_version":"1","source_application":"party-directory","id":%q,"tenant_id":%q,"kind":"organization","legal_name":"受票方","status":"active","revision":1}`, integrationRecipientParty, tenantID)
		case "/api/v1/references/projects/" + projectA:
			_, _ = fmt.Fprintf(w, `{"id":%q,"tenant_id":%q,"resource_type":"project","display_summary":"OA 项目 A","status":%q,"revision":1}`, projectA, tenantID, projectStatus)
		case "/v1/references/files/" + integrationEvidence + "/versions/1":
			_, _ = fmt.Fprintf(w, `{"id":%q,"file_id":%q,"tenant_id":%q,"resource_type":"evidence-file","display_name":"发票.pdf","status":"available","version":1,"revision":1}`, integrationEvidence, integrationEvidence, tenantID)
		default:
			http.NotFound(w, r)
		}
	}))
	defer sources.Close()

	credentials := &integrationCredentials{}
	endpoint := func(audience, action string) references.Endpoint {
		return references.Endpoint{BaseURL: sources.URL, Audience: audience, Action: action, Credentials: credentials}
	}
	resolver, err := references.NewHTTPResolver(time.Second,
		endpoint("party-directory", "party-directory.reference.read"),
		endpoint("project-workspace", "project-workspace.read"),
		endpoint("contract-ledger", "contract-ledger.reference.read"),
		endpoint("money-flow-ledger", "money-flow-ledger.reference.read"),
		endpoint("evidence-service", "evidence-service.metadata.read"),
	)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := invoice.NewPostgresStore(pool)
	service, err := invoice.NewService(store, resolver, "CNY")
	if err != nil {
		t.Fatal(err)
	}
	provider, privateKey := integrationIAMProvider(t)
	server, err := New(service, provider)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	writeToken := integrationIAMToken(t, privateKey, tenantID, []string{auth.ActionRecord, auth.ActionAmountRead}, projectA)
	body := []byte(strings.Replace(createBody(projectA), `"amount_minor":12345`, `"amount_minor":900719925474099301`, 1))
	blindToken := integrationIAMToken(t, privateKey, tenantID, []string{auth.ActionRecord}, projectA)
	blindResponse := doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/invoice-registrations", blindToken, body)
	assertProblem(t, blindResponse, http.StatusForbidden, "CAPABILITY_DENIED")
	assertFactCounts(t, pool, 0, 0, 0, 0)
	mode.set(false, "active")
	response := doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/invoice-registrations", writeToken, body)
	assertProblem(t, response, http.StatusServiceUnavailable, "REFERENCE_SERVICE_UNAVAILABLE")
	assertFactCounts(t, pool, 0, 0, 0, 0)

	mode.set(true, "active")
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/invoice-registrations", writeToken, body)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("recovery create status=%d body=%s", response.StatusCode, readBody(response))
	}
	var created RegistrationView
	decodeResponse(t, response, &created)
	if created.ID == "" || !created.AmountVisible || created.AmountMinor == nil || *created.AmountMinor != 900719925474099301 || created.AmountMinorDecimal == nil || *created.AmountMinorDecimal != "900719925474099301" || created.Currency == nil || created.Revision != 1 {
		t.Fatalf("created view=%#v", created)
	}
	assertFactCounts(t, pool, 1, 1, 1, 1)

	mode.set(false, "active")
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/invoice-registrations", writeToken, body)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("success replay status=%d body=%s", response.StatusCode, readBody(response))
	}
	var replayed RegistrationView
	decodeResponse(t, response, &replayed)
	if replayed.ID != created.ID {
		t.Fatalf("replay id=%s want=%s", replayed.ID, created.ID)
	}
	assertFactCounts(t, pool, 1, 1, 1, 1)

	readToken := integrationIAMToken(t, privateKey, tenantID, []string{auth.ActionRead, auth.ActionHistoryRead}, projectA)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/invoice-registrations/"+created.ID, readToken, nil)
	var redacted RegistrationView
	if response.StatusCode != http.StatusOK {
		t.Fatalf("redacted get status=%d body=%s", response.StatusCode, readBody(response))
	}
	decodeResponse(t, response, &redacted)
	if redacted.AmountVisible || redacted.AmountMinor != nil || redacted.AmountMinorDecimal != nil || redacted.Currency != nil {
		t.Fatalf("amount leaked: %#v", redacted)
	}

	amountToken := integrationIAMToken(t, privateKey, tenantID, []string{auth.ActionRead, auth.ActionAmountRead}, projectA)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/invoice-registrations/"+created.ID, amountToken, nil)
	var visible RegistrationView
	decodeResponse(t, response, &visible)
	if response.StatusCode != http.StatusOK || !visible.AmountVisible || visible.AmountMinor == nil || *visible.AmountMinor != 900719925474099301 || visible.AmountMinorDecimal == nil || *visible.AmountMinorDecimal != "900719925474099301" {
		t.Fatalf("visible view status=%d view=%#v", response.StatusCode, visible)
	}

	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/invoice-registrations/"+created.ID+"/history", readToken, nil)
	var history historyResponse
	decodeResponse(t, response, &history)
	if response.StatusCode != http.StatusOK || len(history.Items) != 1 || history.Items[0].Snapshot.AmountVisible || history.Items[0].Snapshot.AmountMinor != nil || history.Items[0].Snapshot.AmountMinorDecimal != nil {
		t.Fatalf("history status=%d value=%#v", response.StatusCode, history)
	}

	adjacentRole := integrationIAMToken(t, privateKey, tenantID, []string{auth.ActionRead}, projectA)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/invoice-registrations", adjacentRole, body)
	assertProblem(t, response, http.StatusForbidden, "CAPABILITY_DENIED")
	projectBToken := integrationIAMToken(t, privateKey, tenantID, []string{auth.ActionRecord, auth.ActionAmountRead}, projectB)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/invoice-registrations", projectBToken, body)
	assertProblem(t, response, http.StatusForbidden, "SCOPE_DENIED")
	crossTenant := integrationIAMToken(t, privateKey, integrationTenantB, []string{auth.ActionRead}, projectA)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/invoice-registrations/"+created.ID, crossTenant, nil)
	assertProblem(t, response, http.StatusUnauthorized, "UNAUTHENTICATED")

	mode.set(true, "cancelled")
	wrongStateBody := []byte(strings.Replace(createBody(projectA), commandID, integrationCommand2, 1))
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/invoice-registrations", writeToken, wrongStateBody)
	assertProblem(t, response, http.StatusUnprocessableEntity, "REFERENCE_INVALID")
	assertFactCounts(t, pool, 1, 1, 1, 1)

	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/invoice-registrations", readToken, nil)
	assertProblem(t, response, http.StatusForbidden, "SCOPE_DENIED")
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/invoice-registrations?project_id="+projectA, amountToken, nil)
	var listed listResponse
	decodeResponse(t, response, &listed)
	if response.StatusCode != http.StatusOK || len(listed.Items) != 1 || listed.EffectiveAmountMinor == nil || *listed.EffectiveAmountMinor != 900719925474099301 || listed.Items[0].AmountMinorDecimal == nil || *listed.Items[0].AmountMinorDecimal != "900719925474099301" || listed.Currency == nil || *listed.Currency != "CNY" {
		t.Fatalf("list status=%d value=%#v", response.StatusCode, listed)
	}

	correctToken := integrationIAMToken(t, privateKey, tenantID, []string{auth.ActionCorrect}, projectA)
	adjacentCorrection := []byte(`{"command_id":"` + integrationCommand3 + `","expected_revision":1,"reason":"非法换项目","patch":{"source_refs":[{"kind":"project","resource_id":"` + projectB + `","source_revision":1}]}}`)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/invoice-registrations/"+created.ID+"/corrections", correctToken, adjacentCorrection)
	assertProblem(t, response, http.StatusForbidden, "SCOPE_DENIED")
	assertFactCounts(t, pool, 1, 1, 1, 1)

	correction := []byte(`{"command_id":"` + integrationCommand3 + `","expected_revision":1,"reason":"补充登记说明","patch":{"note":"已核对纸质票据"}}`)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/invoice-registrations/"+created.ID+"/corrections", correctToken, correction)
	var corrected RegistrationView
	decodeResponse(t, response, &corrected)
	if response.StatusCode != http.StatusOK || corrected.Revision != 2 || corrected.Note != "已核对纸质票据" {
		t.Fatalf("correction status=%d value=%#v", response.StatusCode, corrected)
	}

	mode.set(true, "active")
	evidenceToken := integrationIAMToken(t, privateKey, tenantID, []string{auth.ActionEvidenceManage}, projectA)
	evidenceBody := []byte(`{"command_id":"` + integrationCommand4 + `","expected_revision":2,"reason":"关联票据扫描件","evidence_refs":[{"file_id":"` + integrationEvidence + `","version":1,"purpose":"invoice_document","display_name":"发票.pdf"}]}`)
	response = doHTTP(t, http.MethodPut, httpServer.URL+"/api/v1/invoice-registrations/"+created.ID+"/evidence", evidenceToken, evidenceBody)
	var withEvidence RegistrationView
	decodeResponse(t, response, &withEvidence)
	if response.StatusCode != http.StatusOK || withEvidence.Revision != 3 || len(withEvidence.EvidenceRefs) != 1 {
		t.Fatalf("evidence status=%d value=%#v", response.StatusCode, withEvidence)
	}

	referenceToken := integrationIAMResourceToken(t, privateKey, tenantID, []string{auth.ActionReferenceRead}, "invoice_registration", created.ID)
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/references/invoice-registrations/"+created.ID, referenceToken, nil)
	var reference invoice.Reference
	decodeResponse(t, response, &reference)
	if response.StatusCode != http.StatusOK || reference.ID != created.ID || reference.Revision != 3 || reference.AmountMinor != 900719925474099301 {
		t.Fatalf("reference status=%d value=%#v", response.StatusCode, reference)
	}

	invalidateToken := integrationIAMToken(t, privateKey, tenantID, []string{auth.ActionRegistrationInvalidate}, projectA)
	invalidateBody := []byte(`{"command_id":"` + integrationCommand5 + `","expected_revision":3,"reason":"重复登记"}`)
	response = doHTTP(t, http.MethodPost, httpServer.URL+"/api/v1/invoice-registrations/"+created.ID+"/invalidation", invalidateToken, invalidateBody)
	var invalidated RegistrationView
	decodeResponse(t, response, &invalidated)
	if response.StatusCode != http.StatusOK || invalidated.Status != invoice.StatusRegistrationInvalidated || invalidated.Revision != 4 {
		t.Fatalf("invalidation status=%d value=%#v", response.StatusCode, invalidated)
	}

	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/invoice-registrations?project_id="+projectA, amountToken, nil)
	decodeResponse(t, response, &listed)
	if response.StatusCode != http.StatusOK || listed.EffectiveAmountMinor == nil || *listed.EffectiveAmountMinor != 0 {
		t.Fatalf("post-invalidation list status=%d value=%#v", response.StatusCode, listed)
	}
	response = doHTTP(t, http.MethodGet, httpServer.URL+"/api/v1/invoice-registrations/"+created.ID+"/history", readToken, nil)
	decodeResponse(t, response, &history)
	if response.StatusCode != http.StatusOK || len(history.Items) != 4 || history.Items[0].Revision != 4 {
		t.Fatalf("lifecycle history status=%d value=%#v", response.StatusCode, history)
	}
	assertFactCounts(t, pool, 1, 4, 4, 4)

	credentials.mu.Lock()
	defer credentials.mu.Unlock()
	if len(credentials.calls) < 3 || !strings.Contains(strings.Join(credentials.calls, "\n"), "project-workspace:project-workspace.read:project:"+projectA) {
		t.Fatalf("credential calls=%#v", credentials.calls)
	}
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
	schema := "billing_http_" + hex.EncodeToString(random[:])
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
	keySet := map[string]any{"keys": []map[string]any{{
		"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": integrationKeyID,
		"x": base64.RawURLEncoding.EncodeToString(publicKey), "status": "active",
	}}}
	raw, _ := json.Marshal(keySet)
	path := filepath.Join(t.TempDir(), "keys.json")
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := auth.NewIAMProvider("https://iam.billing.test", tenantID, path)
	if err != nil {
		t.Fatal(err)
	}
	return provider, privateKey
}

func integrationIAMToken(t *testing.T, privateKey ed25519.PrivateKey, tokenTenant string, actions []string, project string) string {
	return integrationIAMResourceToken(t, privateKey, tokenTenant, actions, "project", project)
}

func integrationIAMResourceToken(t *testing.T, privateKey ed25519.PrivateKey, tokenTenant string, actions []string, resourceType, resourceID string) string {
	t.Helper()
	now := time.Now().UTC()
	header, _ := json.Marshal(map[string]any{"alg": "EdDSA", "typ": "JWT", "kid": integrationKeyID})
	claims, _ := json.Marshal(map[string]any{
		"ver": "1", "iss": "https://iam.billing.test", "sub": subjectID, "tid": tokenTenant,
		"aud": "billing-invoice", "name": "HTTP 集成财务", "actions": actions,
		"scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": resourceType, "id": resourceID}}},
		"iat":   now.Add(-time.Second).Unix(), "nbf": now.Add(-time.Second).Unix(), "exp": now.Add(time.Minute).Unix(),
		"auth_time": now.Add(-time.Second).Unix(), "jti": integrationTokenID,
	})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(unsigned)))
}

func doHTTP(t *testing.T, method, url, token string, body []byte) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Actor-ID", "forged-client-identity")
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
	if response.StatusCode != status || !strings.Contains(body, code) {
		t.Fatalf("status=%d body=%s", response.StatusCode, body)
	}
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
	raw, _ := io.ReadAll(response.Body)
	return string(raw)
}

func assertFactCounts(t *testing.T, pool *pgxpool.Pool, registrations, revisions, commands, audits int) {
	t.Helper()
	var gotRegistrations, gotRevisions, gotCommands, gotAudits int
	err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM invoice_registrations),(SELECT count(*) FROM invoice_registration_revisions),(SELECT count(*) FROM invoice_registration_commands),(SELECT count(*) FROM invoice_registration_audit_events)`).Scan(&gotRegistrations, &gotRevisions, &gotCommands, &gotAudits)
	if err != nil {
		t.Fatal(err)
	}
	if gotRegistrations != registrations || gotRevisions != revisions || gotCommands != commands || gotAudits != audits {
		t.Fatalf("fact counts=%d/%d/%d/%d want=%d/%d/%d/%d", gotRegistrations, gotRevisions, gotCommands, gotAudits, registrations, revisions, commands, audits)
	}
}
