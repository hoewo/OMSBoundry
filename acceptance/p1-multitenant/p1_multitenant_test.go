//go:build integration

package p1multitenant_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	tenantA          = "11111111-1111-4111-8111-111111111111"
	tenantB          = "22222222-2222-4222-8222-222222222222"
	peopleWorkloadID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	issuer           = "https://p1-acceptance.invalid/auth"
	adminAPassword   = "P1 tenant A administrator password"
	adminBPassword   = "P1 tenant B administrator password"
	operatorPassword = "P1 tenant B operator password"
)

type process struct {
	command *exec.Cmd
	exited  chan struct{}
	mu      sync.Mutex
	exitErr error
	logPath string
}

type apiClient struct {
	http *http.Client
	csrf string
}

func TestRealProcessesKeepTenantsIsolatedAcrossRestore(t *testing.T) {
	requireCommand(t, "docker")
	repo := repositoryRoot(t)
	temporary := t.TempDir()
	databasePort := freePort(t)
	iamPort := freePort(t)
	peoplePort := freePort(t)
	container := "oms-p1-multitenant-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	run(t, repo, nil, "docker", "run", "--detach", "--name", container, "-e", "POSTGRES_PASSWORD=postgres", "-p", fmt.Sprintf("127.0.0.1:%d:5432", databasePort), "postgres:18.6")
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "--force", container).Run() })
	waitCommand(t, repo, 45*time.Second, "docker", "exec", container, "pg_isready", "-U", "postgres")
	for _, database := range []string{"identity_access", "people_groups", "identity_access_restore", "people_groups_restore"} {
		run(t, repo, nil, "docker", "exec", container, "createdb", "-U", "postgres", database)
	}

	binaries := buildBinaries(t, repo, temporary)
	privateWorkloadKey := filepath.Join(temporary, "people-workload-private.json")
	publicWorkloadKey := filepath.Join(temporary, "people-workload-public.json")
	run(t, repo, map[string]string{
		"WORKLOAD_ID":              "people-groups",
		"WORKLOAD_SUBJECT_ID":      peopleWorkloadID,
		"WORKLOAD_KEY_FILE":        privateWorkloadKey,
		"WORKLOAD_PUBLIC_KEY_FILE": publicWorkloadKey,
	}, binaries["people-keyctl"], "init")

	policyPath := filepath.Join(temporary, "workload-policy.json")
	writeJSONFile(t, policyPath, map[string]any{
		"schema_version": "1",
		"identities": []any{map[string]any{
			"id": "people-groups", "subject_id": peopleWorkloadID, "display_name": "People & Groups",
			"public_keys_path": publicWorkloadKey,
			"permissions": []any{map[string]any{
				"audience": "identity-access", "action": "identity-access.subject.reference.read", "resource_types": []string{"iam_subject"},
			}},
		}},
	})
	signingKey := filepath.Join(temporary, "iam-signing-keys.json")
	publicSigningKey := filepath.Join(temporary, "iam-public-keys.json")
	run(t, repo, map[string]string{"SIGNING_KEY_FILE": signingKey}, binaries["iam-keyctl"], "public", publicSigningKey)

	manifestPaths := strings.Join([]string{
		filepath.Join(repo, "contracts/access/v1/examples/identity-access-capabilities.json"),
		filepath.Join(repo, "contracts/access/v1/examples/people-groups-capabilities.json"),
	}, ",")
	primaryIAMDatabase := databaseURL(databasePort, "identity_access")
	primaryPeopleDatabase := databaseURL(databasePort, "people_groups")
	iamBaseURL := fmt.Sprintf("http://127.0.0.1:%d", iamPort)
	peopleBaseURL := fmt.Sprintf("http://127.0.0.1:%d", peoplePort)

	iam := startIAM(t, repo, binaries["iam"], temporary, iamPort, primaryIAMDatabase, signingKey, policyPath, manifestPaths)
	waitHTTP(t, iamBaseURL+"/health/ready", http.StatusOK, 45*time.Second, iam)
	provisionTenantB(t, repo, binaries["tenantctl"], temporary, primaryIAMDatabase)
	people := startPeople(t, repo, binaries["people"], temporary, peoplePort, primaryPeopleDatabase, iamBaseURL, privateWorkloadKey, publicSigningKey)
	waitHTTP(t, peopleBaseURL+"/health/ready", http.StatusOK, 45*time.Second, people)

	a := newAPIClient(t, iamBaseURL)
	a.login(t, iamBaseURL, "p1-admin-a", adminAPassword, http.StatusOK)
	aPrincipal := a.session(t, iamBaseURL, http.StatusOK)
	aSubject := stringField(t, aPrincipal, "subject_id")
	assertString(t, aPrincipal, "tenant_id", tenantA)

	b := newAPIClient(t, iamBaseURL)
	b.login(t, iamBaseURL, "p1-admin-b", adminBPassword, http.StatusOK)
	bPrincipal := b.session(t, iamBaseURL, http.StatusOK)
	bSubject := stringField(t, bPrincipal, "subject_id")
	assertString(t, bPrincipal, "tenant_id", tenantB)

	grantPeople(t, a, iamBaseURL, aSubject, true)
	grantPeople(t, b, iamBaseURL, bSubject, true)
	aPeopleToken := issuePeopleCredential(t, a, iamBaseURL, http.StatusOK)
	bPeopleToken := issuePeopleCredential(t, b, iamBaseURL, http.StatusOK)

	personA := bearerJSON(t, http.MethodPost, peopleBaseURL+"/api/v1/persons", aPeopleToken, map[string]any{
		"command_id": "00000000-0000-4000-8000-000000000001", "reference_code": "A-001", "display_name": "Tenant A Person", "iam_subject_id": aSubject,
	}, http.StatusCreated)
	personB := bearerJSON(t, http.MethodPost, peopleBaseURL+"/api/v1/persons", bPeopleToken, map[string]any{
		"command_id": "00000000-0000-4000-8000-000000000002", "reference_code": "B-001", "display_name": "Tenant B Person", "iam_subject_id": bSubject,
	}, http.StatusCreated)
	personAID := stringField(t, personA, "id")
	personBID := stringField(t, personB, "id")
	assertString(t, personA, "tenant_id", tenantA)
	assertString(t, personB, "tenant_id", tenantB)

	assertPersonList(t, bearerJSONArray(t, peopleBaseURL+"/api/v1/persons", aPeopleToken, http.StatusOK), tenantA, personAID)
	assertPersonList(t, bearerJSONArray(t, peopleBaseURL+"/api/v1/persons", bPeopleToken, http.StatusOK), tenantB, personBID)
	bearerJSON(t, http.MethodGet, peopleBaseURL+"/api/v1/persons/"+personAID, bPeopleToken, nil, http.StatusNotFound)
	bearerJSON(t, http.MethodGet, peopleBaseURL+"/api/v1/persons/"+personBID, aPeopleToken, nil, http.StatusNotFound)
	bearerJSON(t, http.MethodPost, peopleBaseURL+"/api/v1/persons", aPeopleToken, map[string]any{
		"command_id": "00000000-0000-4000-8000-000000000003", "display_name": "Cross Tenant Link", "iam_subject_id": bSubject,
	}, http.StatusUnprocessableEntity)

	a.post(t, iamBaseURL+"/api/v1/admin/grants/revoke", map[string]any{
		"subject_id": aSubject, "audience": "people-groups", "action": "people-groups.person.manage",
	}, http.StatusNoContent)
	readOnlyToken := issuePeopleCredential(t, a, iamBaseURL, http.StatusOK)
	bearerJSON(t, http.MethodPost, peopleBaseURL+"/api/v1/persons", readOnlyToken, map[string]any{
		"command_id": "00000000-0000-4000-8000-000000000004", "display_name": "Must Not Be Written",
	}, http.StatusForbidden)
	assertPersonList(t, bearerJSONArray(t, peopleBaseURL+"/api/v1/persons", readOnlyToken, http.StatusOK), tenantA, personAID)

	operator := inviteAndActivate(t, b, iamBaseURL)
	b.post(t, iamBaseURL+"/api/v1/admin/grants", map[string]any{
		"subject_id": operator.subjectID, "audience": "people-groups", "action": "people-groups.view",
	}, http.StatusNoContent)
	operator.client.login(t, iamBaseURL, "p1-operator-b", operatorPassword, http.StatusOK)
	_ = issuePeopleCredential(t, operator.client, iamBaseURL, http.StatusOK)
	b.post(t, iamBaseURL+"/api/v1/admin/status", map[string]any{"subject_id": operator.subjectID, "status": "disabled"}, http.StatusNoContent)
	operator.client.session(t, iamBaseURL, http.StatusUnauthorized)

	stopProcess(t, people)
	stopProcess(t, iam)
	run(t, repo, nil, "docker", "exec", container, "pg_dump", "-U", "postgres", "-Fc", "--no-owner", "--no-privileges", "--exclude-table-data=public.suite_sessions", "-f", "/tmp/identity-access.dump", "identity_access")
	run(t, repo, nil, "docker", "exec", container, "pg_dump", "-U", "postgres", "-Fc", "--no-owner", "--no-privileges", "-f", "/tmp/people-groups.dump", "people_groups")
	run(t, repo, nil, "docker", "exec", container, "pg_restore", "-U", "postgres", "--single-transaction", "--exit-on-error", "--no-owner", "--no-privileges", "-d", "identity_access_restore", "/tmp/identity-access.dump")
	run(t, repo, nil, "docker", "exec", container, "pg_restore", "-U", "postgres", "--single-transaction", "--exit-on-error", "--no-owner", "--no-privileges", "-d", "people_groups_restore", "/tmp/people-groups.dump")

	restoredIAMDatabase := databaseURL(databasePort, "identity_access_restore")
	restoredPeopleDatabase := databaseURL(databasePort, "people_groups_restore")
	iam = startIAM(t, repo, binaries["iam"], temporary, iamPort, restoredIAMDatabase, signingKey, policyPath, manifestPaths)
	waitHTTP(t, iamBaseURL+"/health/ready", http.StatusOK, 45*time.Second, iam)
	people = startPeople(t, repo, binaries["people"], temporary, peoplePort, restoredPeopleDatabase, iamBaseURL, privateWorkloadKey, publicSigningKey)
	waitHTTP(t, peopleBaseURL+"/health/ready", http.StatusOK, 45*time.Second, people)

	restoredA := newAPIClient(t, iamBaseURL)
	restoredA.login(t, iamBaseURL, "p1-admin-a", adminAPassword, http.StatusOK)
	restoredReadToken := issuePeopleCredential(t, restoredA, iamBaseURL, http.StatusOK)
	restoredPeopleA := bearerJSONArray(t, peopleBaseURL+"/api/v1/persons", restoredReadToken, http.StatusOK)
	assertPersonList(t, restoredPeopleA, tenantA, personAID)
	if numberField(t, restoredPeopleA[0], "revision") != 1 || stringField(t, restoredPeopleA[0], "iam_subject_id") != aSubject || stringField(t, restoredPeopleA[0], "iam_subject_status") != "active" {
		t.Fatalf("restored Tenant A Person lost revision or IAM snapshot: %#v", restoredPeopleA[0])
	}
	restoredB := newAPIClient(t, iamBaseURL)
	restoredB.login(t, iamBaseURL, "p1-admin-b", adminBPassword, http.StatusOK)
	restoredBToken := issuePeopleCredential(t, restoredB, iamBaseURL, http.StatusOK)
	assertPersonList(t, bearerJSONArray(t, peopleBaseURL+"/api/v1/persons", restoredBToken, http.StatusOK), tenantB, personBID)

	restoredAPrincipal := restoredA.session(t, iamBaseURL, http.StatusOK)
	restoredA.post(t, iamBaseURL+"/api/v1/admin/grants", map[string]any{
		"subject_id": stringField(t, restoredAPrincipal, "subject_id"), "audience": "people-groups", "action": "people-groups.person.manage",
	}, http.StatusNoContent)
	restoredWriteToken := issuePeopleCredential(t, restoredA, iamBaseURL, http.StatusOK)
	secondA := bearerJSON(t, http.MethodPost, peopleBaseURL+"/api/v1/persons", restoredWriteToken, map[string]any{
		"command_id": "00000000-0000-4000-8000-000000000005", "reference_code": "A-002", "display_name": "Tenant A After Restore",
	}, http.StatusCreated)
	if stringField(t, secondA, "tenant_id") != tenantA || len(bearerJSONArray(t, peopleBaseURL+"/api/v1/persons", restoredWriteToken, http.StatusOK)) != 2 {
		t.Fatalf("restored target did not accept an isolated continuation write: %#v", secondA)
	}
	if len(bearerJSONArray(t, peopleBaseURL+"/api/v1/persons", restoredBToken, http.StatusOK)) != 1 {
		t.Fatal("Tenant A continuation write changed Tenant B projection")
	}
}

