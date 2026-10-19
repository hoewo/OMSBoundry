//go:build integration

package database

import (
	"context"
	"strings"
	"testing"

	"omsbizfoundry/project-workspace/internal/testdatabase"
)

func TestPartyDirectoryResourceLinkMigrationExtendsDatabaseBoundary(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	constraints := map[string]string{}
	rows, err := environment.Pool.Query(ctx, `
		SELECT conname, pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid='project_resource_links'::regclass
		  AND conname IN ('project_resource_links_target_service_check','project_resource_links_resource_type_check')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			t.Fatal(err)
		}
		constraints[name] = definition
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(constraints["project_resource_links_target_service_check"], "party-directory") {
		t.Fatalf("target service constraint does not include Party Directory: %q", constraints["project_resource_links_target_service_check"])
	}
	if !strings.Contains(constraints["project_resource_links_resource_type_check"], "party") {
		t.Fatalf("resource type constraint does not include party: %q", constraints["project_resource_links_resource_type_check"])
	}
	if _, err := RequireCurrentSchema(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
}
