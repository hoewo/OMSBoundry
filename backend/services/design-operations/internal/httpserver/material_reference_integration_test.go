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

	"omsbizfoundry/design-operations/internal/auth"
	"omsbizfoundry/design-operations/internal/database"
	"omsbizfoundry/design-operations/internal/design"
	"omsbizfoundry/design-operations/internal/testdatabase"
)

type materialSeedReferences struct{ design.References }

func (*materialSeedReferences) ValidateProject(_ context.Context, tenant, id string, revision int64) (design.ProjectReference, error) {
	return design.ProjectReference{ID: id, TenantID: tenant, Status: "active", Revision: revision, TypeKeys: []string{"consultation"}}, nil
}
func (*materialSeedReferences) ValidateAssignment(_ context.Context, tenant, project, id string, revision int64) (design.AssignmentReference, error) {
	return design.AssignmentReference{ID: id, TenantID: tenant, ProjectID: project, RoleKey: "production_owner", Status: "active", DisplayName: "synthetic owner", Revision: revision}, nil
}
func (*materialSeedReferences) ValidateEvidence(_ context.Context, tenant, id string, version int, revision int64) (design.EvidenceReference, error) {
	return design.EvidenceReference{FileID: id, TenantID: tenant, Status: "available", DisplayName: "synthetic-report.pdf", Version: version, Revision: revision}, nil
}

// All read-time upstream ports are unimplemented and panic on use. Bootstrap
// references above are synthetic; the read, database and IAM verifier are real.
type materialReadNoUpstream struct{ design.References }

type materialIAMClient struct {
	client *http.Client
	base   string
	csrf   string
}

func materialIAMRequest(t *testing.T, client materialIAMClient, method, path string, input any, expected int, output any) {
	t.Helper()
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, client.base+path, body)
	if err != nil {
		t.Fatal("invalid synthetic IAM endpoint")
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CSRF-Token", client.csrf)
	}
	response, err := client.client.Do(request)
	if err != nil {
		t.Fatal("synthetic IAM request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != expected {
		// Credentials and passwords must never appear in test diagnostics.
		t.Fatalf("synthetic IAM %s %s status=%d expected=%d", method, path, response.StatusCode, expected)
	}
	if output != nil && json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(output) != nil {
		t.Fatal("synthetic IAM returned invalid JSON")
	}
}

func materialIAMLogin(t *testing.T, base, username, password string) materialIAMClient {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := materialIAMClient{base: base, client: &http.Client{Jar: jar, Timeout: 15 * time.Second}}
	var csrf struct {
		Token string `json:"csrf_token"`
	}
	materialIAMRequest(t, client, "GET", "/api/v1/csrf", nil, 200, &csrf)
	client.csrf = csrf.Token
	materialIAMRequest(t, client, "POST", "/api/v1/login", map[string]string{"username": username, "password": password}, 200, nil)
	return client
}

func materialIAMActorCredential(t *testing.T, admin materialIAMClient, label, password, action string, scope map[string]any) string {
	t.Helper()
	var invitation struct {
		Token   string `json:"token"`
		Subject struct {
			ID string `json:"id"`
		} `json:"subject"`
	}
	materialIAMRequest(t, admin, "POST", "/api/v1/admin/invitations", map[string]string{"username": "material-" + label, "display_name": "material-" + label}, 201, &invitation)
	jar, _ := cookiejar.New(nil)
	activation := materialIAMClient{base: admin.base, client: &http.Client{Jar: jar, Timeout: 15 * time.Second}}
	var csrf struct {
		Token string `json:"csrf_token"`
	}
	materialIAMRequest(t, activation, "GET", "/api/v1/csrf", nil, 200, &csrf)
	activation.csrf = csrf.Token
	materialIAMRequest(t, activation, "POST", "/api/v1/activate", map[string]string{"token": invitation.Token, "password": password}, 200, nil)
	materialIAMRequest(t, admin, "POST", "/api/v1/admin/grants", map[string]any{"subject_id": invitation.Subject.ID, "audience": auth.ProductionAudience, "action": action, "scope": scope}, 204, nil)
	client := materialIAMLogin(t, admin.base, "material-"+label, password)
	var credential struct {
		AccessToken string `json:"token"`
	}
	materialIAMRequest(t, client, "POST", "/api/v1/credentials", map[string]any{"audience": auth.ProductionAudience, "scope": scope}, 200, &credential)
	parts := strings.Split(credential.AccessToken, ".")
	if len(parts) != 3 {
		t.Fatal("IAM did not sign a compact credential")
	}
	claims, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal("IAM claims invalid")
	}
	var value struct {
		Actions  []string       `json:"actions"`
		Tenant   string         `json:"tid"`
		Audience string         `json:"aud"`
		Scope    map[string]any `json:"scope"`
	}
	if json.Unmarshal(claims, &value) != nil || value.Tenant != systemTenant || value.Audience != auth.ProductionAudience || len(value.Actions) != 1 || value.Actions[0] != action {
		t.Fatal("issued native action set or identity was expanded")
	}
	actualScope, _ := json.Marshal(value.Scope)
	expectedScope, _ := json.Marshal(scope)
	if string(actualScope) != string(expectedScope) {
		t.Fatal("issued scope differs from the requested resource")
	}
	return credential.AccessToken
}

