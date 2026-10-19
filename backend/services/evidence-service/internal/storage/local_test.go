package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLocalProviderBindsCapabilitiesAndObjectPath(t *testing.T) {
	provider, err := NewLocalProvider(t.TempDir(), "http://127.0.0.1:8085", []byte(strings.Repeat("k", 32)), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	intent := UploadIntent{SessionID: "11111111-1111-4111-8111-111111111111", FileID: "22222222-2222-4222-8222-222222222222", Version: 1, FileName: "report.pdf", MediaType: "application/pdf", ExpectedSize: 7, ExpiresAt: now.Add(15 * time.Minute), Object: ObjectRef{TenantID: "33333333-3333-4333-8333-333333333333", Provider: "local", ObjectKey: "tenants/t/files/f/versions/1/s"}}
	authorization, err := provider.AuthorizeUpload(context.Background(), intent)
	if err != nil {
		t.Fatal(err)
	}
	if authorization.Provider != "local-http" || authorization.Method != "PUT" || authorization.Scope.ObjectKey != intent.Object.ObjectKey || len(authorization.Scope.AllowedActions) != 1 {
		t.Fatalf("unexpected authorization: %+v", authorization)
	}
	token := queryToken(t, authorization.UploadURL)
	if err := provider.VerifyUploadToken(token, intent); err != nil {
		t.Fatal(err)
	}
	other := intent
	other.Object.ObjectKey += "-other"
	if err := provider.VerifyUploadToken(token, other); !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatalf("cross-object token accepted: %v", err)
	}
	etag, err := provider.Put(context.Background(), intent, strings.NewReader("content"), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(etag) != 64 {
		t.Fatalf("unexpected etag %q", etag)
	}
	object, err := provider.Open(context.Background(), intent.Object)
	if err != nil {
		t.Fatal(err)
	}
	defer object.Reader.Close()
	data, err := io.ReadAll(object.Reader)
	if err != nil || string(data) != "content" {
		t.Fatalf("unexpected object %q: %v", data, err)
	}
	if _, err := provider.Put(context.Background(), intent, strings.NewReader("content"), 7); !errors.Is(err, ErrObjectExists) {
		t.Fatalf("overwrite was not rejected: %v", err)
	}
	if err := provider.Delete(context.Background(), intent.Object); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Open(context.Background(), intent.Object); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted object remains: %v", err)
	}
}

func TestLocalProviderConcurrentPutNeverOverwrites(t *testing.T) {
	provider, err := NewLocalProvider(t.TempDir(), "http://127.0.0.1:8085", []byte(strings.Repeat("k", 32)), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	intent := UploadIntent{
		SessionID: "11111111-1111-4111-8111-111111111111", ExpectedSize: 7,
		Object: ObjectRef{TenantID: "33333333-3333-4333-8333-333333333333", Provider: "local", ObjectKey: "tenants/t/files/f/versions/1/s"},
	}
	start := make(chan struct{})
	errorsByWriter := make([]error, 2)
	contents := []string{"first!!", "second!"}
	var wait sync.WaitGroup
	for index := range contents {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, errorsByWriter[index] = provider.Put(context.Background(), intent, strings.NewReader(contents[index]), 7)
		}(index)
	}
	close(start)
	wait.Wait()
	succeeded := 0
	rejected := 0
	for _, err := range errorsByWriter {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrObjectExists):
			rejected++
		default:
			t.Fatalf("unexpected concurrent PUT error: %v", err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent PUT results: succeeded=%d rejected=%d errors=%v", succeeded, rejected, errorsByWriter)
	}
	object, err := provider.Open(context.Background(), intent.Object)
	if err != nil {
		t.Fatal(err)
	}
	defer object.Reader.Close()
	value, err := io.ReadAll(object.Reader)
	if err != nil || (string(value) != contents[0] && string(value) != contents[1]) {
		t.Fatalf("published object=%q error=%v", value, err)
	}
}

func TestLocalProviderExpiresAndRejectsTraversal(t *testing.T) {
	provider, err := NewLocalProvider(t.TempDir(), "https://evidence.example", []byte(strings.Repeat("k", 32)), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	provider.now = func() time.Time { return now }
	intent := UploadIntent{SessionID: "session", ExpiresAt: now.Add(time.Minute), Object: ObjectRef{TenantID: "tenant", Provider: "local", ObjectKey: "safe/object"}}
	authorization, err := provider.AuthorizeUpload(context.Background(), intent)
	if err != nil {
		t.Fatal(err)
	}
	token := queryToken(t, authorization.UploadURL)
	provider.now = func() time.Time { return now.Add(2 * time.Minute) }
	if err := provider.VerifyUploadToken(token, intent); !errors.Is(err, ErrExpiredAuthorization) {
		t.Fatalf("expired token accepted: %v", err)
	}
	if _, err := provider.Open(context.Background(), ObjectRef{Provider: "local", ObjectKey: "../outside"}); !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatalf("traversal accepted: %v", err)
	}
}

func queryToken(t *testing.T, raw string) string {
	t.Helper()
	index := strings.Index(raw, "token=")
	if index < 0 {
		t.Fatalf("missing token in %q", raw)
	}
	return raw[index+len("token="):]
}
