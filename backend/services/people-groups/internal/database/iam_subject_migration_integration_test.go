//go:build integration

package database

import (
	"context"
	"os"
	"testing"
)

func TestIAMSubjectMigrationPreservesLegacyLinkAsUnverified(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	legacy, err := migrationFiles.ReadFile("migrations/000001_people_groups.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(legacy)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES('000001_people_groups.sql')`); err != nil {
		t.Fatal(err)
	}
	const tenantID = "11111111-1111-4111-8111-111111111111"
	const personID = "22222222-2222-4222-8222-222222222222"
	const subjectID = "33333333-3333-4333-8333-333333333333"
	const actorID = "44444444-4444-4444-8444-444444444444"
	if _, err := pool.Exec(ctx, `INSERT INTO persons(id,tenant_id,display_name,status,iam_subject_id,revision,created_at,updated_at) VALUES($1,$2,'Legacy Person','active',$3,1,clock_timestamp(),clock_timestamp())`, personID, tenantID, subjectID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO person_revisions(tenant_id,person_id,revision,event_kind,display_name,status,iam_subject_id,occurred_at,actor_kind,actor_id,actor_display_name) VALUES($1,$2,1,'created','Legacy Person','active',$3,clock_timestamp(),'iam_subject',$4,'Legacy Actor')`, tenantID, personID, subjectID, actorID); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var currentStatus, historyStatus string
	var currentRevision, historyRevision int64
	if err := pool.QueryRow(ctx, `SELECT iam_subject_status,iam_subject_revision FROM persons WHERE tenant_id=$1 AND id=$2`, tenantID, personID).Scan(&currentStatus, &currentRevision); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT iam_subject_status,iam_subject_revision FROM person_revisions WHERE tenant_id=$1 AND person_id=$2 AND revision=1`, tenantID, personID).Scan(&historyStatus, &historyRevision); err != nil {
		t.Fatal(err)
	}
	if currentStatus != "legacy_unverified" || historyStatus != "legacy_unverified" || currentRevision != 1 || historyRevision != 1 {
		t.Fatalf("current=%s/%d history=%s/%d", currentStatus, currentRevision, historyStatus, historyRevision)
	}
	if _, err := pool.Exec(ctx, `UPDATE person_revisions SET display_name='Tampered' WHERE tenant_id=$1 AND person_id=$2 AND revision=1`, tenantID, personID); err == nil {
		t.Fatal("person revision immutability trigger was not restored")
	}
}
