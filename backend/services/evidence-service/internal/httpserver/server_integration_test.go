//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/evidence-service/internal/auth"
	"omsbizfoundry/evidence-service/internal/database"
	"omsbizfoundry/evidence-service/internal/evidence"
	"omsbizfoundry/evidence-service/internal/storage"
)

func TestHTTPFullLocalFileLifecycle(t *testing.T) {
	pool := httpIntegrationPool(t)
	provider, err := storage.NewLocalProvider(t.TempDir(), "http://evidence.invalid", []byte(strings.Repeat("k", 32)), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service, err := evidence.NewService(pool, provider, 15*time.Minute, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := auth.NewTemplateProvider("33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444", "Evidence Admin", "this-is-a-valid-template-token")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := New(service, identity, provider, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := server.Client()
	content := []byte("%PDF-1.7\nhttp integration")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	unauthorized := jsonRequest(t, http.MethodPost, server.URL+"/v1/upload-sessions", map[string]any{"file_name": "x", "size_bytes": 1})
	unauthorized.Header.Set("Idempotency-Key", "unauthorized")
	response, err := client.Do(unauthorized)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous create returned %d", response.StatusCode)
	}
	invalidID := jsonRequest(t, http.MethodPost, server.URL+"/v1/upload-sessions/not-a-uuid/abort", map[string]any{"reason": "user_cancelled"})
	authorize(invalidID, "this-is-a-valid-template-token")
	invalidID.Header.Set("Idempotency-Key", "invalid-id")
	response, err = client.Do(invalidID)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("invalid session id returned %d", response.StatusCode)
	}

	create := jsonRequest(t, http.MethodPost, server.URL+"/v1/upload-sessions", map[string]any{"file_name": "report.pdf", "size_bytes": len(content), "media_type": "application/pdf", "sha256": digest})
	authorize(create, "this-is-a-valid-template-token")
	create.Header.Set("Idempotency-Key", "http-create-1")
	response, err = client.Do(create)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create returned %d: %s", response.StatusCode, readBody(response))
	}
	var session evidence.UploadSession
	decodeResponse(t, response, &session)
	uploadURL := replaceHost(t, session.Authorization.UploadURL, server.URL)
	upload, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	upload.Header.Set("Content-Type", "application/pdf")
	response, err = client.Do(upload)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("upload returned %d", response.StatusCode)
	}

	complete := jsonRequest(t, http.MethodPost, server.URL+"/v1/upload-sessions/"+session.SessionID+"/complete", map[string]any{"observed_size_bytes": len(content), "observed_sha256": digest})
	authorize(complete, "this-is-a-valid-template-token")
	complete.Header.Set("Idempotency-Key", "http-complete-1")
	response, err = client.Do(complete)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("complete returned %d: %s", response.StatusCode, readBody(response))
	}
	var version evidence.Version
	decodeResponse(t, response, &version)
	if version.SHA256 != digest || version.MediaType != "application/pdf" {
		t.Fatalf("unexpected version: %+v", version)
	}

	revisionContent := []byte("%PDF-1.7\nhttp integration revision two")
	revisionSum := sha256.Sum256(revisionContent)
	revisionDigest := hex.EncodeToString(revisionSum[:])
	createRevision := jsonRequest(t, http.MethodPost, server.URL+"/v1/files/"+version.FileID+"/versions/upload-sessions", map[string]any{"file_name": "report-v2.pdf", "size_bytes": len(revisionContent), "media_type": "application/pdf", "sha256": revisionDigest})
	authorize(createRevision, "this-is-a-valid-template-token")
	createRevision.Header.Set("Idempotency-Key", "http-create-version-2")
	response, err = client.Do(createRevision)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create revision returned %d: %s", response.StatusCode, readBody(response))
	}
	var revisionSession evidence.UploadSession
	decodeResponse(t, response, &revisionSession)
	if revisionSession.FileID != version.FileID || revisionSession.Version != 2 {
		t.Fatalf("unexpected revision session: %+v", revisionSession)
	}
	revisionUploadURL := replaceHost(t, revisionSession.Authorization.UploadURL, server.URL)
	revisionUpload, err := http.NewRequest(http.MethodPut, revisionUploadURL, bytes.NewReader(revisionContent))
	if err != nil {
		t.Fatal(err)
	}
	revisionUpload.Header.Set("Content-Type", "application/pdf")
	response, err = client.Do(revisionUpload)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("revision upload returned %d", response.StatusCode)
	}
	completeRevision := jsonRequest(t, http.MethodPost, server.URL+"/v1/upload-sessions/"+revisionSession.SessionID+"/complete", map[string]any{"observed_size_bytes": len(revisionContent), "observed_sha256": revisionDigest})
	authorize(completeRevision, "this-is-a-valid-template-token")
	completeRevision.Header.Set("Idempotency-Key", "http-complete-version-2")
	response, err = client.Do(completeRevision)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("complete revision returned %d: %s", response.StatusCode, readBody(response))
	}
	var revision evidence.Version
	decodeResponse(t, response, &revision)
	if revision.Version != 2 || revision.FileID != version.FileID || revision.SHA256 != revisionDigest {
		t.Fatalf("unexpected revision: %+v", revision)
	}
	stableReference, err := http.NewRequest(http.MethodGet, server.URL+"/v1/references/files/"+version.FileID, nil)
	if err != nil {
		t.Fatal(err)
	}
	authorize(stableReference, "this-is-a-valid-template-token")
	response, err = client.Do(stableReference)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("stable reference returned %d: %s", response.StatusCode, readBody(response))
	}
	var latestReference map[string]any
	decodeResponse(t, response, &latestReference)
	if latestReference["version"] != float64(2) || latestReference["revision"] != float64(2) || latestReference["sha256"] != revisionDigest {
		t.Fatalf("stable reference did not return latest revision: %#v", latestReference)
	}

	grantRequest := jsonRequest(t, http.MethodPost, server.URL+"/v1/files/"+version.FileID+"/versions/1/access-grants", map[string]any{"disposition": "inline", "delivery_mode": "direct"})
	authorize(grantRequest, "this-is-a-valid-template-token")
	response, err = client.Do(grantRequest)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("grant returned %d: %s", response.StatusCode, readBody(response))
	}
	var grant evidence.AccessGrant
	decodeResponse(t, response, &grant)
	downloadURL := replaceHost(t, grant.URL, server.URL)
	download, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	download.Header.Set("Range", "bytes=0-3")
	response, err = client.Do(download)
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(response)
	if response.StatusCode != http.StatusPartialContent || body != "%PDF" {
		t.Fatalf("range download returned %d %q", response.StatusCode, body)
	}

	bad, err := url.Parse(downloadURL)
	if err != nil {
		t.Fatal(err)
	}
	query := bad.Query()
	query.Set("file_name", "changed.pdf")
	bad.RawQuery = query.Encode()
	response, err = client.Get(bad.String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tampered capability returned %d", response.StatusCode)
	}
}

func authorize(request *http.Request, token string) {
	request.Header.Set("Authorization", "Bearer "+token)
}
func jsonRequest(t *testing.T, method, target string, payload any) *http.Request {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(method, target, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	return request
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
	data, _ := io.ReadAll(response.Body)
	return string(data)
}
func replaceHost(t *testing.T, raw, base string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	target, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Scheme = target.Scheme
	parsed.Host = target.Host
	return parsed.String()
}

func httpIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "evidence_http_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`) })
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}