func buildBinaries(t *testing.T, repo, temporary string) map[string]string {
	t.Helper()
	values := map[string]string{
		"iam": filepath.Join(temporary, "iam-server"), "iam-keyctl": filepath.Join(temporary, "iam-keyctl"),
		"tenantctl": filepath.Join(temporary, "tenantctl"), "people": filepath.Join(temporary, "people-server"),
		"people-keyctl": filepath.Join(temporary, "people-keyctl"),
	}
	packages := map[string]string{
		"iam": "./backend/platform/identity-access/cmd/server", "iam-keyctl": "./backend/platform/identity-access/cmd/keyctl",
		"tenantctl": "./backend/platform/identity-access/cmd/tenantctl", "people": "./backend/services/people-groups/cmd/server",
		"people-keyctl": "./backend/services/people-groups/cmd/workload-keyctl",
	}
	for name, destination := range values {
		run(t, repo, nil, "go", "build", "-o", destination, packages[name])
	}
	return values
}

func startIAM(t *testing.T, repo, binary, temporary string, port int, database, signingKey, policyPath, manifestPaths string) *process {
	t.Helper()
	return startProcess(t, repo, filepath.Join(temporary, "iam.log"), map[string]string{
		"HTTP_ADDR": fmt.Sprintf("127.0.0.1:%d", port), "DATABASE_URL": database, "MIGRATION_MODE": "auto",
		"TENANT_ID": tenantA, "TENANT_NAME": "P1 Tenant A", "PUBLIC_ISSUER": issuer, "SIGNING_KEY_FILE": signingKey,
		"BOOTSTRAP_ADMIN_USERNAME": "p1-admin-a", "BOOTSTRAP_ADMIN_DISPLAY_NAME": "P1 Tenant A Admin", "BOOTSTRAP_ADMIN_PASSWORD": adminAPassword,
		"CAPABILITY_MANIFEST_PATHS": manifestPaths, "WORKLOAD_IDENTITY_POLICY_PATH": policyPath, "SESSION_COOKIE_SECURE": "false",
	}, binary)
}

