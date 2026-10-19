//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"omsbizfoundry/approval-audit-results/internal/auth"
	"omsbizfoundry/approval-audit-results/internal/results"
)

type materialSeedResolver struct{ results.ReferenceResolver }

func (*materialSeedResolver) ResolveProject(_ context.Context, tenant string, ref results.ProjectRef) (results.ProjectReference, error) {
	return results.ProjectReference{ProjectSnapshot: results.ProjectSnapshot{ID: ref.ID, Revision: ref.Revision, Code: "C5-material-fixture", DisplayName: "synthetic report project"}, TenantID: tenant, Status: "active"}, nil
}
func (*materialSeedResolver) ResolveSource(_ context.Context, tenant string, project results.ProjectRef, ref results.SourceRef) (results.SourceReference, error) {
	components, total, err := results.NormalizeComponents([]results.AmountComponent{{Kind: results.ComponentDesignFee, AmountMinor: 125}, {Kind: results.ComponentSurveyFee}, {Kind: results.ComponentConsultationFee}}, false)
	if err != nil {
		return results.SourceReference{}, err
	}
	return results.SourceReference{SourceSnapshot: results.SourceSnapshot{SourceRef: ref, State: "confirmed", ProjectID: project.ID, Currency: "CNY", AmountProfile: results.AmountProfileReferenceOAV1, AmountComponents: components, TotalAmountMinor: total, Digest: "synthetic-private-amount-digest", DisplaySummary: "synthetic confirmed settlement"}, TenantID: tenant, Eligible: true}, nil
}
func (*materialSeedResolver) ResolveEvidence(_ context.Context, tenant string, ref results.EvidenceRef) (results.EvidenceReference, error) {
	return results.EvidenceReference{EvidenceSnapshot: results.EvidenceSnapshot{EvidenceRef: ref, Digest: "synthetic-byte-digest", DisplayName: "synthetic-report.pdf"}, TenantID: tenant, Status: "available"}, nil
}

type materialReadNoUpstream struct{ results.ReferenceResolver }

type resultsMaterialIAMClient struct {
	client     *http.Client
	base, csrf string
}

