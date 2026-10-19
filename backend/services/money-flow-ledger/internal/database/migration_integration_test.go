//go:build integration

package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

func TestMoneyFlowMigrationsFromBootstrapAndReplay(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	if err := applyMigration(ctx, connection, "000001_bootstrap.sql"); err != nil {
		connection.Release()
		t.Fatalf("apply historical bootstrap migration: %v", err)
	}
	connection.Release()

	var beforeVersions int
	var usersBefore *string
	if err := environment.Pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&beforeVersions); err != nil {
		t.Fatalf("count historical migrations: %v", err)
	}
	if err := environment.Pool.QueryRow(ctx, `SELECT to_regclass('public.local_users')::text`).Scan(&usersBefore); err != nil {
		t.Fatalf("inspect historical local_users: %v", err)
	}
	if beforeVersions != 1 || usersBefore != nil {
		t.Fatalf("historical state versions=%d local_users=%v", beforeVersions, usersBefore)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var migrationCount, tableCount, indexCount, triggerCount, foreignKeyCount int
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			(SELECT count(*) FROM pg_class WHERE relnamespace = 'public'::regnamespace AND relname IN ('local_users', 'local_sessions', 'audit_entries', 'parties', 'party_create_commands', 'obligations', 'obligation_create_commands', 'obligation_revisions', 'obligation_confirmation_commands', 'cash_transactions', 'cash_transaction_create_commands', 'allocations', 'allocation_apply_commands', 'internal_transfers', 'internal_transfer_create_commands', 'allocation_reversals', 'allocation_reversal_create_commands', 'cash_transaction_reversals', 'cash_transaction_reversal_create_commands', 'obligation_amount_adjustments', 'obligation_amount_adjustment_commands', 'obligation_remissions', 'obligation_remission_commands', 'obligation_closures', 'obligation_closure_commands', 'obligation_cash_expectations', 'obligation_cash_expectation_commands', 'party_migration_mappings', 'receipt_sets', 'receipt_set_items', 'receipt_set_confirm_commands', 'cash_transaction_evidence_revisions') AND relkind = 'r'),
			(SELECT count(*) FROM pg_class WHERE relnamespace = 'public'::regnamespace AND relname IN ('local_users_username_unique', 'local_sessions_user_idx', 'audit_entries_timeline_idx', 'parties_legal_name_id_idx', 'party_create_commands_party_idx', 'obligations_created_id_idx', 'obligations_status_due_id_idx', 'cash_transactions_recorded_id_idx', 'cash_transactions_type_occurred_id_idx', 'cash_transactions_external_reference_unique', 'allocations_applied_id_idx', 'allocations_cash_applied_id_idx', 'allocations_obligation_applied_id_idx', 'internal_transfers_recorded_id_idx', 'internal_transfers_occurred_id_idx', 'internal_transfers_external_reference_unique', 'allocation_reversals_reversed_id_idx', 'allocation_reversals_cash_reversed_id_idx', 'allocation_reversals_obligation_reversed_id_idx', 'cash_transaction_reversals_reversed_id_idx', 'obligation_amount_adjustments_obligation_adjusted_id_idx', 'obligation_remissions_obligation_recorded_id_idx', 'obligation_closures_closed_id_idx', 'obligation_revisions_recorded_revision_idx', 'obligation_cash_expectations_current_idx', 'obligation_cash_expectations_schedule_idx', 'party_migration_mappings_target_idx', 'receipt_sets_tenant_project_confirmed_idx', 'receipt_set_items_allocation_idx', 'cash_transaction_evidence_revisions_latest_idx') AND relkind = 'i'),
			(SELECT count(*) FROM pg_trigger WHERE tgname IN ('audit_entries_immutable', 'party_create_commands_immutable', 'obligation_create_commands_immutable', 'obligation_revisions_immutable', 'obligation_confirmation_commands_immutable', 'cash_transactions_immutable', 'cash_transaction_create_commands_immutable', 'allocations_validate_insert', 'allocations_validate_remission_insert', 'allocations_immutable', 'allocation_apply_commands_immutable', 'internal_transfers_immutable', 'internal_transfer_create_commands_immutable', 'allocation_reversals_validate_insert', 'allocation_reversals_immutable', 'allocation_reversal_create_commands_immutable', 'cash_transaction_reversals_validate_insert', 'cash_transaction_reversals_immutable', 'cash_transaction_reversal_create_commands_immutable', 'obligation_amount_adjustments_validate_insert', 'obligation_amount_adjustments_validate_remission_floor', 'obligation_revisions_validate_insert', 'obligations_validate_current_revision_update', 'obligation_amount_adjustments_immutable', 'obligation_amount_adjustment_commands_immutable', 'obligation_remissions_validate_insert', 'obligation_remissions_immutable', 'obligation_remission_commands_immutable', 'obligation_closures_validate_insert', 'obligations_validate_closure_status_update', 'obligation_closures_require_closed_status', 'obligation_closures_immutable', 'obligation_closure_commands_immutable', 'zz_obligation_revisions_recorded_time_monotonic', 'zz_obligation_amount_adjustments_recorded_time_monotonic', 'zz_allocations_recorded_time_monotonic', 'zz_allocation_reversals_recorded_time_monotonic', 'zz_obligation_remissions_recorded_time_monotonic', 'zz_obligation_closures_recorded_time_monotonic', 'obligation_cash_expectations_validate_insert', 'obligation_cash_expectations_immutable', 'obligation_cash_expectation_commands_immutable', 'party_migration_mappings_immutable', 'receipt_set_items_validate_insert', 'receipt_sets_total_consistent', 'receipt_set_items_total_consistent', 'receipt_sets_immutable', 'receipt_set_items_immutable', 'receipt_set_confirm_commands_immutable', 'cash_transaction_evidence_revisions_immutable') AND NOT tgisinternal),
			(SELECT count(*) FROM pg_constraint WHERE conrelid IN ('obligations'::regclass, 'obligation_create_commands'::regclass, 'obligation_revisions'::regclass, 'obligation_confirmation_commands'::regclass, 'cash_transactions'::regclass, 'cash_transaction_create_commands'::regclass, 'allocations'::regclass, 'allocation_apply_commands'::regclass, 'internal_transfers'::regclass, 'internal_transfer_create_commands'::regclass, 'allocation_reversals'::regclass, 'allocation_reversal_create_commands'::regclass, 'cash_transaction_reversals'::regclass, 'cash_transaction_reversal_create_commands'::regclass, 'obligation_amount_adjustments'::regclass, 'obligation_amount_adjustment_commands'::regclass, 'obligation_remissions'::regclass, 'obligation_remission_commands'::regclass, 'obligation_closures'::regclass, 'obligation_closure_commands'::regclass, 'obligation_cash_expectations'::regclass, 'obligation_cash_expectation_commands'::regclass, 'party_migration_mappings'::regclass, 'receipt_sets'::regclass, 'receipt_set_items'::regclass, 'receipt_set_confirm_commands'::regclass, 'cash_transaction_evidence_revisions'::regclass) AND contype = 'f')
		`).Scan(&migrationCount, &tableCount, &indexCount, &triggerCount, &foreignKeyCount); err != nil {
		t.Fatalf("inspect migration objects: %v", err)
	}
	if migrationCount != 24 || tableCount != 32 || indexCount != 30 || triggerCount != 50 || foreignKeyCount != 50 {
		t.Fatalf("migration objects versions=%d tables=%d indexes=%d triggers=%d foreign_keys=%d", migrationCount, tableCount, indexCount, triggerCount, foreignKeyCount)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO local_users (
			id, username, display_name, password_hash, role, status,
			session_generation, password_changed_at, created_at, updated_at
		) VALUES (
			'11111111-1111-4111-8111-111111111111', 'admin', '管理员', $1,
			'administrator', 'active', 1, $2, $2, $2
		)
	`, "$2a$10$12345678901234567890123456789012345678901234567890123", now); err != nil {
		t.Fatalf("insert migration fixture user: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO audit_entries (
			id, object_type, object_id, action, actor_kind, actor_id,
			actor_display_name, occurred_at, summary, details
		) VALUES (
			'22222222-2222-4222-8222-222222222222', 'local_user',
			'11111111-1111-4111-8111-111111111111', 'local_user_bootstrapped',
			'bootstrap_configuration', '11111111-1111-4111-8111-111111111111',
			'管理员', $1, '建立首位管理员', '{}'::jsonb
		)
	`, now); err != nil {
		t.Fatalf("insert migration fixture audit: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (
			id, kind, legal_name, registration_identifier, revision, created_at, updated_at
		) VALUES (
			'44444444-4444-4444-8444-444444444444', 'organization', '迁移验证主体', NULL, 1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert migration fixture party: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO party_create_commands (
			command_id, party_id, payload_hash, actor_kind, actor_id, actor_display_name, created_at
		) VALUES (
			'55555555-5555-4555-8555-555555555555',
			'44444444-4444-4444-8444-444444444444',
			decode(repeat('ab', 32), 'hex'), 'local_user',
			'11111111-1111-4111-8111-111111111111', '管理员', $1
		)
	`, now); err != nil {
		t.Fatalf("insert migration fixture party command: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (
			id, kind, legal_name, registration_identifier, revision, created_at, updated_at
		) VALUES (
			'66666666-6666-4666-8666-666666666666', 'individual', '迁移验证往来方', 'P-001', 1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert second migration fixture party: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO obligations (
			id, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			status, draft_version, created_at, updated_at
		) VALUES (
			'77777777-7777-4777-8777-777777777777', 'receivable', '迁移验证义务',
			'44444444-4444-4444-8444-444444444444',
			'44444444-4444-4444-8444-444444444444', 1, 'organization', '迁移验证主体', NULL,
			'66666666-6666-4666-8666-666666666666', 1, 'individual', '迁移验证往来方', 'P-001',
			100.5, 'CNY', '2026-10-31', 'manual', '迁移验证依据',
			'draft', 1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert migration fixture obligation: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO obligation_create_commands (
			command_id, obligation_id, payload_hash, actor_kind, actor_id, actor_display_name, created_at
		) VALUES (
			'88888888-8888-4888-8888-888888888888',
			'77777777-7777-4777-8777-777777777777',
			decode(repeat('cd', 32), 'hex'), 'local_user',
			'11111111-1111-4111-8111-111111111111', '管理员', $1
		)
	`, now); err != nil {
		t.Fatalf("insert migration fixture obligation command: %v", err)
	}

	for _, statement := range []string{
		`UPDATE audit_entries SET summary = 'changed' WHERE id = '22222222-2222-4222-8222-222222222222'`,
		`DELETE FROM audit_entries WHERE id = '22222222-2222-4222-8222-222222222222'`,
		`UPDATE party_create_commands SET actor_display_name = 'changed' WHERE command_id = '55555555-5555-4555-8555-555555555555'`,
		`DELETE FROM party_create_commands WHERE command_id = '55555555-5555-4555-8555-555555555555'`,
		`UPDATE obligation_create_commands SET actor_display_name = 'changed' WHERE command_id = '88888888-8888-4888-8888-888888888888'`,
		`DELETE FROM obligation_create_commands WHERE command_id = '88888888-8888-4888-8888-888888888888'`,
	} {
		_, err := environment.Pool.Exec(ctx, statement)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "55000" {
			t.Fatalf("immutable statement error = %v", err)
		}
	}

	_, err = environment.Pool.Exec(ctx, `
		INSERT INTO local_users (
			id, username, display_name, password_hash, role, status,
			session_generation, password_changed_at, created_at, updated_at
		) VALUES (
			'33333333-3333-4333-8333-333333333333', 'Bad User', '错误', $1,
			'administrator', 'active', 1, $2, $2, $2
		)
	`, "$2a$10$12345678901234567890123456789012345678901234567890123", now)
	var constraintError *pgconn.PgError
	if !errors.As(err, &constraintError) || constraintError.Code != "23514" {
		t.Fatalf("invalid username constraint error = %v", err)
	}

	_, err = environment.Pool.Exec(ctx, `
		UPDATE obligations
		SET ledger_owner_party_id = debtor_party_id
		WHERE id = '77777777-7777-4777-8777-777777777777'
	`)
	if !errors.As(err, &constraintError) || constraintError.Code != "23514" {
		t.Fatalf("invalid obligation owner constraint error = %v", err)
	}
}

func TestObligationDraftMigrationFromPartySchema(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	var beforeVersions int
	var obligationsBefore *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			to_regclass('public.obligations')::text
	`).Scan(&beforeVersions, &obligationsBefore); err != nil {
		t.Fatalf("inspect party schema state: %v", err)
	}
	if beforeVersions != 3 || obligationsBefore != nil {
		t.Fatalf("party schema versions=%d obligations=%v", beforeVersions, obligationsBefore)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
	var afterVersions int
	var obligationsAfter, commandsAfter *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			to_regclass('public.obligations')::text,
			to_regclass('public.obligation_create_commands')::text
	`).Scan(&afterVersions, &obligationsAfter, &commandsAfter); err != nil {
		t.Fatalf("inspect obligation schema state: %v", err)
	}
	if afterVersions != 24 || obligationsAfter == nil || commandsAfter == nil {
		t.Fatalf("obligation schema versions=%d obligations=%v commands=%v", afterVersions, obligationsAfter, commandsAfter)
	}
}

func TestObligationConfirmationMigrationFromDraftSchema(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
		"000004_obligation_drafts.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (id, kind, legal_name, revision, created_at, updated_at)
		VALUES
			('11111111-1111-4111-8111-111111111111', 'organization', '升级本方', 1, $1, $1),
			('22222222-2222-4222-8222-222222222222', 'individual', '升级往来方', 1, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert version 4 parties: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO obligations (
			id, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			status, draft_version, created_at, updated_at
		) VALUES (
			'33333333-3333-4333-8333-333333333333', 'receivable', '升级前草稿',
			'11111111-1111-4111-8111-111111111111',
			'11111111-1111-4111-8111-111111111111', 1, 'organization', '升级本方',
			'22222222-2222-4222-8222-222222222222', 1, 'individual', '升级往来方',
			12.5, 'CNY', '2026-12-31', 'manual', '升级前依据',
			'draft', 1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert version 4 obligation: %v", err)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions int
	var status string
	var currentRevision *int64
	var revisionTable, commandTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			status,
			current_revision,
			to_regclass('public.obligation_revisions')::text,
			to_regclass('public.obligation_confirmation_commands')::text
		FROM obligations
		WHERE id = '33333333-3333-4333-8333-333333333333'
	`).Scan(&versions, &status, &currentRevision, &revisionTable, &commandTable); err != nil {
		t.Fatalf("inspect confirmation migration: %v", err)
	}
	if versions != 24 || status != "draft" || currentRevision != nil || revisionTable == nil || commandTable == nil {
		t.Fatalf("confirmation migration versions=%d status=%s current=%v revisions=%v commands=%v", versions, status, currentRevision, revisionTable, commandTable)
	}
}

func TestCashTransactionMigrationFromConfirmationSchema(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
		"000004_obligation_drafts.sql",
		"000005_obligation_confirmation.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (id, kind, legal_name, revision, created_at, updated_at)
		VALUES ('11111111-1111-4111-8111-111111111111', 'organization', '升级保留主体', 1, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert version 5 fixture: %v", err)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions, parties int
	var cashTable, commandTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			(SELECT count(*) FROM parties),
			to_regclass('public.cash_transactions')::text,
			to_regclass('public.cash_transaction_create_commands')::text
	`).Scan(&versions, &parties, &cashTable, &commandTable); err != nil {
		t.Fatalf("inspect cash transaction migration: %v", err)
	}
	if versions != 24 || parties != 1 || cashTable == nil || commandTable == nil {
		t.Fatalf("cash migration versions=%d parties=%d cash=%v commands=%v", versions, parties, cashTable, commandTable)
	}
}

func TestAllocationMigrationFromCashTransactionSchema(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
		"000004_obligation_drafts.sql",
		"000005_obligation_confirmation.sql",
		"000006_manual_cash_transactions.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (id, kind, legal_name, revision, created_at, updated_at)
		VALUES
			('11111111-1111-4111-8111-111111111111', 'organization', '升级本方', 1, $1, $1),
			('22222222-2222-4222-8222-222222222222', 'individual', '升级往来方', 1, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert version 6 parties: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO cash_transactions (
			id, type, description, ledger_owner_party_id,
			payer_party_id, payer_party_revision, payer_kind, payer_legal_name,
			payee_party_id, payee_party_revision, payee_kind, payee_legal_name,
			amount, currency, occurred_on, account_reference,
			source_kind, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		) VALUES (
			'33333333-3333-4333-8333-333333333333', 'receipt', '升级前资金',
			'11111111-1111-4111-8111-111111111111',
			'22222222-2222-4222-8222-222222222222', 1, 'individual', '升级往来方',
			'11111111-1111-4111-8111-111111111111', 1, 'organization', '升级本方',
			100, 'CNY', '2026-09-19', '升级账户', 'manual', '升级回单',
			'local_user', '44444444-4444-4444-8444-444444444444', '升级管理员', $1
		)
	`, now); err != nil {
		t.Fatalf("insert version 6 cash transaction: %v", err)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions, cashTransactions int
	var allocationTable, commandTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			(SELECT count(*) FROM cash_transactions),
			to_regclass('public.allocations')::text,
			to_regclass('public.allocation_apply_commands')::text
	`).Scan(&versions, &cashTransactions, &allocationTable, &commandTable); err != nil {
		t.Fatalf("inspect allocation migration: %v", err)
	}
	if versions != 24 || cashTransactions != 1 || allocationTable == nil || commandTable == nil {
		t.Fatalf("allocation migration versions=%d cash=%d allocations=%v commands=%v", versions, cashTransactions, allocationTable, commandTable)
	}
}

func TestInternalTransferMigrationFromAllocationSchema(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
		"000004_obligation_drafts.sql",
		"000005_obligation_confirmation.sql",
		"000006_manual_cash_transactions.sql",
		"000007_single_allocations.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (id, kind, legal_name, revision, created_at, updated_at)
		VALUES
			('11111111-1111-4111-8111-111111111111', 'organization', '升级本方', 1, $1, $1),
			('22222222-2222-4222-8222-222222222222', 'individual', '升级往来方', 1, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert version 7 parties: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO cash_transactions (
			id, type, description, ledger_owner_party_id,
			payer_party_id, payer_party_revision, payer_kind, payer_legal_name,
			payee_party_id, payee_party_revision, payee_kind, payee_legal_name,
			amount, currency, occurred_on, account_reference,
			source_kind, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		) VALUES (
			'33333333-3333-4333-8333-333333333333', 'receipt', '升级前经营收款',
			'11111111-1111-4111-8111-111111111111',
			'22222222-2222-4222-8222-222222222222', 1, 'individual', '升级往来方',
			'11111111-1111-4111-8111-111111111111', 1, 'organization', '升级本方',
			100, 'CNY', '2026-09-19', '升级账户', 'manual', '升级回单',
			'local_user', '44444444-4444-4444-8444-444444444444', '升级管理员', $1
		)
	`, now); err != nil {
		t.Fatalf("insert version 7 cash transaction: %v", err)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions, cashTransactions int
	var transferTable, commandTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			(SELECT count(*) FROM cash_transactions),
			to_regclass('public.internal_transfers')::text,
			to_regclass('public.internal_transfer_create_commands')::text
	`).Scan(&versions, &cashTransactions, &transferTable, &commandTable); err != nil {
		t.Fatalf("inspect internal transfer migration: %v", err)
	}
	if versions != 24 || cashTransactions != 1 || transferTable == nil || commandTable == nil {
		t.Fatalf("internal transfer migration versions=%d cash=%d transfers=%v commands=%v", versions, cashTransactions, transferTable, commandTable)
	}
}

func TestAllocationReversalMigrationFromInternalTransferSchema(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
		"000004_obligation_drafts.sql",
		"000005_obligation_confirmation.sql",
		"000006_manual_cash_transactions.sql",
		"000007_single_allocations.sql",
		"000008_internal_transfers.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (id, kind, legal_name, revision, created_at, updated_at)
		VALUES
			('11111111-1111-4111-8111-111111111111', 'organization', '升级本方', 1, $1, $1),
			('22222222-2222-4222-8222-222222222222', 'individual', '升级往来方', 1, $1, $1)
	`, now); err != nil {
		t.Fatalf("insert version 8 parties: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO obligations (
			id, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			status, draft_version, created_at, updated_at
		) VALUES (
			'33333333-3333-4333-8333-333333333333', 'receivable', '升级前义务',
			'11111111-1111-4111-8111-111111111111',
			'11111111-1111-4111-8111-111111111111', 1, 'organization', '升级本方',
			'22222222-2222-4222-8222-222222222222', 1, 'individual', '升级往来方',
			100, 'CNY', '2026-10-31', 'manual', '升级义务依据',
			'draft', 1, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert version 8 obligation: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO obligation_revisions (
			obligation_id, revision, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			source_draft_version, confirmed_by_actor_kind, confirmed_by_actor_id,
			confirmed_by_actor_display_name, confirmed_at
		) VALUES (
			'33333333-3333-4333-8333-333333333333', 1, 'receivable', '升级前义务',
			'11111111-1111-4111-8111-111111111111',
			'11111111-1111-4111-8111-111111111111', 1, 'organization', '升级本方',
			'22222222-2222-4222-8222-222222222222', 1, 'individual', '升级往来方',
			100, 'CNY', '2026-10-31', 'manual', '升级义务依据',
			1, 'local_user', '44444444-4444-4444-8444-444444444444', '升级管理员', $1
		)
	`, now); err != nil {
		t.Fatalf("insert version 8 obligation revision: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		UPDATE obligations
		SET status = 'open', current_revision = 1, updated_at = $1
		WHERE id = '33333333-3333-4333-8333-333333333333'
	`, now); err != nil {
		t.Fatalf("open version 8 obligation: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO cash_transactions (
			id, type, description, ledger_owner_party_id,
			payer_party_id, payer_party_revision, payer_kind, payer_legal_name,
			payee_party_id, payee_party_revision, payee_kind, payee_legal_name,
			amount, currency, occurred_on, account_reference,
			source_kind, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		) VALUES (
			'55555555-5555-4555-8555-555555555555', 'receipt', '升级前经营收款',
			'11111111-1111-4111-8111-111111111111',
			'22222222-2222-4222-8222-222222222222', 1, 'individual', '升级往来方',
			'11111111-1111-4111-8111-111111111111', 1, 'organization', '升级本方',
			100, 'CNY', '2026-09-19', '升级账户', 'manual', '升级回单',
			'local_user', '44444444-4444-4444-8444-444444444444', '升级管理员', $1
		)
	`, now); err != nil {
		t.Fatalf("insert version 8 cash transaction: %v", err)
	}
	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id, applied_by_actor_display_name, applied_at
		) VALUES (
			'66666666-6666-4666-8666-666666666666',
			'55555555-5555-4555-8555-555555555555',
			'33333333-3333-4333-8333-333333333333', 1,
			40, 'CNY', 'manual', '升级前分配依据',
			'local_user', '44444444-4444-4444-8444-444444444444', '升级管理员', $1
		)
	`, now); err != nil {
		t.Fatalf("insert version 8 allocation: %v", err)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions, allocations, reversals int
	var netAllocated string
	var reversalTable, commandTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_reversals),
			(SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)::text
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id),
			to_regclass('public.allocation_reversals')::text,
			to_regclass('public.allocation_reversal_create_commands')::text
	`).Scan(&versions, &allocations, &reversals, &netAllocated, &reversalTable, &commandTable); err != nil {
		t.Fatalf("inspect allocation reversal migration: %v", err)
	}
	if versions != 24 || allocations != 1 || reversals != 0 || netAllocated != "40.000000" || reversalTable == nil || commandTable == nil {
		t.Fatalf("allocation reversal migration versions=%d allocations=%d reversals=%d net=%s tables=%v/%v",
			versions, allocations, reversals, netAllocated, reversalTable, commandTable)
	}
}

func TestCashTransactionReversalMigrationFromAllocationReversalSchema(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
		"000004_obligation_drafts.sql",
		"000005_obligation_confirmation.sql",
		"000006_manual_cash_transactions.sql",
		"000007_single_allocations.sql",
		"000008_internal_transfers.sql",
		"000009_allocation_reversals.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (id, kind, legal_name, revision, created_at, updated_at)
		VALUES
			('a1000000-0000-4000-8000-000000000001', 'organization', '冲销升级本方', 1, clock_timestamp(), clock_timestamp()),
			('a1000000-0000-4000-8000-000000000002', 'individual', '冲销升级往来方', 1, clock_timestamp(), clock_timestamp());

		INSERT INTO obligations (
			id, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			status, draft_version, created_at, updated_at
		) VALUES (
			'a2000000-0000-4000-8000-000000000001', 'receivable', '冲销升级义务',
			'a1000000-0000-4000-8000-000000000001',
			'a1000000-0000-4000-8000-000000000001', 1, 'organization', '冲销升级本方',
			'a1000000-0000-4000-8000-000000000002', 1, 'individual', '冲销升级往来方',
			100, 'CNY', '2026-10-31', 'manual', '冲销升级义务依据',
			'draft', 1, clock_timestamp(), clock_timestamp()
		);

		INSERT INTO obligation_revisions (
			obligation_id, revision, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			source_draft_version, confirmed_by_actor_kind, confirmed_by_actor_id,
			confirmed_by_actor_display_name, confirmed_at
		) VALUES (
			'a2000000-0000-4000-8000-000000000001', 1, 'receivable', '冲销升级义务',
			'a1000000-0000-4000-8000-000000000001',
			'a1000000-0000-4000-8000-000000000001', 1, 'organization', '冲销升级本方',
			'a1000000-0000-4000-8000-000000000002', 1, 'individual', '冲销升级往来方',
			100, 'CNY', '2026-10-31', 'manual', '冲销升级义务依据',
			1, 'local_user', 'a3000000-0000-4000-8000-000000000001', '冲销升级管理员', clock_timestamp()
		);

		UPDATE obligations
		SET status = 'open', current_revision = 1, updated_at = clock_timestamp()
		WHERE id = 'a2000000-0000-4000-8000-000000000001';

		INSERT INTO cash_transactions (
			id, type, description, ledger_owner_party_id,
			payer_party_id, payer_party_revision, payer_kind, payer_legal_name,
			payee_party_id, payee_party_revision, payee_kind, payee_legal_name,
			amount, currency, occurred_on, account_reference,
			source_kind, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		) VALUES (
			'a4000000-0000-4000-8000-000000000001', 'receipt', '冲销升级收款',
			'a1000000-0000-4000-8000-000000000001',
			'a1000000-0000-4000-8000-000000000002', 1, 'individual', '冲销升级往来方',
			'a1000000-0000-4000-8000-000000000001', 1, 'organization', '冲销升级本方',
			100, 'CNY', '2026-09-19', '冲销升级账户', 'manual', '冲销升级回单',
			'local_user', 'a3000000-0000-4000-8000-000000000001', '冲销升级管理员', clock_timestamp()
		);

		INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id, applied_by_actor_display_name, applied_at
		) VALUES (
			'a5000000-0000-4000-8000-000000000001',
			'a4000000-0000-4000-8000-000000000001',
			'a2000000-0000-4000-8000-000000000001', 1,
			40, 'CNY', 'manual', '冲销升级分配依据',
			'local_user', 'a3000000-0000-4000-8000-000000000001', '冲销升级管理员', clock_timestamp()
		);

		INSERT INTO allocation_reversals (
			id, allocation_id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, reversal_reason,
			reversed_by_actor_kind, reversed_by_actor_id,
			reversed_by_actor_display_name, reversed_at
		) VALUES (
			'a6000000-0000-4000-8000-000000000001',
			'a5000000-0000-4000-8000-000000000001',
			'a4000000-0000-4000-8000-000000000001',
			'a2000000-0000-4000-8000-000000000001', 1,
			40, 'CNY', 'manual', '冲销升级前已反向',
			'local_user', 'a3000000-0000-4000-8000-000000000001', '冲销升级管理员', clock_timestamp()
		)
	`); err != nil {
		t.Fatalf("insert version 9 cash reversal fixture: %v", err)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions, cashTransactions, allocations, allocationReversals, cashReversals int
	var netAllocated string
	var reversalTable, commandTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			(SELECT count(*) FROM cash_transactions),
			(SELECT count(*) FROM allocations),
			(SELECT count(*) FROM allocation_reversals),
			(SELECT count(*) FROM cash_transaction_reversals),
			(SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)::text
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id),
			to_regclass('public.cash_transaction_reversals')::text,
			to_regclass('public.cash_transaction_reversal_create_commands')::text
	`).Scan(
		&versions, &cashTransactions, &allocations, &allocationReversals, &cashReversals,
		&netAllocated, &reversalTable, &commandTable,
	); err != nil {
		t.Fatalf("inspect cash transaction reversal migration: %v", err)
	}
	if versions != 24 || cashTransactions != 1 || allocations != 1 || allocationReversals != 1 ||
		cashReversals != 0 || netAllocated != "0.000000" || reversalTable == nil || commandTable == nil {
		t.Fatalf("cash reversal migration versions=%d cash=%d allocations=%d/%d cash_reversals=%d net=%s tables=%v/%v",
			versions, cashTransactions, allocations, allocationReversals, cashReversals, netAllocated, reversalTable, commandTable)
	}
}

func TestObligationAmountAdjustmentMigrationFromCashReversalSchema(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
		"000004_obligation_drafts.sql",
		"000005_obligation_confirmation.sql",
		"000006_manual_cash_transactions.sql",
		"000007_single_allocations.sql",
		"000008_internal_transfers.sql",
		"000009_allocation_reversals.sql",
		"000010_cash_transaction_reversals.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (id, kind, legal_name, revision, created_at, updated_at)
		VALUES
			('b1000000-0000-4000-8000-000000000001', 'organization', '调整升级本方', 1, clock_timestamp(), clock_timestamp()),
			('b1000000-0000-4000-8000-000000000002', 'individual', '调整升级往来方', 1, clock_timestamp(), clock_timestamp());

		INSERT INTO obligations (
			id, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			status, draft_version, created_at, updated_at
		) VALUES (
			'b2000000-0000-4000-8000-000000000001', 'receivable', '调整升级义务',
			'b1000000-0000-4000-8000-000000000001',
			'b1000000-0000-4000-8000-000000000001', 1, 'organization', '调整升级本方',
			'b1000000-0000-4000-8000-000000000002', 1, 'individual', '调整升级往来方',
			100, 'CNY', '2026-12-31', 'manual', '调整升级义务依据',
			'draft', 1, clock_timestamp(), clock_timestamp()
		);

		INSERT INTO obligation_revisions (
			obligation_id, revision, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			source_draft_version, confirmed_by_actor_kind, confirmed_by_actor_id,
			confirmed_by_actor_display_name, confirmed_at
		) VALUES (
			'b2000000-0000-4000-8000-000000000001', 1, 'receivable', '调整升级义务',
			'b1000000-0000-4000-8000-000000000001',
			'b1000000-0000-4000-8000-000000000001', 1, 'organization', '调整升级本方',
			'b1000000-0000-4000-8000-000000000002', 1, 'individual', '调整升级往来方',
			100, 'CNY', '2026-12-31', 'manual', '调整升级义务依据',
			1, 'local_user', 'b3000000-0000-4000-8000-000000000001', '调整升级管理员', clock_timestamp()
		);

		INSERT INTO obligation_confirmation_commands (
			command_id, obligation_id, expected_draft_version, resulting_revision,
			payload_hash, actor_kind, actor_id, actor_display_name, confirmed_at
		) VALUES (
			'b3000000-0000-4000-8000-000000000002',
			'b2000000-0000-4000-8000-000000000001', 1, 1,
			decode(repeat('ab', 32), 'hex'), 'local_user',
			'b3000000-0000-4000-8000-000000000001', '调整升级管理员', clock_timestamp()
		);

		UPDATE obligations
		SET status = 'open', current_revision = 1, updated_at = clock_timestamp()
		WHERE id = 'b2000000-0000-4000-8000-000000000001';

		INSERT INTO cash_transactions (
			id, type, description, ledger_owner_party_id,
			payer_party_id, payer_party_revision, payer_kind, payer_legal_name,
			payee_party_id, payee_party_revision, payee_kind, payee_legal_name,
			amount, currency, occurred_on, account_reference,
			source_kind, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		) VALUES
		(
			'b4000000-0000-4000-8000-000000000001', 'receipt', '调整升级有效收款',
			'b1000000-0000-4000-8000-000000000001',
			'b1000000-0000-4000-8000-000000000002', 1, 'individual', '调整升级往来方',
			'b1000000-0000-4000-8000-000000000001', 1, 'organization', '调整升级本方',
			40, 'CNY', '2026-09-19', '调整升级账户一', 'manual', '调整升级回单一',
			'local_user', 'b3000000-0000-4000-8000-000000000001', '调整升级管理员', clock_timestamp()
		),
		(
			'b4000000-0000-4000-8000-000000000002', 'receipt', '调整升级已冲销收款',
			'b1000000-0000-4000-8000-000000000001',
			'b1000000-0000-4000-8000-000000000002', 1, 'individual', '调整升级往来方',
			'b1000000-0000-4000-8000-000000000001', 1, 'organization', '调整升级本方',
			20, 'CNY', '2026-09-19', '调整升级账户二', 'manual', '调整升级回单二',
			'local_user', 'b3000000-0000-4000-8000-000000000001', '调整升级管理员', clock_timestamp()
		);

		INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id, applied_by_actor_display_name, applied_at
		) VALUES
		(
			'b5000000-0000-4000-8000-000000000001',
			'b4000000-0000-4000-8000-000000000001',
			'b2000000-0000-4000-8000-000000000001', 1,
			40, 'CNY', 'manual', '调整升级有效分配',
			'local_user', 'b3000000-0000-4000-8000-000000000001', '调整升级管理员', clock_timestamp()
		),
		(
			'b5000000-0000-4000-8000-000000000002',
			'b4000000-0000-4000-8000-000000000002',
			'b2000000-0000-4000-8000-000000000001', 1,
			20, 'CNY', 'manual', '调整升级已反向分配',
			'local_user', 'b3000000-0000-4000-8000-000000000001', '调整升级管理员', clock_timestamp()
		);

		INSERT INTO allocation_reversals (
			id, allocation_id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, reversal_reason,
			reversed_by_actor_kind, reversed_by_actor_id,
			reversed_by_actor_display_name, reversed_at
		) VALUES (
			'b6000000-0000-4000-8000-000000000001',
			'b5000000-0000-4000-8000-000000000002',
			'b4000000-0000-4000-8000-000000000002',
			'b2000000-0000-4000-8000-000000000001', 1,
			20, 'CNY', 'manual', '调整升级前已反向',
			'local_user', 'b3000000-0000-4000-8000-000000000001', '调整升级管理员', clock_timestamp()
		);

		INSERT INTO cash_transaction_reversals (
			id, cash_transaction_id, transaction_type, amount, currency,
			source_kind, reversal_reason,
			reversed_by_actor_kind, reversed_by_actor_id,
			reversed_by_actor_display_name, reversed_at
		) VALUES (
			'b7000000-0000-4000-8000-000000000001',
			'b4000000-0000-4000-8000-000000000002', 'receipt', 20, 'CNY',
			'manual', '调整升级前资金冲销',
			'local_user', 'b3000000-0000-4000-8000-000000000001', '调整升级管理员', clock_timestamp()
		)
	`); err != nil {
		t.Fatalf("insert version 10 amount adjustment fixture: %v", err)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions, revisionCount, adjustmentCount, commandCount, allocations, allocationReversals, cashReversals int
	var rootOriginal, effectiveAmount, netAllocated, outstanding, revisionKind, recordedKind, recordedID, recordedName string
	var sourceDraft int64
	var sourceRevision *int64
	var adjustmentTable, adjustmentCommandTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			o.original_amount::text,
			r.effective_amount::text,
			(COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0))::text,
			(r.effective_amount - (COALESCE(SUM(a.amount), 0) - COALESCE(SUM(ar.amount), 0)))::text,
			r.revision_kind,
			r.source_draft_version,
			r.source_revision,
			r.recorded_by_actor_kind,
			r.recorded_by_actor_id,
			r.recorded_by_actor_display_name,
			(SELECT count(*) FROM obligation_revisions WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_amount_adjustments WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_amount_adjustment_commands WHERE obligation_id = o.id),
			(SELECT count(*) FROM allocations WHERE obligation_id = o.id),
			(SELECT count(*) FROM allocation_reversals WHERE obligation_id = o.id),
			(SELECT count(*) FROM cash_transaction_reversals),
			to_regclass('public.obligation_amount_adjustments')::text,
			to_regclass('public.obligation_amount_adjustment_commands')::text
		FROM obligations o
		JOIN obligation_revisions r ON r.obligation_id = o.id AND r.revision = o.current_revision
		LEFT JOIN allocations a ON a.obligation_id = o.id
		LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
		WHERE o.id = 'b2000000-0000-4000-8000-000000000001'
		GROUP BY o.id, r.obligation_id, r.revision
	`).Scan(
		&versions, &rootOriginal, &effectiveAmount, &netAllocated, &outstanding,
		&revisionKind, &sourceDraft, &sourceRevision,
		&recordedKind, &recordedID, &recordedName,
		&revisionCount, &adjustmentCount, &commandCount,
		&allocations, &allocationReversals, &cashReversals,
		&adjustmentTable, &adjustmentCommandTable,
	); err != nil {
		t.Fatalf("inspect obligation amount adjustment migration: %v", err)
	}
	if versions != 24 || rootOriginal != "100.000000" || effectiveAmount != "100.000000" ||
		netAllocated != "40.000000" || outstanding != "60.000000" || revisionKind != "confirmation" ||
		sourceDraft != 1 || sourceRevision != nil || recordedKind != "local_user" ||
		recordedID != "b3000000-0000-4000-8000-000000000001" || recordedName != "调整升级管理员" ||
		revisionCount != 1 || adjustmentCount != 0 || commandCount != 0 ||
		allocations != 2 || allocationReversals != 1 || cashReversals != 1 ||
		adjustmentTable == nil || adjustmentCommandTable == nil {
		t.Fatalf("amount adjustment migration versions=%d amounts=%s/%s/%s/%s revision=%s/%d/%v actor=%s/%s/%s facts=%d/%d/%d/%d/%d/%d tables=%v/%v",
			versions, rootOriginal, effectiveAmount, netAllocated, outstanding,
			revisionKind, sourceDraft, sourceRevision, recordedKind, recordedID, recordedName,
			revisionCount, adjustmentCount, commandCount, allocations, allocationReversals, cashReversals,
			adjustmentTable, adjustmentCommandTable)
	}
}

func TestObligationRemissionMigrationFromAmountAdjustmentSchema(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
		"000004_obligation_drafts.sql",
		"000005_obligation_confirmation.sql",
		"000006_manual_cash_transactions.sql",
		"000007_single_allocations.sql",
		"000008_internal_transfers.sql",
		"000009_allocation_reversals.sql",
		"000010_cash_transaction_reversals.sql",
		"000011_obligation_amount_adjustments.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (id, kind, legal_name, revision, created_at, updated_at)
		VALUES
			('c1000000-0000-4000-8000-000000000001', 'organization', '减免升级本方', 1, now(), now()),
			('c1000000-0000-4000-8000-000000000002', 'individual', '减免升级往来方', 1, now(), now());

		INSERT INTO obligations (
			id, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			status, draft_version, created_at, updated_at
		) VALUES (
			'c2000000-0000-4000-8000-000000000001', 'receivable', '减免升级义务',
			'c1000000-0000-4000-8000-000000000001',
			'c1000000-0000-4000-8000-000000000001', 1, 'organization', '减免升级本方',
			'c1000000-0000-4000-8000-000000000002', 1, 'individual', '减免升级往来方',
			100, 'CNY', '2026-12-31', 'manual', '减免升级义务依据',
			'draft', 1, now(), now()
		);

		INSERT INTO obligation_revisions (
			obligation_id, revision, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind, creditor_legal_name,
			debtor_party_id, debtor_party_revision, debtor_kind, debtor_legal_name,
			effective_amount, currency, due_on, source_kind, manual_source_reference,
			source_draft_version, recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at, revision_kind, source_revision
		) VALUES (
			'c2000000-0000-4000-8000-000000000001', 1, 'receivable', '减免升级义务',
			'c1000000-0000-4000-8000-000000000001',
			'c1000000-0000-4000-8000-000000000001', 1, 'organization', '减免升级本方',
			'c1000000-0000-4000-8000-000000000002', 1, 'individual', '减免升级往来方',
			100, 'CNY', '2026-12-31', 'manual', '减免升级义务依据',
			1, 'local_user', 'c3000000-0000-4000-8000-000000000001', '减免升级管理员', now(),
			'confirmation', NULL
		);

		INSERT INTO obligation_confirmation_commands (
			command_id, obligation_id, expected_draft_version, resulting_revision,
			payload_hash, actor_kind, actor_id, actor_display_name, confirmed_at
		) VALUES (
			'c3000000-0000-4000-8000-000000000002',
			'c2000000-0000-4000-8000-000000000001', 1, 1,
			decode(repeat('ab', 32), 'hex'), 'local_user',
			'c3000000-0000-4000-8000-000000000001', '减免升级管理员', now()
		);

		UPDATE obligations
		SET status = 'open', current_revision = 1, updated_at = now()
		WHERE id = 'c2000000-0000-4000-8000-000000000001';

		INSERT INTO obligation_amount_adjustments (
			id, obligation_id, source_revision, resulting_revision,
			adjustment_type, adjustment_amount, previous_effective_amount,
			resulting_effective_amount, currency, source_kind, adjustment_reason,
			adjusted_by_actor_kind, adjusted_by_actor_id, adjusted_by_actor_display_name, adjusted_at
		) VALUES (
			'c4000000-0000-4000-8000-000000000001',
			'c2000000-0000-4000-8000-000000000001', 1, 2,
			'increase', 20, 100, 120, 'CNY', 'manual', '减免升级前金额增加',
			'local_user', 'c3000000-0000-4000-8000-000000000001', '减免升级管理员', now()
		);

		INSERT INTO obligation_revisions (
			obligation_id, revision, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			effective_amount, currency, due_on,
			source_kind, manual_source_reference, source_draft_version,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at,
			revision_kind, source_revision
		)
		SELECT
			obligation_id, 2, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			120, currency, due_on,
			source_kind, manual_source_reference, NULL,
			'local_user', 'c3000000-0000-4000-8000-000000000001', '减免升级管理员', now(),
			'amount_adjustment', 1
		FROM obligation_revisions
		WHERE obligation_id = 'c2000000-0000-4000-8000-000000000001' AND revision = 1;

		INSERT INTO obligation_amount_adjustment_commands (
			command_id, obligation_amount_adjustment_id, obligation_id,
			expected_current_revision, resulting_revision, payload_hash,
			actor_kind, actor_id, actor_display_name, adjusted_at
		) VALUES (
			'c4000000-0000-4000-8000-000000000002',
			'c4000000-0000-4000-8000-000000000001',
			'c2000000-0000-4000-8000-000000000001', 1, 2,
			decode(repeat('cd', 32), 'hex'), 'local_user',
			'c3000000-0000-4000-8000-000000000001', '减免升级管理员', now()
		);

		UPDATE obligations
		SET current_revision = 2, updated_at = now()
		WHERE id = 'c2000000-0000-4000-8000-000000000001';

		INSERT INTO cash_transactions (
			id, type, description, ledger_owner_party_id,
			payer_party_id, payer_party_revision, payer_kind, payer_legal_name,
			payee_party_id, payee_party_revision, payee_kind, payee_legal_name,
			amount, currency, occurred_on, account_reference,
			source_kind, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id, recorded_by_actor_display_name, recorded_at
		) VALUES (
			'c5000000-0000-4000-8000-000000000001', 'receipt', '减免升级收款',
			'c1000000-0000-4000-8000-000000000001',
			'c1000000-0000-4000-8000-000000000002', 1, 'individual', '减免升级往来方',
			'c1000000-0000-4000-8000-000000000001', 1, 'organization', '减免升级本方',
			40, 'CNY', '2026-09-19', '减免升级账户', 'manual', '减免升级回单',
			'local_user', 'c3000000-0000-4000-8000-000000000001', '减免升级管理员', now()
		);

		INSERT INTO allocations (
			id, cash_transaction_id, obligation_id, obligation_revision,
			amount, currency, source_kind, manual_source_reference,
			applied_by_actor_kind, applied_by_actor_id, applied_by_actor_display_name, applied_at
		) VALUES (
			'c6000000-0000-4000-8000-000000000001',
			'c5000000-0000-4000-8000-000000000001',
			'c2000000-0000-4000-8000-000000000001', 2,
			40, 'CNY', 'manual', '减免升级分配',
			'local_user', 'c3000000-0000-4000-8000-000000000001', '减免升级管理员', now()
		)
	`); err != nil {
		t.Fatalf("insert version 11 remission fixture: %v", err)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions, revisions, adjustments, allocations, remissions, commands int
	var effective, allocated, remitted, outstanding string
	var remissionTable, commandTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			r.effective_amount::text,
			(SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)::text
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE a.obligation_id = o.id),
			(SELECT COALESCE(SUM(x.amount), 0)::text FROM obligation_remissions x WHERE x.obligation_id = o.id),
			(r.effective_amount
			 - (SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
			    FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			    WHERE a.obligation_id = o.id)
			 - (SELECT COALESCE(SUM(x.amount), 0) FROM obligation_remissions x WHERE x.obligation_id = o.id))::text,
			(SELECT count(*) FROM obligation_revisions WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_amount_adjustments WHERE obligation_id = o.id),
			(SELECT count(*) FROM allocations WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_remissions WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_remission_commands WHERE obligation_id = o.id),
			to_regclass('public.obligation_remissions')::text,
			to_regclass('public.obligation_remission_commands')::text
		FROM obligations o
		JOIN obligation_revisions r ON r.obligation_id = o.id AND r.revision = o.current_revision
		WHERE o.id = 'c2000000-0000-4000-8000-000000000001'
	`).Scan(
		&versions, &effective, &allocated, &remitted, &outstanding,
		&revisions, &adjustments, &allocations, &remissions, &commands,
		&remissionTable, &commandTable,
	); err != nil {
		t.Fatalf("inspect obligation remission migration: %v", err)
	}
	if versions != 24 || effective != "120.000000" || allocated != "40.000000" ||
		remitted != "0" || outstanding != "80.000000" || revisions != 2 || adjustments != 1 ||
		allocations != 1 || remissions != 0 || commands != 0 || remissionTable == nil || commandTable == nil {
		t.Fatalf("remission migration versions=%d amounts=%s/%s/%s/%s facts=%d/%d/%d/%d/%d tables=%v/%v",
			versions, effective, allocated, remitted, outstanding,
			revisions, adjustments, allocations, remissions, commands, remissionTable, commandTable)
	}
}

func TestObligationClosureMigrationFromRemissionSchema(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
		"000004_obligation_drafts.sql",
		"000005_obligation_confirmation.sql",
		"000006_manual_cash_transactions.sql",
		"000007_single_allocations.sql",
		"000008_internal_transfers.sql",
		"000009_allocation_reversals.sql",
		"000010_cash_transaction_reversals.sql",
		"000011_obligation_amount_adjustments.sql",
		"000012_obligation_remissions.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	if _, err := environment.Pool.Exec(ctx, `
		INSERT INTO parties (
			id, kind, legal_name, registration_identifier, revision, created_at, updated_at
		) VALUES
			('d1000000-0000-4000-8000-000000000001', 'organization', '关闭升级本方', 'CLOSE-OWNER', 1, now(), now()),
			('d1000000-0000-4000-8000-000000000002', 'individual', '关闭升级往来方', 'CLOSE-COUNTERPARTY', 1, now(), now());

		INSERT INTO obligations (
			id, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			status, draft_version, created_at, updated_at
		) VALUES (
			'd2000000-0000-4000-8000-000000000001', 'receivable', '关闭升级义务',
			'd1000000-0000-4000-8000-000000000001',
			'd1000000-0000-4000-8000-000000000001', 1, 'organization', '关闭升级本方', 'CLOSE-OWNER',
			'd1000000-0000-4000-8000-000000000002', 1, 'individual', '关闭升级往来方', 'CLOSE-COUNTERPARTY',
			100, 'CNY', '2026-12-31', 'manual', '关闭升级义务依据',
			'draft', 1, now(), now()
		);

		INSERT INTO obligation_revisions (
			obligation_id, revision, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			effective_amount, currency, due_on,
			source_kind, manual_source_reference, source_draft_version,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at,
			revision_kind, source_revision
		) VALUES (
			'd2000000-0000-4000-8000-000000000001', 1, 'receivable', '关闭升级义务',
			'd1000000-0000-4000-8000-000000000001',
			'd1000000-0000-4000-8000-000000000001', 1, 'organization', '关闭升级本方', 'CLOSE-OWNER',
			'd1000000-0000-4000-8000-000000000002', 1, 'individual', '关闭升级往来方', 'CLOSE-COUNTERPARTY',
			100, 'CNY', '2026-12-31', 'manual', '关闭升级义务依据', 1,
			'local_user', 'd3000000-0000-4000-8000-000000000001', '关闭升级管理员', now(),
			'confirmation', NULL
		);

		INSERT INTO obligation_confirmation_commands (
			command_id, obligation_id, expected_draft_version, resulting_revision,
			payload_hash, actor_kind, actor_id, actor_display_name, confirmed_at
		) VALUES (
			'd3000000-0000-4000-8000-000000000002',
			'd2000000-0000-4000-8000-000000000001', 1, 1,
			decode(repeat('ab', 32), 'hex'), 'local_user',
			'd3000000-0000-4000-8000-000000000001', '关闭升级管理员', now()
		);

		UPDATE obligations
		SET status = 'open', current_revision = 1, updated_at = now()
		WHERE id = 'd2000000-0000-4000-8000-000000000001';

		INSERT INTO obligation_remissions (
			id, obligation_id, obligation_revision,
			amount, recorded_effective_amount, currency,
			source_kind, remission_reason, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		) VALUES (
			'd4000000-0000-4000-8000-000000000001',
			'd2000000-0000-4000-8000-000000000001', 1,
			40, 100, 'CNY', 'manual', '关闭升级前部分减免', '关闭升级前减免确认函',
			'local_user', 'd3000000-0000-4000-8000-000000000001', '关闭升级管理员', now()
		)
	`); err != nil {
		t.Fatalf("insert version 12 closure fixture: %v", err)
	}

	var beforeVersions int
	var closureTableBefore *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT count(*), to_regclass('public.obligation_closures')::text
		FROM schema_migrations
	`).Scan(&beforeVersions, &closureTableBefore); err != nil {
		t.Fatalf("inspect version 12 closure state: %v", err)
	}
	if beforeVersions != 12 || closureTableBefore != nil {
		t.Fatalf("version 12 closure state versions=%d table=%v", beforeVersions, closureTableBefore)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions, revisions, remissions, closures, commands int
	var status, effective, allocated, remitted, outstanding string
	var closureTable, commandTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			o.status,
			r.effective_amount::text,
			(SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)::text
			 FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			 WHERE a.obligation_id = o.id),
			(SELECT COALESCE(SUM(x.amount), 0)::text FROM obligation_remissions x WHERE x.obligation_id = o.id),
			(r.effective_amount
			 - (SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
			    FROM allocations a LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
			    WHERE a.obligation_id = o.id)
			 - (SELECT COALESCE(SUM(x.amount), 0) FROM obligation_remissions x WHERE x.obligation_id = o.id))::text,
			(SELECT count(*) FROM obligation_revisions WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_remissions WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_closures WHERE obligation_id = o.id),
			(SELECT count(*) FROM obligation_closure_commands WHERE obligation_id = o.id),
			to_regclass('public.obligation_closures')::text,
			to_regclass('public.obligation_closure_commands')::text
		FROM obligations o
		JOIN obligation_revisions r ON r.obligation_id = o.id AND r.revision = o.current_revision
		WHERE o.id = 'd2000000-0000-4000-8000-000000000001'
	`).Scan(
		&versions, &status, &effective, &allocated, &remitted, &outstanding,
		&revisions, &remissions, &closures, &commands, &closureTable, &commandTable,
	); err != nil {
		t.Fatalf("inspect obligation closure migration: %v", err)
	}
	if versions != 24 || status != "open" || effective != "100.000000" || allocated != "0" ||
		remitted != "40.000000" || outstanding != "60.000000" || revisions != 1 || remissions != 1 ||
		closures != 0 || commands != 0 || closureTable == nil || commandTable == nil {
		t.Fatalf("closure migration versions=%d status=%s amounts=%s/%s/%s/%s facts=%d/%d/%d/%d tables=%v/%v",
			versions, status, effective, allocated, remitted, outstanding,
			revisions, remissions, closures, commands, closureTable, commandTable)
	}
}

func TestObligationRecordedTimeGuardMigrationPreservesValidVersion13Facts(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	confirmationAt := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	remissionAt := confirmationAt.Add(time.Hour)
	prepareRecordedTimeMigrationFixture(t, ctx, environment, confirmationAt, remissionAt)

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions, monotonicTriggers int
	var storedConfirmationAt, storedRemissionAt time.Time
	var guardFunction, revisionIndex *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			(SELECT count(*) FROM pg_trigger
			 WHERE tgname LIKE 'zz_%_recorded_time_monotonic' AND NOT tgisinternal),
			to_regprocedure('public.obligation_latest_recorded_fact_at(uuid)')::text,
			to_regclass('public.obligation_revisions_recorded_revision_idx')::text,
			(SELECT recorded_at FROM obligation_revisions
			 WHERE obligation_id = 'e2000000-0000-4000-8000-000000000001' AND revision = 1),
			(SELECT recorded_at FROM obligation_remissions
			 WHERE id = 'e4000000-0000-4000-8000-000000000001')
	`).Scan(&versions, &monotonicTriggers, &guardFunction, &revisionIndex, &storedConfirmationAt, &storedRemissionAt); err != nil {
		t.Fatalf("inspect recorded-time migration: %v", err)
	}
	if versions != 24 || monotonicTriggers != 6 || guardFunction == nil || revisionIndex == nil ||
		!storedConfirmationAt.Equal(confirmationAt) || !storedRemissionAt.Equal(remissionAt) {
		t.Fatalf("recorded-time migration versions=%d triggers=%d function=%v index=%v times=%v/%v",
			versions, monotonicTriggers, guardFunction, revisionIndex, storedConfirmationAt, storedRemissionAt)
	}
}

