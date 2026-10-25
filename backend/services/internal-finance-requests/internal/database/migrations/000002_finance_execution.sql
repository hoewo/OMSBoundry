ALTER TABLE finance_requests DROP CONSTRAINT finance_requests_status_check;
ALTER TABLE finance_requests
    ADD COLUMN detail_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(detail_snapshot)='object'),
    ADD COLUMN approval_policy text NOT NULL DEFAULT 'not_required' CHECK (approval_policy IN ('not_required','approval_required')),
    ADD COLUMN approval_status text NOT NULL DEFAULT 'not_applicable' CHECK (approval_status IN ('not_applicable','pending','approved','rejected')),
    ADD COLUMN paid_minor bigint NOT NULL DEFAULT 0 CHECK (paid_minor>=0),
    ADD COLUMN allocated_minor bigint NOT NULL DEFAULT 0 CHECK (allocated_minor>=0),
    ADD COLUMN overage_minor bigint NOT NULL DEFAULT 0 CHECK (overage_minor>=0),
    ADD COLUMN payment_unknown_count bigint NOT NULL DEFAULT 0 CHECK (payment_unknown_count>=0),
    ADD COLUMN submitted_at timestamptz NULL,
    ADD COLUMN withdrawn_at timestamptz NULL,
    ADD COLUMN withdrawal_reason text NULL;
ALTER TABLE finance_requests ADD CONSTRAINT finance_requests_status_check CHECK (status IN ('draft','payable','partially_paid','paid','paid_with_overpayment','withdrawn'));
ALTER TABLE finance_requests ADD CONSTRAINT finance_requests_totals_check CHECK (allocated_minor<=amount_minor AND paid_minor=allocated_minor+overage_minor);

ALTER TABLE finance_request_revisions DROP CONSTRAINT finance_request_revisions_event_kind_check;
ALTER TABLE finance_request_revisions
    ADD COLUMN detail_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(detail_snapshot)='object'),
    ADD COLUMN approval_policy text NOT NULL DEFAULT 'not_required',
    ADD COLUMN approval_status text NOT NULL DEFAULT 'not_applicable',
    ADD COLUMN paid_minor bigint NOT NULL DEFAULT 0,
    ADD COLUMN allocated_minor bigint NOT NULL DEFAULT 0,
    ADD COLUMN overage_minor bigint NOT NULL DEFAULT 0,
    ADD COLUMN payment_unknown_count bigint NOT NULL DEFAULT 0,
    ADD COLUMN submitted_at timestamptz NULL,
    ADD COLUMN withdrawn_at timestamptz NULL,
    ADD COLUMN withdrawal_reason text NULL;
ALTER TABLE finance_request_revisions ADD CONSTRAINT finance_request_revisions_event_kind_check CHECK (event_kind IN ('created','draft_updated','submitted','withdrawn','payment_recorded'));

ALTER TABLE finance_request_audit_events DROP CONSTRAINT finance_request_audit_events_action_check;
ALTER TABLE finance_request_audit_events ADD CONSTRAINT finance_request_audit_events_action_check CHECK (action IN ('finance_request_created','finance_request_draft_updated','finance_request_submitted','finance_request_withdrawn','finance_request_payment_recorded'));

