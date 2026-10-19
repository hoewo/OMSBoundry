//go:build integration

package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/evidence-service/internal/database"
	"omsbizfoundry/evidence-service/internal/storage"
)

func TestLocalProviderFullLifecycleAndCleanup(t *testing.T) {
	pool := integrationPool(t)
	provider, err := storage.NewLocalProvider(t.TempDir(), "http://127.0.0.1:8085", []byte(strings.Repeat("k", 32)), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(pool, provider, 15*time.Minute, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{TenantID: "33333333-3333-4333-8333-333333333333", SubjectID: "44444444-4444-4444-8444-444444444444", DisplayName: "Evidence Admin", Kind: "template_subject"}
	content := []byte("%PDF-1.7\ncontent")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	input := CreateUploadInput{FileName: "report.pdf", SizeBytes: int64(len(content)), MediaType: "application/pdf", SHA256: digest, ClientContext: ClientContext{OwnerRef: "opaque-contract-ref", Purpose: "supporting-document"}}
	session, err := service.CreateUploadSession(context.Background(), actor, "create-1", input)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.CreateUploadSession(context.Background(), actor, "create-1", input)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.SessionID != session.SessionID || replayed.FileID != session.FileID {
		t.Fatalf("create replay changed identity: %+v vs %+v", replayed, session)
	}
	changed := input
	changed.FileName = "changed.pdf"
	if _, err := service.CreateUploadSession(context.Background(), actor, "create-1", changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed replay accepted: %v", err)
	}
	token := tokenFromURL(t, session.Authorization.UploadURL)
	intent, status, err := service.GetUploadIntentByGlobalID(context.Background(), session.SessionID)
	if err != nil || status != "pending" {
		t.Fatalf("unexpected upload intent %s: %v", status, err)
	}
	if err := provider.VerifyUploadToken(token, intent); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Put(context.Background(), intent, strings.NewReader(string(content)), int64(len(content))); err != nil {
		t.Fatal(err)
	}
	version, err := service.CompleteUpload(context.Background(), actor, session.SessionID, "complete-1", CompleteUploadInput{ObservedSizeBytes: int64(len(content)), ObservedSHA256: digest})
	if err != nil {
		t.Fatal(err)
	}
	if version.FileID != session.FileID || version.SHA256 != digest || version.SizeBytes != int64(len(content)) || version.MediaType != "application/pdf" {
		t.Fatalf("unexpected confirmed version: %+v", version)
	}
	replayedVersion, err := service.CompleteUpload(context.Background(), actor, session.SessionID, "complete-1", CompleteUploadInput{ObservedSizeBytes: int64(len(content)), ObservedSHA256: digest})
	if err != nil || replayedVersion.SHA256 != digest {
		t.Fatalf("complete replay failed: %+v %v", replayedVersion, err)
	}
	secondCommandVersion, err := service.CompleteUpload(context.Background(), actor, session.SessionID, "complete-1-again", CompleteUploadInput{ObservedSizeBytes: int64(len(content)), ObservedSHA256: digest})
	if err != nil || secondCommandVersion.FileID != version.FileID || secondCommandVersion.SHA256 != digest {
		t.Fatalf("completed session replay with new command failed: %+v %v", secondCommandVersion, err)
	}
	if _, err := service.GetVersion(context.Background(), "55555555-5555-4555-8555-555555555555", version.FileID, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant read succeeded: %v", err)
	}
	revisionContent := []byte("%PDF-1.7\ncontent revision two")
	revisionSum := sha256.Sum256(revisionContent)
	revisionDigest := hex.EncodeToString(revisionSum[:])
	revisionInput := CreateUploadInput{FileName: "report-v2.pdf", SizeBytes: int64(len(revisionContent)), MediaType: "application/pdf", SHA256: revisionDigest, ClientContext: ClientContext{OwnerRef: "opaque-contract-ref", Purpose: "supporting-document-revision"}}
	revisionSession, err := service.CreateVersionUploadSession(context.Background(), actor, version.FileID, "create-version-2", revisionInput)
	if err != nil {
		t.Fatal(err)
	}
	if revisionSession.FileID != version.FileID || revisionSession.Version != 2 {
		t.Fatalf("unexpected revision session: %+v", revisionSession)
	}
	revisionReplay, err := service.CreateVersionUploadSession(context.Background(), actor, version.FileID, "create-version-2", revisionInput)
	if err != nil || revisionReplay.SessionID != revisionSession.SessionID {
		t.Fatalf("revision replay failed: %+v %v", revisionReplay, err)
	}
	changedRevision := revisionInput
	changedRevision.FileName = "different-v2.pdf"
	if _, err := service.CreateVersionUploadSession(context.Background(), actor, version.FileID, "create-version-2", changedRevision); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed revision replay accepted: %v", err)
	}
	revisionIntent, _, err := service.GetUploadIntentByGlobalID(context.Background(), revisionSession.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Put(context.Background(), revisionIntent, strings.NewReader(string(revisionContent)), int64(len(revisionContent))); err != nil {
		t.Fatal(err)
	}
	revision, err := service.CompleteUpload(context.Background(), actor, revisionSession.SessionID, "complete-version-2", CompleteUploadInput{ObservedSizeBytes: int64(len(revisionContent)), ObservedSHA256: revisionDigest})
	if err != nil || revision.FileID != version.FileID || revision.Version != 2 || revision.SHA256 != revisionDigest {
		t.Fatalf("complete revision failed: %+v %v", revision, err)
	}
	latest, err := service.GetLatestVersion(context.Background(), actor.TenantID, version.FileID)
	if err != nil || latest.Version != 2 || latest.SHA256 != revisionDigest {
		t.Fatalf("latest revision=%+v error=%v", latest, err)
	}
	original, err := service.GetVersion(context.Background(), actor.TenantID, version.FileID, 1)
	if err != nil || original.SHA256 != digest {
		t.Fatalf("original revision changed: %+v %v", original, err)
	}
	grant, err := service.CreateAccessGrant(context.Background(), actor, version.FileID, 1, AccessInput{Disposition: "inline", DeliveryMode: "direct"})
	if err != nil {
		t.Fatal(err)
	}
	grantURL, err := url.Parse(grant.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.VerifyAccessToken(grantURL.Query().Get("token"), storage.AccessIntent{FileID: version.FileID, Version: 1, FileName: "report.pdf", Disposition: "inline", Object: version.Object}); err != nil {
		t.Fatal(err)
	}

	second, err := service.CreateUploadSession(context.Background(), actor, "create-2", CreateUploadInput{FileName: "orphan.txt", SizeBytes: 6, MediaType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	secondIntent, _, err := service.GetUploadIntentByGlobalID(context.Background(), second.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Put(context.Background(), secondIntent, strings.NewReader("orphan"), 6); err != nil {
		t.Fatal(err)
	}
	startAbort := make(chan struct{})
	abortErrors := make([]error, 2)
	var abortWait sync.WaitGroup
	for index := range abortErrors {
		abortWait.Add(1)
		go func(index int) {
			defer abortWait.Done()
			<-startAbort
			abortErrors[index] = service.AbortUpload(context.Background(), actor, second.SessionID, "abort-2", AbortUploadInput{Reason: "user_cancelled"})
		}(index)
	}
	close(startAbort)
	abortWait.Wait()
	for _, abortErr := range abortErrors {
		if abortErr != nil {
			t.Fatalf("concurrent abort failed: %v", abortErr)
		}
	}
	crossSession, err := service.CreateUploadSession(context.Background(), actor, "create-cross-session", CreateUploadInput{FileName: "cross.txt", SizeBytes: int64(len(content)), MediaType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteUpload(context.Background(), actor, crossSession.SessionID, "complete-1", CompleteUploadInput{ObservedSizeBytes: int64(len(content)), ObservedSHA256: digest}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("complete idempotency key was reused across sessions: %v", err)
	}
	if err := service.AbortUpload(context.Background(), actor, crossSession.SessionID, "abort-2", AbortUploadInput{Reason: "user_cancelled"}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("abort idempotency key was reused across sessions: %v", err)
	}
	result, err := service.Cleanup(context.Background(), 100, time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if result.Cleaned < 1 || result.Failed != 0 {
		t.Fatalf("unexpected cleanup result: %+v", result)
	}
	if _, err := provider.Open(context.Background(), secondIntent.Object); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("orphan object survived cleanup: %v", err)
	}

	third, err := service.CreateUploadSession(context.Background(), actor, "create-3", CreateUploadInput{FileName: "commit-failure.txt", SizeBytes: 6, MediaType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	thirdIntent, _, err := service.GetUploadIntentByGlobalID(context.Background(), third.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Put(context.Background(), thirdIntent, strings.NewReader("failed"), 6); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `CREATE FUNCTION fail_evidence_version_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='forced finalization failure'; END $$; CREATE TRIGGER force_evidence_version_failure BEFORE INSERT ON file_versions FOR EACH ROW EXECUTE FUNCTION fail_evidence_version_insert()`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteUpload(context.Background(), actor, third.SessionID, "complete-3", CompleteUploadInput{ObservedSizeBytes: 6}); err == nil {
		t.Fatal("forced database finalization failure returned success")
	}
	if _, err := pool.Exec(context.Background(), `DROP TRIGGER force_evidence_version_failure ON file_versions; DROP FUNCTION fail_evidence_version_insert()`); err != nil {
		t.Fatal(err)
	}
	result, err = service.Cleanup(context.Background(), 100, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if result.Cleaned < 1 {
		t.Fatalf("stale completing session was not cleaned: %+v", result)
	}
	if _, err := provider.Open(context.Background(), thirdIntent.Object); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("object from failed database commit survived cleanup: %v", err)
	}

	var versionCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM file_versions`).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if versionCount != 2 {
		t.Fatalf("unexpected version count %d", versionCount)
	}
	_, err = pool.Exec(context.Background(), `UPDATE file_versions SET file_name='tampered.pdf' WHERE file_id=$1`, version.FileID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55000" {
		t.Fatalf("immutable version update was not rejected: %v", err)
	}
	_, err = pool.Exec(context.Background(), `UPDATE idempotency_commands SET created_at=created_at + interval '1 second' WHERE action='upload.complete'`)
	if !errors.As(err, &pgErr) || pgErr.Code != "55000" {
		t.Fatalf("immutable idempotency update was not rejected: %v", err)
	}
}

func TestExpiredCreateReplayReturnsExpired(t *testing.T) {
	pool := integrationPool(t)
	provider, err := storage.NewLocalProvider(t.TempDir(), "http://127.0.0.1:8085", []byte(strings.Repeat("k", 32)), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(pool, provider, time.Minute, 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	actor := Actor{TenantID: "33333333-3333-4333-8333-333333333333", SubjectID: "44444444-4444-4444-8444-444444444444", DisplayName: "Evidence Admin", Kind: "template_subject"}
	input := CreateUploadInput{FileName: "expires.txt", SizeBytes: 1, MediaType: "text/plain"}
	if _, err := service.CreateUploadSession(context.Background(), actor, "expires-create", input); err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := service.CreateUploadSession(context.Background(), actor, "expires-create", input); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired create replay error=%v", err)
	}
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM upload_sessions WHERE tenant_id=$1`, actor.TenantID).Scan(&status); err != nil || status != "expired" {
		t.Fatalf("expired session status=%q error=%v", status, err)
	}
}

func integrationPool(t *testing.T) *pgxpool.Pool {
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
	schema := "evidence_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
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
func tokenFromURL(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	token := parsed.Query().Get("token")
	if token == "" {
		t.Fatalf("missing token in %q", raw)
	}
	return token
}
