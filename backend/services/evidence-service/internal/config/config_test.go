package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadRequiresAuthenticatedLocalConfiguration(t *testing.T) {
	setValidTemplate(t)
	t.Setenv("LOCAL_SIGNING_KEY", base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	t.Setenv("LOCAL_STORAGE_ROOT", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorageProvider != StorageLocal || len(cfg.LocalSigningKey) != 32 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing template credential was accepted")
	}
}

func TestLoadRejectsUnsupportedPublicBaseURLScheme(t *testing.T) {
	setValidTemplate(t)
	t.Setenv("LOCAL_SIGNING_KEY", base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	t.Setenv("LOCAL_STORAGE_ROOT", t.TempDir())
	t.Setenv("PUBLIC_BASE_URL", "ftp://files.example.test")
	if _, err := Load(); err == nil {
		t.Fatal("unsupported PUBLIC_BASE_URL scheme was accepted")
	}
}

func TestLoadSeparatesAliyunAndLocalSecrets(t *testing.T) {
	setValidTemplate(t)
	t.Setenv("STORAGE_PROVIDER", "aliyun-oss")
	t.Setenv("OSS_REGION", "cn-beijing")
	t.Setenv("OSS_BUCKET", "private-bucket")
	t.Setenv("OSS_ROLE_ARN", "acs:ram::1234567890123456:role/evidence-upload")
	t.Setenv("LOCAL_SIGNING_KEY", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorageProvider != StorageAliyunOSS {
		t.Fatalf("unexpected provider %q", cfg.StorageProvider)
	}
	t.Setenv("LOCAL_SIGNING_KEY", base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	if _, err := Load(); err == nil {
		t.Fatal("local signing key was accepted in OSS mode")
	}
}

func TestLoadSeparatesBrowserAndServiceOSSEndpoints(t *testing.T) {
	setValidTemplate(t)
	t.Setenv("STORAGE_PROVIDER", "aliyun-oss")
	t.Setenv("OSS_REGION", "cn-beijing")
	t.Setenv("OSS_BUCKET", "private-bucket")
	t.Setenv("OSS_ROLE_ARN", "acs:ram::1234567890123456:role/evidence-upload")
	t.Setenv("OSS_BROWSER_ENDPOINT", "https://oss-cn-beijing.aliyuncs.com")
	t.Setenv("OSS_SERVICE_ENDPOINT", "https://oss-cn-beijing-internal.aliyuncs.com")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OSSBrowserEndpoint != "https://oss-cn-beijing.aliyuncs.com" || cfg.OSSServiceEndpoint != "https://oss-cn-beijing-internal.aliyuncs.com" {
		t.Fatalf("unexpected endpoints: browser=%q service=%q", cfg.OSSBrowserEndpoint, cfg.OSSServiceEndpoint)
	}
}

func TestLoadSupportsLegacyOSSEndpointAsFallback(t *testing.T) {
	setValidTemplate(t)
	t.Setenv("STORAGE_PROVIDER", "aliyun-oss")
	t.Setenv("OSS_REGION", "cn-beijing")
	t.Setenv("OSS_BUCKET", "private-bucket")
	t.Setenv("OSS_ROLE_ARN", "acs:ram::1234567890123456:role/evidence-upload")
	t.Setenv("OSS_ENDPOINT", "https://oss-cn-beijing.aliyuncs.com")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OSSBrowserEndpoint != cfg.OSSServiceEndpoint || cfg.OSSBrowserEndpoint != "https://oss-cn-beijing.aliyuncs.com" {
		t.Fatalf("legacy endpoint fallback failed: browser=%q service=%q", cfg.OSSBrowserEndpoint, cfg.OSSServiceEndpoint)
	}
}

func setValidTemplate(t *testing.T) {
	t.Helper()
	t.Setenv("IDENTITY_PROVIDER", "template")
	t.Setenv("TEMPLATE_TENANT_ID", "33333333-3333-4333-8333-333333333333")
	t.Setenv("TEMPLATE_SUBJECT_ID", "44444444-4444-4444-8444-444444444444")
	t.Setenv("TEMPLATE_DISPLAY_NAME", "Test Admin")
	t.Setenv("TEMPLATE_ACCESS_TOKEN", "this-is-a-replaced-template-token")
	t.Setenv("IAM_ISSUER", "")
	t.Setenv("IAM_TENANT_ID", "")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "")
	t.Setenv("OSS_REGION", "")
	t.Setenv("OSS_ENDPOINT", "")
	t.Setenv("OSS_BROWSER_ENDPOINT", "")
	t.Setenv("OSS_SERVICE_ENDPOINT", "")
	t.Setenv("OSS_BUCKET", "")
	t.Setenv("OSS_ROLE_ARN", "")
	t.Setenv("OSS_BROWSER_CNAME", "")
	t.Setenv("STORAGE_PROVIDER", "local")
	t.Setenv("PRIMARY_STORAGE_PROVIDER", "")
	t.Setenv("LOCAL_SIGNING_KEY", "")
}

func TestCNameRequiresExplicitHTTPSOriginAndBoolean(t *testing.T) {
	setValidTemplate(t)
	t.Setenv("STORAGE_PROVIDER", "aliyun-oss")
	t.Setenv("OSS_REGION", "cn-hangzhou")
	t.Setenv("OSS_BUCKET", "private-bucket")
	t.Setenv("OSS_ROLE_ARN", "acs:ram::1234567890123456:role/evidence-upload")
	t.Setenv("OSS_BROWSER_CNAME", "true")
	for _, endpoint := range []string{"", "http://files.test", "https://files.test/path", "https://files.test?token=x", "https://user:pass@files.test"} {
		t.Setenv("OSS_BROWSER_ENDPOINT", endpoint)
		if _, err := Load(); err == nil {
			t.Fatal("unsafe or implicit CNAME endpoint accepted")
		}
	}
	t.Setenv("OSS_BROWSER_ENDPOINT", "https://files.example.test")
	cfg, err := Load()
	if err != nil || !cfg.OSSBrowserCName {
		t.Fatalf("CNAME config: %v", err)
	}
	t.Setenv("OSS_BROWSER_CNAME", "invalid")
	if _, err := Load(); err == nil {
		t.Fatal("invalid CNAME boolean accepted")
	}
}

func TestHybridRequiresBothConfigurationsAndExplicitPrimary(t *testing.T) {
	setValidTemplate(t)
	t.Setenv("STORAGE_PROVIDER", "hybrid")
	t.Setenv("LOCAL_STORAGE_ROOT", t.TempDir())
	t.Setenv("LOCAL_SIGNING_KEY", base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	t.Setenv("OSS_REGION", "cn-hangzhou")
	t.Setenv("OSS_BUCKET", "private-bucket")
	t.Setenv("OSS_ROLE_ARN", "acs:ram::1234567890123456:role/evidence-upload")
	if _, err := Load(); err == nil {
		t.Fatal("implicit primary accepted")
	}
	for _, primary := range []string{"local", "aliyun-oss"} {
		t.Setenv("PRIMARY_STORAGE_PROVIDER", primary)
		cfg, err := Load()
		if err != nil || string(cfg.PrimaryStorageProvider) != primary {
			t.Fatalf("hybrid config: %v", err)
		}
	}
	t.Setenv("OSS_BUCKET", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing OSS configuration accepted")
	}
	t.Setenv("OSS_BUCKET", "private-bucket")
	t.Setenv("LOCAL_SIGNING_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing local key accepted")
	}
	t.Setenv("STORAGE_PROVIDER", "aliyun-oss")
	if _, err := Load(); err == nil {
		t.Fatal("primary setting accepted outside hybrid mode")
	}
}
