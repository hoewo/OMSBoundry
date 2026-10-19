//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"omsbizfoundry/cost-outsourcing/internal/auth"
	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	"os"
	"strings"
	"testing"
	"time"
)

type materialSeedResolver struct {
	costoutsourcing.ReferenceResolver
}

func (*materialSeedResolver) ResolveProject(_ context.Context, tenant string, ref costoutsourcing.ProjectRef) (costoutsourcing.ProjectReference, error) {
	return costoutsourcing.ProjectReference{ProjectSnapshot: costoutsourcing.ProjectSnapshot{ID: ref.ID, Revision: ref.Revision, Code: "C6-material-fixture", DisplayName: "synthetic cost project"}, TenantID: tenant, Status: "active"}, nil
}
func (*materialSeedResolver) ResolveParty(_ context.Context, tenant string, ref costoutsourcing.PartyRef) (costoutsourcing.PartyReference, error) {
	return costoutsourcing.PartyReference{PartySnapshot: costoutsourcing.PartySnapshot{ID: ref.ID, Revision: ref.Revision, Kind: "organization", LegalName: "synthetic provider"}, TenantID: tenant, Status: "active"}, nil
}
func (*materialSeedResolver) ResolveEvidence(_ context.Context, tenant string, ref costoutsourcing.EvidenceRef) (costoutsourcing.EvidenceReference, error) {
	return costoutsourcing.EvidenceReference{EvidenceSnapshot: costoutsourcing.EvidenceSnapshot{EvidenceRef: ref, Digest: "synthetic-file-digest", DisplayName: "synthetic-receipt.pdf"}, TenantID: tenant, Status: "available"}, nil
}

type materialReadNoUpstream struct {
	costoutsourcing.ReferenceResolver
}

type costMaterialIAMClient struct {
	client     *http.Client
	base, csrf string
}

