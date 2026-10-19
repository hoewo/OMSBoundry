package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("SESSION_TTL", "")
	t.Setenv("SESSION_COOKIE_SECURE", "")
	t.Setenv("BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("BOOTSTRAP_ADMIN_DISPLAY_NAME", "")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "")
	t.Setenv("MIGRATION_MODE", "")
	t.Setenv("IDENTITY_PROVIDER", "")
	t.Setenv("IAM_ISSUER", "")
	t.Setenv("IAM_TENANT_ID", "")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "")
	t.Setenv("PUBLIC_BASE_PATH", "")
	t.Setenv("REFERENCE_TENANT_ID", "")
	t.Setenv("REFERENCE_ACCESS_TOKEN", "")
	t.Setenv("EVIDENCE_BASE_URL", "")
	t.Setenv("EVIDENCE_ACCESS_TOKEN", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.HTTPAddress != defaultHTTPAddress {
		t.Fatalf("HTTPAddress = %q, want %q", got.HTTPAddress, defaultHTTPAddress)
	}
	if got.DatabaseURL != defaultDatabaseURL {
		t.Fatalf("DatabaseURL = %q, want %q", got.DatabaseURL, defaultDatabaseURL)
	}
	if got.ShutdownPeriod != defaultShutdownPeriod {
		t.Fatalf("ShutdownPeriod = %s, want %s", got.ShutdownPeriod, defaultShutdownPeriod)
	}
	if got.SessionTTL != defaultSessionTTL || got.SessionCookieSecure {
		t.Fatalf("session defaults = %#v", got)
	}
	if got.MigrationMode != MigrationModeAuto {
		t.Fatalf("MigrationMode = %q, want %q", got.MigrationMode, MigrationModeAuto)
	}
	if got.IdentityProvider != IdentityProviderLocal || got.PublicBasePath != "" {
		t.Fatalf("identity defaults = %#v", got)
	}
}

func TestLoadIAMProvider(t *testing.T) {
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://oms.example.test/auth/")
	t.Setenv("IAM_TENANT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/run/oms/iam-public-keys.json")
	t.Setenv("PUBLIC_BASE_PATH", "/money")
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.IdentityProvider != IdentityProviderIAM || got.IAMIssuer != "https://oms.example.test/auth" || got.PublicBasePath != "/money" {
		t.Fatalf("IAM config=%#v", got)
	}
	t.Setenv("BOOTSTRAP_ADMIN_USERNAME", "admin")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "not-allowed")
	if _, err := Load(); err == nil {
		t.Fatal("IAM mode accepted local bootstrap")
	}
}

func TestLoadReferenceCredentialPair(t *testing.T) {
	t.Setenv("REFERENCE_TENANT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("REFERENCE_ACCESS_TOKEN", "local-reference-token-1234567890")
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ReferenceTenantID == "" || got.ReferenceAccessToken == "" {
		t.Fatalf("reference config = %#v", got)
	}

	t.Setenv("REFERENCE_TENANT_ID", "")
	if _, err := Load(); err == nil {
		t.Fatal("partial reference credential accepted")
	}
}

func TestLoadEvidenceConfiguration(t *testing.T) {
	t.Setenv("EVIDENCE_BASE_URL", "https://evidence.example.test/")
	t.Setenv("EVIDENCE_ACCESS_TOKEN", "local-evidence-token-1234567890")
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.EvidenceBaseURL != "https://evidence.example.test" || got.EvidenceAccessToken == "" {
		t.Fatalf("Evidence config = %#v", got)
	}

	t.Setenv("EVIDENCE_ACCESS_TOKEN", "")
	if _, err := Load(); err == nil {
		t.Fatal("partial local Evidence configuration accepted")
	}
}

func TestLoadIAMEvidenceConfiguration(t *testing.T) {
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://oms.example.test/auth")
	t.Setenv("IAM_TENANT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/run/oms/iam-public-keys.json")
	t.Setenv("EVIDENCE_BASE_URL", "http://evidence-service:8085")
	t.Setenv("EVIDENCE_ACCESS_TOKEN", "")
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.EvidenceBaseURL != "http://evidence-service:8085" {
		t.Fatalf("EvidenceBaseURL = %q", got.EvidenceBaseURL)
	}

	t.Setenv("EVIDENCE_ACCESS_TOKEN", "static-is-not-allowed-in-iam")
	if _, err := Load(); err == nil {
		t.Fatal("IAM mode accepted a static Evidence token")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":19082")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("SESSION_TTL", "2h")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("BOOTSTRAP_ADMIN_USERNAME", " admin ")
	t.Setenv("BOOTSTRAP_ADMIN_DISPLAY_NAME", " 资金管理员 ")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "example-password")
	t.Setenv("MIGRATION_MODE", " VERIFY ")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.HTTPAddress != ":19082" || got.DatabaseURL != "postgres://example" || got.ShutdownPeriod != 3*time.Second ||
		got.SessionTTL != 2*time.Hour || !got.SessionCookieSecure || got.BootstrapAdminUsername != "admin" ||
		got.BootstrapAdminDisplayName != "资金管理员" || got.BootstrapAdminPassword != "example-password" ||
		got.MigrationMode != MigrationModeVerify {
		t.Fatalf("Load() = %#v", got)
	}
}

func TestLoadRejectsInvalidMigrationMode(t *testing.T) {
	t.Setenv("MIGRATION_MODE", "force")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want migration mode validation error")
	}
}

func TestLoadRejectsInvalidShutdownTimeout(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "never")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}

func TestLoadRejectsInvalidIdentityConfiguration(t *testing.T) {
	tests := []struct {
		name string
		set  func(*testing.T)
	}{
		{name: "invalid session ttl", set: func(t *testing.T) { t.Setenv("SESSION_TTL", "never") }},
		{name: "non-positive session ttl", set: func(t *testing.T) { t.Setenv("SESSION_TTL", "0s") }},
		{name: "invalid secure flag", set: func(t *testing.T) { t.Setenv("SESSION_COOKIE_SECURE", "sometimes") }},
		{name: "username only", set: func(t *testing.T) { t.Setenv("BOOTSTRAP_ADMIN_USERNAME", "admin") }},
		{name: "password only", set: func(t *testing.T) { t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "example-password") }},
		{name: "display only", set: func(t *testing.T) { t.Setenv("BOOTSTRAP_ADMIN_DISPLAY_NAME", "管理员") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("SESSION_TTL", "")
			t.Setenv("SESSION_COOKIE_SECURE", "")
			t.Setenv("BOOTSTRAP_ADMIN_USERNAME", "")
			t.Setenv("BOOTSTRAP_ADMIN_DISPLAY_NAME", "")
			t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "")
			test.set(t)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
		})
	}
}
