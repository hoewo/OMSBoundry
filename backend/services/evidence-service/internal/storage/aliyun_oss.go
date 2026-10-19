package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sts "github.com/alibabacloud-go/sts-20150401/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	osscredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	cloudcredentials "github.com/aliyun/credentials-go/credentials"
)

var AliyunMultipartActions = []string{"oss:PutObject", "oss:AbortMultipartUpload", "oss:ListParts"}

type AliyunOSSOptions struct {
	Region          string
	BrowserEndpoint string
	BrowserCName    bool
	ServiceEndpoint string
	Bucket          string
	RoleARN         string
	STSEndpoint     string
	STSDuration     time.Duration
	AccessTTL       time.Duration
}

type stsAssumer interface {
	AssumeRoleWithOptions(*sts.AssumeRoleRequest, *util.RuntimeOptions) (*sts.AssumeRoleResponse, error)
}
type ossClient interface {
	GetObject(context.Context, *oss.GetObjectRequest, ...func(*oss.Options)) (*oss.GetObjectResult, error)
	DeleteObject(context.Context, *oss.DeleteObjectRequest, ...func(*oss.Options)) (*oss.DeleteObjectResult, error)
	Presign(context.Context, any, ...func(*oss.PresignOptions)) (*oss.PresignResult, error)
}

type AliyunOSSProvider struct {
	options     AliyunOSSOptions
	credentials cloudcredentials.Credential
	sts         stsAssumer
	serviceOSS  ossClient
	browserOSS  ossClient
	now         func() time.Time
}