func TestCashExpectationMigrationFromRecordedTimeSchemaPreservesFacts(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	confirmationAt := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	remissionAt := confirmationAt.Add(time.Hour)
	prepareRecordedTimeMigrationFixture(t, ctx, environment, confirmationAt, remissionAt)
	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire version 14 migration connection: %v", err)
	}
	if err := applyMigration(ctx, connection, "000014_obligation_recorded_time_guards.sql"); err != nil {
		connection.Release()
		t.Fatalf("apply version 14 migration: %v", err)
	}
	connection.Release()

	var beforeVersions, beforeObligations, beforeRevisions, beforeRemissions int
	var expectationTableBefore *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			(SELECT count(*) FROM obligations),
			(SELECT count(*) FROM obligation_revisions),
			(SELECT count(*) FROM obligation_remissions),
			to_regclass('public.obligation_cash_expectations')::text
	`).Scan(&beforeVersions, &beforeObligations, &beforeRevisions, &beforeRemissions, &expectationTableBefore); err != nil {
		t.Fatalf("inspect version 14 state: %v", err)
	}
	if beforeVersions != 14 || beforeObligations != 1 || beforeRevisions != 1 || beforeRemissions != 1 || expectationTableBefore != nil {
		t.Fatalf("version 14 state versions=%d facts=%d/%d/%d expectation_table=%v",
			beforeVersions, beforeObligations, beforeRevisions, beforeRemissions, expectationTableBefore)
	}

	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, environment.Pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var versions, obligations, revisions, remissions, expectations, commands int
	var expectationTable, commandTable *string
	if err := environment.Pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM schema_migrations),
			(SELECT count(*) FROM obligations),
			(SELECT count(*) FROM obligation_revisions),
			(SELECT count(*) FROM obligation_remissions),
			(SELECT count(*) FROM obligation_cash_expectations),
			(SELECT count(*) FROM obligation_cash_expectation_commands),
			to_regclass('public.obligation_cash_expectations')::text,
			to_regclass('public.obligation_cash_expectation_commands')::text
	`).Scan(&versions, &obligations, &revisions, &remissions, &expectations, &commands, &expectationTable, &commandTable); err != nil {
		t.Fatalf("inspect version 15 state: %v", err)
	}
	if versions != 24 || obligations != beforeObligations || revisions != beforeRevisions || remissions != beforeRemissions ||
		expectations != 0 || commands != 0 || expectationTable == nil || commandTable == nil {
		t.Fatalf("version 15 state versions=%d facts=%d/%d/%d expectations=%d/%d tables=%v/%v",
			versions, obligations, revisions, remissions, expectations, commands, expectationTable, commandTable)
	}
}

