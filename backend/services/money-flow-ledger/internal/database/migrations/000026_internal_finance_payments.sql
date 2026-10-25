ALTER TABLE obligations DROP CONSTRAINT obligations_source_check;
ALTER TABLE obligations ADD CONSTRAINT obligations_source_check CHECK (
    (source_kind='manual' AND manual_source_reference IS NOT NULL AND manual_source_reference=btrim(manual_source_reference) AND char_length(manual_source_reference) BETWEEN 1 AND 200 AND source_application IS NULL AND source_resource_type IS NULL AND source_resource_id IS NULL AND source_revision IS NULL AND source_digest IS NULL)
    OR (source_kind='cost_outsourcing_payable_suggestion' AND manual_source_reference IS NULL AND source_application='cost-outsourcing' AND source_resource_type IN ('cost_entry','outsourcing_engagement') AND source_resource_id IS NOT NULL AND source_revision>=1 AND source_digest ~ '^sha256:[0-9a-f]{64}$')
    OR (source_kind='bonus_payable_suggestion' AND manual_source_reference IS NULL AND source_application='bonus-distribution' AND source_resource_type='bonus_allocation' AND source_resource_id IS NOT NULL AND source_revision>=1 AND source_digest ~ '^sha256:[0-9a-f]{64}$')
    OR (source_kind='payroll_payment' AND manual_source_reference IS NULL AND source_application='payroll' AND source_resource_type='payroll_line' AND source_resource_id IS NOT NULL AND source_revision>=1 AND source_digest ~ '^sha256:[0-9a-f]{64}$')
    OR (source_kind='internal_finance_payment' AND manual_source_reference IS NULL AND source_application='internal-finance-requests' AND source_resource_type='finance_request' AND source_resource_id IS NOT NULL AND source_revision>=1 AND source_digest ~ '^sha256:[0-9a-f]{64}$')
);

ALTER TABLE obligation_revisions DROP CONSTRAINT obligation_revisions_source_check;
ALTER TABLE obligation_revisions ADD CONSTRAINT obligation_revisions_source_check CHECK (
    (source_kind='manual' AND manual_source_reference IS NOT NULL AND manual_source_reference=btrim(manual_source_reference) AND char_length(manual_source_reference) BETWEEN 1 AND 200 AND source_application IS NULL AND source_resource_type IS NULL AND source_resource_id IS NULL AND source_fact_revision IS NULL AND source_digest IS NULL)
    OR (source_kind='cost_outsourcing_payable_suggestion' AND manual_source_reference IS NULL AND source_application='cost-outsourcing' AND source_resource_type IN ('cost_entry','outsourcing_engagement') AND source_resource_id IS NOT NULL AND source_fact_revision>=1 AND source_digest ~ '^sha256:[0-9a-f]{64}$')
    OR (source_kind='bonus_payable_suggestion' AND manual_source_reference IS NULL AND source_application='bonus-distribution' AND source_resource_type='bonus_allocation' AND source_resource_id IS NOT NULL AND source_fact_revision>=1 AND source_digest ~ '^sha256:[0-9a-f]{64}$')
    OR (source_kind='payroll_payment' AND manual_source_reference IS NULL AND source_application='payroll' AND source_resource_type='payroll_line' AND source_resource_id IS NOT NULL AND source_fact_revision>=1 AND source_digest ~ '^sha256:[0-9a-f]{64}$')
    OR (source_kind='internal_finance_payment' AND manual_source_reference IS NULL AND source_application='internal-finance-requests' AND source_resource_type='finance_request' AND source_resource_id IS NOT NULL AND source_fact_revision>=1 AND source_digest ~ '^sha256:[0-9a-f]{64}$')
);

CREATE TABLE internal_finance_ledger_owner_setting_revisions (
    tenant_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision>=1),
    command_id uuid NOT NULL,
    local_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    local_party_revision bigint NOT NULL CHECK (local_party_revision>=1),
    local_party_kind text NOT NULL CHECK (local_party_kind='organization'),
    local_party_legal_name text NOT NULL,
    reason text NOT NULL CHECK (reason=btrim(reason) AND char_length(reason) BETWEEN 1 AND 1000),
    actor_kind text NOT NULL,
    actor_id text NOT NULL,
    actor_display_name text NOT NULL,
    recorded_at timestamptz NOT NULL,
    request_sha256 text NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
    PRIMARY KEY (tenant_id,revision),
    UNIQUE (command_id)
);

CREATE TABLE internal_finance_payment_obligation_bindings (
    tenant_id uuid NOT NULL,
    application_id uuid NOT NULL,
    total_payable_minor bigint NOT NULL CHECK (total_payable_minor>0),
    source_revision bigint NOT NULL CHECK (source_revision>=1),
    source_digest text NOT NULL CHECK (source_digest ~ '^sha256:[0-9a-f]{64}$'),
    obligation_id uuid NOT NULL REFERENCES obligations(id) ON DELETE RESTRICT,
    obligation_revision bigint NOT NULL CHECK (obligation_revision>=1),
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,application_id),
    UNIQUE (tenant_id,obligation_id)
);

CREATE TABLE internal_finance_payment_deliveries (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_digest text NOT NULL CHECK (request_digest ~ '^sha256:[0-9a-f]{64}$'),
    application_id uuid NOT NULL,
    application_revision bigint NOT NULL CHECK (application_revision>=1),
    actual_amount_minor bigint NOT NULL CHECK (actual_amount_minor>0),
    money_payment_id uuid NOT NULL REFERENCES cash_transactions(id) ON DELETE RESTRICT,
    money_allocation_id uuid NULL REFERENCES allocations(id) ON DELETE RESTRICT,
    allocated_amount_minor bigint NOT NULL CHECK (allocated_amount_minor>=0),
    overage_amount_minor bigint NOT NULL CHECK (overage_amount_minor>=0),
    response_snapshot jsonb NOT NULL CHECK (jsonb_typeof(response_snapshot)='object'),
    actor_kind text NOT NULL,
    actor_id text NOT NULL,
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,command_id),
    CHECK (allocated_amount_minor+overage_amount_minor=actual_amount_minor),
    CHECK ((allocated_amount_minor=0 AND money_allocation_id IS NULL) OR (allocated_amount_minor>0 AND money_allocation_id IS NOT NULL))
);
CREATE INDEX internal_finance_payment_deliveries_app_idx ON internal_finance_payment_deliveries(tenant_id,application_id,recorded_at,command_id);

CREATE TRIGGER internal_finance_ledger_owner_setting_revisions_immutable BEFORE UPDATE OR DELETE ON internal_finance_ledger_owner_setting_revisions FOR EACH ROW EXECUTE FUNCTION reject_payroll_payment_history_change();
CREATE TRIGGER internal_finance_payment_obligation_bindings_immutable BEFORE UPDATE OR DELETE ON internal_finance_payment_obligation_bindings FOR EACH ROW EXECUTE FUNCTION reject_payroll_payment_history_change();
CREATE TRIGGER internal_finance_payment_deliveries_immutable BEFORE UPDATE OR DELETE ON internal_finance_payment_deliveries FOR EACH ROW EXECUTE FUNCTION reject_payroll_payment_history_change();
