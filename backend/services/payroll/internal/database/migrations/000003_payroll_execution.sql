ALTER TABLE payroll_runs DROP CONSTRAINT payroll_runs_status_check;
ALTER TABLE payroll_runs DROP CONSTRAINT payroll_runs_tenant_id_period_key;
ALTER TABLE payroll_runs
    ADD COLUMN run_kind text NOT NULL DEFAULT 'regular' CHECK (run_kind IN ('regular','adjustment')),
    ADD COLUMN replaces_run_id uuid NULL,
    ADD COLUMN planned_pay_date date NOT NULL DEFAULT CURRENT_DATE,
    ADD COLUMN approval_policy_snapshot jsonb NOT NULL DEFAULT '{"mode":"not_required","version":1}'::jsonb CHECK (jsonb_typeof(approval_policy_snapshot)='object'),
    ADD COLUMN approval_status text NOT NULL DEFAULT 'not_required' CHECK (approval_status IN ('not_required','pending','approved','rejected')),
    ADD COLUMN contribution_scheme_id uuid NULL,
    ADD COLUMN contribution_scheme_revision bigint NULL CHECK (contribution_scheme_revision IS NULL OR contribution_scheme_revision >= 1),
    ADD COLUMN contribution_scheme_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(contribution_scheme_snapshot)='object'),
    ADD COLUMN void_reason text NOT NULL DEFAULT '' CHECK (void_reason=btrim(void_reason) AND char_length(void_reason) <= 500),
    ADD CONSTRAINT payroll_runs_status_check CHECK (status IN ('draft','review_pending','ready_to_pay','partially_paid','paid','recovery_pending','voided')),
    ADD CONSTRAINT payroll_runs_replacement_check CHECK ((replaces_run_id IS NULL) OR (run_kind='regular' AND replaces_run_id<>id));
CREATE UNIQUE INDEX payroll_runs_active_regular_period_unique ON payroll_runs(tenant_id,period) WHERE run_kind='regular' AND status<>'voided';
CREATE INDEX payroll_runs_replacement_idx ON payroll_runs(tenant_id,replaces_run_id) WHERE replaces_run_id IS NOT NULL;

ALTER TABLE payroll_run_revisions DROP CONSTRAINT payroll_run_revisions_event_kind_check;
ALTER TABLE payroll_run_revisions
    ADD COLUMN run_kind text NOT NULL DEFAULT 'regular',
    ADD COLUMN replaces_run_id uuid NULL,
    ADD COLUMN planned_pay_date date NOT NULL DEFAULT CURRENT_DATE,
    ADD COLUMN approval_policy_snapshot jsonb NOT NULL DEFAULT '{"mode":"not_required","version":1}'::jsonb,
    ADD COLUMN approval_status text NOT NULL DEFAULT 'not_required',
    ADD COLUMN contribution_scheme_id uuid NULL,
    ADD COLUMN contribution_scheme_revision bigint NULL,
    ADD COLUMN contribution_scheme_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN void_reason text NOT NULL DEFAULT '',
    ADD CONSTRAINT payroll_run_revisions_event_kind_check CHECK (event_kind IN ('created','draft_updated','generated','adjustment_generated','line_revised','import_committed','reviewed','line_zero_confirmed','payment_recorded','payment_unknown','payment_failed','voided','member_synced','compensation_synced','payment_profile_synced'));

ALTER TABLE payroll_audit_events DROP CONSTRAINT payroll_audit_events_action_check;
ALTER TABLE payroll_audit_events ADD CONSTRAINT payroll_audit_events_action_check CHECK (action IN ('payroll_run_created','payroll_run_draft_updated','payroll_run_generated','payroll_adjustment_run_generated','payroll_line_revised','payroll_import_committed','payroll_run_reviewed','payroll_line_zero_confirmed','payroll_payment_recorded','payroll_payment_unknown','payroll_payment_failed','payroll_run_voided','payroll_member_synced','payroll_compensation_synced','payroll_payment_profile_synced'));