func resultsMaterialIAMRequest(t *testing.T, client resultsMaterialIAMClient, method, path string, input any, expected int, output any) {
	t.Helper()
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, client.base+path, body)
	if err != nil {
		t.Fatal("invalid private IAM endpoint")
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CSRF-Token", client.csrf)
	}
	response, err := client.client.Do(request)
	if err != nil {
		t.Fatal("private IAM request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != expected {
		t.Fatalf("private IAM %s %s status=%d expected=%d", method, path, response.StatusCode, expected)
	}
	if output != nil && json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(output) != nil {
		t.Fatal("invalid private IAM JSON")
	}
}
func resultsMaterialIAMLogin(t *testing.T, base, username, password string) resultsMaterialIAMClient {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := resultsMaterialIAMClient{base: base, client: &http.Client{Jar: jar, Timeout: 15 * time.Second}}
	var csrf struct {
		Token string `json:"csrf_token"`
	}
	resultsMaterialIAMRequest(t, client, "GET", "/api/v1/csrf", nil, 200, &csrf)
	client.csrf = csrf.Token
	resultsMaterialIAMRequest(t, client, "POST", "/api/v1/login", map[string]string{"username": username, "password": password}, 200, nil)
	return client
}
func resultsMaterialIAMCredential(t *testing.T, admin resultsMaterialIAMClient, label, password, action string, scope map[string]any) string {
	t.Helper()
	var invitation struct {
		Token   string `json:"token"`
		Subject struct {
			ID string `json:"id"`
		} `json:"subject"`
	}
	resultsMaterialIAMRequest(t, admin, "POST", "/api/v1/admin/invitations", map[string]string{"username": "results-material-" + label, "display_name": "results-material-" + label}, 201, &invitation)
	jar, _ := cookiejar.New(nil)
	activation := resultsMaterialIAMClient{base: admin.base, client: &http.Client{Jar: jar, Timeout: 15 * time.Second}}
	var csrf struct {
		Token string `json:"csrf_token"`
	}
	resultsMaterialIAMRequest(t, activation, "GET", "/api/v1/csrf", nil, 200, &csrf)
	activation.csrf = csrf.Token
	resultsMaterialIAMRequest(t, activation, "POST", "/api/v1/activate", map[string]string{"token": invitation.Token, "password": password}, 200, nil)
	resultsMaterialIAMRequest(t, admin, "POST", "/api/v1/admin/grants", map[string]any{"subject_id": invitation.Subject.ID, "audience": "approval-audit-results", "action": action, "scope": scope}, 204, nil)
	client := resultsMaterialIAMLogin(t, admin.base, "results-material-"+label, password)
	var credential struct {
		Token string `json:"token"`
	}
	resultsMaterialIAMRequest(t, client, "POST", "/api/v1/credentials", map[string]any{"audience": "approval-audit-results", "scope": scope}, 200, &credential)
	parts := strings.Split(credential.Token, ".")
	if len(parts) != 3 {
		t.Fatal("IAM did not issue a compact signed token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal("invalid IAM claims")
	}
	var claims struct {
		Tenant   string         `json:"tid"`
		Audience string         `json:"aud"`
		Actions  []string       `json:"actions"`
		Scope    map[string]any `json:"scope"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.Tenant != httpTenant || claims.Audience != "approval-audit-results" || len(claims.Actions) != 1 || claims.Actions[0] != action {
		t.Fatal("issued native action set or identity expanded")
	}
	actual, _ := json.Marshal(claims.Scope)
	expected, _ := json.Marshal(scope)
	if string(actual) != string(expected) {
		t.Fatal("issued scope differs from requested scope")
	}
	return credential.Token
}
func resultsMaterialHTTPFacts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	facts := make([][2]string, 0, 5)
	for _, table := range []string{"approval_audit_results", "approval_audit_result_revisions", "approval_audit_result_commands", "approval_audit_result_audit_events", "schema_migrations"} {
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

func TestResultsMaterialHTTPPostgresNativeIAM(t *testing.T) {
	base, issuer, keys, password := os.Getenv("RESULTS_MATERIAL_IAM_URL"), os.Getenv("RESULTS_MATERIAL_IAM_ISSUER"), os.Getenv("RESULTS_MATERIAL_IAM_KEYS"), os.Getenv("RESULTS_MATERIAL_IAM_PASSWORD")
	if base == "" || issuer == "" || keys == "" || password == "" {
		t.Skip("isolated real IAM fixture is not configured")
	}
	ctx := context.Background()
	pool, cleanup := httpIntegrationPool(t, ctx)
	defer cleanup()
	store, _ := results.NewPostgresStore(pool)
	writer, _ := results.NewService(store, &materialSeedResolver{})
	input := results.RecordInput{CommandID: httpCommand, ProjectRef: results.ProjectRef{ID: httpProject, Revision: 7}, Kind: results.KindApproval, Currency: "CNY", SourceRef: results.SourceRef{Application: "contract-ledger", ResourceType: "settlement", ResourceID: httpContract, Revision: 4}, EvidenceRefs: []results.EvidenceRef{{FileID: httpEvidence, Version: 2, Slot: "approval_document"}}, ExternalConfirmedBy: "synthetic written approval", ExternalConfirmedAt: time.Now().UTC()}
	actor := results.Actor{ID: httpSubject, TenantID: httpTenant, Kind: "test_subject", CredentialID: "synthetic-bootstrap"}
	created, err := writer.Record(ctx, actor, input)
	if err != nil {
		t.Fatal(err)
	}
	reader, _ := results.NewService(store, &materialReadNoUpstream{})
	provider, err := auth.NewIAMProvider(issuer, httpTenant, keys)
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(reader, provider)
	if err != nil {
		t.Fatal(err)
	}
	admin := resultsMaterialIAMLogin(t, base, "fixture-admin", password)
	scope := func(kind, id string) map[string]any {
		return map[string]any{"kind": "resources", "resources": []map[string]string{{"type": kind, "id": id}}}
	}
	tokens := []struct {
		label, token string
		status       int
	}{
		{"anonymous", "", 401},
		{"exact-result", resultsMaterialIAMCredential(t, admin, "exact", password, auth.ActionRead, scope("approval_audit_result", created.ID)), 200},
		{"neighbor-result", resultsMaterialIAMCredential(t, admin, "neighbor", password, auth.ActionRead, scope("approval_audit_result", httpContract)), 403},
		{"amount-without-read", resultsMaterialIAMCredential(t, admin, "amount", password, auth.ActionAmountRead, scope("approval_audit_result", created.ID)), 403},
		{"project", resultsMaterialIAMCredential(t, admin, "project", password, auth.ActionRead, scope("project", httpProject)), 200},
		{"tenant", resultsMaterialIAMCredential(t, admin, "tenant", password, auth.ActionRead, map[string]any{"kind": "tenant", "resources": []map[string]string{}}), 200},
	}
	before := resultsMaterialHTTPFacts(t, pool)
	for _, test := range tokens {
		request := httptest.NewRequest("GET", "/api/v1/references/approval-audit-results/"+created.ID+"/materials", nil)
		if test.token != "" {
			request.Header.Set("Authorization", "Bearer "+test.token)
		}
		request.Header.Set("X-Tenant-ID", "forged")
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != test.status || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("case=%s status=%d expected=%d", test.label, response.Code, test.status)
		}
		if test.status == 200 {
			var projection results.MaterialReferenceProjection
			if json.Unmarshal(response.Body.Bytes(), &projection) != nil || projection.ID != created.ID || projection.ProjectID != httpProject || projection.Revision != 1 || len(projection.Items) != 1 || projection.Items[0].Version != 2 || projection.Items[0].EvidenceSourceRevision != 2 {
				t.Fatal("wrong owned material projection")
			}
			for _, forbidden := range []string{"amount", "currency", "private", "object_key", "url", "credential"} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Fatalf("projection leaked %q", forbidden)
				}
			}
		}
	}
	wrongTenantProvider, err := auth.NewIAMProvider(issuer, httpTenantB, keys)
	if err != nil {
		t.Fatal(err)
	}
	wrongServer, _ := New(reader, wrongTenantProvider)
	request := httptest.NewRequest("GET", "/api/v1/references/approval-audit-results/"+created.ID+"/materials", nil)
	request.Header.Set("Authorization", "Bearer "+tokens[1].token)
	response := httptest.NewRecorder()
	wrongServer.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatal("wrong tenant accepted real signed credential")
	}
	if resultsMaterialHTTPFacts(t, pool) != before {
		t.Fatal("material HTTP reads changed Results-owned facts")
	}
	t.Log("actual IAM-issued native actions and result/project/tenant scopes, PG18, five owned table digests unchanged, no upstream read and no amount.read requirement")
}