func startPeople(t *testing.T, repo, binary, temporary string, port int, database, iamBaseURL, privateKey, publicSigningKey string) *process {
	t.Helper()
	return startProcess(t, repo, filepath.Join(temporary, "people.log"), map[string]string{
		"HTTP_ADDR": fmt.Sprintf("127.0.0.1:%d", port), "DATABASE_URL": database, "MIGRATION_MODE": "auto", "IDENTITY_PROVIDER": "iam",
		"IAM_ISSUER": issuer, "IAM_TENANT_ID": tenantA, "IAM_PUBLIC_KEYS_PATH": publicSigningKey,
		"IAM_BASE_URL": iamBaseURL, "IAM_SERVICE_CREDENTIAL_URL": iamBaseURL + "/api/v1/service-credentials",
		"WORKLOAD_ID": "people-groups", "WORKLOAD_SUBJECT_ID": peopleWorkloadID, "WORKLOAD_KEY_FILE": privateKey,
		"SESSION_COOKIE_SECURE": "false", "REFERENCE_TIMEOUT": "3s", "WORKLOAD_REFRESH_BEFORE": "5s",
	}, binary)
}

func provisionTenantB(t *testing.T, repo, tenantctl, temporary, database string) {
	t.Helper()
	passwordFile := filepath.Join(temporary, "tenant-b-password")
	if err := os.WriteFile(passwordFile, []byte(adminBPassword+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run(t, repo, map[string]string{"DATABASE_URL": database}, tenantctl, "create",
		"--tenant-id", tenantB, "--tenant-name", "P1 Tenant B", "--admin-username", "p1-admin-b",
		"--admin-display-name", "P1 Tenant B Admin", "--admin-password-file", passwordFile,
		"--operator-id", "p1-acceptance", "--operator-name", "P1 Acceptance Operator")
}

func grantPeople(t *testing.T, client *apiClient, baseURL, subjectID string, manage bool) {
	t.Helper()
	actions := []string{"people-groups.view"}
	if manage {
		actions = append(actions, "people-groups.person.manage")
	}
	for _, action := range actions {
		client.post(t, baseURL+"/api/v1/admin/grants", map[string]any{"subject_id": subjectID, "audience": "people-groups", "action": action}, http.StatusNoContent)
	}
}

func issuePeopleCredential(t *testing.T, client *apiClient, baseURL string, expected int) string {
	t.Helper()
	value := client.post(t, baseURL+"/api/v1/credentials", map[string]any{"audience": "people-groups"}, expected)
	if expected != http.StatusOK {
		return ""
	}
	return stringField(t, value, "token")
}

type activatedOperator struct {
	client    *apiClient
	subjectID string
}

func inviteAndActivate(t *testing.T, administrator *apiClient, baseURL string) activatedOperator {
	t.Helper()
	invitation := administrator.post(t, baseURL+"/api/v1/admin/invitations", map[string]any{"username": "p1-operator-b", "display_name": "P1 Tenant B Operator"}, http.StatusCreated)
	operator := newAPIClient(t, baseURL)
	operator.post(t, baseURL+"/api/v1/activate", map[string]any{"token": stringField(t, invitation, "token"), "password": operatorPassword}, http.StatusOK)
	return activatedOperator{client: operator, subjectID: stringField(t, objectField(t, invitation, "subject"), "id")}
}

func newAPIClient(t *testing.T, baseURL string) *apiClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &apiClient{http: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
	value := client.request(t, http.MethodGet, baseURL+"/api/v1/csrf", nil, "", http.StatusOK)
	client.csrf = stringField(t, value, "csrf_token")
	return client
}

func (c *apiClient) login(t *testing.T, baseURL, username, password string, expected int) {
	t.Helper()
	c.post(t, baseURL+"/api/v1/login", map[string]any{"username": username, "password": password}, expected)
}

func (c *apiClient) session(t *testing.T, baseURL string, expected int) map[string]any {
	t.Helper()
	value := c.request(t, http.MethodGet, baseURL+"/api/v1/session", nil, "", expected)
	if expected != http.StatusOK {
		return value
	}
	return objectField(t, value, "principal")
}

func (c *apiClient) post(t *testing.T, url string, body any, expected int) map[string]any {
	t.Helper()
	return c.request(t, http.MethodPost, url, body, c.csrf, expected)
}

func (c *apiClient) request(t *testing.T, method, url string, body any, csrf string, expected int) map[string]any {
	t.Helper()
	data := []byte(nil)
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response, err := c.http.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != expected {
		t.Fatalf("%s %s returned %d, want %d: %s", method, url, response.StatusCode, expected, payload)
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("decode %s %s: %v: %s", method, url, err, payload)
	}
	return value
}

func bearerJSON(t *testing.T, method, url, token string, body any, expected int) map[string]any {
	t.Helper()
	payload := []byte(nil)
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != expected {
		t.Fatalf("%s %s returned %d, want %d: %s", method, url, response.StatusCode, expected, data)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode %s %s: %v: %s", method, url, err, data)
	}
	return value
}

func bearerJSONArray(t *testing.T, url, token string, expected int) []map[string]any {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != expected {
		t.Fatalf("GET %s returned %d, want %d: %s", url, response.StatusCode, expected, data)
	}
	var values []map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("decode GET %s: %v: %s", url, err, data)
	}
	return values
}

func assertPersonList(t *testing.T, values []map[string]any, tenantID, personID string) {
	t.Helper()
	if len(values) != 1 || stringField(t, values[0], "tenant_id") != tenantID || stringField(t, values[0], "id") != personID {
		t.Fatalf("unexpected Person list for %s: %#v", tenantID, values)
	}
}

func objectField(t *testing.T, value map[string]any, name string) map[string]any {
	t.Helper()
	object, ok := value[name].(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object in %#v", name, value)
	}
	return object
}

func stringField(t *testing.T, value map[string]any, name string) string {
	t.Helper()
	text, ok := value[name].(string)
	if !ok || text == "" {
		t.Fatalf("%s is not a non-empty string in %#v", name, value)
	}
	return text
}

func numberField(t *testing.T, value map[string]any, name string) float64 {
	t.Helper()
	number, ok := value[name].(float64)
	if !ok {
		t.Fatalf("%s is not a number in %#v", name, value)
	}
	return number
}

func assertString(t *testing.T, value map[string]any, name, expected string) {
	t.Helper()
	if actual := stringField(t, value, name); actual != expected {
		t.Fatalf("%s = %q, want %q", name, actual, expected)
	}
}

func startProcess(t *testing.T, directory, logPath string, environment map[string]string, name string, arguments ...string) *process {
	t.Helper()
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = mergedEnvironment(environment)
	command.Stdout, command.Stderr = logFile, logFile
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		t.Fatalf("start %s: %v", name, err)
	}
	exited := make(chan struct{})
	result := &process{command: command, exited: exited, logPath: logPath}
	go func() {
		err := command.Wait()
		result.mu.Lock()
		result.exitErr = err
		result.mu.Unlock()
		_ = logFile.Close()
		close(exited)
	}()
	t.Cleanup(func() { stopProcess(t, result) })
	return result
}