CREATE TABLE payroll_lines (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    payroll_run_id uuid NOT NULL,
    person_id uuid NOT NULL,
	person_reference_code text NOT NULL DEFAULT '' CHECK (person_reference_code=btrim(person_reference_code) AND char_length(person_reference_code) <= 64),
    person_display_name text NOT NULL,
    person_source_revision bigint NOT NULL CHECK (person_source_revision >= 1),
	person_source_digest text NOT NULL CHECK (person_source_digest ~ '^sha256:[0-9a-f]{64}$'),
    person_status_snapshot text NOT NULL CHECK (person_status_snapshot IN ('active','inactive')),
    compensation_profile_id uuid NULL,
    compensation_profile_revision bigint NULL,
    payment_profile_id uuid NULL,
    payment_profile_revision bigint NULL,
    payment_account_name text NOT NULL DEFAULT '',
    payment_bank_name text NOT NULL DEFAULT '',
    payment_account_number text NOT NULL DEFAULT '',
    payment_account_missing boolean NOT NULL,
    fixed_salary_minor bigint NOT NULL CHECK (fixed_salary_minor >= 0),
    social_insurance_base_minor bigint NOT NULL CHECK (social_insurance_base_minor >= 0),
    housing_fund_base_minor bigint NOT NULL CHECK (housing_fund_base_minor >= 0),
    performance_minor bigint NOT NULL DEFAULT 0 CHECK (performance_minor >= 0),
    adjustments jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(adjustments)='array'),
    gross_minor bigint NOT NULL,
    employee_contributions jsonb NOT NULL CHECK (jsonb_typeof(employee_contributions)='object'),
    employer_contributions jsonb NOT NULL CHECK (jsonb_typeof(employer_contributions)='object'),
    employee_contribution_minor bigint NOT NULL CHECK (employee_contribution_minor >= 0),
    employer_contribution_minor bigint NOT NULL CHECK (employer_contribution_minor >= 0),
    tax_minor bigint NOT NULL DEFAULT 0 CHECK (tax_minor >= 0),
    tax_source_kind text NOT NULL DEFAULT 'manual' CHECK (tax_source_kind IN ('manual','import')),
    tax_source_note text NOT NULL DEFAULT '' CHECK (tax_source_note=btrim(tax_source_note) AND char_length(tax_source_note) <= 500),
    pre_tax_payable_minor bigint NOT NULL,
    net_payable_minor bigint NOT NULL,
    employer_cost_minor bigint NOT NULL,
    actual_paid_minor bigint NOT NULL DEFAULT 0 CHECK (actual_paid_minor >= 0),
    allocated_paid_minor bigint NOT NULL DEFAULT 0 CHECK (allocated_paid_minor >= 0),
    overage_minor bigint NOT NULL DEFAULT 0 CHECK (overage_minor >= 0),
    status text NOT NULL CHECK (status IN ('review_pending','ready_to_pay','zero_pending','partially_paid','paid','no_payment_required','recovery_pending','voided')),
    revision bigint NOT NULL CHECK (revision >= 1),
    line_kind text NOT NULL DEFAULT 'regular' CHECK (line_kind IN ('regular','supplement','recovery')),
    source_payroll_line_id uuid NULL,
    source_payroll_line_revision bigint NULL,
    adjustment_reason text NOT NULL DEFAULT '' CHECK (adjustment_reason=btrim(adjustment_reason) AND char_length(adjustment_reason) <= 500),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id),
    UNIQUE (tenant_id,payroll_run_id,person_id),
    FOREIGN KEY (tenant_id,payroll_run_id) REFERENCES payroll_runs(tenant_id,id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id,source_payroll_line_id) REFERENCES payroll_lines(tenant_id,id) ON DELETE RESTRICT,
    CHECK ((line_kind='regular' AND source_payroll_line_id IS NULL AND source_payroll_line_revision IS NULL AND adjustment_reason='') OR
           (line_kind IN ('supplement','recovery') AND source_payroll_line_id IS NOT NULL AND source_payroll_line_revision >= 1 AND adjustment_reason<>'')),
    CHECK ((payment_account_missing AND payment_profile_id IS NULL AND payment_profile_revision IS NULL AND payment_account_number='') OR
           (NOT payment_account_missing AND payment_profile_id IS NOT NULL AND payment_profile_revision >= 1 AND payment_account_number ~ '^[0-9]{4,32}$')),
    CHECK (allocated_paid_minor <= CASE WHEN net_payable_minor > 0 THEN net_payable_minor ELSE 0 END),
    CHECK (actual_paid_minor = allocated_paid_minor + overage_minor)
);
CREATE INDEX payroll_lines_run_idx ON payroll_lines(tenant_id,payroll_run_id,person_display_name,id);
CREATE INDEX payroll_lines_person_idx ON payroll_lines(tenant_id,person_id,payroll_run_id);

