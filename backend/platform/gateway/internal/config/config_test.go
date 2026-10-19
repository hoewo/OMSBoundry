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
