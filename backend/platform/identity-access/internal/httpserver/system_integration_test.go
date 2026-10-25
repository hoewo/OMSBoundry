//go:build integration

package httpserver_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/identity-access/internal/apikey"
	"omsbizfoundry/identity-access/internal/capability"
	"omsbizfoundry/identity-access/internal/database"
	"omsbizfoundry/identity-access/internal/httpserver"
	"omsbizfoundry/identity-access/internal/identity"
	"omsbizfoundry/identity-access/internal/signing"
)

const testTenantID = "22222222-2222-4222-8222-222222222222"
const otherTenantID = "33333333-3333-4333-8333-333333333333"
const testProjectID = "55555555-5555-4555-8555-555555555555"
const otherProjectID = "66666666-6666-4666-8666-666666666666"

type echoPersonResolver struct{}

func (echoPersonResolver) ResolveBySubject(_ context.Context, tenantID, _ string) (apikey.PersonReference, error) {
	return apikey.PersonReference{ID: "77777777-7777-4777-8777-777777777777", TenantID: tenantID, Status: "active", Revision: 1, ReferenceCode: "EMP-001", DisplayName: "测试员工", Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, nil
}

func TestFreshDatabaseIdentityLifecycleAndCredentialIssuance(t *testing.T) {
	pool := openIsolatedDatabase(t)
	if err := database.Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	if version, err := database.RequireCurrentSchema(context.Background(), pool); err != nil || version != "000007_open_api_credentials.sql" {
		t.Fatalf("schema version = %q, %v", version, err)
	}
	keys, err := signing.Open(t.TempDir()+"/signing-keys.json", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := capability.Load([]string{"../../../../../contracts/access/v1/examples/identity-access-capabilities.json", "../../../../../contracts/access/v1/examples/open-api-capabilities.json", "../../../../../contracts/access/v1/examples/contract-ledger-capabilities.json", "../../../../../contracts/access/v1/examples/money-flow-ledger-capabilities.json", "../../../../../contracts/access/v1/examples/people-groups-capabilities.json", "../../../../../contracts/access/v1/examples/project-workspace-capabilities.json"})
	if err != nil {
		t.Fatal(err)
	}
	service, err := identity.NewService(identity.NewPostgresRepository(pool), keys, catalog, testTenantID, "测试企业", "https://oms.example.test/auth", 12*time.Hour, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.EnsureBootstrap(context.Background(), identity.BootstrapInput{Username: "admin", DisplayName: "系统管理员", Password: "correct horse battery staple"})
	if err != nil || !created {
		t.Fatalf("bootstrap = %v, %v", created, err)
	}
	created, err = service.EnsureBootstrap(context.Background(), identity.BootstrapInput{})
	if err != nil || created {
		t.Fatalf("repeat bootstrap = %v, %v", created, err)
	}
	provisioner, err := identity.NewTenantProvisioner(identity.NewPostgresRepository(pool))
	if err != nil {
		t.Fatal(err)
	}
	otherAdministrator, err := provisioner.Provision(context.Background(), identity.TenantProvisionInput{
		TenantID: otherTenantID, TenantName: "第二测试企业", AdminUsername: "admin-b", AdminDisplayName: "第二企业管理员",
		AdminPassword: "second correct horse battery staple", OperatorID: "integration-test", OperatorDisplayName: "集成测试平台操作员",
	})
	if err != nil {
		t.Fatal(err)
	}
	apiKeys, err := apikey.New(pool, keys, echoPersonResolver{}, "https://oms.example.test/auth")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := httpserver.New(pool, service, keys, httpserver.Options{APIKeys: apiKeys})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	admin := newClient(t)
	adminCSRF := fetchCSRF(t, admin, server.URL)
	login(t, admin, server.URL, adminCSRF, "admin", "correct horse battery staple", http.StatusOK)
	adminPrincipal := currentSession(t, admin, server.URL, http.StatusOK)
	adminID := stringField(t, adminPrincipal, "subject_id")
	if stringField(t, adminPrincipal, "tenant_id") != testTenantID {
		t.Fatalf("bootstrap administrator tenant = %#v", adminPrincipal)
	}

	otherAdmin := newClient(t)
	otherAdminCSRF := fetchCSRF(t, otherAdmin, server.URL)
	login(t, otherAdmin, server.URL, otherAdminCSRF, "admin-b", "second correct horse battery staple", http.StatusOK)
	otherAdminPrincipal := currentSession(t, otherAdmin, server.URL, http.StatusOK)
	if stringField(t, otherAdminPrincipal, "tenant_id") != otherTenantID || stringField(t, otherAdminPrincipal, "subject_id") != otherAdministrator.ID {
		t.Fatalf("second administrator principal = %#v", otherAdminPrincipal)
	}

	inviteResponse := postJSON(t, admin, server.URL+"/api/v1/admin/invitations", adminCSRF, map[string]any{"username": "operator", "display_name": "业务操作员"}, http.StatusCreated)
	inviteToken := stringField(t, inviteResponse, "token")
	operatorObject := objectField(t, inviteResponse, "subject")
	operatorID := stringField(t, operatorObject, "id")

	otherInvite := postJSON(t, otherAdmin, server.URL+"/api/v1/admin/invitations", otherAdminCSRF, map[string]any{"username": "operator-b", "display_name": "第二企业操作员"}, http.StatusCreated)
	otherOperatorID := stringField(t, objectField(t, otherInvite, "subject"), "id")
	otherOperator := newClient(t)
	otherOperatorCSRF := fetchCSRF(t, otherOperator, server.URL)
	postJSON(t, otherOperator, server.URL+"/api/v1/activate", otherOperatorCSRF, map[string]any{"token": stringField(t, otherInvite, "token"), "password": "second operator password 123"}, http.StatusOK)
	login(t, otherOperator, server.URL, otherOperatorCSRF, "operator-b", "second operator password 123", http.StatusOK)
	postJSON(t, otherAdmin, server.URL+"/api/v1/admin/grants", otherAdminCSRF, map[string]any{"subject_id": otherOperatorID, "audience": "contract-ledger", "action": "contract-ledger.view"}, http.StatusNoContent)
	otherCredential := postJSON(t, otherOperator, server.URL+"/api/v1/credentials", otherOperatorCSRF, map[string]any{"audience": "contract-ledger"}, http.StatusOK)
	otherClaims := verifyCredential(t, stringField(t, otherCredential, "token"), keys)
	if otherClaims.Subject != otherOperatorID || otherClaims.Tenant != otherTenantID {
		t.Fatalf("second tenant credential claims: %#v", otherClaims)
	}

	subjectCredential, err := service.IssueWorkloadCredential(context.Background(), otherTenantID, identity.ServiceIdentity{
		ID: "people-groups", SubjectID: otherOperatorID, DisplayName: "People & Groups",
		KeyID: testProjectID, AssertionID: otherProjectID, AuthTime: time.Now().UTC(),
	}, "identity-access", "identity-access.subject.reference.read", identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "iam_subject", ID: otherOperatorID}}})
	if err != nil {
		t.Fatal(err)
	}
	subjectReference := getBearerJSON(t, server.URL+"/api/v1/references/subjects/"+otherOperatorID, subjectCredential.Token, http.StatusOK)
	if stringField(t, subjectReference, "tenant_id") != otherTenantID || stringField(t, subjectReference, "id") != otherOperatorID || subjectReference["status"] != "active" || subjectReference["revision"] != float64(2) {
		t.Fatalf("subject reference = %#v", subjectReference)
	}
	wrongTenantCredential, err := service.IssueWorkloadCredential(context.Background(), testTenantID, identity.ServiceIdentity{
		ID: "people-groups", SubjectID: operatorID, DisplayName: "People & Groups",
		KeyID: testProjectID, AssertionID: otherProjectID, AuthTime: time.Now().UTC(),
	}, "identity-access", "identity-access.subject.reference.read", identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "iam_subject", ID: otherOperatorID}}})
	if err != nil {
		t.Fatal(err)
	}
	getBearerJSON(t, server.URL+"/api/v1/references/subjects/"+otherOperatorID, wrongTenantCredential.Token, http.StatusNotFound)

	// A tenant administrator cannot operate on a stable Subject ID from B.
	postJSON(t, admin, server.URL+"/api/v1/admin/password-resets", adminCSRF, map[string]any{"subject_id": otherOperatorID}, http.StatusNotFound)
	postJSON(t, admin, server.URL+"/api/v1/admin/grants", adminCSRF, map[string]any{"subject_id": otherOperatorID, "audience": "contract-ledger", "action": "contract-ledger.view"}, http.StatusNotFound)
	postJSON(t, admin, server.URL+"/api/v1/admin/status", adminCSRF, map[string]any{"subject_id": otherOperatorID, "status": "disabled"}, http.StatusNotFound)
	assertSubjectList(t, getJSON(t, admin, server.URL+"/api/v1/admin/subjects", http.StatusOK), testTenantID, 2)
	assertSubjectList(t, getJSON(t, otherAdmin, server.URL+"/api/v1/admin/subjects", http.StatusOK), otherTenantID, 2)

	operator := newClient(t)
	operatorCSRF := fetchCSRF(t, operator, server.URL)
	postJSON(t, operator, server.URL+"/api/v1/activate", operatorCSRF, map[string]any{"token": inviteToken, "password": "operator password 123"}, http.StatusOK)
	postJSON(t, operator, server.URL+"/api/v1/activate", operatorCSRF, map[string]any{"token": inviteToken, "password": "operator password 123"}, http.StatusUnauthorized)
	login(t, operator, server.URL, operatorCSRF, "operator", "operator password 123", http.StatusOK)
	getJSON(t, operator, server.URL+"/api/v1/api-keys", http.StatusForbidden)
	for _, action := range []string{apikey.ActionUse, apikey.ActionKeyManage, apikey.ActionSelfAudit} {
		postJSON(t, admin, server.URL+"/api/v1/admin/grants", adminCSRF, map[string]any{"subject_id": operatorID, "audience": "open-api", "action": action}, http.StatusNoContent)
	}
	createdKey := postJSON(t, operator, server.URL+"/api/v1/api-keys", operatorCSRF, map[string]any{"label": "员工报销 CLI", "ttl_days": 180}, http.StatusCreated)
	keyID := stringField(t, createdKey, "id")
	if secret := stringField(t, createdKey, "secret"); !strings.HasPrefix(secret, "bfk_") {
		t.Fatalf("created API Key secret = %q", secret)
	}
	keyList := getJSON(t, operator, server.URL+"/api/v1/api-keys", http.StatusOK)
	items, ok := keyList["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("API Key list = %#v", keyList)
	}
	listed, ok := items[0].(map[string]any)
	if !ok || listed["id"] != keyID || listed["secret"] != nil || listed["tenant_id"] != nil || listed["subject_id"] != nil {
		t.Fatalf("listed API Key leaked private fields: %#v", listed)
	}
	getJSON(t, operator, server.URL+"/api/v1/open-api-usage?scope=tenant", http.StatusForbidden)
	postJSON(t, operator, server.URL+"/api/v1/api-keys/"+keyID+"/revoke", operatorCSRF, map[string]any{}, http.StatusOK)

	postJSON(t, admin, server.URL+"/api/v1/admin/grants", adminCSRF, map[string]any{"subject_id": operatorID, "audience": "contract-ledger", "action": "contract-ledger.view"}, http.StatusNoContent)
	credentialResponse := postJSON(t, operator, server.URL+"/api/v1/credentials", operatorCSRF, map[string]any{"audience": "contract-ledger"}, http.StatusOK)
	token := stringField(t, credentialResponse, "token")
	claims := verifyCredential(t, token, keys)
	if claims.Subject != operatorID || claims.Tenant != testTenantID || claims.Audience != "contract-ledger" || len(claims.Actions) != 1 || claims.Actions[0] != "contract-ledger.view" || claims.ExpiresAt-claims.IssuedAt != 300 {
		t.Fatalf("unexpected credential claims: %#v", claims)
	}
	postJSON(t, operator, server.URL+"/api/v1/credentials", operatorCSRF, map[string]any{"audience": "money-flow-ledger"}, http.StatusForbidden)

	projectScope := map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "project", "id": testProjectID}}}
	postJSON(t, admin, server.URL+"/api/v1/admin/grants", adminCSRF, map[string]any{"subject_id": operatorID, "audience": "project-workspace", "action": "project-workspace.read", "scope": projectScope}, http.StatusNoContent)
	postJSON(t, admin, server.URL+"/api/v1/admin/grants", adminCSRF, map[string]any{"subject_id": operatorID, "audience": "project-workspace", "action": "project-workspace.update", "scope": projectScope}, http.StatusNoContent)
	postJSON(t, admin, server.URL+"/api/v1/admin/grants", adminCSRF, map[string]any{"subject_id": operatorID, "audience": "project-workspace", "action": "project-workspace.read", "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "contract", "id": testProjectID}}}}, http.StatusUnprocessableEntity)
	postJSON(t, operator, server.URL+"/api/v1/credentials", operatorCSRF, map[string]any{"audience": "project-workspace"}, http.StatusForbidden)
	projectCredential := postJSON(t, operator, server.URL+"/api/v1/credentials", operatorCSRF, map[string]any{"audience": "project-workspace", "scope": projectScope}, http.StatusOK)
	projectClaims := verifyCredential(t, stringField(t, projectCredential, "token"), keys)
	if projectClaims.Audience != "project-workspace" || len(projectClaims.Actions) != 2 || projectClaims.Actions[0] != "project-workspace.read" || projectClaims.Actions[1] != "project-workspace.update" || projectClaims.Scope.Kind != "resources" || len(projectClaims.Scope.Resources) != 1 || projectClaims.Scope.Resources[0].Type != "project" || projectClaims.Scope.Resources[0].ID != testProjectID {
		t.Fatalf("unexpected project credential claims: %#v", projectClaims)
	}
	postJSON(t, admin, server.URL+"/api/v1/admin/grants/revoke", adminCSRF, map[string]any{"subject_id": operatorID, "audience": "project-workspace", "action": "project-workspace.read", "scope": projectScope}, http.StatusNoContent)
	afterRevoke := postJSON(t, operator, server.URL+"/api/v1/credentials", operatorCSRF, map[string]any{"audience": "project-workspace", "scope": projectScope}, http.StatusOK)
	afterRevokeClaims := verifyCredential(t, stringField(t, afterRevoke, "token"), keys)
	if len(afterRevokeClaims.Actions) != 1 || afterRevokeClaims.Actions[0] != "project-workspace.update" {
		t.Fatalf("per-grant revocation changed unrelated grants: %#v", afterRevokeClaims)
	}
	postJSON(t, operator, server.URL+"/api/v1/credentials", operatorCSRF, map[string]any{"audience": "project-workspace", "scope": map[string]any{"kind": "resources", "resources": []map[string]string{{"type": "project", "id": otherProjectID}}}}, http.StatusForbidden)

	resetResponse := postJSON(t, admin, server.URL+"/api/v1/admin/password-resets", adminCSRF, map[string]any{"subject_id": operatorID}, http.StatusCreated)
	resetToken := stringField(t, resetResponse, "token")
	postJSON(t, newClient(t), server.URL+"/api/v1/password-reset/complete", "", map[string]any{"token": resetToken, "password": "replacement password 123"}, http.StatusForbidden)
	resetClient := newClient(t)
	resetCSRF := fetchCSRF(t, resetClient, server.URL)
	postJSON(t, resetClient, server.URL+"/api/v1/password-reset/complete", resetCSRF, map[string]any{"token": resetToken, "password": "replacement password 123"}, http.StatusOK)
	currentSession(t, operator, server.URL, http.StatusUnauthorized)
	login(t, operator, server.URL, operatorCSRF, "operator", "operator password 123", http.StatusUnauthorized)
	login(t, operator, server.URL, operatorCSRF, "operator", "replacement password 123", http.StatusOK)

	postJSON(t, admin, server.URL+"/api/v1/admin/status", adminCSRF, map[string]any{"subject_id": adminID, "status": "disabled"}, http.StatusConflict)
	postJSON(t, admin, server.URL+"/api/v1/admin/status", adminCSRF, map[string]any{"subject_id": operatorID, "status": "disabled"}, http.StatusNoContent)
	currentSession(t, operator, server.URL, http.StatusUnauthorized)

	var auditCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM iam_audit_entries`).Scan(&auditCount); err != nil || auditCount < 7 {
		t.Fatalf("audit count = %d, %v", auditCount, err)
	}
	_, err = pool.Exec(context.Background(), `UPDATE iam_audit_entries SET action='tampered' WHERE id=(SELECT id FROM iam_audit_entries LIMIT 1)`)
	var databaseError *pgconn.PgError
	if !errorsAs(err, &databaseError) || databaseError.Code != "55000" {
		t.Fatalf("immutable audit update error = %v", err)
	}
}

func openIsolatedDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	config, err := pgxpool.ParseConfig(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	maintenanceConfig := config.ConnConfig.Copy()
	maintenanceConfig.Database = "postgres"
	maintenance, err := pgx.ConnectConfig(context.Background(), maintenanceConfig)
	if err != nil {
		t.Fatal(err)
	}
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	databaseName := "iam_test_" + hex.EncodeToString(suffix[:])
	identifier := pgx.Identifier{databaseName}.Sanitize()
	if _, err := maintenance.Exec(context.Background(), "CREATE DATABASE "+identifier); err != nil {
		_ = maintenance.Close(context.Background())
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = maintenance.Exec(context.Background(), "DROP DATABASE "+identifier+" WITH (FORCE)")
		_ = maintenance.Close(context.Background())
	})
	targetConfig := config.Copy()
	targetConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(context.Background(), targetConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func fetchCSRF(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	response, err := client.Get(baseURL + "/api/v1/csrf")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var value map[string]any
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	return stringField(t, value, "csrf_token")
}

func login(t *testing.T, client *http.Client, baseURL, csrf, username, password string, status int) {
	t.Helper()
	postJSON(t, client, baseURL+"/api/v1/login", csrf, map[string]any{"username": username, "password": password}, status)
}

func currentSession(t *testing.T, client *http.Client, baseURL string, status int) map[string]any {
	t.Helper()
	response, err := client.Get(baseURL + "/api/v1/session")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != status {
		t.Fatalf("GET session status = %d, want %d: %s", response.StatusCode, status, readBody(response.Body))
	}
	if status != http.StatusOK {
		return nil
	}
	var value map[string]any
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	return objectField(t, value, "principal")
}

func getJSON(t *testing.T, client *http.Client, endpoint string, status int) map[string]any {
	t.Helper()
	response, err := client.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status {
		t.Fatalf("GET %s status = %d, want %d: %s", endpoint, response.StatusCode, status, data)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func getBearerJSON(t *testing.T, endpoint, token string, status int) map[string]any {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status {
		t.Fatalf("GET %s status = %d, want %d: %s", endpoint, response.StatusCode, status, data)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode response: %v: %s", err, data)
	}
	return value
}

func assertSubjectList(t *testing.T, response map[string]any, tenantID string, count int) {
	t.Helper()
	values, ok := response["subjects"].([]any)
	if !ok || len(values) != count {
		t.Fatalf("subjects = %#v, want %d", response["subjects"], count)
	}
	for _, raw := range values {
		value, ok := raw.(map[string]any)
		if !ok || value["tenant_id"] != tenantID {
			t.Fatalf("cross-tenant subject in response: %#v", raw)
		}
	}
}

func postJSON(t *testing.T, client *http.Client, endpoint, csrf string, body any, status int) map[string]any {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status {
		t.Fatalf("POST %s status = %d, want %d: %s", endpoint, response.StatusCode, status, responseData)
	}
	if len(bytes.TrimSpace(responseData)) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(responseData, &value); err != nil {
		t.Fatalf("decode response: %v: %s", err, responseData)
	}
	return value
}

func verifyCredential(t *testing.T, token string, store *signing.Store) identity.CapabilityClaims {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatal("credential is not a compact JWS")
	}
	headerData, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var header map[string]string
	if err := json.Unmarshal(headerData, &header); err != nil {
		t.Fatal(err)
	}
	keys, err := store.PublicKeys(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	var publicKey ed25519.PublicKey
	for _, key := range keys {
		if key.KeyID == header["kid"] {
			publicKey = key.PublicKey
		}
	}
	signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
	if header["alg"] != "EdDSA" || len(publicKey) == 0 || !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		t.Fatal("credential signature is invalid")
	}
	claimsData, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims identity.CapabilityClaims
	if err := json.Unmarshal(claimsData, &claims); err != nil {
		t.Fatal(err)
	}
	return claims
}

func stringField(t *testing.T, value map[string]any, name string) string {
	t.Helper()
	result, ok := value[name].(string)
	if !ok || result == "" {
		t.Fatalf("field %s missing in %#v", name, value)
	}
	return result
}

func objectField(t *testing.T, value map[string]any, name string) map[string]any {
	t.Helper()
	result, ok := value[name].(map[string]any)
	if !ok {
		t.Fatalf("field %s is not object in %#v", name, value)
	}
	return result
}

func readBody(reader io.Reader) string    { data, _ := io.ReadAll(reader); return string(data) }
func errorsAs(err error, target any) bool { return err != nil && errors.As(err, target) }
