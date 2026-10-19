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

	"omsbizfoundry/identity-access/internal/capability"
	"omsbizfoundry/identity-access/internal/database"
	"omsbizfoundry/identity-access/internal/httpserver"
	"omsbizfoundry/identity-access/internal/identity"
	"omsbizfoundry/identity-access/internal/signing"
)

const testTenantID = "22222222-2222-4222-8222-222222222222"
const testProjectID = "55555555-5555-4555-8555-555555555555"
const otherProjectID = "66666666-6666-4666-8666-666666666666"

func TestFreshDatabaseIdentityLifecycleAndCredentialIssuance(t *testing.T) {
	pool := openIsolatedDatabase(t)
	if err := database.Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	if version, err := database.RequireCurrentSchema(context.Background(), pool); err != nil || version != "000005_explicit_capability_namespaces.sql" {
		t.Fatalf("schema version = %q, %v", version, err)
	}
	keys, err := signing.Open(t.TempDir()+"/signing-keys.json", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := capability.Load([]string{"../../../../../contracts/access/v1/examples/contract-ledger-capabilities.json", "../../../../../contracts/access/v1/examples/money-flow-ledger-capabilities.json", "../../../../../contracts/access/v1/examples/people-groups-capabilities.json", "../../../../../contracts/access/v1/examples/project-workspace-capabilities.json"})
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
	handler, err := httpserver.New(pool, service, keys, httpserver.Options{})
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

	inviteResponse := postJSON(t, admin, server.URL+"/api/v1/admin/invitations", adminCSRF, map[string]any{"username": "operator", "display_name": "业务操作员"}, http.StatusCreated)
	inviteToken := stringField(t, inviteResponse, "token")
	operatorObject := objectField(t, inviteResponse, "subject")
	operatorID := stringField(t, operatorObject, "id")

	operator := newClient(t)
	operatorCSRF := fetchCSRF(t, operator, server.URL)
	postJSON(t, operator, server.URL+"/api/v1/activate", operatorCSRF, map[string]any{"token": inviteToken, "password": "operator password 123"}, http.StatusOK)
	postJSON(t, operator, server.URL+"/api/v1/activate", operatorCSRF, map[string]any{"token": inviteToken, "password": "operator password 123"}, http.StatusUnauthorized)
	login(t, operator, server.URL, operatorCSRF, "operator", "operator password 123", http.StatusOK)

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