CREATE OR REPLACE FUNCTION ensure_finance_request_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE current_row finance_requests%ROWTYPE; latest_row finance_request_revisions%ROWTYPE; target_tenant uuid; target_request uuid;
BEGIN
    target_tenant := NEW.tenant_id;
    target_request := COALESCE((to_jsonb(NEW)->>'id')::uuid,(to_jsonb(NEW)->>'request_id')::uuid);
    SELECT * INTO current_row FROM finance_requests WHERE tenant_id=target_tenant AND id=target_request;
    SELECT * INTO latest_row FROM finance_request_revisions WHERE tenant_id=target_tenant AND request_id=target_request ORDER BY revision DESC LIMIT 1;
    IF NOT FOUND OR current_row.revision IS DISTINCT FROM latest_row.revision
       OR current_row.applicant_subject_id IS DISTINCT FROM latest_row.applicant_subject_id
       OR current_row.request_type IS DISTINCT FROM latest_row.request_type
       OR current_row.title IS DISTINCT FROM latest_row.title
       OR current_row.amount_minor IS DISTINCT FROM latest_row.amount_minor
       OR current_row.currency IS DISTINCT FROM latest_row.currency
       OR current_row.status IS DISTINCT FROM latest_row.status
       OR current_row.detail_snapshot IS DISTINCT FROM latest_row.detail_snapshot
       OR current_row.approval_policy IS DISTINCT FROM latest_row.approval_policy
       OR current_row.approval_status IS DISTINCT FROM latest_row.approval_status
       OR current_row.paid_minor IS DISTINCT FROM latest_row.paid_minor
       OR current_row.allocated_minor IS DISTINCT FROM latest_row.allocated_minor
       OR current_row.overage_minor IS DISTINCT FROM latest_row.overage_minor
       OR current_row.payment_unknown_count IS DISTINCT FROM latest_row.payment_unknown_count
       OR current_row.submitted_at IS DISTINCT FROM latest_row.submitted_at
       OR current_row.withdrawn_at IS DISTINCT FROM latest_row.withdrawn_at
       OR current_row.withdrawal_reason IS DISTINCT FROM latest_row.withdrawal_reason THEN
        RAISE EXCEPTION 'finance request current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TABLE finance_payment_deliveries (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    request_id uuid NOT NULL,
    source_request_revision bigint NOT NULL CHECK (source_request_revision>=1),
    actual_amount_minor bigint NOT NULL CHECK (actual_amount_minor>0),
    occurred_on date NOT NULL,
    note text NOT NULL DEFAULT '',
    overage_confirmed boolean NOT NULL DEFAULT false,
    overage_reason text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('pending','failed','unknown','succeeded')),
    attempt_count bigint NOT NULL DEFAULT 1 CHECK (attempt_count>=1),
    money_payment_id uuid NULL,
    money_payment_revision bigint NULL,
    money_allocation_id uuid NULL,
    money_allocation_revision bigint NULL,
    money_obligation_id uuid NULL,
    money_obligation_revision bigint NULL,
    allocated_amount_minor bigint NOT NULL DEFAULT 0 CHECK (allocated_amount_minor>=0),
    overage_amount_minor bigint NOT NULL DEFAULT 0 CHECK (overage_amount_minor>=0),
    last_error_code text NOT NULL DEFAULT '',
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,command_id),
    FOREIGN KEY(tenant_id,request_id) REFERENCES finance_requests(tenant_id,id) ON DELETE RESTRICT,
    CHECK ((status='succeeded' AND money_payment_id IS NOT NULL AND actual_amount_minor=allocated_amount_minor+overage_amount_minor) OR status<>'succeeded')
);
CREATE INDEX finance_payment_deliveries_request_idx ON finance_payment_deliveries(tenant_id,request_id,created_at,command_id);

CREATE TABLE finance_payment_delivery_events (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    event_sequence bigint NOT NULL CHECK (event_sequence>=1),
    status text NOT NULL CHECK (status IN ('pending','failed','unknown','succeeded')),
    response_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(response_snapshot)='object'),
    error_code text NOT NULL DEFAULT '',
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,command_id,event_sequence),
    FOREIGN KEY(tenant_id,command_id) REFERENCES finance_payment_deliveries(tenant_id,command_id) ON DELETE RESTRICT
);

CREATE TABLE finance_expense_categories (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    category_key text NOT NULL CHECK (category_key ~ '^[a-z][a-z0-9_]{1,63}$'),
    display_name text NOT NULL CHECK (display_name=btrim(display_name) AND char_length(display_name) BETWEEN 1 AND 80),
    source text NOT NULL CHECK (source IN ('platform','tenant')),
    enabled boolean NOT NULL,
    revision bigint NOT NULL CHECK (revision>=1),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,id),
    UNIQUE(tenant_id,category_key)
);
CREATE TABLE finance_expense_category_revisions (
    tenant_id uuid NOT NULL,
    category_id uuid NOT NULL,
    revision bigint NOT NULL,
    command_id uuid NOT NULL,
    category_key text NOT NULL,
    display_name text NOT NULL,
    enabled boolean NOT NULL,
    reason text NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,category_id,revision),
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,category_id) REFERENCES finance_expense_categories(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE finance_sensitive_access_audit_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    request_id uuid NOT NULL,
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    purpose text NOT NULL CHECK (purpose=btrim(purpose) AND char_length(purpose) BETWEEN 1 AND 200),
    result text NOT NULL CHECK (result IN ('succeeded','not_found','failed')),
    occurred_at timestamptz NOT NULL
);
CREATE INDEX finance_sensitive_access_audit_request_idx ON finance_sensitive_access_audit_events(tenant_id,request_id,occurred_at);

CREATE TRIGGER finance_payment_delivery_events_immutable BEFORE UPDATE OR DELETE ON finance_payment_delivery_events FOR EACH ROW EXECUTE FUNCTION reject_finance_request_immutable_change();
CREATE TRIGGER finance_expense_category_revisions_immutable BEFORE UPDATE OR DELETE ON finance_expense_category_revisions FOR EACH ROW EXECUTE FUNCTION reject_finance_request_immutable_change();
CREATE TRIGGER finance_sensitive_access_audit_events_immutable BEFORE UPDATE OR DELETE ON finance_sensitive_access_audit_events FOR EACH ROW EXECUTE FUNCTION reject_finance_request_immutable_change();
