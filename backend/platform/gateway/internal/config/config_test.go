package config

import "testing"

func TestLoadTrustProxyClientIP(t *testing.T) {
	t.Setenv("SUITE_MANIFEST_PATH", "/config/suite.json")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/public/keys.json")
	t.Setenv("IAM_ISSUER", "https://oms.example.test/auth")
	t.Setenv("TRUST_PROXY_CLIENT_IP", "true")

	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !value.TrustProxyClientIP {
		t.Fatal("trusted proxy client IP mode was not enabled")
	}
}

func TestLoadRejectsInvalidTrustProxyClientIP(t *testing.T) {
	t.Setenv("SUITE_MANIFEST_PATH", "/config/suite.json")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/public/keys.json")
	t.Setenv("IAM_ISSUER", "https://oms.example.test/auth")
	t.Setenv("TRUST_PROXY_CLIENT_IP", "sometimes")

	if _, err := Load(); err == nil {
		t.Fatal("invalid TRUST_PROXY_CLIENT_IP was accepted")
	}
}

func TestLoadOpenAPIDefaultsAndOverrides(t *testing.T) {
	t.Setenv("SUITE_MANIFEST_PATH", "/config/suite.json")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/public/keys.json")
	t.Setenv("IAM_ISSUER", "https://oms.example.test/auth")
	value, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if value.OpenAPIKeyRequestsPerMinute != 60 || value.OpenAPITenantRequestsPerMinute != 600 || !value.OpenAPIRequireHTTPS {
		t.Fatalf("open API defaults = %#v", value)
	}
	t.Setenv("OPEN_API_KEY_REQUESTS_PER_MINUTE", "75")
	t.Setenv("OPEN_API_TENANT_REQUESTS_PER_MINUTE", "900")
	t.Setenv("OPEN_API_REQUIRE_HTTPS", "false")
	value, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if value.OpenAPIKeyRequestsPerMinute != 75 || value.OpenAPITenantRequestsPerMinute != 900 || value.OpenAPIRequireHTTPS {
		t.Fatalf("open API overrides = %#v", value)
	}
}

func TestLoadRejectsOpenAPITenantLimitBelowDefaultKeyOverride(t *testing.T) {
	t.Setenv("SUITE_MANIFEST_PATH", "/config/suite.json")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/public/keys.json")
	t.Setenv("IAM_ISSUER", "https://oms.example.test/auth")
	t.Setenv("OPEN_API_KEY_REQUESTS_PER_MINUTE", "700")
	if _, err := Load(); err == nil {
		t.Fatal("per-Key limit above the tenant limit was accepted")
	}
}