func NewAliyunOSSProvider(options AliyunOSSOptions) (*AliyunOSSProvider, error) {
	options.Region = strings.TrimSpace(options.Region)
	options.BrowserEndpoint = strings.TrimRight(strings.TrimSpace(options.BrowserEndpoint), "/")
	options.ServiceEndpoint = strings.TrimRight(strings.TrimSpace(options.ServiceEndpoint), "/")
	options.Bucket = strings.TrimSpace(options.Bucket)
	options.RoleARN = strings.TrimSpace(options.RoleARN)
	options.STSEndpoint = strings.TrimSpace(options.STSEndpoint)
	if options.Region == "" || options.Bucket == "" || options.RoleARN == "" || options.STSDuration < 15*time.Minute || options.STSDuration > time.Hour || options.AccessTTL <= 0 {
		return nil, fmt.Errorf("Aliyun OSS configuration is invalid")
	}
	if options.BrowserEndpoint == "" {
		options.BrowserEndpoint = "https://oss-" + options.Region + ".aliyuncs.com"
	}
	if options.ServiceEndpoint == "" {
		options.ServiceEndpoint = options.BrowserEndpoint
	}
	if !strings.HasPrefix(options.BrowserEndpoint, "https://") || !strings.HasPrefix(options.ServiceEndpoint, "https://") {
		return nil, fmt.Errorf("Aliyun OSS endpoints must use HTTPS")
	}
	credential, err := cloudcredentials.NewCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create Alibaba Cloud credential chain: %w", err)
	}
	stsConfig := (&openapi.Config{}).SetCredential(credential).SetRegionId(options.Region)
	if options.STSEndpoint != "" {
		stsConfig.SetEndpoint(options.STSEndpoint)
	}
	stsClient, err := sts.NewClient(stsConfig)
	if err != nil {
		return nil, fmt.Errorf("create STS client: %w", err)
	}
	provider := osscredentials.CredentialsProviderFunc(func(_ context.Context) (osscredentials.Credentials, error) {
		value, err := credential.GetCredential()
		if err != nil {
			return osscredentials.Credentials{}, err
		}
		result := osscredentials.Credentials{}
		if value.AccessKeyId != nil {
			result.AccessKeyID = *value.AccessKeyId
		}
		if value.AccessKeySecret != nil {
			result.AccessKeySecret = *value.AccessKeySecret
		}
		if value.SecurityToken != nil {
			result.SecurityToken = *value.SecurityToken
		}
		return result, nil
	})
	serviceConfig := serviceOSSConfig(options, provider)
	browserConfig := browserOSSConfig(options, provider)
	return &AliyunOSSProvider{options: options, credentials: credential, sts: stsClient, serviceOSS: oss.NewClient(serviceConfig), browserOSS: oss.NewClient(browserConfig), now: func() time.Time { return time.Now().UTC() }}, nil
}
func serviceOSSConfig(options AliyunOSSOptions, provider osscredentials.CredentialsProvider) *oss.Config {
	// Integrity checks compare the stored object's length and SHA-256. OSS can
	// gzip text responses; Go then transparently inflates them and removes the
	// Content-Length header. Request the original representation explicitly.
	return oss.LoadDefaultConfig().WithRegion(options.Region).WithEndpoint(options.ServiceEndpoint).WithCredentialsProvider(provider).WithRetryMaxAttempts(3).WithDefaultRequestHeaders(map[string]string{"Accept-Encoding": "identity"})
}
func browserOSSConfig(options AliyunOSSOptions, provider osscredentials.CredentialsProvider) *oss.Config {
	return oss.LoadDefaultConfig().WithRegion(options.Region).WithEndpoint(options.BrowserEndpoint).WithUseCName(options.BrowserCName).WithCredentialsProvider(provider).WithRetryMaxAttempts(3)
}
func (p *AliyunOSSProvider) Name() string       { return "aliyun-oss" }
func (p *AliyunOSSProvider) BucketName() string { return p.options.Bucket }
func (p *AliyunOSSProvider) Ready(_ context.Context) error {
	value, err := p.credentials.GetCredential()
	if err != nil {
		return err
	}
	if value.AccessKeyId == nil || value.AccessKeySecret == nil || *value.AccessKeyId == "" || *value.AccessKeySecret == "" {
		return fmt.Errorf("Alibaba Cloud credentials are unavailable")
	}
	return nil
}
func (p *AliyunOSSProvider) AuthorizeUpload(_ context.Context, intent UploadIntent) (UploadAuthorization, error) {
	if intent.Object.Provider != p.Name() || intent.Object.Bucket != p.options.Bucket || intent.Object.ObjectKey == "" || !intent.ExpiresAt.After(p.now()) {
		return UploadAuthorization{}, ErrInvalidAuthorization
	}
	policy, err := AliyunUploadPolicy(p.options.Bucket, intent.Object.ObjectKey)
	if err != nil {
		return UploadAuthorization{}, err
	}
	duration := p.options.STSDuration
	if remaining := intent.ExpiresAt.Sub(p.now()); remaining < duration {
		duration = remaining
	}
	if duration < 15*time.Minute {
		duration = 15 * time.Minute
	}
	seconds := int64(duration / time.Second)
	request := (&sts.AssumeRoleRequest{}).SetRoleArn(p.options.RoleARN).SetRoleSessionName(roleSessionName(intent.SessionID)).SetPolicy(policy).SetDurationSeconds(seconds)
	runtime := (&util.RuntimeOptions{}).SetAutoretry(true).SetMaxAttempts(3).SetConnectTimeout(5000).SetReadTimeout(10000)
	response, err := p.sts.AssumeRoleWithOptions(request, runtime)
	if err != nil {
		return UploadAuthorization{}, fmt.Errorf("assume upload role: %w", err)
	}
	if response == nil || response.Body == nil || response.Body.Credentials == nil {
		return UploadAuthorization{}, fmt.Errorf("STS response is missing credentials")
	}
	credentials := response.Body.Credentials
	if credentials.AccessKeyId == nil || credentials.AccessKeySecret == nil || credentials.SecurityToken == nil || credentials.Expiration == nil {
		return UploadAuthorization{}, fmt.Errorf("STS response credentials are incomplete")
	}
	expires, err := time.Parse(time.RFC3339, *credentials.Expiration)
	if err != nil {
		return UploadAuthorization{}, fmt.Errorf("parse STS expiration: %w", err)
	}
	return UploadAuthorization{Provider: "aliyun-oss", Region: p.options.Region, Endpoint: p.options.BrowserEndpoint, EndpointIsCName: p.options.BrowserCName, Bucket: p.options.Bucket, ObjectKey: intent.Object.ObjectKey, AccessKeyID: *credentials.AccessKeyId, AccessKeySecret: *credentials.AccessKeySecret, SecurityToken: *credentials.SecurityToken, ExpiresAt: expires, Multipart: &MultipartOptions{ThresholdBytes: 20 * 1024 * 1024, PartSizeBytes: 8 * 1024 * 1024, Parallel: 3}, Scope: AuthorizationScope{Bucket: p.options.Bucket, ObjectKey: intent.Object.ObjectKey, AllowedActions: append([]string(nil), AliyunMultipartActions...)}, Headers: map[string]string{"x-oss-forbid-overwrite": "true", "Content-Type": normalizedMediaType(intent.MediaType)}}, nil
}
func (p *AliyunOSSProvider) Open(ctx context.Context, ref ObjectRef) (ObjectReader, error) {
	if err := p.validateRef(ref); err != nil {
		return ObjectReader{}, err
	}
	result, err := p.serviceOSS.GetObject(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(p.options.Bucket), Key: oss.Ptr(ref.ObjectKey)})
	if err != nil {
		if isOSSNotFound(err) {
			return ObjectReader{}, ErrNotFound
		}
		return ObjectReader{}, err
	}
	modified := time.Time{}
	if result.LastModified != nil {
		modified = result.LastModified.UTC()
	}
	etag := ""
	if result.ETag != nil {
		etag = strings.Trim(*result.ETag, "\"")
	}
	return ObjectReader{Reader: result.Body, Metadata: ObjectMetadata{SizeBytes: result.ContentLength, ETag: etag, LastModified: modified}}, nil
}
func (p *AliyunOSSProvider) AuthorizeAccess(ctx context.Context, intent AccessIntent) (AccessGrant, error) {
	if err := p.validateRef(intent.Object); err != nil {
		return AccessGrant{}, err
	}
	disposition := mime.FormatMediaType(intent.Disposition, map[string]string{"filename": intent.FileName})
	request := &oss.GetObjectRequest{Bucket: oss.Ptr(p.options.Bucket), Key: oss.Ptr(intent.Object.ObjectKey), ResponseContentDisposition: oss.Ptr(disposition)}
	result, err := p.browserOSS.Presign(ctx, request, func(options *oss.PresignOptions) { options.Expires = p.options.AccessTTL })
	if err != nil {
		return AccessGrant{}, err
	}
	expires := result.Expiration
	if expires.IsZero() {
		expires = p.now().Add(p.options.AccessTTL)
	}
	return AccessGrant{URL: result.URL, ExpiresAt: expires, RefreshAfter: expires.Add(-minDuration(time.Minute, p.options.AccessTTL/2))}, nil
}
func (p *AliyunOSSProvider) Delete(ctx context.Context, ref ObjectRef) error {
	if err := p.validateRef(ref); err != nil {
		return err
	}
	_, err := p.serviceOSS.DeleteObject(ctx, &oss.DeleteObjectRequest{Bucket: oss.Ptr(p.options.Bucket), Key: oss.Ptr(ref.ObjectKey)})
	if isOSSNotFound(err) {
		return nil
	}
	return err
}
func (p *AliyunOSSProvider) validateRef(ref ObjectRef) error {
	if ref.Provider != p.Name() || ref.Bucket != p.options.Bucket || ref.ObjectKey == "" || strings.HasPrefix(ref.ObjectKey, "/") || strings.Contains(ref.ObjectKey, "..") {
		return ErrInvalidAuthorization
	}
	return nil
}

