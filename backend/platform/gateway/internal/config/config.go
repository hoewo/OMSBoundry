package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddress        string
	SuiteManifestPath  string
	PublicKeysPath     string
	Issuer             string
	CookieSecure       bool
	TrustProxyClientIP bool
	ShutdownPeriod     time.Duration
	RequestsPerMinute  int
}

func Load() (Config, error) {
	value := Config{HTTPAddress: env("HTTP_ADDR", ":8088"), SuiteManifestPath: strings.TrimSpace(os.Getenv("SUITE_MANIFEST_PATH")), PublicKeysPath: strings.TrimSpace(os.Getenv("IAM_PUBLIC_KEYS_PATH")), Issuer: strings.TrimRight(strings.TrimSpace(os.Getenv("IAM_ISSUER")), "/"), ShutdownPeriod: 10 * time.Second, RequestsPerMinute: 120}
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
	return value, nil
}
func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
