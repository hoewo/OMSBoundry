//go:build integration

package database

import (
	"context"
	"strings"
	"testing"

	"omsbizfoundry/project-workspace/internal/testdatabase"
)

func TestProjectPartyCandidateMigrationIsProjectOwnedAndImmutable(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	var definition string
	if err := environment.Pool.QueryRow(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='project_party_candidates'::regclass AND contype='c' AND pg_get_constraintdef(oid) LIKE '%role_key%'`).Scan(&definition); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(definition, "client") || !strings.Contains(definition, "contractor") {
		t.Fatalf("role constraint=%q", definition)
	}
	var trigger bool
	if err := environment.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid='project_party_candidates'::regclass AND tgname='project_party_candidates_immutable')`).Scan(&trigger); err != nil || !trigger {
		t.Fatalf("immutable trigger=%v error=%v", trigger, err)
	}
	if _, err := RequireCurrentSchema(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
}
