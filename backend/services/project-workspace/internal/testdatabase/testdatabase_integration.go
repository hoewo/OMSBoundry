//go:build integration

package testdatabase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Environment struct {
	Pool *pgxpool.Pool
	URL  string
	Name string
}

func New(t testing.TB, baseDatabaseURL string) Environment {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	baseConfig, err := pgxpool.ParseConfig(baseDatabaseURL)
	if err != nil {
		t.Fatalf("parse integration database URL: %v", err)
	}
	adminConfig := baseConfig.Copy()
	adminConfig.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("open integration database administrator: %v", err)
	}
	if err := adminPool.Ping(ctx); err != nil {
		adminPool.Close()
		t.Fatalf("ping integration database administrator: %v", err)
	}
	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		t.Fatal(err)
	}
	databaseName := "project_workspace_test_" + hex.EncodeToString(randomBytes)
	targetURL, err := url.Parse(baseDatabaseURL)
	if err != nil || (targetURL.Scheme != "postgres" && targetURL.Scheme != "postgresql") {
		t.Fatalf("integration database URL must use postgres:// or postgresql://: %v", err)
	}
	targetURL.Path = "/" + databaseName
	targetURL.RawPath = ""
	quotedName := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+quotedName); err != nil {
		adminPool.Close()
		t.Fatalf("create integration database: %v", err)
	}
	targetConfig := baseConfig.Copy()
	targetConfig.ConnConfig.Database = databaseName
	targetPool, err := pgxpool.NewWithConfig(ctx, targetConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := targetPool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		targetPool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_, _ = adminPool.Exec(cleanupCtx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1 AND pid<>pg_backend_pid()`, databaseName)
		if _, err := adminPool.Exec(cleanupCtx, "DROP DATABASE "+quotedName); err != nil {
			t.Errorf("drop integration database: %v", err)
		}
		adminPool.Close()
	})
	return Environment{Pool: targetPool, URL: strings.TrimSpace(targetURL.String()), Name: databaseName}
}

func RequireURL(t testing.TB) string {
	t.Helper()
	value := os.Getenv("TEST_DATABASE_URL")
	if value == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return value
}
