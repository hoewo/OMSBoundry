package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type MigrationMode string
type IdentityProvider string
type StorageProvider string

const (
	MigrationAuto    MigrationMode    = "auto"
	MigrationVerify  MigrationMode    = "verify"
	IdentityIAM      IdentityProvider = "iam"
	IdentityTemplate IdentityProvider = "template"
	StorageLocal     StorageProvider  = "local"
	StorageAliyunOSS StorageProvider  = "aliyun-oss"
	StorageHybrid    StorageProvider  = "hybrid"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Config struct {
	HTTPAddress            string
	DatabaseURL            string
	ShutdownTimeout        time.Duration
	MigrationMode          MigrationMode
	IdentityProvider       IdentityProvider
	IAMIssuer              string
	IAMTenantID            string
	IAMPublicKeysPath      string
	TemplateTenantID       string
	TemplateSubjectID      string
	TemplateDisplayName    string
	TemplateAccessToken    string
	StorageProvider        StorageProvider
	PrimaryStorageProvider StorageProvider
	PublicBaseURL          string
	UploadSessionTTL       time.Duration
	AccessGrantTTL         time.Duration
	MaxUploadBytes         int64
	LocalStorageRoot       string
	LocalSigningKey        []byte
	OSSRegion              string
	OSSBrowserEndpoint     string
	OSSBrowserCName        bool
	OSSServiceEndpoint     string
	OSSBucket              string
	OSSRoleARN             string
	OSSSTSEndpoint         string
	OSSSTSDuration         time.Duration
	CleanupStaleAfter      time.Duration
	CleanupBatchSize       int
	UnreferencedRetention  time.Duration
	ReferenceProtectionURL string
	ReferenceAccessToken   string
	ReferenceTimeout       time.Duration
}

func Load() (Config, error) {
	legacyOSSEndpoint := strings.TrimSpace(os.Getenv("OSS_ENDPOINT"))
	browserOSSEndpoint := strings.TrimSpace(os.Getenv("OSS_BROWSER_ENDPOINT"))
	serviceOSSEndpoint := strings.TrimSpace(os.Getenv("OSS_SERVICE_ENDPOINT"))
	if browserOSSEndpoint == "" {
		browserOSSEndpoint = legacyOSSEndpoint
	}
	if serviceOSSEndpoint == "" {
		serviceOSSEndpoint = legacyOSSEndpoint
	}
	cfg := Config{
		HTTPAddress:            env("HTTP_ADDR", ":8085"),
		DatabaseURL:            env("DATABASE_URL", "postgres://evidence_service:evidence_service_dev@127.0.0.1:55436/evidence_service?sslmode=disable"),
		ShutdownTimeout:        10 * time.Second,
		MigrationMode:          MigrationMode(strings.ToLower(strings.TrimSpace(env("MIGRATION_MODE", string(MigrationAuto))))),
		IdentityProvider:       IdentityProvider(strings.ToLower(strings.TrimSpace(env("IDENTITY_PROVIDER", string(IdentityTemplate))))),
		IAMIssuer:              strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"),
		IAMTenantID:            strings.ToLower(strings.TrimSpace(os.Getenv("IAM_TENANT_ID"))),
		IAMPublicKeysPath:      strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")),
		TemplateTenantID:       strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_TENANT_ID"))),
		TemplateSubjectID:      strings.ToLower(strings.TrimSpace(os.Getenv("TEMPLATE_SUBJECT_ID"))),
		TemplateDisplayName:    strings.TrimSpace(os.Getenv("TEMPLATE_DISPLAY_NAME")),
		TemplateAccessToken:    os.Getenv("TEMPLATE_ACCESS_TOKEN"),
		StorageProvider:        StorageProvider(strings.ToLower(strings.TrimSpace(env("STORAGE_PROVIDER", string(StorageLocal))))),
		PrimaryStorageProvider: StorageProvider(strings.ToLower(strings.TrimSpace(os.Getenv("PRIMARY_STORAGE_PROVIDER")))),
		PublicBaseURL:          strings.TrimRight(strings.TrimSpace(env("PUBLIC_BASE_URL", "http://127.0.0.1:8085")), "/"),
		UploadSessionTTL:       15 * time.Minute,
		AccessGrantTTL:         5 * time.Minute,
		MaxUploadBytes:         2 * 1024 * 1024 * 1024,
		LocalStorageRoot:       strings.TrimSpace(env("LOCAL_STORAGE_ROOT", "./.runtime/evidence")),
		OSSRegion:              strings.TrimSpace(os.Getenv("OSS_REGION")),
		OSSBrowserEndpoint:     browserOSSEndpoint,
		OSSServiceEndpoint:     serviceOSSEndpoint,
		OSSBucket:              strings.TrimSpace(os.Getenv("OSS_BUCKET")),
		OSSRoleARN:             strings.TrimSpace(os.Getenv("OSS_ROLE_ARN")),
		OSSSTSEndpoint:         strings.TrimSpace(os.Getenv("OSS_STS_ENDPOINT")),
		OSSSTSDuration:         15 * time.Minute,
		CleanupStaleAfter:      30 * time.Minute,
		CleanupBatchSize:       100,
		UnreferencedRetention:  7 * 24 * time.Hour,
		ReferenceProtectionURL: strings.TrimSpace(os.Getenv("REFERENCE_PROTECTION_URL")),
		ReferenceAccessToken:   os.Getenv("REFERENCE_PROTECTION_ACCESS_TOKEN"),
		ReferenceTimeout:       5 * time.Second,
	}
	var err error
	if raw := strings.TrimSpace(os.Getenv("OSS_BROWSER_CNAME")); raw != "" {
		cfg.OSSBrowserCName, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("OSS_BROWSER_CNAME must be a boolean")
		}
	}
	if cfg.OSSBrowserCName {
		u, err := url.Parse(cfg.OSSBrowserEndpoint)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || cfg.StorageProvider == StorageLocal {
			return Config{}, fmt.Errorf("OSS_BROWSER_CNAME requires an explicit HTTPS host-only endpoint in OSS or hybrid mode")
		}
	}
	if cfg.ShutdownTimeout, err = durationEnv("SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.UploadSessionTTL, err = durationEnv("UPLOAD_SESSION_TTL", cfg.UploadSessionTTL); err != nil {
		return Config{}, err
	}
	if cfg.AccessGrantTTL, err = durationEnv("ACCESS_GRANT_TTL", cfg.AccessGrantTTL); err != nil {
		return Config{}, err
	}
	if cfg.OSSSTSDuration, err = durationEnv("OSS_STS_DURATION", cfg.OSSSTSDuration); err != nil {
		return Config{}, err
	}
	if cfg.CleanupStaleAfter, err = durationEnv("CLEANUP_STALE_AFTER", cfg.CleanupStaleAfter); err != nil {
		return Config{}, err
	}
	if cfg.UnreferencedRetention, err = durationEnv("UNREFERENCED_RETENTION", cfg.UnreferencedRetention); err != nil {
		return Config{}, err
	}
	if cfg.ReferenceTimeout, err = durationEnv("REFERENCE_PROTECTION_TIMEOUT", cfg.ReferenceTimeout); err != nil {
		return Config{}, err
	}
	if raw := strings.TrimSpace(os.Getenv("MAX_UPLOAD_BYTES")); raw != "" {
		cfg.MaxUploadBytes, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || cfg.MaxUploadBytes <= 0 {
			return Config{}, fmt.Errorf("MAX_UPLOAD_BYTES must be a positive integer")
		}
	}
	if raw := strings.TrimSpace(os.Getenv("CLEANUP_BATCH_SIZE")); raw != "" {
		cfg.CleanupBatchSize, err = strconv.Atoi(raw)
		if err != nil || cfg.CleanupBatchSize < 1 || cfg.CleanupBatchSize > 1000 {
			return Config{}, fmt.Errorf("CLEANUP_BATCH_SIZE must be between 1 and 1000")
		}
	}
	if raw := strings.TrimSpace(os.Getenv("LOCAL_SIGNING_KEY")); raw != "" {
		cfg.LocalSigningKey, err = base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return Config{}, fmt.Errorf("LOCAL_SIGNING_KEY must be unpadded base64url: %w", err)
		}
	}
	if cfg.MigrationMode != MigrationAuto && cfg.MigrationMode != MigrationVerify {
		return Config{}, fmt.Errorf("MIGRATION_MODE must be auto or verify")
	}
	if cfg.IdentityProvider != IdentityIAM && cfg.IdentityProvider != IdentityTemplate {
		return Config{}, fmt.Errorf("IDENTITY_PROVIDER must be iam or template")
	}
	if cfg.StorageProvider != StorageLocal && cfg.StorageProvider != StorageAliyunOSS && cfg.StorageProvider != StorageHybrid {
		return Config{}, fmt.Errorf("STORAGE_PROVIDER must be local, aliyun-oss or hybrid")
	}
	if cfg.StorageProvider == StorageHybrid {
		if cfg.PrimaryStorageProvider != StorageLocal && cfg.PrimaryStorageProvider != StorageAliyunOSS {
			return Config{}, fmt.Errorf("hybrid mode requires PRIMARY_STORAGE_PROVIDER=local or aliyun-oss")
		}
	} else if cfg.PrimaryStorageProvider != "" {
		return Config{}, fmt.Errorf("PRIMARY_STORAGE_PROVIDER is only allowed in hybrid mode")
	}
	base, err := url.Parse(cfg.PublicBaseURL)
	if err != nil || !base.IsAbs() || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.RawQuery != "" || base.Fragment != "" {
		return Config{}, fmt.Errorf("PUBLIC_BASE_URL must be an absolute HTTP(S) URL without query or fragment")
	}
	if cfg.UploadSessionTTL < time.Minute || cfg.UploadSessionTTL > time.Hour {
		return Config{}, fmt.Errorf("UPLOAD_SESSION_TTL must be between 1m and 1h")
	}
	if cfg.AccessGrantTTL < time.Minute || cfg.AccessGrantTTL > 15*time.Minute {
		return Config{}, fmt.Errorf("ACCESS_GRANT_TTL must be between 1m and 15m")
	}
	if cfg.UnreferencedRetention < 24*time.Hour || cfg.ReferenceTimeout < time.Second || cfg.ReferenceTimeout > 30*time.Second {
		return Config{}, fmt.Errorf("UNREFERENCED_RETENTION must be at least 24h and REFERENCE_PROTECTION_TIMEOUT must be between 1s and 30s")
	}
	if cfg.IdentityProvider == IdentityIAM {
		if cfg.IAMIssuer == "" || !uuidPattern.MatchString(cfg.IAMTenantID) || cfg.IAMPublicKeysPath == "" {
			return Config{}, fmt.Errorf("IAM_ISSUER, IAM_TENANT_ID and IAM_PUBLIC_KEYS_PATH are required in iam mode")
		}
		if cfg.TemplateAccessToken != "" {
			return Config{}, fmt.Errorf("template credentials are not allowed in iam mode")
		}
	} else {
		if !uuidPattern.MatchString(cfg.TemplateTenantID) || !uuidPattern.MatchString(cfg.TemplateSubjectID) || cfg.TemplateDisplayName == "" || len(cfg.TemplateAccessToken) < 24 || strings.HasPrefix(cfg.TemplateAccessToken, "replace-with-") {
			return Config{}, fmt.Errorf("template mode requires valid TEMPLATE_TENANT_ID, TEMPLATE_SUBJECT_ID, TEMPLATE_DISPLAY_NAME and a replaced 24+ byte TEMPLATE_ACCESS_TOKEN")
		}
		if cfg.IAMIssuer != "" || cfg.IAMTenantID != "" || cfg.IAMPublicKeysPath != "" {
			return Config{}, fmt.Errorf("IAM settings are not allowed in template mode")
		}
	}
	if cfg.StorageProvider == StorageLocal || cfg.StorageProvider == StorageHybrid {
		root, err := filepath.Abs(cfg.LocalStorageRoot)
		if err != nil {
			return Config{}, fmt.Errorf("resolve LOCAL_STORAGE_ROOT: %w", err)
		}
		cfg.LocalStorageRoot = filepath.Clean(root)
		if len(cfg.LocalSigningKey) < 32 {
			return Config{}, fmt.Errorf("local mode requires LOCAL_SIGNING_KEY with at least 32 decoded bytes")
		}
		if cfg.StorageProvider == StorageLocal && (cfg.OSSRegion != "" || cfg.OSSBrowserEndpoint != "" || cfg.OSSServiceEndpoint != "" || cfg.OSSBucket != "" || cfg.OSSRoleARN != "") {
			return Config{}, fmt.Errorf("OSS settings are not allowed in local mode")
		}
	}
	if cfg.StorageProvider == StorageAliyunOSS || cfg.StorageProvider == StorageHybrid {
		if cfg.OSSRegion == "" || cfg.OSSBucket == "" || cfg.OSSRoleARN == "" {
			return Config{}, fmt.Errorf("OSS_REGION, OSS_BUCKET and OSS_ROLE_ARN are required in aliyun-oss mode")
		}
		if cfg.OSSSTSDuration < 15*time.Minute || cfg.OSSSTSDuration > time.Hour {
			return Config{}, fmt.Errorf("OSS_STS_DURATION must be between 15m and 1h")
		}
		if cfg.StorageProvider == StorageAliyunOSS && len(cfg.LocalSigningKey) != 0 {
			return Config{}, fmt.Errorf("LOCAL_SIGNING_KEY is not allowed in aliyun-oss mode")
		}
	}
	return cfg, nil
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
