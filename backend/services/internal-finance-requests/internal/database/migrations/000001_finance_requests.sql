CREATE TABLE finance_requests (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    applicant_subject_id uuid NOT NULL,
    request_type text NOT NULL CHECK (request_type IN ('reimbursement','payment')),
    title text NOT NULL CHECK (title=btrim(title) AND char_length(title) BETWEEN 1 AND 200),
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency char(3) NOT NULL CHECK (currency='CNY'),
    status text NOT NULL CHECK (status IN ('draft')),
    revision bigint NOT NULL CHECK (revision > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id)
);
CREATE INDEX finance_requests_tenant_updated_idx ON finance_requests(tenant_id,updated_at DESC,id);
CREATE INDEX finance_requests_applicant_idx ON finance_requests(tenant_id,applicant_subject_id,updated_at DESC,id);

CREATE TABLE finance_request_revisions (
    tenant_id uuid NOT NULL,
    request_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    command_id uuid NOT NULL,
    event_kind text NOT NULL CHECK (event_kind IN ('created','draft_updated')),
    applicant_subject_id uuid NOT NULL,
    request_type text NOT NULL CHECK (request_type IN ('reimbursement','payment')),
    title text NOT NULL,
    amount_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    status text NOT NULL,
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,request_id,revision),
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,request_id) REFERENCES finance_requests(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE finance_request_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    actor_subject_id uuid NOT NULL,
    response_json jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,command_id)
);

CREATE TABLE finance_request_audit_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    request_id uuid NOT NULL,
    command_id uuid NOT NULL,
    source_revision bigint NOT NULL CHECK (source_revision >= 0),
    result_revision bigint NOT NULL CHECK (result_revision > source_revision),
    action text NOT NULL CHECK (action IN ('finance_request_created','finance_request_draft_updated')),
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,request_id,result_revision) REFERENCES finance_request_revisions(tenant_id,request_id,revision) ON DELETE RESTRICT
);

CREATE FUNCTION ensure_finance_request_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
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
       OR current_row.status IS DISTINCT FROM latest_row.status THEN
        RAISE EXCEPTION 'finance request current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;
CREATE CONSTRAINT TRIGGER finance_requests_current_consistency AFTER INSERT OR UPDATE ON finance_requests DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_finance_request_matches_latest_revision();
CREATE CONSTRAINT TRIGGER finance_request_revisions_current_consistency AFTER INSERT ON finance_request_revisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_finance_request_matches_latest_revision();

CREATE FUNCTION reject_finance_request_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'immutable finance request record'; END; $$;
CREATE TRIGGER finance_request_revisions_immutable BEFORE UPDATE OR DELETE ON finance_request_revisions FOR EACH ROW EXECUTE FUNCTION reject_finance_request_immutable_change();
CREATE TRIGGER finance_request_commands_immutable BEFORE UPDATE OR DELETE ON finance_request_commands FOR EACH ROW EXECUTE FUNCTION reject_finance_request_immutable_change();
CREATE TRIGGER finance_request_audit_immutable BEFORE UPDATE OR DELETE ON finance_request_audit_events FOR EACH ROW EXECUTE FUNCTION reject_finance_request_immutable_change();
