package storage

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	sts "github.com/alibabacloud-go/sts-20150401/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	osscredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

func TestServiceSDKReadsOriginalTextRepresentationForIntegrity(t *testing.T) {
	content := strings.Repeat("Auth58 stored text bytes\n", 128)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if r.Header.Get("Accept-Encoding") != "identity" {
			w.Header().Set("Content-Encoding", "gzip")
			compressed := gzip.NewWriter(w)
			_, _ = io.WriteString(compressed, content)
			_ = compressed.Close()
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		_, _ = io.WriteString(w, content)
	}))
	defer server.Close()
	credentials := osscredentials.NewStaticCredentialsProvider("fixture-id", "fixture-secret")
	options := AliyunOSSOptions{Region: "cn-hangzhou", ServiceEndpoint: server.URL, Bucket: "fixture-bucket"}
	client := oss.NewClient(serviceOSSConfig(options, credentials).WithHttpClient(server.Client()).WithUsePathStyle(true))
	provider := &AliyunOSSProvider{options: options, serviceOSS: client}
	object, err := provider.Open(context.Background(), ObjectRef{Provider: "aliyun-oss", Bucket: options.Bucket, ObjectKey: "tenants/fixture/text.txt"})
	if err != nil {
		t.Fatal(err)
	}
	defer object.Reader.Close()
	actual, err := io.ReadAll(object.Reader)
	if err != nil || string(actual) != content || object.Metadata.SizeBytes != int64(len(content)) {
		t.Fatalf("stored representation changed: length=%d, metadata=%d, err=%v", len(actual), object.Metadata.SizeBytes, err)
	}
}

