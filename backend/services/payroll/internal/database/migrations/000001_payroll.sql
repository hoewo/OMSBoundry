CREATE TABLE payroll_runs (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    period char(7) NOT NULL CHECK (period ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    label text NOT NULL CHECK (label=btrim(label) AND char_length(label) BETWEEN 1 AND 200),
    status text NOT NULL CHECK (status IN ('draft')),
    revision bigint NOT NULL CHECK (revision > 0),
    created_by_subject_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id),
    UNIQUE (tenant_id,period)
);
CREATE INDEX payroll_runs_tenant_updated_idx ON payroll_runs(tenant_id,updated_at DESC,id);

CREATE TABLE payroll_run_revisions (
    tenant_id uuid NOT NULL,
    payroll_run_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    command_id uuid NOT NULL,
    event_kind text NOT NULL CHECK (event_kind IN ('created','draft_updated')),
    period char(7) NOT NULL,
    label text NOT NULL,
    status text NOT NULL,
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,payroll_run_id,revision),
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,payroll_run_id) REFERENCES payroll_runs(tenant_id,id) ON DELETE RESTRICT
);
CREATE TABLE payroll_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    actor_subject_id uuid NOT NULL,
    response_json jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,command_id)
);
CREATE TABLE payroll_audit_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    payroll_run_id uuid NOT NULL,
    command_id uuid NOT NULL,
    source_revision bigint NOT NULL CHECK (source_revision >= 0),
    result_revision bigint NOT NULL CHECK (result_revision > source_revision),
    action text NOT NULL CHECK (action IN ('payroll_run_created','payroll_run_draft_updated')),
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,payroll_run_id,result_revision) REFERENCES payroll_run_revisions(tenant_id,payroll_run_id,revision) ON DELETE RESTRICT
);
CREATE FUNCTION ensure_payroll_run_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE current_row payroll_runs%ROWTYPE; latest_row payroll_run_revisions%ROWTYPE; target_tenant uuid; target_run uuid;
BEGIN
    target_tenant := NEW.tenant_id;
    target_run := COALESCE((to_jsonb(NEW)->>'id')::uuid,(to_jsonb(NEW)->>'payroll_run_id')::uuid);
    SELECT * INTO current_row FROM payroll_runs WHERE tenant_id=target_tenant AND id=target_run;
    SELECT * INTO latest_row FROM payroll_run_revisions WHERE tenant_id=target_tenant AND payroll_run_id=target_run ORDER BY revision DESC LIMIT 1;
    IF NOT FOUND OR current_row.revision IS DISTINCT FROM latest_row.revision
       OR current_row.period IS DISTINCT FROM latest_row.period
       OR current_row.label IS DISTINCT FROM latest_row.label
       OR current_row.status IS DISTINCT FROM latest_row.status THEN
        RAISE EXCEPTION 'payroll run current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;
CREATE CONSTRAINT TRIGGER payroll_runs_current_consistency AFTER INSERT OR UPDATE ON payroll_runs DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_payroll_run_matches_latest_revision();
CREATE CONSTRAINT TRIGGER payroll_run_revisions_current_consistency AFTER INSERT ON payroll_run_revisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_payroll_run_matches_latest_revision();
CREATE FUNCTION reject_payroll_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'immutable payroll record'; END; $$;
CREATE TRIGGER payroll_run_revisions_immutable BEFORE UPDATE OR DELETE ON payroll_run_revisions FOR EACH ROW EXECUTE FUNCTION reject_payroll_immutable_change();
CREATE TRIGGER payroll_commands_immutable BEFORE UPDATE OR DELETE ON payroll_commands FOR EACH ROW EXECUTE FUNCTION reject_payroll_immutable_change();
CREATE TRIGGER payroll_audit_immutable BEFORE UPDATE OR DELETE ON payroll_audit_events FOR EACH ROW EXECUTE FUNCTION reject_payroll_immutable_change();
