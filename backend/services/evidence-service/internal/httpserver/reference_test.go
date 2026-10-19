package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/evidence-service/internal/auth"
	"omsbizfoundry/evidence-service/internal/evidence"
	"omsbizfoundry/evidence-service/internal/storage"
)

const (
	referenceTenant = "11111111-1111-4111-8111-111111111111"
	referenceFile   = "22222222-2222-4222-8222-222222222222"
	otherFile       = "33333333-3333-4333-8333-333333333333"
)

type referenceUseCases struct {
	version        evidence.Version
	listResult     evidence.VersionPage
	getCalls       int
	latestCalls    int
	versionUploads int
	accessCalls    int
}

func (*referenceUseCases) Ping(context.Context) error { return nil }
func (s *referenceUseCases) ListVersions(context.Context, string, int) (evidence.VersionPage, error) {
	return s.listResult, nil
}

func TestDemoEntryAndVersionListUseProtectedBusinessAPI(t *testing.T) {
	useCases := &referenceUseCases{listResult: evidence.VersionPage{Items: []evidence.Version{{FileID: referenceFile, Version: 1, Status: "available", FileName: "方案.pdf"}}}}
	server, err := New(useCases, referenceAuthenticator{principal: tenantViewPrincipal()}, referenceProvider{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	page := httptest.NewRecorder()
	server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "文件库") || !strings.Contains(page.Body.String(), "上传新版本") || !strings.Contains(page.Body.String(), "/assets/demo.js") {
		t.Fatalf("demo page status=%d body=%q", page.Code, page.Body.String())
	}
	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/v1/files", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "方案.pdf") {
		t.Fatalf("version list status=%d body=%q", list.Code, list.Body.String())
	}
}
func (*referenceUseCases) CreateUploadSession(context.Context, evidence.Actor, string, evidence.CreateUploadInput) (evidence.UploadSession, error) {
	return evidence.UploadSession{}, nil
}
func (s *referenceUseCases) CreateVersionUploadSession(context.Context, evidence.Actor, string, string, evidence.CreateUploadInput) (evidence.UploadSession, error) {
	s.versionUploads++
	return evidence.UploadSession{FileID: referenceFile, Version: 2, Status: "pending"}, nil
}
func (*referenceUseCases) CompleteUpload(context.Context, evidence.Actor, string, string, evidence.CompleteUploadInput) (evidence.Version, error) {
	return evidence.Version{}, nil
}
func (*referenceUseCases) AbortUpload(context.Context, evidence.Actor, string, string, evidence.AbortUploadInput) error {
	return nil
}
func (s *referenceUseCases) CreateAccessGrant(context.Context, evidence.Actor, string, int, evidence.AccessInput) (evidence.AccessGrant, error) {
	s.accessCalls++
	return evidence.AccessGrant{}, nil
}
func (s *referenceUseCases) GetVersion(context.Context, string, string, int) (evidence.Version, error) {
	s.getCalls++
	return s.version, nil
}
func (s *referenceUseCases) GetLatestVersion(context.Context, string, string) (evidence.Version, error) {
	s.latestCalls++
	return s.version, nil
}
func (*referenceUseCases) GetUploadIntentByGlobalID(context.Context, string) (storage.UploadIntent, string, error) {
	return storage.UploadIntent{}, "", nil
}
func (*referenceUseCases) GetVersionByGlobalID(context.Context, string, int) (evidence.Version, error) {
	return evidence.Version{}, nil
}

type referenceAuthenticator struct{ principal auth.Principal }

func (referenceAuthenticator) ProviderName() string { return "test" }
func (a referenceAuthenticator) Authenticate(*http.Request) (auth.Principal, error) {
	return a.principal, nil
}

func tenantViewPrincipal() auth.Principal {
	return auth.Principal{TenantID: referenceTenant, ScopeKind: "tenant", AllowedActions: map[string]bool{auth.ActionView: true}, Resources: map[string]map[string]bool{}}
}

