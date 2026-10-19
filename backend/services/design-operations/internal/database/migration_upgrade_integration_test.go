//go:build integration

package database

import (
	"context"
	"testing"

	"omsbizfoundry/design-operations/internal/testdatabase"
)

func TestProductionTypeMigrationPreservesExistingDesignRows(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	firstMigration, err := migrationFiles.ReadFile("migrations/000001_design_operations.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, `CREATE TABLE schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, string(firstMigration)); err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES('000001_design_operations.sql')`); err != nil {
		t.Fatal(err)
	}

	const (
		tenant  = "11111111-1111-4111-8111-111111111111"
		item    = "22222222-2222-4222-8222-222222222222"
		project = "33333333-3333-4333-8333-333333333333"
		actor   = "44444444-4444-4444-8444-444444444444"
	)
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO design_work_items(id,tenant_id,project_id,project_source_revision,type_key,type_definition_version,status,revision,current_stage_key,created_by,created_at,updated_at)
		VALUES($1,$2,$3,7,'design-v1',1,'draft',1,'scheme',$4,clock_timestamp(),clock_timestamp())`, item, tenant, project, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO design_stages(tenant_id,work_item_id,stage_key,sequence,status,updated_at) VALUES
		($1,$2,'scheme',10,'ready',clock_timestamp()),
		($1,$2,'preliminary-design',20,'pending',clock_timestamp()),
		($1,$2,'construction-drawing',30,'pending',clock_timestamp()),
		($1,$2,'construction-support',40,'pending',clock_timestamp())`, tenant, item); err != nil {
		t.Fatal(err)
	}
	const (
		deliverable = "55555555-5555-4555-8555-555555555555"
		evidence    = "66666666-6666-4666-8666-666666666666"
		command     = "77777777-7777-4777-8777-777777777777"
		history     = "88888888-8888-4888-8888-888888888888"
	)
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO design_deliverables(id,tenant_id,work_item_id,stage_key,requirement_key,evidence_file_id,evidence_version,evidence_source_revision,display_snapshot,created_work_item_revision,created_at)
		VALUES($1,$2,$3,'scheme','primary-output',$4,2,5,'既有方案.pdf',1,clock_timestamp())`, deliverable, tenant, item, evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO design_commands(tenant_id,command_id,request_hash,response_status,response_json,created_at)
		VALUES($1,$2,repeat('a',64),201,'{"work_item":{"id":"22222222-2222-4222-8222-222222222222"}}'::jsonb,clock_timestamp())`, tenant, command); err != nil {
		t.Fatal(err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO design_history_events(id,tenant_id,work_item_id,work_item_revision,event_sequence,event_type,command_id,actor_subject,occurred_at,payload)
		VALUES($1,$2,$3,1,1,'work_item_created',$4,$5,clock_timestamp(),'{"type_key":"design-v1","project_source_revision":7}'::jsonb)`, history, tenant, item, command, actor); err != nil {
		t.Fatal(err)
	}
	fixtureDigest := func() string {
		t.Helper()
		var digest string
		err := environment.Pool.QueryRow(ctx, `
			SELECT md5(
				(SELECT row_to_json(v)::text FROM (
					SELECT id,tenant_id,project_id,project_source_revision,type_key,type_definition_version,status,revision,owner_assignment_id,owner_source_revision,current_stage_key,cancellation_reason,created_by,created_at,updated_at
					FROM design_work_items WHERE tenant_id=$1 AND id=$2
				) v) || '|' ||
				(SELECT json_agg(v ORDER BY sequence)::text FROM (
					SELECT tenant_id,work_item_id,stage_key,sequence,status,not_applicable_reason,completed_at,completed_work_item_revision,updated_at
					FROM design_stages WHERE tenant_id=$1 AND work_item_id=$2
				) v) || '|' ||
				(SELECT json_agg(v ORDER BY created_at,id)::text FROM (
					SELECT id,tenant_id,work_item_id,stage_key,requirement_key,evidence_file_id,evidence_version,evidence_source_revision,display_snapshot,created_work_item_revision,superseded_work_item_revision,created_at,superseded_at
					FROM design_deliverables WHERE tenant_id=$1 AND work_item_id=$2
				) v) || '|' ||
				(SELECT json_agg(v ORDER BY event_sequence,id)::text FROM (
					SELECT id,tenant_id,work_item_id,work_item_revision,event_sequence,event_type,command_id,actor_subject,occurred_at,payload
					FROM design_history_events WHERE tenant_id=$1 AND work_item_id=$2
				) v) || '|' ||
				(SELECT json_agg(v ORDER BY command_id)::text FROM (
					SELECT tenant_id,command_id,request_hash,response_status,response_json,created_at
					FROM design_commands WHERE tenant_id=$1
				) v)
			)`, tenant, item).Scan(&digest)
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}
	beforeDigest := fixtureDigest()

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	version, err := RequireCurrentSchema(ctx, environment.Pool)
	if err != nil || version != "000007_type_scoped_test_report_definition.sql" {
		t.Fatalf("version=%q err=%v", version, err)
	}
	var typeKey, currentStage string
	var typeVersion, projectRevision, revision int64
	if err := environment.Pool.QueryRow(ctx, `SELECT type_key,type_definition_version,project_source_revision,revision,current_stage_key FROM design_work_items WHERE tenant_id=$1 AND id=$2`, tenant, item).Scan(&typeKey, &typeVersion, &projectRevision, &revision, &currentStage); err != nil {
		t.Fatal(err)
	}
	if typeKey != "design-v1" || typeVersion != 1 || projectRevision != 7 || revision != 1 || currentStage != "scheme" {
		t.Fatalf("design row changed during migration: type=%s/%d project_revision=%d revision=%d stage=%s", typeKey, typeVersion, projectRevision, revision, currentStage)
	}
	var stageCount int
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM design_stages WHERE tenant_id=$1 AND work_item_id=$2`, tenant, item).Scan(&stageCount); err != nil || stageCount != 4 {
		t.Fatalf("stage count=%d err=%v", stageCount, err)
	}
	var displaySnapshot string
	var evidenceVersion int
	var evidenceRevision int64
	if err := environment.Pool.QueryRow(ctx, `SELECT display_snapshot,evidence_version,evidence_source_revision FROM design_deliverables WHERE tenant_id=$1 AND id=$2`, tenant, deliverable).Scan(&displaySnapshot, &evidenceVersion, &evidenceRevision); err != nil {
		t.Fatal(err)
	}
	if displaySnapshot != "既有方案.pdf" || evidenceVersion != 2 || evidenceRevision != 5 {
		t.Fatalf("deliverable changed during migration: display=%q version=%d revision=%d", displaySnapshot, evidenceVersion, evidenceRevision)
	}
	var eventType, payloadType string
	var eventRevision int64
	if err := environment.Pool.QueryRow(ctx, `SELECT event_type,work_item_revision,payload->>'type_key' FROM design_history_events WHERE tenant_id=$1 AND id=$2`, tenant, history).Scan(&eventType, &eventRevision, &payloadType); err != nil {
		t.Fatal(err)
	}
	if eventType != "work_item_created" || eventRevision != 1 || payloadType != "design-v1" {
		t.Fatalf("history changed during migration: event=%s revision=%d type=%s", eventType, eventRevision, payloadType)
	}
	var responseID string
	if err := environment.Pool.QueryRow(ctx, `SELECT response_json->'work_item'->>'id' FROM design_commands WHERE tenant_id=$1 AND command_id=$2`, tenant, command).Scan(&responseID); err != nil || responseID != item {
		t.Fatalf("command changed during migration: response_id=%s err=%v", responseID, err)
	}
	if _, err := environment.Pool.Exec(ctx, `UPDATE design_history_events SET payload='{}'::jsonb WHERE tenant_id=$1 AND id=$2`, tenant, history); err == nil {
		t.Fatal("history immutability trigger disappeared during migration")
	}
	if afterDigest := fixtureDigest(); afterDigest != beforeDigest {
		t.Fatalf("deterministic fixture digest changed during migration: before=%s after=%s", beforeDigest, afterDigest)
	}
}