func TestAliyunUploadPolicyIsExactObjectAndMinimalActions(t *testing.T) {
	policy, err := AliyunUploadPolicy("private-bucket", "tenants/t/files/f/versions/1/s")
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Version   string `json:"Version"`
		Statement []struct {
			Effect   string   `json:"Effect"`
			Action   []string `json:"Action"`
			Resource []string `json:"Resource"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(policy), &value); err != nil {
		t.Fatal(err)
	}
	if value.Version != "1" || len(value.Statement) != 1 {
		t.Fatalf("unexpected policy: %s", policy)
	}
	statement := value.Statement[0]
	if statement.Effect != "Allow" || !reflect.DeepEqual(statement.Action, AliyunMultipartActions) || !reflect.DeepEqual(statement.Resource, []string{"acs:oss:*:*:private-bucket/tenants/t/files/f/versions/1/s"}) {
		t.Fatalf("policy is not minimal: %s", policy)
	}
	for _, forbidden := range []string{"ListObjects", "DeleteObject", "GetObject", "InitiateMultipartUpload", "UploadPart", "CompleteMultipartUpload"} {
		if strings.Contains(policy, forbidden) {
			t.Fatalf("policy contains forbidden action %s: %s", forbidden, policy)
		}
	}
}

func TestAliyunUploadPolicyRejectsUnsafeKey(t *testing.T) {
	for _, key := range []string{"", "/absolute", "safe/../other"} {
		if _, err := AliyunUploadPolicy("bucket", key); err == nil {
			t.Fatalf("accepted unsafe key %q", key)
		}
	}
}

type fakeSTS struct{ request *sts.AssumeRoleRequest }

func (f *fakeSTS) AssumeRoleWithOptions(request *sts.AssumeRoleRequest, _ *util.RuntimeOptions) (*sts.AssumeRoleResponse, error) {
	f.request = request
	expiration := time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339)
	return &sts.AssumeRoleResponse{Body: &sts.AssumeRoleResponseBody{Credentials: &sts.AssumeRoleResponseBodyCredentials{AccessKeyId: stringPointer("STS.test"), AccessKeySecret: stringPointer("temporary-secret"), SecurityToken: stringPointer("temporary-token"), Expiration: &expiration}}}, nil
}

func TestAliyunAuthorizationUsesBoundPolicyAndTemporaryCredentials(t *testing.T) {
	assumer := &fakeSTS{}
	provider := &AliyunOSSProvider{options: AliyunOSSOptions{Region: "cn-beijing", BrowserEndpoint: "https://oss-cn-beijing.aliyuncs.com", ServiceEndpoint: "https://oss-cn-beijing-internal.aliyuncs.com", Bucket: "private-bucket", RoleARN: "acs:ram::1234567890123456:role/evidence-upload", STSDuration: 15 * time.Minute, AccessTTL: 5 * time.Minute}, sts: assumer, now: func() time.Time { return time.Now().UTC() }}
	intent := UploadIntent{SessionID: "11111111-1111-4111-8111-111111111111", MediaType: "application/pdf", ExpiresAt: time.Now().UTC().Add(15 * time.Minute), Object: ObjectRef{Provider: "aliyun-oss", Bucket: "private-bucket", ObjectKey: "tenants/t/files/f/versions/1/s"}}
	authorization, err := provider.AuthorizeUpload(context.Background(), intent)
	if err != nil {
		t.Fatal(err)
	}
	if authorization.AccessKeyID != "STS.test" || authorization.SecurityToken != "temporary-token" || authorization.AccessKeySecret == "" {
		t.Fatalf("unexpected STS authorization: %+v", authorization)
	}
	if assumer.request == nil || assumer.request.Policy == nil {
		t.Fatal("AssumeRole policy was not provided")
	}
	expectedPolicy, err := AliyunUploadPolicy("private-bucket", intent.Object.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if *assumer.request.Policy != expectedPolicy {
		t.Fatalf("unexpected policy %s", *assumer.request.Policy)
	}
	if !reflect.DeepEqual(authorization.Scope.AllowedActions, AliyunMultipartActions) {
		t.Fatalf("unexpected scope: %+v", authorization.Scope)
	}
	if authorization.Endpoint != "https://oss-cn-beijing.aliyuncs.com" {
		t.Fatalf("browser received non-public endpoint %q", authorization.Endpoint)
	}
}

type recordingOSSClient struct {
	getCalls     int
	deleteCalls  int
	presignCalls int
}

func (c *recordingOSSClient) GetObject(context.Context, *oss.GetObjectRequest, ...func(*oss.Options)) (*oss.GetObjectResult, error) {
	c.getCalls++
	return &oss.GetObjectResult{Body: io.NopCloser(strings.NewReader("x")), ContentLength: 1}, nil
}

func (c *recordingOSSClient) DeleteObject(context.Context, *oss.DeleteObjectRequest, ...func(*oss.Options)) (*oss.DeleteObjectResult, error) {
	c.deleteCalls++
	return &oss.DeleteObjectResult{}, nil
}

func (c *recordingOSSClient) Presign(context.Context, any, ...func(*oss.PresignOptions)) (*oss.PresignResult, error) {
	c.presignCalls++
	return &oss.PresignResult{URL: "https://private-bucket.oss-cn-beijing.aliyuncs.com/signed", Expiration: time.Now().UTC().Add(5 * time.Minute)}, nil
}

func TestAliyunProviderKeepsServiceAndBrowserOSSPathsSeparate(t *testing.T) {
	serviceClient := &recordingOSSClient{}
	browserClient := &recordingOSSClient{}
	provider := &AliyunOSSProvider{
		options:    AliyunOSSOptions{Bucket: "private-bucket", AccessTTL: 5 * time.Minute},
		serviceOSS: serviceClient,
		browserOSS: browserClient,
		now:        func() time.Time { return time.Now().UTC() },
	}
	ref := ObjectRef{Provider: "aliyun-oss", Bucket: "private-bucket", ObjectKey: "tenants/t/files/f/versions/1/s"}
	object, err := provider.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := object.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.AuthorizeAccess(context.Background(), AccessIntent{Object: ref, Disposition: "attachment", FileName: "evidence.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := provider.Delete(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	if serviceClient.getCalls != 1 || serviceClient.deleteCalls != 1 || serviceClient.presignCalls != 0 {
		t.Fatalf("unexpected service client calls: %+v", serviceClient)
	}
	if browserClient.getCalls != 0 || browserClient.deleteCalls != 0 || browserClient.presignCalls != 1 {
		t.Fatalf("unexpected browser client calls: %+v", browserClient)
	}
}

func stringPointer(value string) *string { return &value }

func TestBrowserSDKPresignUsesExactCustomDomainWithoutBucketPrefix(t *testing.T) {
	credentials := osscredentials.NewStaticCredentialsProvider("fixture-id", "fixture-secret", "fixture-token")
	for _, cname := range []bool{false, true} {
		opts := AliyunOSSOptions{Region: "cn-hangzhou", BrowserEndpoint: "https://files.example.test", BrowserCName: cname}
		client := oss.NewClient(browserOSSConfig(opts, credentials))
		result, err := client.Presign(context.Background(), &oss.GetObjectRequest{Bucket: oss.Ptr("test-bucket"), Key: oss.Ptr("tenants/t/file")})
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(result.URL)
		if err != nil {
			t.Fatal(err)
		}
		expected := "test-bucket.files.example.test"
		if cname {
			expected = "files.example.test"
		}
		if u.Host != expected || u.Path != "/tenants/t/file" {
			t.Fatal("SDK CNAME addressing is incorrect")
		}
	}
}
