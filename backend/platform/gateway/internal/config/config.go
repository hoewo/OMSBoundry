package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddress                    string
	SuiteManifestPath              string
	PublicKeysPath                 string
	Issuer                         string
	CookieSecure                   bool
	TrustProxyClientIP             bool
	ShutdownPeriod                 time.Duration
	RequestsPerMinute              int
	OpenAPIKeyRequestsPerMinute    int
	OpenAPITenantRequestsPerMinute int
	OpenAPIUploadConcurrency       int
	OpenAPIRequireHTTPS            bool
}

func Load() (Config, error) {
	value := Config{HTTPAddress: env("HTTP_ADDR", ":8088"), SuiteManifestPath: strings.TrimSpace(os.Getenv("SUITE_MANIFEST_PATH")), PublicKeysPath: strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")), Issuer: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"), ShutdownPeriod: 10 * time.Second, RequestsPerMinute: 120, OpenAPIKeyRequestsPerMinute: 60, OpenAPITenantRequestsPerMinute: 600, OpenAPIUploadConcurrency: 4, OpenAPIRequireHTTPS: true}
	if value.SuiteManifestPath == "" || value.PublicKeysPath == "" || value.Issuer == "" {
		return Config{}, fmt.Errorf("SUITE_MANIFEST_PATH, IAM_PUBLIC_KEYS_PATH and IAM_ISSUER are required")
	}
	var err error
	if raw := os.Getenv("COOKIE_SECURE"); raw != "" {
		value.CookieSecure, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse COOKIE_SECURE: %w", err)
		}
	}
	if raw := os.Getenv("TRUST_PROXY_CLIENT_IP"); raw != "" {
		value.TrustProxyClientIP, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse TRUST_PROXY_CLIENT_IP: %w", err)
		}
	}
	if raw := os.Getenv("REQUESTS_PER_MINUTE"); raw != "" {
		value.RequestsPerMinute, err = strconv.Atoi(raw)
		if err != nil || value.RequestsPerMinute < 10 {
			return Config{}, fmt.Errorf("REQUESTS_PER_MINUTE must be at least 10")
		}
	}
	if raw := os.Getenv("OPEN_API_KEY_REQUESTS_PER_MINUTE"); raw != "" {
		value.OpenAPIKeyRequestsPerMinute, err = strconv.Atoi(raw)
		if err != nil || value.OpenAPIKeyRequestsPerMinute < 1 {
			return Config{}, fmt.Errorf("OPEN_API_KEY_REQUESTS_PER_MINUTE must be positive")
		}
	}
	if raw := os.Getenv("OPEN_API_TENANT_REQUESTS_PER_MINUTE"); raw != "" {
		value.OpenAPITenantRequestsPerMinute, err = strconv.Atoi(raw)
		if err != nil || value.OpenAPITenantRequestsPerMinute < value.OpenAPIKeyRequestsPerMinute {
			return Config{}, fmt.Errorf("OPEN_API_TENANT_REQUESTS_PER_MINUTE must be at least the per-Key limit")
		}
	}
	if raw := os.Getenv("OPEN_API_UPLOAD_CONCURRENCY"); raw != "" {
		value.OpenAPIUploadConcurrency, err = strconv.Atoi(raw)
		if err != nil || value.OpenAPIUploadConcurrency < 1 || value.OpenAPIUploadConcurrency > 128 {
			return Config{}, fmt.Errorf("OPEN_API_UPLOAD_CONCURRENCY must be between 1 and 128")
		}
	}
	if raw := os.Getenv("OPEN_API_REQUIRE_HTTPS"); raw != "" {
		value.OpenAPIRequireHTTPS, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse OPEN_API_REQUIRE_HTTPS: %w", err)
		}
	}
	if value.OpenAPITenantRequestsPerMinute < value.OpenAPIKeyRequestsPerMinute {
		return Config{}, fmt.Errorf("OPEN_API_TENANT_REQUESTS_PER_MINUTE must be at least the per-Key limit")
	}
	return value, nil
}
func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
