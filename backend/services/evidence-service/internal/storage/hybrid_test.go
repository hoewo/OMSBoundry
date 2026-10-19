package storage

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"
)

type transitionOSS struct {
	calls []string
	err   error
}

func (p *transitionOSS) Name() string                { return "aliyun-oss" }
func (p *transitionOSS) BucketName() string          { return "test-bucket" }
func (p *transitionOSS) Ready(context.Context) error { return p.err }
func (p *transitionOSS) AuthorizeUpload(context.Context, UploadIntent) (UploadAuthorization, error) {
	p.calls = append(p.calls, "upload")
	return UploadAuthorization{Provider: p.Name()}, p.err
}
func (p *transitionOSS) Open(context.Context, ObjectRef) (ObjectReader, error) {
	p.calls = append(p.calls, "open")
	return ObjectReader{Reader: io.NopCloser(strings.NewReader("oss bytes"))}, p.err
}
func (p *transitionOSS) AuthorizeAccess(context.Context, AccessIntent) (AccessGrant, error) {
	p.calls = append(p.calls, "access")
	return AccessGrant{URL: "https://oss.example.test"}, p.err
}
func (p *transitionOSS) Delete(context.Context, ObjectRef) error {
	p.calls = append(p.calls, "delete")
	return p.err
}

func TestHybridPreservesLocalUploadsAndReadsAcrossPrimarySwitch(t *testing.T) {
	ctx := context.Background()
	local, err := NewLocalProvider(t.TempDir(), "https://files.example.test", []byte(strings.Repeat("k", 32)), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	oss := &transitionOSS{}
	intent := UploadIntent{SessionID: "session", FileID: "file", Version: 1, ExpectedSize: 5, ExpiresAt: time.Now().Add(time.Minute), Object: ObjectRef{TenantID: "t", Provider: "local", ObjectKey: "tenants/t/files/file/versions/1/session"}}
	for _, primary := range []string{"aliyun-oss", "local"} {
		p, err := NewHybridProvider(primary, local, oss)
		if err != nil {
			t.Fatal(err)
		}
		if p.Name() != primary || p.StorageMode() != "hybrid" {
			t.Fatal("new session provider is incorrect")
		}
		if (primary == "local" && p.BucketName() != "") || (primary == "aliyun-oss" && p.BucketName() != "test-bucket") {
			t.Fatal("new session bucket is incorrect")
		}
		auth, err := p.AuthorizeUpload(ctx, intent)
		if err != nil || auth.Provider != "local-http" {
			t.Fatalf("resume local upload: %v", err)
		}
		u, _ := url.Parse(auth.UploadURL)
		if err := p.VerifyUploadToken(u.Query().Get("token"), intent); err != nil {
			t.Fatal(err)
		}
		if primary == "aliyun-oss" {
			if _, err := p.Put(ctx, intent, strings.NewReader("local"), 5); err != nil {
				t.Fatal(err)
			}
		}
		reader, err := p.Open(ctx, intent.Object)
		if err != nil {
			t.Fatal(err)
		}
		bytes, _ := io.ReadAll(reader.Reader)
		_ = reader.Reader.Close()
		if string(bytes) != "local" {
			t.Fatal("legacy bytes changed")
		}
		access := AccessIntent{FileID: "file", Version: 1, Object: intent.Object}
		grant, err := p.AuthorizeAccess(ctx, access)
		if err != nil {
			t.Fatal(err)
		}
		grantURL, _ := url.Parse(grant.URL)
		if err := p.VerifyAccessToken(grantURL.Query().Get("token"), access); err != nil {
			t.Fatal(err)
		}
		remote := intent.Object
		remote.Provider, remote.Bucket = "aliyun-oss", "test-bucket"
		remoteIntent := intent
		remoteIntent.Object = remote
		if _, err := p.AuthorizeUpload(ctx, remoteIntent); err != nil {
			t.Fatal(err)
		}
		reader, err = p.Open(ctx, remote)
		if err != nil {
			t.Fatal(err)
		}
		_ = reader.Reader.Close()
		if _, err := p.AuthorizeAccess(ctx, AccessIntent{Object: remote}); err != nil {
			t.Fatal(err)
		}
		if err := p.Delete(ctx, remote); err != nil {
			t.Fatal(err)
		}
		if err := p.Ready(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(oss.calls) != 8 {
		t.Fatalf("unexpected routing calls: %v", oss.calls)
	}
	oss.err = ErrNotFound
	p, _ := NewHybridProvider("local", local, oss)
	remote := intent.Object
	remote.Provider, remote.Bucket = "aliyun-oss", "test-bucket"
	if _, err := p.Open(ctx, remote); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing OSS object silently fell back to existing local object")
	}
	if err := p.Ready(ctx); err == nil {
		t.Fatal("unavailable secondary provider reported ready")
	}
}

func TestHybridRejectsUnknownProviderBucketAndCrossDataPlane(t *testing.T) {
	local, _ := NewLocalProvider(t.TempDir(), "https://files.example.test", []byte(strings.Repeat("k", 32)), time.Minute)
	oss := &transitionOSS{}
	p, _ := NewHybridProvider("aliyun-oss", local, oss)
	ctx := context.Background()
	for _, ref := range []ObjectRef{{Provider: "other"}, {Provider: "local", Bucket: "test-bucket"}, {Provider: "aliyun-oss", Bucket: "other-bucket"}} {
		if _, err := p.Open(ctx, ref); !errors.Is(err, ErrInvalidAuthorization) {
			t.Fatal("invalid read accepted")
		}
		if _, err := p.AuthorizeUpload(ctx, UploadIntent{Object: ref}); !errors.Is(err, ErrInvalidAuthorization) {
			t.Fatal("invalid upload accepted")
		}
		if _, err := p.AuthorizeAccess(ctx, AccessIntent{Object: ref}); !errors.Is(err, ErrInvalidAuthorization) {
			t.Fatal("invalid access accepted")
		}
		if err := p.Delete(ctx, ref); !errors.Is(err, ErrInvalidAuthorization) {
			t.Fatal("invalid cleanup accepted")
		}
	}
	remote := ObjectRef{Provider: "aliyun-oss", Bucket: "test-bucket", ObjectKey: "object"}
	if err := p.VerifyUploadToken("token", UploadIntent{Object: remote}); !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatal("OSS token treated as local")
	}
	if _, err := p.Put(ctx, UploadIntent{Object: remote}, strings.NewReader("x"), 1); !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatal("OSS upload used local data plane")
	}
	if err := p.VerifyAccessToken("token", AccessIntent{Object: remote}); !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatal("OSS access used local data plane")
	}
	if len(oss.calls) != 0 {
		t.Fatal("rejected input reached OSS")
	}
	for _, primary := range []string{"", "hybrid", "other"} {
		if _, err := NewHybridProvider(primary, local, oss); err == nil {
			t.Fatal("invalid primary accepted")
		}
	}
}
