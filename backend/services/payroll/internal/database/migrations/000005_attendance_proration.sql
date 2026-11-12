ALTER TABLE payroll_lines
    ADD COLUMN attendance_summary_snapshot jsonb NULL CHECK (attendance_summary_snapshot IS NULL OR jsonb_typeof(attendance_summary_snapshot)='object'),
    ADD COLUMN attendance_proration_minor bigint NOT NULL DEFAULT 0;

ALTER TABLE payroll_line_revisions DROP CONSTRAINT payroll_line_revisions_event_kind_check;
ALTER TABLE payroll_line_revisions ADD CONSTRAINT payroll_line_revisions_event_kind_check CHECK (event_kind IN ('generated','adjustment_generated','revised','import_committed','reviewed','zero_confirmed','payment_recorded','payment_unknown','payment_failed','member_synced','compensation_synced','payment_profile_synced','attendance_applied','voided'));

ALTER TABLE payroll_run_revisions DROP CONSTRAINT payroll_run_revisions_event_kind_check;
ALTER TABLE payroll_run_revisions ADD CONSTRAINT payroll_run_revisions_event_kind_check CHECK (event_kind IN ('created','draft_updated','generated','adjustment_generated','line_revised','import_committed','reviewed','line_zero_confirmed','payment_recorded','payment_unknown','payment_failed','voided','member_synced','compensation_synced','payment_profile_synced','attendance_applied'));

ALTER TABLE payroll_audit_events DROP CONSTRAINT payroll_audit_events_action_check;
ALTER TABLE payroll_audit_events ADD CONSTRAINT payroll_audit_events_action_check CHECK (action IN ('payroll_run_created','payroll_run_draft_updated','payroll_run_generated','payroll_adjustment_run_generated','payroll_line_revised','payroll_import_committed','payroll_run_reviewed','payroll_line_zero_confirmed','payroll_payment_recorded','payroll_payment_unknown','payroll_payment_failed','payroll_run_voided','payroll_member_synced','payroll_compensation_synced','payroll_payment_profile_synced','payroll_attendance_applied'));

CREATE OR REPLACE FUNCTION ensure_payroll_line_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
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
       OR current_row.attendance_summary_snapshot IS DISTINCT FROM snapshot->'attendance_summary'
       OR current_row.attendance_proration_minor IS DISTINCT FROM COALESCE((snapshot->>'attendance_proration_minor')::bigint,0)
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