CREATE TABLE payroll_line_revisions (
    tenant_id uuid NOT NULL,
    payroll_run_id uuid NOT NULL,
    payroll_run_revision bigint NOT NULL,
    payroll_line_id uuid NOT NULL,
    revision bigint NOT NULL,
    command_id uuid NOT NULL,
    event_kind text NOT NULL CHECK (event_kind IN ('generated','adjustment_generated','revised','import_committed','reviewed','zero_confirmed','payment_recorded','payment_unknown','payment_failed','member_synced','compensation_synced','payment_profile_synced','voided')),
    snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot)='object'),
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,payroll_line_id,revision),
    FOREIGN KEY (tenant_id,payroll_line_id) REFERENCES payroll_lines(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX payroll_line_revisions_run_idx ON payroll_line_revisions(tenant_id,payroll_run_id,payroll_run_revision,payroll_line_id);

CREATE TABLE payroll_import_previews (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    payroll_run_id uuid NOT NULL,
    payroll_run_revision bigint NOT NULL CHECK (payroll_run_revision >= 1),
    preview_hash text NOT NULL CHECK (preview_hash ~ '^[0-9a-f]{64}$'),
    rows_snapshot jsonb NOT NULL CHECK (jsonb_typeof(rows_snapshot)='array' AND jsonb_array_length(rows_snapshot) BETWEEN 1 AND 1000),
	confirmable boolean NOT NULL,
    status text NOT NULL CHECK (status IN ('open','committed')),
    created_by_subject_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL CHECK (expires_at > created_at),
    committed_command_id uuid NULL,
    committed_at timestamptz NULL,
    UNIQUE (tenant_id,id),
    FOREIGN KEY (tenant_id,payroll_run_id) REFERENCES payroll_runs(tenant_id,id) ON DELETE RESTRICT,
    CHECK ((status='open' AND committed_command_id IS NULL AND committed_at IS NULL) OR
           (status='committed' AND committed_command_id IS NOT NULL AND committed_at IS NOT NULL))
);
CREATE INDEX payroll_import_previews_expiry_idx ON payroll_import_previews(tenant_id,status,expires_at,id);

CREATE TABLE payroll_payment_deliveries (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    payroll_run_id uuid NOT NULL,
    payroll_line_id uuid NOT NULL,
    payroll_line_revision bigint NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
	intent_snapshot jsonb NOT NULL CHECK (jsonb_typeof(intent_snapshot)='object'),
    status text NOT NULL CHECK (status IN ('pending','succeeded','failed','unknown')),
    actual_amount_minor bigint NOT NULL CHECK (actual_amount_minor > 0),
    allocated_amount_minor bigint NOT NULL DEFAULT 0 CHECK (allocated_amount_minor >= 0),
    overage_amount_minor bigint NOT NULL DEFAULT 0 CHECK (overage_amount_minor >= 0),
    occurred_on date NOT NULL,
    note text NOT NULL DEFAULT '' CHECK (note=btrim(note) AND char_length(note) <= 500),
    overage_confirmed boolean NOT NULL DEFAULT false,
    overage_reason text NOT NULL DEFAULT '' CHECK (overage_reason=btrim(overage_reason) AND char_length(overage_reason) <= 500),
    payment_profile_id uuid NOT NULL,
    payment_profile_revision bigint NOT NULL,
    payment_account_name text NOT NULL,
    payment_bank_name text NOT NULL,
    payment_account_number text NOT NULL,
    money_payment_id uuid NULL,
    money_payment_revision bigint NULL,
    money_allocation_id uuid NULL,
    money_allocation_revision bigint NULL,
    failure_code text NOT NULL DEFAULT '',
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,command_id),
    FOREIGN KEY (tenant_id,payroll_run_id) REFERENCES payroll_runs(tenant_id,id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id,payroll_line_id) REFERENCES payroll_lines(tenant_id,id) ON DELETE RESTRICT,
    CHECK ((status='succeeded' AND money_payment_id IS NOT NULL AND money_payment_revision >= 1 AND allocated_amount_minor + overage_amount_minor = actual_amount_minor)
        OR (status<>'succeeded' AND money_payment_id IS NULL AND money_payment_revision IS NULL AND allocated_amount_minor=0 AND overage_amount_minor=0))
);
CREATE INDEX payroll_payment_deliveries_line_idx ON payroll_payment_deliveries(tenant_id,payroll_line_id,created_at,id);