func AliyunUploadPolicy(bucket, objectKey string) (string, error) {
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(objectKey) == "" || strings.HasPrefix(objectKey, "/") || strings.Contains(objectKey, "..") {
		return "", ErrInvalidAuthorization
	}
	value := struct {
		Version   string `json:"Version"`
		Statement []struct {
			Effect   string   `json:"Effect"`
			Action   []string `json:"Action"`
			Resource []string `json:"Resource"`
		} `json:"Statement"`
	}{Version: "1"}
	value.Statement = append(value.Statement, struct {
		Effect   string   `json:"Effect"`
		Action   []string `json:"Action"`
		Resource []string `json:"Resource"`
	}{Effect: "Allow", Action: append([]string(nil), AliyunMultipartActions...), Resource: []string{"acs:oss:*:*:" + bucket + "/" + objectKey}})
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if len(data) > 2048 {
		return "", fmt.Errorf("upload policy exceeds STS limit")
	}
	return string(data), nil
}
func roleSessionName(sessionID string) string {
	value := "evidence-" + strings.ReplaceAll(sessionID, "_", "-")
	if len(value) > 64 {
		value = value[:64]
	}
	if len(value) < 2 {
		return "evidence-upload"
	}
	return value
}
func isOSSNotFound(err error) bool {
	if err == nil {
		return false
	}
	var service *oss.ServiceError
	return errors.As(err, &service) && (service.StatusCode == 404 || service.Code == "NoSuchKey")
}