func productionMaterialFacts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	rows := make([][2]string, 0, 6)
	for _, table := range []string{"design_commands", "design_deliverables", "design_history_events", "design_stages", "design_work_items", "schema_migrations"} {
		var value string
		query := `SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM ` + pgx.Identifier{"public", table}.Sanitize() + ` t`
		if err := pool.QueryRow(context.Background(), query).Scan(&value); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, [2]string{table, value})
	}
	raw, _ := json.Marshal(rows)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func TestProductionMaterialHTTPPostgresNativeIAM(t *testing.T) {
	base, issuer, keyFile, password := os.Getenv("PRODUCTION_MATERIAL_IAM_URL"), os.Getenv("PRODUCTION_MATERIAL_IAM_ISSUER"), os.Getenv("PRODUCTION_MATERIAL_IAM_KEYS"), os.Getenv("PRODUCTION_MATERIAL_IAM_PASSWORD")
	if base == "" || issuer == "" || keyFile == "" || password == "" {
		t.Skip("isolated real IAM fixture is not configured")
	}
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := design.NewPostgresStore(environment.Pool)
	writer, _ := design.NewService(store, &materialSeedReferences{})
	actor := design.Actor{Kind: "test_subject", ID: systemSubject, TenantID: systemTenant}
	created, err := writer.Create(ctx, actor, design.CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-000000000301", ProjectID: systemProject, ProjectSourceRevision: 4, TypeKey: design.ConsultationTypeKey})
	if err != nil {
		t.Fatal(err)
	}
	id := created.WorkItem.ID
	if _, err := writer.SetOwner(ctx, actor, id, design.OwnerInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-000000000302", ExpectedRevision: 1, AssignmentID: systemAssignment, AssignmentSourceRevision: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.PutDeliverable(ctx, actor, id, "scheme", design.DeliverableInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-000000000303", ExpectedRevision: 2, RequirementKey: design.RequirementPrimary, EvidenceFileID: systemFile, EvidenceVersion: 2, EvidenceSourceRevision: 7}); err != nil {
		t.Fatal(err)
	}
	reader, _ := design.NewService(store, &materialReadNoUpstream{})
	legacyIAM, err := auth.NewIAMProvider(issuer, systemTenant, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	nativeIAM, err := auth.NewProductionIAMProvider(issuer, systemTenant, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewWithProduction(reader, legacyIAM, nativeIAM)
	if err != nil {
		t.Fatal(err)
	}
	admin := materialIAMLogin(t, base, "fixture-admin", password)
	itemScope := func(item string) map[string]any {
		return map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "production-work-item", "id": item}}}
	}
	tokens := []struct {
		label, token string
		status       int
	}{
		{"anonymous", "", 401},
		{"exact-item", materialIAMActorCredential(t, admin, "exact", password, auth.ProductionActionReferenceRead, itemScope(id)), 200},
		{"neighbor-item", materialIAMActorCredential(t, admin, "neighbor", password, auth.ProductionActionReferenceRead, itemScope(systemAssignment)), 403},
		{"missing-action", materialIAMActorCredential(t, admin, "no-reference", password, auth.ProductionActionRead, itemScope(id)), 403},
		{"tenant", materialIAMActorCredential(t, admin, "tenant", password, auth.ProductionActionReferenceRead, map[string]any{"kind": "tenant", "resources": []map[string]string{}}), 200},
	}
	before := productionMaterialFacts(t, environment.Pool)
	for _, test := range tokens {
		request := httptest.NewRequest("GET", "/api/v1/references/production-work-items/"+id+"/materials", nil)
		if test.token != "" {
			request.Header.Set("Authorization", "Bearer "+test.token)
		}
		request.Header.Set("X-Actor-ID", "forged")
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != test.status || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("native IAM case=%s status=%d expected=%d", test.label, response.Code, test.status)
		}
		if test.status == 200 {
			var projection design.MaterialReferenceProjection
			if json.Unmarshal(response.Body.Bytes(), &projection) != nil || projection.ID != id || projection.ProjectID != systemProject || projection.Revision != 3 || len(projection.Items) != 1 || projection.Items[0].RelationKey != "scheme.primary-output" || projection.Items[0].Version != 2 || projection.Items[0].EvidenceSourceRevision != 7 {
				t.Fatal("native IAM read returned the wrong source projection")
			}
		}
	}
	wrongTenantIAM, _ := auth.NewProductionIAMProvider(issuer, systemAssignment, keyFile)
	wrongTenantServer, _ := NewWithProduction(reader, legacyIAM, wrongTenantIAM)
	request := httptest.NewRequest("GET", "/api/v1/references/production-work-items/"+id+"/materials", nil)
	request.Header.Set("Authorization", "Bearer "+tokens[1].token)
	response := httptest.NewRecorder()
	wrongTenantServer.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatal("wrong tenant accepted a real signed token")
	}
	if productionMaterialFacts(t, environment.Pool) != before {
		t.Fatal("native IAM material reads changed any Production-owned table")
	}
	t.Log("actual IAM-issued exact native actions and scopes, PostgreSQL 18, all six owned table digests unchanged, no upstream reads")
}