CREATE TABLE payroll_payment_delivery_revisions (
    tenant_id uuid NOT NULL,
    delivery_id uuid NOT NULL,
    revision bigint NOT NULL,
    status text NOT NULL,
    response_snapshot jsonb NOT NULL CHECK (jsonb_typeof(response_snapshot)='object'),
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,delivery_id,revision),
    FOREIGN KEY (delivery_id) REFERENCES payroll_payment_deliveries(id) ON DELETE RESTRICT
);

CREATE OR REPLACE FUNCTION ensure_payroll_run_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE current_row payroll_runs%ROWTYPE; latest_row payroll_run_revisions%ROWTYPE; target_tenant uuid; target_run uuid;
BEGIN
    target_tenant := NEW.tenant_id;
    target_run := COALESCE((to_jsonb(NEW)->>'id')::uuid,(to_jsonb(NEW)->>'payroll_run_id')::uuid);
    SELECT * INTO current_row FROM payroll_runs WHERE tenant_id=target_tenant AND id=target_run;
    SELECT * INTO latest_row FROM payroll_run_revisions WHERE tenant_id=target_tenant AND payroll_run_id=target_run ORDER BY revision DESC LIMIT 1;
    IF NOT FOUND OR current_row.revision IS DISTINCT FROM latest_row.revision OR current_row.period IS DISTINCT FROM latest_row.period
       OR current_row.label IS DISTINCT FROM latest_row.label OR current_row.status IS DISTINCT FROM latest_row.status
       OR current_row.run_kind IS DISTINCT FROM latest_row.run_kind OR current_row.replaces_run_id IS DISTINCT FROM latest_row.replaces_run_id
       OR current_row.planned_pay_date IS DISTINCT FROM latest_row.planned_pay_date
       OR current_row.approval_policy_snapshot IS DISTINCT FROM latest_row.approval_policy_snapshot
       OR current_row.approval_status IS DISTINCT FROM latest_row.approval_status
       OR current_row.contribution_scheme_id IS DISTINCT FROM latest_row.contribution_scheme_id
       OR current_row.contribution_scheme_revision IS DISTINCT FROM latest_row.contribution_scheme_revision
       OR current_row.contribution_scheme_snapshot IS DISTINCT FROM latest_row.contribution_scheme_snapshot
       OR current_row.void_reason IS DISTINCT FROM latest_row.void_reason THEN
        RAISE EXCEPTION 'payroll run current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION ensure_payroll_line_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE current_row payroll_lines%ROWTYPE; latest_row payroll_line_revisions%ROWTYPE; target_tenant uuid; target_line uuid; snapshot jsonb;
