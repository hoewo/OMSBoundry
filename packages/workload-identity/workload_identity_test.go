package workloadidentity

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const (
	testTenant  = "11111111-1111-4111-8111-111111111111"
	testSubject = "22222222-2222-4222-8222-222222222222"
)

func TestWorkloadKeysRotateWithoutReopeningStore(t *testing.T) {
	directory := t.TempDir()
	privatePath, publicPath := filepath.Join(directory, "private.json"), filepath.Join(directory, "public.json")
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	if err := Initialize(privatePath, publicPath, "project-workspace", testSubject, now); err != nil {
		t.Fatal(err)
	}
	store, err := OpenKeyStore(privatePath, "project-workspace", testSubject)
	if err != nil {
		t.Fatal(err)
	}
	_, firstKey, err := store.SignAssertion(testTenant, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", now)
	if err != nil {
		t.Fatal(err)
	}
	rotatedKey, err := Rotate(privatePath, publicPath, "project-workspace", testSubject, now.Add(time.Minute), 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_, secondKey, err := store.SignAssertion(testTenant, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if firstKey == secondKey || secondKey != rotatedKey {
		t.Fatalf("first=%s second=%s rotated=%s", firstKey, secondKey, rotatedKey)
	}
}

func TestTokenSourceCachesRefreshesAndReloadsRotatedKey(t *testing.T) {
	directory := t.TempDir()
	privatePath, publicPath := filepath.Join(directory, "private.json"), filepath.Join(directory, "public.json")
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	if err := Initialize(privatePath, publicPath, "project-workspace", testSubject, now); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	calls := 0
	keyIDs := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input exchangeRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		parts := splitCompact(t, input.Assertion)
		var header assertionHeader
		decodeJSONPart(t, parts[0], &header)
		mu.Lock()
		calls++
		keyIDs = append(keyIDs, header.KeyID)
		call := calls
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "token-" + string(rune('0'+call)), "token_type": "Bearer", "expires_at": now.Add(time.Minute), "claims": map[string]any{}})
	}))
	defer server.Close()
	source, err := NewTokenSource(server.URL, testTenant, "project-workspace", testSubject, privatePath, time.Second, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	source.now = func() time.Time { return now }
	first, err := source.Credential(t.Context(), "people-groups", "people-groups.view", "person", testSubject)
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.Credential(t.Context(), "people-groups", "people-groups.view", "person", testSubject)
	if err != nil || second != first || calls != 1 {
		t.Fatalf("second=%q first=%q calls=%d error=%v", second, first, calls, err)
	}
	if _, err := Rotate(privatePath, publicPath, "project-workspace", testSubject, now.Add(10*time.Second), 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	source.Invalidate("people-groups", "people-groups.view", "person", testSubject)
	if _, err := source.Credential(t.Context(), "people-groups", "people-groups.view", "person", testSubject); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || keyIDs[0] == keyIDs[1] {
		t.Fatalf("calls=%d key IDs=%v", calls, keyIDs)
	}
	now = now.Add(50 * time.Second)
	if _, err := source.Credential(t.Context(), "people-groups", "people-groups.view", "person", testSubject); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("expired credential was not refreshed; calls=%d", calls)
	}
}

