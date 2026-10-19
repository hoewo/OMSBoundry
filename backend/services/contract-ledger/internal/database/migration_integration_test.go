//go:build integration

package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIdentityAuditAndInstrumentMigrationsPreserveHistoricalFacts(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := openIsolatedMigrationDatabase(t, ctx, databaseURL)
	connection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	defer connection.Release()
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_parties.sql",
		"000003_contract_drafts.sql",
		"000004_contract_draft_parties.sql",
		"000005_contract_draft_change_commands.sql",
		"000006_contract_activation_revision.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			t.Fatalf("apply %s: %v", version, err)
		}
	}

	seedHistoricalFacts(t, ctx, pool)
	if err := applyMigration(ctx, connection, "000007_local_identity_and_write_audit.sql"); err != nil {
		t.Fatalf("apply identity and audit migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000007_local_identity_and_write_audit.sql"); err != nil {
		t.Fatalf("reapply identity and audit migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000008_effective_supplement_revisions.sql"); err != nil {
		t.Fatalf("apply contract instrument migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000008_effective_supplement_revisions.sql"); err != nil {
		t.Fatalf("reapply contract instrument migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000009_effective_termination_instrument.sql"); err != nil {
		t.Fatalf("apply contract termination migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000009_effective_termination_instrument.sql"); err != nil {
		t.Fatalf("reapply contract termination migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000010_contract_search_indexes.sql"); err != nil {
		t.Fatalf("apply contract search indexes migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000010_contract_search_indexes.sql"); err != nil {
		t.Fatalf("reapply contract search indexes migration: %v", err)
	}
	var searchIndexCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_indexes
		WHERE schemaname = current_schema()
			AND tablename = 'contracts'
			AND indexname IN ('contracts_status_created_id_idx', 'contracts_currency_created_id_idx')
	`).Scan(&searchIndexCount); err != nil {
		t.Fatalf("count contract search indexes: %v", err)
	}
	if searchIndexCount != 2 {
		t.Fatalf("contract search index count = %d, want 2", searchIndexCount)
	}
	if err := applyMigration(ctx, connection, "000011_contract_relations.sql"); err != nil {
		t.Fatalf("apply contract relations migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000011_contract_relations.sql"); err != nil {
		t.Fatalf("reapply contract relations migration: %v", err)
	}
	var relationTableCount, relationIndexCount, relationTriggerCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.tables
		WHERE table_schema = current_schema()
			AND table_name IN ('contract_relations', 'contract_relation_commands')
	`).Scan(&relationTableCount); err != nil {
		t.Fatalf("count contract relation tables: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_indexes
		WHERE schemaname = current_schema()
			AND indexname IN ('contract_relations_source_idx', 'contract_relations_target_idx')
	`).Scan(&relationIndexCount); err != nil {
		t.Fatalf("count contract relation indexes: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_trigger
		WHERE NOT tgisinternal
			AND tgname IN ('contract_relations_immutable', 'contract_relation_commands_immutable')
	`).Scan(&relationTriggerCount); err != nil {
		t.Fatalf("count contract relation triggers: %v", err)
	}
	if relationTableCount != 2 || relationIndexCount != 2 || relationTriggerCount != 2 {
		t.Fatalf("contract relation schema tables=%d indexes=%d triggers=%d", relationTableCount, relationIndexCount, relationTriggerCount)
	}
	if err := applyMigration(ctx, connection, "000012_contract_settlements.sql"); err != nil {
		t.Fatalf("apply contract settlements migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000012_contract_settlements.sql"); err != nil {
		t.Fatalf("reapply contract settlements migration: %v", err)
	}
	var settlementTableCount, settlementIndexCount, settlementTriggerCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.tables
		WHERE table_schema = current_schema()
			AND table_name IN (
				'contract_settlements',
				'contract_settlement_commands',
				'contract_settlement_parties'
			)
	`).Scan(&settlementTableCount); err != nil {
		t.Fatalf("count contract settlement tables: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_indexes
		WHERE schemaname = current_schema()
			AND indexname IN (
				'contract_settlements_number_unique',
				'contract_settlements_final_unique',
				'contract_settlements_timeline_idx'
			)
	`).Scan(&settlementIndexCount); err != nil {
		t.Fatalf("count contract settlement indexes: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_trigger
		WHERE NOT tgisinternal
			AND tgname IN (
				'contract_settlements_immutable',
				'contract_settlement_commands_immutable',
				'contract_settlement_parties_immutable'
			)
	`).Scan(&settlementTriggerCount); err != nil {
		t.Fatalf("count contract settlement triggers: %v", err)
	}
	if settlementTableCount != 3 || settlementIndexCount != 3 || settlementTriggerCount != 3 {
		t.Fatalf("contract settlement schema tables=%d indexes=%d triggers=%d", settlementTableCount, settlementIndexCount, settlementTriggerCount)
	}
	if err := applyMigration(ctx, connection, "000013_contract_operational_closures.sql"); err != nil {
		t.Fatalf("apply contract closure migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000013_contract_operational_closures.sql"); err != nil {
		t.Fatalf("reapply contract closure migration: %v", err)
	}
	var closureTableCount, closureTriggerCount, closureConstraintCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.tables
		WHERE table_schema = current_schema()
			AND table_name IN ('contract_closures', 'contract_closure_commands')
	`).Scan(&closureTableCount); err != nil {
		t.Fatalf("count contract closure tables: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_trigger
		WHERE NOT tgisinternal
			AND tgname IN ('contract_closures_immutable', 'contract_closure_commands_immutable')
	`).Scan(&closureTriggerCount); err != nil {
		t.Fatalf("count contract closure triggers: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_constraint
		WHERE conname IN (
			'contract_settlements_contract_id_id_unique',
			'contract_closures_revision_fk',
			'contract_closures_final_settlement_fk',
			'contract_closures_settlement_disposition_consistency_check'
		)
	`).Scan(&closureConstraintCount); err != nil {
		t.Fatalf("count contract closure constraints: %v", err)
	}
	if closureTableCount != 2 || closureTriggerCount != 2 || closureConstraintCount != 4 {
		t.Fatalf("contract closure schema tables=%d triggers=%d constraints=%d", closureTableCount, closureTriggerCount, closureConstraintCount)
	}
	if err := applyMigration(ctx, connection, "000014_contract_evidence.sql"); err != nil {
		t.Fatalf("apply contract evidence migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000014_contract_evidence.sql"); err != nil {
		t.Fatalf("reapply contract evidence migration: %v", err)
	}
	var evidenceTableCount, evidenceIndexCount, evidenceTriggerCount, evidenceConstraintCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.tables
		WHERE table_schema = current_schema()
			AND table_name IN (
				'contract_evidence_documents',
				'contract_evidence_versions',
				'contract_evidence_commands'
			)
	`).Scan(&evidenceTableCount); err != nil {
		t.Fatalf("count contract evidence tables: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_indexes
		WHERE schemaname = current_schema()
			AND indexname = 'contract_evidence_documents_contract_timeline_idx'
	`).Scan(&evidenceIndexCount); err != nil {
		t.Fatalf("count contract evidence indexes: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_trigger
		WHERE NOT tgisinternal
			AND tgname IN (
				'contract_evidence_documents_target',
				'contract_evidence_documents_immutable',
				'contract_evidence_versions_immutable',
				'contract_evidence_commands_immutable'
			)
	`).Scan(&evidenceTriggerCount); err != nil {
		t.Fatalf("count contract evidence triggers: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_constraint
		WHERE conname IN (
			'contract_evidence_documents_contract_id_id_unique',
			'contract_evidence_versions_sequence_unique',
			'contract_evidence_commands_document_fk',
			'contract_evidence_commands_version_fk'
		)
	`).Scan(&evidenceConstraintCount); err != nil {
		t.Fatalf("count contract evidence constraints: %v", err)
	}
	if evidenceTableCount != 3 || evidenceIndexCount != 1 || evidenceTriggerCount != 4 || evidenceConstraintCount != 4 {
		t.Fatalf("contract evidence schema tables=%d indexes=%d triggers=%d constraints=%d", evidenceTableCount, evidenceIndexCount, evidenceTriggerCount, evidenceConstraintCount)
	}
	if err := applyMigration(ctx, connection, "000015_effective_contract_corrections.sql"); err != nil {
		t.Fatalf("apply contract correction migration: %v", err)
	}
	if err := applyMigration(ctx, connection, "000015_effective_contract_corrections.sql"); err != nil {
		t.Fatalf("reapply contract correction migration: %v", err)
	}
	var correctionTableCount, correctionIndexCount, correctionTriggerCount, correctionConstraintCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.tables
		WHERE table_schema = current_schema()
			AND table_name = 'contract_correction_commands'
	`).Scan(&correctionTableCount); err != nil {
		t.Fatalf("count contract correction tables: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_indexes
		WHERE schemaname = current_schema()
			AND indexname = 'contract_correction_commands_contract_idx'
	`).Scan(&correctionIndexCount); err != nil {
		t.Fatalf("count contract correction indexes: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_trigger
		WHERE NOT tgisinternal
			AND tgname = 'contract_correction_commands_immutable'
	`).Scan(&correctionTriggerCount); err != nil {
		t.Fatalf("count contract correction triggers: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_constraint
		WHERE conname IN (
			'contract_instruments_instrument_type_check',
			'contract_instruments_amount_effect_check',
			'contract_instruments_type_payload_check',
			'contract_correction_commands_revision_order_check',
			'contract_correction_commands_expected_revision_fk',
			'contract_correction_commands_resulting_revision_fk'
		)
	`).Scan(&correctionConstraintCount); err != nil {
		t.Fatalf("count contract correction constraints: %v", err)
	}
	if correctionTableCount != 1 || correctionIndexCount != 1 || correctionTriggerCount != 1 || correctionConstraintCount != 6 {
		t.Fatalf("contract correction schema tables=%d indexes=%d triggers=%d constraints=%d", correctionTableCount, correctionIndexCount, correctionTriggerCount, correctionConstraintCount)
	}
	beforeRestoreUpgrade, err := InspectSchema(ctx, pool)
	wantPending := []string{"000016_empty_database_contract_restore.sql", "000017_local_user_iam_subject_links.sql", "000018_party_migration_mappings.sql", "000019_party_runtime_transitions.sql", "000020_reference_oa_contract_drafts.sql", "000021_reference_oa_amendment_confirmations.sql", "000022_reference_oa_materials.sql", "000023_party_directory_runtime_projections.sql", "000024_contract_external_evidence_revisions.sql"}
	if err != nil || beforeRestoreUpgrade.State != SchemaPending || !slices.Equal(beforeRestoreUpgrade.Pending, wantPending) {
		t.Fatalf("schema before explicit restore upgrade = %#v, %v", beforeRestoreUpgrade, err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("explicitly migrate contract restore schema: %v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("reapply explicit contract restore migration: %v", err)
	}
	if current, err := RequireCurrentSchema(ctx, pool); err != nil || current.Current != "000024_contract_external_evidence_revisions.sql" {
		t.Fatalf("schema after explicit restore upgrade = %#v, %v", current, err)
	}
	var restoreTableCount, restoreTriggerCount, restoreConstraintCount, nullableStorageColumns int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name = 'contract_restore_operations'
	`).Scan(&restoreTableCount); err != nil {
		t.Fatalf("count contract restore tables: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_trigger
		WHERE NOT tgisinternal AND tgname = 'contract_restore_operations_immutable'
	`).Scan(&restoreTriggerCount); err != nil {
		t.Fatalf("count contract restore triggers: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_constraint
		WHERE conname = 'contract_evidence_versions_content_shape_check'
	`).Scan(&restoreConstraintCount); err != nil {
		t.Fatalf("count restored evidence constraints: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema()
			AND table_name = 'contract_evidence_versions'
			AND column_name IN ('storage_provider', 'storage_key')
			AND is_nullable = 'YES'
	`).Scan(&nullableStorageColumns); err != nil {
		t.Fatalf("count nullable evidence storage columns: %v", err)
	}
	if restoreTableCount != 1 || restoreTriggerCount != 1 || restoreConstraintCount != 1 || nullableStorageColumns != 2 {
		t.Fatalf("contract restore schema tables=%d triggers=%d constraints=%d nullable storage=%d", restoreTableCount, restoreTriggerCount, restoreConstraintCount, nullableStorageColumns)
	}
	var identityLinkTableCount, identityLinkTriggerCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='local_user_iam_subject_links'`).Scan(&identityLinkTableCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal AND tgname='local_user_iam_subject_links_immutable'`).Scan(&identityLinkTriggerCount); err != nil {
		t.Fatal(err)
	}
	if identityLinkTableCount != 1 || identityLinkTriggerCount != 1 {
		t.Fatalf("identity link schema tables=%d triggers=%d", identityLinkTableCount, identityLinkTriggerCount)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO local_users (
			id, username, display_name, password_hash, role, status,
			password_changed_at, created_at, updated_at
		) VALUES (
			'81000000-0000-4000-8000-000000000001', 'legacy.user', '历史本地用户',
			repeat('x', 60), 'viewer', 'active',
			'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
		);
		INSERT INTO local_user_iam_subject_links (
			local_user_id, tenant_id, iam_subject_id,
			linked_by_iam_subject_id, linked_by_display_name, reason, linked_at
		) VALUES (
			'81000000-0000-4000-8000-000000000001',
			'82000000-0000-4000-8000-000000000001',
			'83000000-0000-4000-8000-000000000001',
			'84000000-0000-4000-8000-000000000001',
			'IAM 迁移管理员', 'M7 合同历史身份显式映射验收', '2026-09-20T00:00:00Z'
		)
	`); err != nil {
		t.Fatalf("seed explicit local-to-IAM identity link: %v", err)
	}
	_, err = pool.Exec(ctx, `
		UPDATE local_user_iam_subject_links
		SET reason = 'tampered'
		WHERE local_user_id = '81000000-0000-4000-8000-000000000001'
	`)
	var identityLinkError *pgconn.PgError
	if !errors.As(err, &identityLinkError) || identityLinkError.Code != "55000" {
		t.Fatalf("immutable identity link update error = %v", err)
	}

	legacyActions := map[string]int{
		"party_created":              3,
		"contract_draft_created":     2,
		"contract_draft_party_added": 1,
		"contract_draft_updated":     1,
	}
	for action, want := range legacyActions {
		var got int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM audit_entries
			WHERE action = $1
				AND actor_kind = 'legacy_unknown'
				AND actor_id = 'actor-not-recorded-before-cl-m1-006'
		`, action).Scan(&got); err != nil {
			t.Fatalf("count projected action %q: %v", action, err)
		}
		if got != want {
			t.Fatalf("projected action %q count = %d, want %d", action, got, want)
		}
	}

	var actorKind, actorID string
	if err := pool.QueryRow(ctx, `
		SELECT actor_kind, actor_id
		FROM audit_entries
		WHERE action = 'contract_activated'
	`).Scan(&actorKind, &actorID); err != nil {
		t.Fatalf("read projected activation actor: %v", err)
	}
	if actorKind != "local" || actorID != "legacy-operator" {
		t.Fatalf("activation actor = %s:%s", actorKind, actorID)
	}

	var instrumentType, instrumentNumber, amountEffect, sourceInstrumentID string
	var effectiveOrder, resultingRevision int64
	if err := pool.QueryRow(ctx, `
		SELECT i.instrument_type, i.instrument_number, i.amount_effect,
			i.effective_order, i.resulting_revision, r.source_instrument_id::text
		FROM contract_instruments i
		JOIN contract_revisions r
			ON r.contract_id = i.contract_id AND r.revision = i.resulting_revision
	`).Scan(
		&instrumentType,
		&instrumentNumber,
		&amountEffect,
		&effectiveOrder,
		&resultingRevision,
		&sourceInstrumentID,
	); err != nil {
		t.Fatalf("read backfilled original instrument: %v", err)
	}
	if instrumentType != "original_contract" || instrumentNumber != "ACTIVE-1" || amountEffect != "original" ||
		effectiveOrder != 1 || resultingRevision != 1 || sourceInstrumentID == "" {
		t.Fatalf("backfilled original instrument = %s|%s|%s|%d|%d|%s", instrumentType, instrumentNumber, amountEffect, effectiveOrder, resultingRevision, sourceInstrumentID)
	}
	var originalInstrumentCount, changeCommandCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contract_instruments`).Scan(&originalInstrumentCount); err != nil {
		t.Fatalf("count original instruments: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contract_change_commands`).Scan(&changeCommandCount); err != nil {
		t.Fatalf("count change commands: %v", err)
	}
	if originalInstrumentCount != 1 || changeCommandCount != 0 {
		t.Fatalf("instrument migration counts instruments=%d commands=%d", originalInstrumentCount, changeCommandCount)
	}
	var terminationCommandCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contract_termination_commands`).Scan(&terminationCommandCount); err != nil {
		t.Fatalf("count termination commands: %v", err)
	}
	if terminationCommandCount != 0 {
		t.Fatalf("termination command count = %d, want 0", terminationCommandCount)
	}

	var originalCommandCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM contract_draft_party_commands) +
			(SELECT count(*) FROM contract_draft_change_commands) +
			(SELECT count(*) FROM contract_activation_commands)
	`).Scan(&originalCommandCount); err != nil {
		t.Fatalf("count original commands: %v", err)
	}
	if originalCommandCount != 3 {
		t.Fatalf("original command count = %d, want 3", originalCommandCount)
	}

	_, err = pool.Exec(ctx, `UPDATE audit_entries SET summary = 'tampered'`)
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
		t.Fatalf("immutable projected audit error = %v", err)
	}
}

func seedHistoricalFacts(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO parties (id, kind, legal_name, registration_identifier, revision, created_at, updated_at)
		VALUES
			('10000000-0000-4000-8000-000000000001', 'organization', '历史主体一', 'LEGACY-1', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
			('10000000-0000-4000-8000-000000000002', 'organization', '历史主体二', 'LEGACY-2', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
			('10000000-0000-4000-8000-000000000003', 'organization', '历史主体三', 'LEGACY-3', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');

		INSERT INTO contracts (
			id, command_id, number_namespace, contract_number, title, currency,
			status, draft_version, current_revision, created_at, updated_at
		)
		VALUES
			('20000000-0000-4000-8000-000000000001', '30000000-0000-4000-8000-000000000001', 'legacy', 'DRAFT-1', '历史草稿', 'CNY', 'draft', 3, NULL, '2026-01-02T00:00:00Z', '2026-01-04T00:00:00Z'),
			('20000000-0000-4000-8000-000000000002', '30000000-0000-4000-8000-000000000002', 'legacy', 'ACTIVE-1', '历史已激活合同', 'CNY', 'active', 3, 1, '2026-02-01T00:00:00Z', '2026-02-04T00:00:00Z');

		INSERT INTO contract_draft_parties (id, contract_id, party_id, role, added_at)
		VALUES
			('40000000-0000-4000-8000-000000000001', '20000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', '委托方', '2026-01-03T00:00:00Z'),
			('40000000-0000-4000-8000-000000000002', '20000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000002', '委托方', '2026-02-02T00:00:00Z'),
			('40000000-0000-4000-8000-000000000003', '20000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000003', '受托方', '2026-02-03T00:00:00Z');

		INSERT INTO contract_draft_party_commands (
			command_id, contract_id, action, party_link_id, party_id, role,
			expected_draft_version, resulting_draft_version, created_at
		)
		VALUES (
			'50000000-0000-4000-8000-000000000001', '20000000-0000-4000-8000-000000000001', 'add',
			'40000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', '委托方',
			1, 2, '2026-01-03T00:00:00Z'
		);

		INSERT INTO contract_draft_change_commands (
			command_id, contract_id, action, expected_draft_version, resulting_draft_version,
			number_namespace, contract_number, title, currency, created_at
		)
		VALUES (
			'50000000-0000-4000-8000-000000000002', '20000000-0000-4000-8000-000000000001', 'update', 2, 3,
			'legacy', 'DRAFT-1', '历史草稿', 'CNY', '2026-01-04T00:00:00Z'
		);

		INSERT INTO contract_activation_commands (
			command_id, contract_id, expected_draft_version, actor_kind, actor_id, resulting_revision, occurred_at
		)
		VALUES (
			'50000000-0000-4000-8000-000000000003', '20000000-0000-4000-8000-000000000002', 3,
			'local', 'legacy-operator', 1, '2026-02-04T00:00:00Z'
		);

		INSERT INTO contract_revisions (
			contract_id, revision, activation_command_id, source_draft_version,
			number_namespace, contract_number, title, currency, signed_on, effective_on,
			actor_kind, actor_id, activated_at
		)
		VALUES (
			'20000000-0000-4000-8000-000000000002', 1, '50000000-0000-4000-8000-000000000003', 3,
			'legacy', 'ACTIVE-1', '历史已激活合同', 'CNY', '2026-02-01', '2026-02-02',
			'local', 'legacy-operator', '2026-02-04T00:00:00Z'
		);

		INSERT INTO contract_revision_parties (
			contract_id, revision, position, source_party_link_id, source_party_id,
			source_party_revision, party_kind, legal_name, registration_identifier, role
		)
		VALUES
			('20000000-0000-4000-8000-000000000002', 1, 1, '40000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000002', 1, 'organization', '历史主体二', 'LEGACY-2', '委托方'),
			('20000000-0000-4000-8000-000000000002', 1, 2, '40000000-0000-4000-8000-000000000003', '10000000-0000-4000-8000-000000000003', 1, 'organization', '历史主体三', 'LEGACY-3', '受托方');

		INSERT INTO contract_audit_entries (
			id, contract_id, event_type, command_id, actor_kind, actor_id,
			occurred_at, source_draft_version, resulting_revision
		)
		VALUES (
			'60000000-0000-4000-8000-000000000001', '20000000-0000-4000-8000-000000000002',
			'contract_activated', '50000000-0000-4000-8000-000000000003', 'local', 'legacy-operator',
			'2026-02-04T00:00:00Z', 3, 1
		);
	`)
	if err != nil {
		t.Fatalf("seed historical facts: %v", err)
	}
}

func openIsolatedMigrationDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	baseConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	adminConfig := baseConfig.Copy()
	adminConfig.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("open administration database: %v", err)
	}
	defer adminPool.Close()
	databaseName := fmt.Sprintf("contract_ledger_migration_%d", time.Now().UnixNano())
	quotedName := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminPool.Exec(ctx, `CREATE DATABASE `+quotedName); err != nil {
		t.Fatalf("create isolated database: %v", err)
	}
	testConfig := baseConfig.Copy()
	testConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		t.Fatalf("open isolated database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cleanupConfig := baseConfig.Copy()
		cleanupConfig.ConnConfig.Database = "postgres"
		cleanupPool, cleanupErr := pgxpool.NewWithConfig(cleanupCtx, cleanupConfig)
		if cleanupErr != nil {
			t.Errorf("open cleanup database: %v", cleanupErr)
			return
		}
		defer cleanupPool.Close()
		if _, cleanupErr := cleanupPool.Exec(cleanupCtx, `DROP DATABASE `+quotedName+` WITH (FORCE)`); cleanupErr != nil {
			t.Errorf("drop isolated database %q: %v", databaseName, cleanupErr)
		}
	})
	return pool
}