BEGIN
    target_tenant := NEW.tenant_id;
    target_line := COALESCE((to_jsonb(NEW)->>'id')::uuid,(to_jsonb(NEW)->>'payroll_line_id')::uuid);
    SELECT * INTO current_row FROM payroll_lines WHERE tenant_id=target_tenant AND id=target_line;
    SELECT * INTO latest_row FROM payroll_line_revisions WHERE tenant_id=target_tenant AND payroll_line_id=target_line ORDER BY revision DESC LIMIT 1;
    snapshot := latest_row.snapshot;
    IF NOT FOUND OR current_row.revision IS DISTINCT FROM (snapshot->>'revision')::bigint
       OR current_row.payroll_run_id::text IS DISTINCT FROM snapshot->>'payroll_run_id'
       OR current_row.person_id::text IS DISTINCT FROM snapshot->>'person_id'
	   OR current_row.person_reference_code IS DISTINCT FROM COALESCE(snapshot->>'person_reference_code','')
       OR current_row.person_display_name IS DISTINCT FROM snapshot->>'person_display_name'
       OR current_row.person_source_revision IS DISTINCT FROM (snapshot->>'person_source_revision')::bigint
	   OR current_row.person_source_digest IS DISTINCT FROM snapshot->>'person_source_digest'
       OR current_row.person_status_snapshot IS DISTINCT FROM snapshot->>'person_status_snapshot'
       OR COALESCE(current_row.compensation_profile_id::text,'') IS DISTINCT FROM COALESCE(snapshot->>'compensation_profile_id','')
       OR COALESCE(current_row.compensation_profile_revision,0) IS DISTINCT FROM COALESCE((snapshot->>'compensation_profile_revision')::bigint,0)
       OR COALESCE(current_row.payment_profile_id::text,'') IS DISTINCT FROM COALESCE(snapshot->>'payment_profile_id','')
       OR COALESCE(current_row.payment_profile_revision,0) IS DISTINCT FROM COALESCE((snapshot->>'payment_profile_revision')::bigint,0)
       OR current_row.payment_account_name IS DISTINCT FROM COALESCE(snapshot->>'payment_account_name','')
       OR current_row.payment_bank_name IS DISTINCT FROM COALESCE(snapshot->>'payment_bank_name','')
       OR current_row.payment_account_number IS DISTINCT FROM COALESCE(snapshot->>'payment_account_number','')
       OR current_row.payment_account_missing IS DISTINCT FROM (snapshot->>'payment_account_missing')::boolean
       OR current_row.fixed_salary_minor IS DISTINCT FROM (snapshot->>'fixed_salary_minor')::bigint
       OR current_row.social_insurance_base_minor IS DISTINCT FROM (snapshot->>'social_insurance_base_minor')::bigint
       OR current_row.housing_fund_base_minor IS DISTINCT FROM (snapshot->>'housing_fund_base_minor')::bigint
       OR current_row.performance_minor IS DISTINCT FROM (snapshot->>'performance_minor')::bigint
       OR current_row.adjustments IS DISTINCT FROM snapshot->'adjustments'
       OR current_row.gross_minor IS DISTINCT FROM (snapshot->>'gross_minor')::bigint
       OR current_row.employee_contributions IS DISTINCT FROM COALESCE((SELECT jsonb_object_agg(item->>'key',item) FROM jsonb_array_elements(snapshot->'contributions') item),'{}'::jsonb)
       OR current_row.employer_contributions IS DISTINCT FROM COALESCE((SELECT jsonb_object_agg(item->>'key',item) FROM jsonb_array_elements(snapshot->'contributions') item),'{}'::jsonb)
       OR current_row.employee_contribution_minor IS DISTINCT FROM (snapshot->>'employee_contribution_minor')::bigint
       OR current_row.employer_contribution_minor IS DISTINCT FROM (snapshot->>'employer_contribution_minor')::bigint
       OR current_row.tax_minor IS DISTINCT FROM (snapshot->>'tax_minor')::bigint
       OR current_row.tax_source_kind IS DISTINCT FROM snapshot->>'tax_source_kind'
       OR current_row.tax_source_note IS DISTINCT FROM COALESCE(snapshot->>'tax_source_note','')
       OR current_row.pre_tax_payable_minor IS DISTINCT FROM (snapshot->>'pre_tax_payable_minor')::bigint
       OR current_row.net_payable_minor IS DISTINCT FROM (snapshot->>'net_payable_minor')::bigint
       OR current_row.employer_cost_minor IS DISTINCT FROM (snapshot->>'employer_cost_minor')::bigint
       OR current_row.actual_paid_minor IS DISTINCT FROM (snapshot->>'actual_paid_minor')::bigint
       OR current_row.allocated_paid_minor IS DISTINCT FROM (snapshot->>'allocated_paid_minor')::bigint
       OR current_row.overage_minor IS DISTINCT FROM (snapshot->>'overage_minor')::bigint
       OR current_row.line_kind IS DISTINCT FROM COALESCE(snapshot->>'line_kind','regular')
       OR COALESCE(current_row.source_payroll_line_id::text,'') IS DISTINCT FROM COALESCE(snapshot->>'source_payroll_line_id','')
       OR COALESCE(current_row.source_payroll_line_revision,0) IS DISTINCT FROM COALESCE((snapshot->>'source_payroll_line_revision')::bigint,0)
       OR current_row.adjustment_reason IS DISTINCT FROM COALESCE(snapshot->>'adjustment_reason','')
       OR current_row.status IS DISTINCT FROM snapshot->>'status' THEN
        RAISE EXCEPTION 'payroll line current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER payroll_lines_current_consistency AFTER INSERT OR UPDATE ON payroll_lines DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_payroll_line_matches_latest_revision();
CREATE CONSTRAINT TRIGGER payroll_line_revisions_current_consistency AFTER INSERT ON payroll_line_revisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_payroll_line_matches_latest_revision();

CREATE TRIGGER payroll_line_revisions_immutable BEFORE UPDATE OR DELETE ON payroll_line_revisions FOR EACH ROW EXECUTE FUNCTION reject_payroll_immutable_change();
CREATE TRIGGER payroll_payment_delivery_revisions_immutable BEFORE UPDATE OR DELETE ON payroll_payment_delivery_revisions FOR EACH ROW EXECUTE FUNCTION reject_payroll_immutable_change();
