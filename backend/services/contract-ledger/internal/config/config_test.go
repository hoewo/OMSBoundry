package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, name := range []string{"EXTERNAL_EVIDENCE_BASE_URL", "EXTERNAL_EVIDENCE_ACCESS_TOKEN", "IAM_SERVICE_CREDENTIAL_URL", "WORKLOAD_ID", "WORKLOAD_SUBJECT_ID", "WORKLOAD_KEY_FILE", "REFERENCE_TIMEOUT"} {
		t.Setenv(name, "")
	}
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("SESSION_TTL", "")
	t.Setenv("SESSION_COOKIE_SECURE", "")
	t.Setenv("BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "")
	t.Setenv("BOOTSTRAP_ADMIN_DISPLAY_NAME", "")
	t.Setenv("EVIDENCE_STORAGE_DIR", "")
	t.Setenv("EVIDENCE_MAX_UPLOAD_BYTES", "")
	t.Setenv("MIGRATION_MODE", "")
	t.Setenv("IDENTITY_PROVIDER", "")
	t.Setenv("IAM_ISSUER", "")
	t.Setenv("IAM_TENANT_ID", "")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "")
	t.Setenv("PUBLIC_BASE_PATH", "")
	t.Setenv("REFERENCE_TENANT_ID", "")
	t.Setenv("REFERENCE_ACCESS_TOKEN", "")

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
	if got.EvidenceStorageDirectory != defaultEvidenceStorageDirectory || got.EvidenceMaxUploadBytes != defaultEvidenceMaxUploadBytes {
		t.Fatalf("evidence defaults = %#v", got)
	}
	if got.MigrationMode != MigrationModeAuto {
		t.Fatalf("migration default = %q", got.MigrationMode)
	}
}

func TestExternalEvidenceOptionalAndIAMWorkloadConfiguration(t *testing.T) {
	for _, name := range []string{"EXTERNAL_EVIDENCE_BASE_URL", "EXTERNAL_EVIDENCE_ACCESS_TOKEN", "IAM_SERVICE_CREDENTIAL_URL", "WORKLOAD_ID", "WORKLOAD_SUBJECT_ID", "WORKLOAD_KEY_FILE", "BOOTSTRAP_ADMIN_USERNAME", "BOOTSTRAP_ADMIN_PASSWORD", "REFERENCE_ACCESS_TOKEN", "REFERENCE_TENANT_ID"} {
		t.Setenv(name, "")
	}
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://identity.example.test")
	t.Setenv("IAM_TENANT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/fixture/public.json")
	if _, e := Load(); e != nil {
		t.Fatal("optional Evidence became startup prerequisite", e)
	}
	t.Setenv("EXTERNAL_EVIDENCE_BASE_URL", "https://evidence.example.test")
	if _, e := Load(); e == nil {
		t.Fatal("missing workload accepted")
	}
	t.Setenv("IAM_SERVICE_CREDENTIAL_URL", "https://identity.example.test/api/v1/service-credentials")
	t.Setenv("WORKLOAD_ID", "contract-reference-oa")
	t.Setenv("WORKLOAD_SUBJECT_ID", "33333333-3333-4333-8333-333333333333")
	t.Setenv("WORKLOAD_KEY_FILE", "/fixture/private.json")
	if _, e := Load(); e != nil {
		t.Fatal(e)
	}
	t.Setenv("EXTERNAL_EVIDENCE_ACCESS_TOKEN", "long-static-token-not-allowed")
	if _, e := Load(); e == nil {
		t.Fatal("static token in IAM mode accepted")
	}
	t.Setenv("EXTERNAL_EVIDENCE_ACCESS_TOKEN", "")
	t.Setenv("EXTERNAL_EVIDENCE_BASE_URL", "https://user:password@evidence.example.test")
	if _, e := Load(); e == nil {
		t.Fatal("credential URL accepted")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":19081")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("SESSION_TTL", "2h")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("BOOTSTRAP_ADMIN_USERNAME", " local-admin ")
	t.Setenv("BOOTSTRAP_ADMIN_DISPLAY_NAME", " 本地管理员 ")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "a-secure-password")
	t.Setenv("EVIDENCE_STORAGE_DIR", "/srv/contract-evidence")
	t.Setenv("EVIDENCE_MAX_UPLOAD_BYTES", "1048576")
	t.Setenv("MIGRATION_MODE", "verify")
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://oms.example.test/auth")
	t.Setenv("IAM_TENANT_ID", "22222222-2222-4222-8222-222222222222")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "/run/oms/iam-public-keys.json")
	t.Setenv("PUBLIC_BASE_PATH", "/contract")
	t.Setenv("BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("BOOTSTRAP_ADMIN_DISPLAY_NAME", "")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.HTTPAddress != ":19081" || got.DatabaseURL != "postgres://example" || got.ShutdownPeriod != 3*time.Second ||
		got.SessionTTL != 2*time.Hour || !got.SessionCookieSecure || got.BootstrapAdminUsername != "" ||
		got.BootstrapAdminDisplayName != "" || got.BootstrapAdminPassword != "" ||
		got.EvidenceStorageDirectory != "/srv/contract-evidence" || got.EvidenceMaxUploadBytes != 1048576 || got.MigrationMode != MigrationModeVerify || got.IdentityProvider != IdentityProviderIAM || got.PublicBasePath != "/contract" {
		t.Fatalf("Load() = %#v", got)
	}
}

func TestLoadRejectsIncompleteIAMConfiguration(t *testing.T) {
	t.Setenv("IDENTITY_PROVIDER", "iam")
	t.Setenv("IAM_ISSUER", "https://oms.example.test/auth")
	t.Setenv("IAM_TENANT_ID", "")
	t.Setenv("IAM_PUBLIC_KEYS_PATH", "")
	if _, err := Load(); err == nil {
		t.Fatal("incomplete IAM configuration accepted")
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

	t.Setenv("REFERENCE_ACCESS_TOKEN", "")
	if _, err := Load(); err == nil {
		t.Fatal("partial reference credential accepted")
	}
}

func TestLoadRejectsInvalidMigrationMode(t *testing.T) {
	t.Setenv("MIGRATION_MODE", "repair")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want migration mode validation error")
	}
}

func TestLoadRejectsPartialBootstrapCredentials(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_USERNAME", "admin")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}

func TestLoadRejectsInvalidSessionConfiguration(t *testing.T) {
	t.Run("ttl", func(t *testing.T) {
		t.Setenv("SESSION_TTL", "never")
		if _, err := Load(); err == nil {
			t.Fatal("Load() error = nil, want validation error")
		}
	})
	t.Run("secure", func(t *testing.T) {
		t.Setenv("SESSION_COOKIE_SECURE", "sometimes")
		if _, err := Load(); err == nil {
			t.Fatal("Load() error = nil, want validation error")
		}
	})
}

func TestLoadRejectsInvalidShutdownTimeout(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "never")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}

func TestLoadRejectsInvalidEvidenceMaxUploadBytes(t *testing.T) {
	for _, value := range []string{"invalid", "0", "-1"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("EVIDENCE_MAX_UPLOAD_BYTES", value)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
		})
	}
}
