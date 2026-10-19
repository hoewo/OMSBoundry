//go:build cloudacceptance

package storage_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	osscredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"omsbizfoundry/evidence-service/internal/storage"
)

// This opt-in destructive drill is limited to the exact synthetic transition
// fixture. It is not a production backup tool, and cannot target arbitrary files.
func TestIsolatedOSSObjectOffHostRecovery(t *testing.T) {
	phase := os.Getenv("RECOVERY_PHASE")
	if phase == "" {
		t.Skip("explicit isolated cloud acceptance only")
	}
	var record struct {
		FileName  string            `json:"file_name"`
		FileID    string            `json:"file_id"`
		SessionID string            `json:"session_id"`
		Version   int               `json:"version"`
		Size      int64             `json:"size_bytes"`
		SHA       string            `json:"sha256"`
		Object    storage.ObjectRef `json:"object"`
	}
	if json.Unmarshal([]byte(os.Getenv("RECOVERY_RECORD")), &record) != nil {
		t.Fatal("invalid isolated record")
	}
	expected := bytes.Repeat([]byte("Auth58 isolated OSS transition bytes\n"), 128)
	digest := sha256.Sum256(expected)
	expectedSHA := hex.EncodeToString(digest[:])
	if record.FileName != "auth58-transition-cloud.txt" || record.Version != 1 || record.Size != int64(len(expected)) || record.SHA != expectedSHA || record.Object.Provider != "aliyun-oss" || !strings.Contains(record.Object.ObjectKey, "/files/"+record.FileID+"/versions/1/") {
		t.Fatal("refuse anything except the synthetic transition fixture")
	}
	path := os.Getenv("RECOVERY_BLOB")
	if !strings.HasPrefix(path, "/backups/") || filepath.Clean(path) != path {
		t.Fatal("explicit protected backup path required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Minute)
	defer cancel()
	provider, err := storage.NewAliyunOSSProvider(storage.AliyunOSSOptions{Region: os.Getenv("OSS_REGION"), Bucket: os.Getenv("OSS_BUCKET"), RoleARN: os.Getenv("OSS_ROLE_ARN"), ServiceEndpoint: os.Getenv("OSS_SERVICE_ENDPOINT"), STSEndpoint: os.Getenv("OSS_STS_ENDPOINT"), STSDuration: 15 * time.Minute, AccessTTL: 5 * time.Minute})
	if err != nil {
		t.Fatal("provider initialization failed; sensitive diagnostics suppressed")
	}
	read := func() []byte {
		object, err := provider.Open(ctx, record.Object)
		if err != nil {
			t.Fatal("object read failed; sensitive diagnostics suppressed")
		}
		defer object.Reader.Close()
		content, err := io.ReadAll(io.LimitReader(object.Reader, record.Size+1))
		if err != nil || object.Metadata.SizeBytes != record.Size || !bytes.Equal(content, expected) {
			t.Fatal("object bytes or metadata differ")
		}
		return content
	}
	switch phase {
	case "expiry":
		read()
		intent := storage.UploadIntent{SessionID: record.SessionID, FileID: record.FileID, Version: 1, ExpectedSize: record.Size, MediaType: "text/plain", ExpiresAt: time.Now().UTC().Add(30 * time.Minute), Object: record.Object}
		auth, err := provider.AuthorizeUpload(ctx, intent)
		if err != nil {
			t.Fatal("initial bounded STS failed")
		}
		put := func(auth storage.UploadAuthorization) error {
			client := oss.NewClient(oss.LoadDefaultConfig().WithRegion(auth.Region).WithEndpoint(os.Getenv("OSS_SERVICE_ENDPOINT")).WithRetryMaxAttempts(1).WithCredentialsProvider(osscredentials.NewStaticCredentialsProvider(auth.AccessKeyID, auth.AccessKeySecret, auth.SecurityToken)))
			_, err := client.PutObject(ctx, &oss.PutObjectRequest{Bucket: oss.Ptr(auth.Bucket), Key: oss.Ptr(auth.ObjectKey), Body: bytes.NewReader(expected), ContentType: oss.Ptr("text/plain"), ForbidOverwrite: oss.Ptr("true")})
			return err
		}
		serviceError := func(err error, status int, code string) bool {
			var actual *oss.ServiceError
			return errors.As(err, &actual) && actual.StatusCode == status && (code == "" || actual.Code == code)
		}
		if !serviceError(put(auth), 409, "FileAlreadyExists") {
			t.Fatal("initial STS not accepted or overwrite guard absent")
		}
		wait := time.Until(auth.ExpiresAt) + 10*time.Second
		if wait <= 0 || wait > 16*time.Minute {
			t.Fatal("unexpected live STS lifetime")
		}
		t.Logf("waiting for actual STS expiry at %s, no clock injection", auth.ExpiresAt.Format(time.RFC3339))
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			t.Fatal("expiry wait cancelled")
		case <-timer.C:
		}
		if !serviceError(put(auth), 403, "SecurityTokenExpired") {
			t.Fatal("expired STS did not fail closed")
		}
		refreshed, err := provider.AuthorizeUpload(ctx, intent)
		if err != nil || !refreshed.ExpiresAt.After(auth.ExpiresAt) || refreshed.AccessKeyID == auth.AccessKeyID {
			t.Fatal("fresh bounded STS issuance failed")
		}
		if !serviceError(put(refreshed), 409, "FileAlreadyExists") {
			t.Fatal("fresh STS unusable or overwrite guard absent")
		}
		read()
		t.Log("actual expired STS rejected 403 SecurityTokenExpired; fresh STS accepted with overwrite guard")
	case "export":
		content := read()
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal("refuse replacing an existing backup")
		}
		_, writeErr := file.Write(content)
		syncErr := file.Sync()
		closeErr := file.Close()
		if writeErr != nil || syncErr != nil || closeErr != nil {
			t.Fatal("backup byte export failed")
		}
	case "restore":
		if os.Getenv("RECOVERY_ALLOW_REMOVE_ISOLATED") != "yes" {
			t.Fatal("explicit isolated removal confirmation required")
		}
		content, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(content, expected) {
			t.Fatal("off-host recovery blob invalid; object not removed")
		}
		read() // Refuse deletion unless the original is exactly this synthetic file.
		if err := provider.Delete(ctx, record.Object); err != nil {
			t.Fatal("isolated object removal failed")
		}
		if _, err := provider.Open(ctx, record.Object); !errors.Is(err, storage.ErrNotFound) {
			t.Fatal("missing object was not actually observed")
		}
		t.Log("isolated object GET rejected as not found after removal")
		auth, err := provider.AuthorizeUpload(ctx, storage.UploadIntent{SessionID: record.SessionID, FileID: record.FileID, Version: 1, ExpectedSize: record.Size, MediaType: "text/plain", ExpiresAt: time.Now().UTC().Add(15 * time.Minute), Object: record.Object})
		if err != nil {
			t.Fatal("bounded recovery STS failed; byte backup retained")
		}
		client := oss.NewClient(oss.LoadDefaultConfig().WithRegion(auth.Region).WithEndpoint(os.Getenv("OSS_SERVICE_ENDPOINT")).WithCredentialsProvider(osscredentials.NewStaticCredentialsProvider(auth.AccessKeyID, auth.AccessKeySecret, auth.SecurityToken)))
		_, err = client.PutObject(ctx, &oss.PutObjectRequest{Bucket: oss.Ptr(auth.Bucket), Key: oss.Ptr(auth.ObjectKey), Body: bytes.NewReader(content), ContentType: oss.Ptr("text/plain"), ForbidOverwrite: oss.Ptr("true")})
		if err != nil {
			t.Fatal("isolated restore upload failed; byte backup retained")
		}
		read()
	case "verify":
		read()
	default:
		t.Fatal("unknown recovery phase")
	}
	t.Logf("phase=%s bytes=%d sha256=%s", phase, record.Size, record.SHA)
}