func stopProcess(t *testing.T, process *process) {
	t.Helper()
	if process == nil || process.command == nil || process.command.Process == nil {
		return
	}
	select {
	case <-process.exited:
		process.command = nil
		return
	default:
	}
	_ = process.command.Process.Signal(syscall.SIGTERM)
	select {
	case <-process.exited:
	case <-time.After(5 * time.Second):
		_ = process.command.Process.Kill()
		<-process.exited
	}
	process.command = nil
}

func waitHTTP(t *testing.T, url string, expected int, timeout time.Duration, process *process) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-process.exited:
			process.mu.Lock()
			err := process.exitErr
			process.mu.Unlock()
			data, _ := os.ReadFile(process.logPath)
			t.Fatalf("process exited before %s was ready: %v\n%s", url, err, data)
		default:
		}
		response, err := (&http.Client{Timeout: time.Second}).Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == expected {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	data, _ := os.ReadFile(process.logPath)
	t.Fatalf("timeout waiting for %s\n%s", url, data)
}

func waitCommand(t *testing.T, directory string, timeout time.Duration, name string, arguments ...string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last []byte
	for time.Now().Before(deadline) {
		command := exec.Command(name, arguments...)
		command.Dir = directory
		last, _ = command.CombinedOutput()
		if command.ProcessState != nil && command.ProcessState.Success() {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s %s: %s", name, strings.Join(arguments, " "), last)
}

func run(t *testing.T, directory string, environment map[string]string, name string, arguments ...string) []byte {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = mergedEnvironment(environment)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
	return output
}

func mergedEnvironment(overrides map[string]string) []string {
	values := map[string]string{}
	for _, item := range os.Environ() {
		name, value, found := strings.Cut(item, "=")
		if found {
			values[name] = value
		}
	}
	for name, value := range overrides {
		values[name] = value
	}
	result := make([]string, 0, len(values))
	for name, value := range values {
		result = append(result, name+"="+value)
	}
	return result
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func databaseURL(port int, database string) string {
	return fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/%s?sslmode=disable", port, database)
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(directory, "../.."))
	if _, err := os.Stat(filepath.Join(root, "go.work")); err != nil {
		t.Fatalf("repository root: %v", err)
	}
	return root
}

func requireCommand(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is unavailable", name)
	}
}