func fileViewPrincipal(fileID string) auth.Principal {
	return auth.Principal{TenantID: referenceTenant, ScopeKind: "resources", AllowedActions: map[string]bool{auth.ActionView: true}, Resources: map[string]map[string]bool{"file": {fileID: true}}}
}

func fileCapabilityPrincipal(fileID, action string) auth.Principal {
	return auth.Principal{TenantID: referenceTenant, ScopeKind: "resources", AllowedActions: map[string]bool{action: true}, Resources: map[string]map[string]bool{"file": {fileID: true}}}
}

type referenceProvider struct{}

func (referenceProvider) Name() string                { return "test" }
func (referenceProvider) Ready(context.Context) error { return nil }
func (referenceProvider) AuthorizeUpload(context.Context, storage.UploadIntent) (storage.UploadAuthorization, error) {
	return storage.UploadAuthorization{}, nil
}
func (referenceProvider) Open(context.Context, storage.ObjectRef) (storage.ObjectReader, error) {
	return storage.ObjectReader{}, nil
}
func (referenceProvider) AuthorizeAccess(context.Context, storage.AccessIntent) (storage.AccessGrant, error) {
	return storage.AccessGrant{}, nil
}
func (referenceProvider) Delete(context.Context, storage.ObjectRef) error { return nil }