func costMaterialIAMRequest(t *testing.T, client costMaterialIAMClient, method, path string, input any, expected int, output any) {
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
func costMaterialIAMLogin(t *testing.T, base, username, password string) costMaterialIAMClient {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := costMaterialIAMClient{base: base, client: &http.Client{Jar: jar, Timeout: 15 * time.Second}}
	var csrf struct {
		Token string `json:"csrf_token"`
	}
	costMaterialIAMRequest(t, client, "GET", "/api/v1/csrf", nil, 200, &csrf)
	client.csrf = csrf.Token
	costMaterialIAMRequest(t, client, "POST", "/api/v1/login", map[string]string{"username": username, "password": password}, 200, nil)
	return client
}
func costMaterialIAMCredential(t *testing.T, admin costMaterialIAMClient, label, password, action string, scope map[string]any) string {
	t.Helper()
	var invitation struct {
		Token   string `json:"token"`
		Subject struct {
			ID string `json:"id"`
		} `json:"subject"`
	}
	costMaterialIAMRequest(t, admin, "POST", "/api/v1/admin/invitations", map[string]string{"username": "results-material-" + label, "display_name": "results-material-" + label}, 201, &invitation)
	jar, _ := cookiejar.New(nil)
	activation := costMaterialIAMClient{base: admin.base, client: &http.Client{Jar: jar, Timeout: 15 * time.Second}}
	var csrf struct {
		Token string `json:"csrf_token"`
	}
	costMaterialIAMRequest(t, activation, "GET", "/api/v1/csrf", nil, 200, &csrf)
	activation.csrf = csrf.Token
	costMaterialIAMRequest(t, activation, "POST", "/api/v1/activate", map[string]string{"token": invitation.Token, "password": password}, 200, nil)
	costMaterialIAMRequest(t, admin, "POST", "/api/v1/admin/grants", map[string]any{"subject_id": invitation.Subject.ID, "audience": "cost-outsourcing", "action": action, "scope": scope}, 204, nil)
	client := costMaterialIAMLogin(t, admin.base, "results-material-"+label, password)
	var credential struct {
		Token string `json:"token"`
	}
	costMaterialIAMRequest(t, client, "POST", "/api/v1/credentials", map[string]any{"audience": "cost-outsourcing", "scope": scope}, 200, &credential)
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
	if json.Unmarshal(raw, &claims) != nil || claims.Tenant != httpTenant || claims.Audience != "cost-outsourcing" || len(claims.Actions) != 1 || claims.Actions[0] != action {
		t.Fatal("issued native action set or identity expanded")
	}
	actual, _ := json.Marshal(claims.Scope)
	expected, _ := json.Marshal(scope)
	if string(actual) != string(expected) {
		t.Fatal("issued scope differs from requested scope")
	}
	return credential.Token
}
func costMaterialHTTPFacts(t *testing.T, pool *pgxpool.Pool) string {
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

func TestCostMaterialHTTPPostgresNativeIAMBothAggregates(t *testing.T) {
	base, issuer, keys, password := os.Getenv("COST_MATERIAL_IAM_URL"), os.Getenv("COST_MATERIAL_IAM_ISSUER"), os.Getenv("COST_MATERIAL_IAM_KEYS"), os.Getenv("COST_MATERIAL_IAM_PASSWORD")
	if base == "" || issuer == "" || keys == "" || password == "" {
		t.Skip("isolated real IAM fixture is not configured")
	}
	ctx := context.Background()
	pool, cleanup := httpIntegrationPool(t, ctx)
	defer cleanup()
	store, _ := costoutsourcing.NewPostgresStore(pool)
	writer, _ := costoutsourcing.NewService(store, &materialSeedResolver{})
	actor := costoutsourcing.Actor{ID: httpSubject, TenantID: httpTenant, Kind: "test_subject", CredentialID: "synthetic-bootstrap", ProjectIDs: []string{httpProject}, Actions: []string{costoutsourcing.ActionCostCreate, costoutsourcing.ActionCostEvidenceManage, costoutsourcing.ActionEngageCreate, costoutsourcing.ActionEngageEvidenceManage}}
	files := []costoutsourcing.EvidenceRef{{FileID: httpEvidence, Version: 2, Slot: "supporting_document"}}
	cost, err := writer.RecordCost(ctx, actor, costoutsourcing.RecordCostInput{CommandID: httpCommand, ProjectRef: costoutsourcing.ProjectRef{ID: httpProject, Revision: 7}, Category: costoutsourcing.CategoryOther, OccurredOn: "2026-10-01", Currency: "CNY", AmountMinor: 125, Description: "synthetic cost", EvidenceRefs: files})
	if err != nil {
		t.Fatal(err)
	}
	engagement, err := writer.RecordEngagement(ctx, actor, costoutsourcing.RecordEngagementInput{CommandID: httpCommand2, ProjectRef: costoutsourcing.ProjectRef{ID: httpProject, Revision: 7}, ProviderRef: costoutsourcing.PartyRef{ID: httpParty, Revision: 5}, Scope: "synthetic outsourcing", Currency: "CNY", AmountMinor: 500, EvidenceRefs: files})
	if err != nil {
		t.Fatal(err)
	}
	reader, _ := costoutsourcing.NewService(store, &materialReadNoUpstream{})
	provider, err := auth.NewIAMProvider(issuer, httpTenant, keys)
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(reader, provider)
	if err != nil {
		t.Fatal(err)
	}
	admin := costMaterialIAMLogin(t, base, "fixture-admin", password)
	scope := func(kind, id string) map[string]any {
		return map[string]any{"kind": "resources", "resources": []map[string]string{{"type": kind, "id": id}}}
	}
	type testCase struct {
		label, path, token, kind, id string
		status                       int
	}
	tests := []testCase{}
	exact := map[string]string{}
	for index, aggregate := range []struct{ path, kind, id, read, amount string }{
		{"cost-entries", "cost_entry", cost.ID, auth.ActionCostRead, auth.ActionCostAmountRead},
		{"outsourcing-engagements", "outsourcing_engagement", engagement.ID, auth.ActionEngagementRead, auth.ActionEngagementAmountRead},
	} {
		label := []string{"cost", "engage"}[index]
		token := costMaterialIAMCredential(t, admin, label+"-exact", password, aggregate.read, scope(aggregate.kind, aggregate.id))
		exact[aggregate.kind] = token
		path := "/api/v1/references/" + aggregate.path + "/" + aggregate.id + "/materials"
		tests = append(tests, testCase{label: label + "-anonymous", path: path, kind: aggregate.kind, id: aggregate.id, status: 401},
			testCase{label: label + "-exact", path: path, token: token, kind: aggregate.kind, id: aggregate.id, status: 200},
			testCase{label: label + "-neighbor", path: path, token: costMaterialIAMCredential(t, admin, label+"-neighbor", password, aggregate.read, scope(aggregate.kind, httpParty)), kind: aggregate.kind, id: aggregate.id, status: 403},
			testCase{label: label + "-amount-only", path: path, token: costMaterialIAMCredential(t, admin, label+"-amount", password, aggregate.amount, scope(aggregate.kind, aggregate.id)), kind: aggregate.kind, id: aggregate.id, status: 403},
			testCase{label: label + "-project", path: path, token: costMaterialIAMCredential(t, admin, label+"-project", password, aggregate.read, scope("project", httpProject)), kind: aggregate.kind, id: aggregate.id, status: 200},
			testCase{label: label + "-tenant", path: path, token: costMaterialIAMCredential(t, admin, label+"-tenant", password, aggregate.read, map[string]any{"kind": "tenant", "resources": []map[string]string{}}), kind: aggregate.kind, id: aggregate.id, status: 200})
	}
	tests = append(tests, testCase{label: "cost-read-cannot-read-engagement", path: "/api/v1/references/outsourcing-engagements/" + engagement.ID + "/materials", token: exact["cost_entry"], status: 403},
		testCase{label: "engagement-read-cannot-read-cost", path: "/api/v1/references/cost-entries/" + cost.ID + "/materials", token: exact["outsourcing_engagement"], status: 403})
	before := costMaterialHTTPFacts(t, pool)
	for _, test := range tests {
		request := httptest.NewRequest("GET", test.path, nil)
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
			var projection costoutsourcing.MaterialReferenceProjection
			if json.Unmarshal(response.Body.Bytes(), &projection) != nil || projection.ID != test.id || projection.ResourceType != test.kind || projection.ProjectID != httpProject || projection.Revision != 1 || len(projection.Items) != 1 || projection.Items[0].Version != 2 || projection.Items[0].EvidenceSourceRevision != 2 {
				t.Fatal("wrong source-owned projection")
			}
			for _, forbidden := range []string{"amount", "currency", "provider", "contract", "suggestion", "assessment", "object_key", "url", "credential"} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Fatalf("projection leaked %q", forbidden)
				}
			}
		}
	}
	wrongProvider, err := auth.NewIAMProvider(issuer, httpTenantB, keys)
	if err != nil {
		t.Fatal(err)
	}
	wrongServer, _ := New(reader, wrongProvider)
	request := httptest.NewRequest("GET", "/api/v1/references/cost-entries/"+cost.ID+"/materials", nil)
	request.Header.Set("Authorization", "Bearer "+exact["cost_entry"])
	response := httptest.NewRecorder()
	wrongServer.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatal("wrong tenant accepted real signed credential")
	}
	if costMaterialHTTPFacts(t, pool) != before {
		t.Fatal("source GET changed any of ten owned tables")
	}
	t.Log("actual IAM-issued single native actions and exact aggregate/project/tenant scopes; both aggregate scopes and cross-action denials; PG18 ten tables unchanged; no amount.read grant or upstream read")
}