func TestTokenSourceSeparatesExactProjectAndFileCredentials(t *testing.T) {
	directory := t.TempDir()
	privatePath, publicPath := filepath.Join(directory, "private.json"), filepath.Join(directory, "public.json")
	now := time.Date(2026, 9, 26, 14, 0, 0, 0, time.UTC)
	if err := Initialize(privatePath, publicPath, "business-engagement", testSubject, now); err != nil {
		t.Fatal(err)
	}
	projectID := "33333333-3333-4333-8333-333333333333"
	fileID := "44444444-4444-4444-8444-444444444444"
	seen := []exchangeTarget{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input exchangeRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		seen = append(seen, input.exchangeTarget)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "credential-for-" + input.Scope.Resources[0].ID, "expires_at": now.Add(time.Minute)})
	}))
	defer server.Close()
	source, err := NewTokenSource(server.URL, testTenant, "business-engagement", testSubject, privatePath, time.Second, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	source.now = func() time.Time { return now }
	projectCredential, err := source.Credential(t.Context(), "project-workspace", "project-workspace.read", "project", projectID)
	if err != nil {
		t.Fatal(err)
	}
	fileCredential, err := source.Credential(t.Context(), "evidence-service", "evidence-service.view", "file", fileID)
	if err != nil {
		t.Fatal(err)
	}
	if projectCredential == fileCredential || len(seen) != 2 {
		t.Fatalf("credentials project=%q file=%q requests=%#v", projectCredential, fileCredential, seen)
	}
	if seen[0].Audience != "project-workspace" || seen[0].Action != "project-workspace.read" || seen[0].Scope.Kind != "resources" || len(seen[0].Scope.Resources) != 1 || seen[0].Scope.Resources[0] != (ResourceRef{Type: "project", ID: projectID}) {
		t.Fatalf("project target=%#v", seen[0])
	}
	if seen[1].Audience != "evidence-service" || seen[1].Action != "evidence-service.view" || seen[1].Scope.Kind != "resources" || len(seen[1].Scope.Resources) != 1 || seen[1].Scope.Resources[0] != (ResourceRef{Type: "file", ID: fileID}) {
		t.Fatalf("file target=%#v", seen[1])
	}
	if _, err := source.Credential(t.Context(), "project-workspace", "project-workspace.read", "project", projectID); err != nil || len(seen) != 2 {
		t.Fatalf("exact project credential was not cached independently; requests=%#v error=%v", seen, err)
	}
}

func TestTokenSourceSeparatesCredentialsByTenant(t *testing.T) {
	directory := t.TempDir()
	privatePath, publicPath := filepath.Join(directory, "private.json"), filepath.Join(directory, "public.json")
	now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	if err := Initialize(privatePath, publicPath, "people-groups", testSubject, now); err != nil {
		t.Fatal(err)
	}
	otherTenant := "33333333-3333-4333-8333-333333333333"
	seenTenants := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input exchangeRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		parts := splitCompact(t, input.Assertion)
		var claims AssertionClaims
		decodeJSONPart(t, parts[1], &claims)
		seenTenants = append(seenTenants, claims.Tenant)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "credential-for-" + claims.Tenant, "expires_at": now.Add(time.Minute)})
	}))
	defer server.Close()
	source, err := NewTokenSource(server.URL, testTenant, "people-groups", testSubject, privatePath, time.Second, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	source.now = func() time.Time { return now }
	first, err := source.CredentialForTenant(t.Context(), testTenant, "identity-access", "identity-access.subject.reference.read", "iam_subject", testSubject)
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.CredentialForTenant(t.Context(), otherTenant, "identity-access", "identity-access.subject.reference.read", "iam_subject", testSubject)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(seenTenants) != 2 || seenTenants[0] != testTenant || seenTenants[1] != otherTenant {
		t.Fatalf("first=%q second=%q tenants=%v", first, second, seenTenants)
	}
	if _, err := source.CredentialForTenant(t.Context(), "not-a-tenant", "identity-access", "identity-access.subject.reference.read", "iam_subject", testSubject); err == nil {
		t.Fatal("malformed tenant accepted")
	}
}

func splitCompact(t *testing.T, raw string) []string {
	t.Helper()
	parts := []string{}
	start := 0
	for index := range raw {
		if raw[index] == '.' {
			parts = append(parts, raw[start:index])
			start = index + 1
		}
	}
	parts = append(parts, raw[start:])
	if len(parts) != 3 {
		t.Fatalf("compact token has %d parts", len(parts))
	}
	return parts
}

func decodeJSONPart(t *testing.T, raw string, destination any) {
	t.Helper()
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || json.Unmarshal(data, destination) != nil {
		t.Fatalf("invalid compact JSON part")
	}
}