func TestFileVersionReferenceDoesNotExposeObjectLocation(t *testing.T) {
	useCases := &referenceUseCases{version: evidence.Version{FileID: referenceFile, Version: 1, Status: "available", FileName: "方案.pdf", SizeBytes: 42, MediaType: "application/pdf", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CreatedAt: time.Now(), Object: storage.ObjectRef{Bucket: "secret-bucket", ObjectKey: "secret/key"}}}
	server, err := New(useCases, referenceAuthenticator{principal: tenantViewPrincipal()}, referenceProvider{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/references/files/"+referenceFile+"/versions/1", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var value map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value["tenant_id"] != referenceTenant || value["resource_type"] != "evidence-file" || value["display_name"] != "方案.pdf" {
		t.Fatalf("reference=%#v", value)
	}
	for _, forbidden := range []string{"bucket", "object_key", "object"} {
		if _, ok := value[forbidden]; ok {
			t.Fatalf("reference exposed %s: %#v", forbidden, value)
		}
	}
}

func TestStableFileReferenceReturnsLatestVersion(t *testing.T) {
	useCases := &referenceUseCases{version: evidence.Version{FileID: referenceFile, Version: 2, Status: "available", FileName: "方案修订版.pdf", SizeBytes: 43, MediaType: "application/pdf", SHA256: strings.Repeat("b", 64)}}
	server, err := New(useCases, referenceAuthenticator{principal: tenantViewPrincipal()}, referenceProvider{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/references/files/"+referenceFile, nil))
	if response.Code != http.StatusOK || useCases.latestCalls != 1 || !strings.Contains(response.Body.String(), `"version":2`) || !strings.Contains(response.Body.String(), `"revision":2`) {
		t.Fatalf("status=%d latest_calls=%d body=%s", response.Code, useCases.latestCalls, response.Body.String())
	}
}

func TestVersionUploadRequiresNamedFileScope(t *testing.T) {
	useCases := &referenceUseCases{}
	server, err := New(useCases, referenceAuthenticator{principal: fileCapabilityPrincipal(referenceFile, auth.ActionUpload)}, referenceProvider{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	allowed := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/files/"+referenceFile+"/versions/upload-sessions", strings.NewReader(`{"file_name":"v2.pdf","size_bytes":2}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "version-create-1")
	server.ServeHTTP(allowed, request)
	if allowed.Code != http.StatusCreated || useCases.versionUploads != 1 {
		t.Fatalf("allowed status=%d calls=%d body=%s", allowed.Code, useCases.versionUploads, allowed.Body.String())
	}
	denied := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/v1/files/"+otherFile+"/versions/upload-sessions", strings.NewReader(`{"file_name":"v2.pdf","size_bytes":2}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "version-create-2")
	server.ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden || useCases.versionUploads != 1 || !strings.Contains(denied.Body.String(), "scope_denied") {
		t.Fatalf("denied status=%d calls=%d body=%s", denied.Code, useCases.versionUploads, denied.Body.String())
	}
}

func TestFileScopedCredentialOnlyReadsNamedFile(t *testing.T) {
	useCases := &referenceUseCases{version: evidence.Version{FileID: referenceFile, Version: 1, Status: "available", FileName: "方案.pdf"}}
	server, err := New(useCases, referenceAuthenticator{principal: fileViewPrincipal(referenceFile)}, referenceProvider{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	allowed := httptest.NewRecorder()
	server.ServeHTTP(allowed, httptest.NewRequest(http.MethodGet, "/v1/references/files/"+referenceFile+"/versions/1", nil))
	if allowed.Code != http.StatusOK || useCases.getCalls != 1 {
		t.Fatalf("allowed status=%d calls=%d body=%s", allowed.Code, useCases.getCalls, allowed.Body.String())
	}
	writtenBefore := useCases.getCalls
	denied := httptest.NewRecorder()
	server.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/v1/references/files/"+otherFile+"/versions/1", nil))
	if denied.Code != http.StatusForbidden || useCases.getCalls != writtenBefore || !strings.Contains(denied.Body.String(), "scope_denied") {
		t.Fatalf("denied status=%d calls=%d body=%s", denied.Code, useCases.getCalls, denied.Body.String())
	}
}

func TestFileScopedCredentialFiltersVersionList(t *testing.T) {
	useCases := &referenceUseCases{listResult: evidence.VersionPage{Items: []evidence.Version{
		{FileID: referenceFile, Version: 1, Status: "available", FileName: "可见.pdf"},
		{FileID: otherFile, Version: 1, Status: "available", FileName: "不可见.pdf"},
	}}}
	server, err := New(useCases, referenceAuthenticator{principal: fileViewPrincipal(referenceFile)}, referenceProvider{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/files", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "可见.pdf") || strings.Contains(response.Body.String(), "不可见.pdf") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMetadataAndDownloadCapabilitiesAreSeparatedWithLegacyCompatibility(t *testing.T) {
	version := evidence.Version{FileID: referenceFile, Version: 1, Status: "available", FileName: "方案.pdf"}
	tests := []struct {
		name               string
		principal          auth.Principal
		wantMetadataStatus int
		wantDownloadStatus int
		wantMetadataCalls  int
		wantDownloadCalls  int
	}{
		{"metadata only", fileCapabilityPrincipal(referenceFile, auth.ActionMetadataRead), http.StatusOK, http.StatusForbidden, 1, 0},
		{"download only", fileCapabilityPrincipal(referenceFile, auth.ActionDownload), http.StatusForbidden, http.StatusOK, 0, 1},
		{"legacy view", fileViewPrincipal(referenceFile), http.StatusOK, http.StatusOK, 1, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCases := &referenceUseCases{version: version}
			server, err := New(useCases, referenceAuthenticator{principal: test.principal}, referenceProvider{}, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			metadata := httptest.NewRecorder()
			server.ServeHTTP(metadata, httptest.NewRequest(http.MethodGet, "/v1/references/files/"+referenceFile+"/versions/1", nil))
			if metadata.Code != test.wantMetadataStatus {
				t.Fatalf("metadata status=%d body=%s", metadata.Code, metadata.Body.String())
			}
			download := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/files/"+referenceFile+"/versions/1/access-grants", strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			server.ServeHTTP(download, request)
			if download.Code != test.wantDownloadStatus || useCases.getCalls != test.wantMetadataCalls || useCases.accessCalls != test.wantDownloadCalls {
				t.Fatalf("download status=%d metadata_calls=%d access_calls=%d body=%s", download.Code, useCases.getCalls, useCases.accessCalls, download.Body.String())
			}
		})
	}
}