func TestObligationRecordedTimeGuardMigrationRejectsCausallyInvertedVersion13Facts(t *testing.T) {
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	confirmationAt := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	prepareRecordedTimeMigrationFixture(t, ctx, environment, confirmationAt, confirmationAt.Add(-time.Hour))

	err := Migrate(ctx, environment.Pool)
	if err == nil || !strings.Contains(err.Error(), "causally inverted recorded times") {
		t.Fatalf("Migrate() error = %v", err)
	}
	var versions int
	var guardFunction *string
	if queryErr := environment.Pool.QueryRow(ctx, `
		SELECT count(*), to_regprocedure('public.obligation_latest_recorded_fact_at(uuid)')::text
		FROM schema_migrations
	`).Scan(&versions, &guardFunction); queryErr != nil {
		t.Fatalf("inspect rejected migration: %v", queryErr)
	}
	if versions != 13 || guardFunction != nil {
		t.Fatalf("rejected migration versions=%d function=%v", versions, guardFunction)
	}
}

func prepareRecordedTimeMigrationFixture(
	t *testing.T,
	ctx context.Context,
	environment testdatabase.Environment,
	confirmationAt, remissionAt time.Time,
) {
	t.Helper()
	connection, err := environment.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire recorded-time migration connection: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		CREATE TABLE schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT clock_timestamp()
		)
	`); err != nil {
		connection.Release()
		t.Fatalf("create historical schema_migrations: %v", err)
	}
	for _, version := range []string{
		"000001_bootstrap.sql",
		"000002_local_administrator_sessions.sql",
		"000003_parties.sql",
		"000004_obligation_drafts.sql",
		"000005_obligation_confirmation.sql",
		"000006_manual_cash_transactions.sql",
		"000007_single_allocations.sql",
		"000008_internal_transfers.sql",
		"000009_allocation_reversals.sql",
		"000010_cash_transaction_reversals.sql",
		"000011_obligation_amount_adjustments.sql",
		"000012_obligation_remissions.sql",
		"000013_obligation_closures.sql",
	} {
		if err := applyMigration(ctx, connection, version); err != nil {
			connection.Release()
			t.Fatalf("apply historical migration %s: %v", version, err)
		}
	}
	connection.Release()

	tx, err := environment.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin recorded-time fixture: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO parties (
			id, kind, legal_name, registration_identifier, revision, created_at, updated_at
		) VALUES
			('e1000000-0000-4000-8000-000000000001', 'organization', '时间门禁本方', 'TIME-OWNER', 1, $1, $1),
			('e1000000-0000-4000-8000-000000000002', 'individual', '时间门禁往来方', NULL, 1, $1, $1)
	`, confirmationAt); err != nil {
		t.Fatalf("insert recorded-time parties: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO obligations (
			id, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			original_amount, currency, due_on, source_kind, manual_source_reference,
			status, draft_version, created_at, updated_at
		) VALUES (
			'e2000000-0000-4000-8000-000000000001', 'receivable', '时间门禁升级义务',
			'e1000000-0000-4000-8000-000000000001',
			'e1000000-0000-4000-8000-000000000001', 1, 'organization', '时间门禁本方', 'TIME-OWNER',
			'e1000000-0000-4000-8000-000000000002', 1, 'individual', '时间门禁往来方', NULL,
			100, 'CNY', '2026-12-31', 'manual', '时间门禁升级义务依据',
			'draft', 1, $1, $1
		)
	`, confirmationAt); err != nil {
		t.Fatalf("insert recorded-time obligation: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_revisions (
			obligation_id, revision, direction, title, ledger_owner_party_id,
			creditor_party_id, creditor_party_revision, creditor_kind,
			creditor_legal_name, creditor_registration_identifier,
			debtor_party_id, debtor_party_revision, debtor_kind,
			debtor_legal_name, debtor_registration_identifier,
			effective_amount, currency, due_on,
			source_kind, manual_source_reference, source_draft_version,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at,
			revision_kind, source_revision
		) VALUES (
			'e2000000-0000-4000-8000-000000000001', 1, 'receivable', '时间门禁升级义务',
			'e1000000-0000-4000-8000-000000000001',
			'e1000000-0000-4000-8000-000000000001', 1, 'organization', '时间门禁本方', 'TIME-OWNER',
			'e1000000-0000-4000-8000-000000000002', 1, 'individual', '时间门禁往来方', NULL,
			100, 'CNY', '2026-12-31', 'manual', '时间门禁升级义务依据', 1,
			'local_user', 'e3000000-0000-4000-8000-000000000001', '时间门禁管理员', $1,
			'confirmation', NULL
		)
	`, confirmationAt); err != nil {
		t.Fatalf("insert recorded-time revision: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_confirmation_commands (
			command_id, obligation_id, expected_draft_version, resulting_revision,
			payload_hash, actor_kind, actor_id, actor_display_name, confirmed_at
		) VALUES (
			'e3000000-0000-4000-8000-000000000002',
			'e2000000-0000-4000-8000-000000000001', 1, 1,
			decode(repeat('ab', 32), 'hex'), 'local_user',
			'e3000000-0000-4000-8000-000000000001', '时间门禁管理员', $1
		)
	`, confirmationAt); err != nil {
		t.Fatalf("insert recorded-time confirmation command: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE obligations
		SET status = 'open', current_revision = 1, updated_at = $1
		WHERE id = 'e2000000-0000-4000-8000-000000000001'
	`, confirmationAt); err != nil {
		t.Fatalf("open recorded-time obligation: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO obligation_remissions (
			id, obligation_id, obligation_revision,
			amount, recorded_effective_amount, currency,
			source_kind, remission_reason, manual_source_reference,
			recorded_by_actor_kind, recorded_by_actor_id,
			recorded_by_actor_display_name, recorded_at
		) VALUES (
			'e4000000-0000-4000-8000-000000000001',
			'e2000000-0000-4000-8000-000000000001', 1,
			40, 100, 'CNY', 'manual', '时间门禁升级前部分减免', '时间门禁减免确认函',
			'local_user', 'e3000000-0000-4000-8000-000000000001', '时间门禁管理员', $1
		)
	`, remissionAt); err != nil {
		t.Fatalf("insert recorded-time remission: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit recorded-time fixture: %v", err)
	}
}
