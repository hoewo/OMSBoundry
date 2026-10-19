CREATE FUNCTION valid_approval_audit_amount_snapshot(snapshot jsonb, expected_total bigint, expected_currency text) RETURNS boolean
LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
    component_count integer;
    kind_count integer;
    amount_sum numeric;
    source_count integer;
    source_kind_count integer;
    source_sum numeric;
BEGIN
    IF jsonb_typeof(snapshot->'amount_components') <> 'array'
       OR jsonb_typeof(snapshot->'source'->'amount_components') <> 'array'
       OR snapshot->>'amount_profile' <> 'reference-oa-v1'
       OR snapshot->'source'->>'amount_profile' <> 'reference-oa-v1'
       OR snapshot->>'currency' <> expected_currency
       OR snapshot->'source'->>'currency' <> expected_currency
       OR (snapshot->>'total_amount_minor')::bigint <> expected_total THEN
        RETURN false;
    END IF;
    SELECT count(*), count(DISTINCT item->>'kind'), sum((item->>'amount_minor')::numeric)
      INTO component_count, kind_count, amount_sum
      FROM jsonb_array_elements(snapshot->'amount_components') AS item
     WHERE item->>'kind' IN ('design_fee','survey_fee','consultation_fee')
       AND (item->>'amount_minor')::numeric >= 0;
    SELECT count(*), count(DISTINCT item->>'kind'), sum((item->>'amount_minor')::numeric)
      INTO source_count, source_kind_count, source_sum
      FROM jsonb_array_elements(snapshot->'source'->'amount_components') AS item
     WHERE item->>'kind' IN ('design_fee','survey_fee','consultation_fee')
       AND (item->>'amount_minor')::numeric >= 0;
    RETURN component_count=3 AND kind_count=3 AND amount_sum=expected_total
       AND source_count=3 AND source_kind_count=3
       AND source_sum=(snapshot->'source'->>'total_amount_minor')::numeric;
EXCEPTION WHEN others THEN
    RETURN false;
END;
$$;

CREATE TABLE approval_audit_results (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    result_kind text NOT NULL CHECK (result_kind IN ('approval','audit')),
    state text NOT NULL CHECK (state IN ('recorded','withdrawn')),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    total_amount_minor bigint NOT NULL CHECK (total_amount_minor >= 0),
    revision bigint NOT NULL CHECK (revision > 0),
    snapshot_json jsonb NOT NULL CHECK (jsonb_typeof(snapshot_json)='object'),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id),
    UNIQUE (tenant_id,project_id,result_kind),
    CHECK (valid_approval_audit_amount_snapshot(snapshot_json,total_amount_minor,currency))
);
CREATE INDEX approval_audit_results_tenant_project_idx ON approval_audit_results(tenant_id,project_id,result_kind);

CREATE TABLE approval_audit_result_revisions (
    tenant_id uuid NOT NULL,
    result_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    event_kind text NOT NULL CHECK (event_kind IN ('recorded','corrected','withdrawn','reregistered')),
    command_id uuid NOT NULL,
    state text NOT NULL CHECK (state IN ('recorded','withdrawn')),
    snapshot_json jsonb NOT NULL CHECK (jsonb_typeof(snapshot_json)='object'),
    reason text NOT NULL CHECK (reason=btrim(reason) AND char_length(reason) BETWEEN 1 AND 1000),
    actor_ref text NOT NULL CHECK (actor_ref=btrim(actor_ref) AND char_length(actor_ref) BETWEEN 1 AND 356),
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,result_id,revision),
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,result_id) REFERENCES approval_audit_results(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE approval_audit_result_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    actor_subject uuid NOT NULL,
    response_json jsonb NOT NULL CHECK (jsonb_typeof(response_json)='object'),
    created_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,command_id)
);

CREATE TABLE approval_audit_result_audit_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    result_id uuid NOT NULL,
    command_id uuid NOT NULL,
    source_revision bigint NOT NULL CHECK (source_revision >= 0),
    result_revision bigint NOT NULL CHECK (result_revision > source_revision),
    action text NOT NULL CHECK (action IN ('result_recorded','result_corrected','result_withdrawn','result_reregistered')),
    reason text NOT NULL CHECK (reason=btrim(reason) AND char_length(reason) BETWEEN 1 AND 1000),
    actor_subject uuid NOT NULL,
    actor_kind text NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,result_id,result_revision) REFERENCES approval_audit_result_revisions(tenant_id,result_id,revision) ON DELETE RESTRICT
);

CREATE FUNCTION ensure_approval_audit_result_current_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    current_row approval_audit_results%ROWTYPE;
    latest_row approval_audit_result_revisions%ROWTYPE;
    target_tenant uuid;
    target_result uuid;
BEGIN
    target_tenant := NEW.tenant_id;
    IF TG_TABLE_NAME = 'approval_audit_results' THEN
        target_result := NEW.id;
    ELSE
        target_result := NEW.result_id;
    END IF;
    SELECT * INTO current_row FROM approval_audit_results WHERE tenant_id=target_tenant AND id=target_result;
    SELECT * INTO latest_row FROM approval_audit_result_revisions WHERE tenant_id=target_tenant AND result_id=target_result ORDER BY revision DESC LIMIT 1;
    IF NOT FOUND OR current_row.revision IS DISTINCT FROM latest_row.revision
       OR current_row.state IS DISTINCT FROM latest_row.state
       OR current_row.snapshot_json IS DISTINCT FROM latest_row.snapshot_json
       OR current_row.snapshot_json->>'id' IS DISTINCT FROM current_row.id::text
       OR current_row.snapshot_json->>'tenant_id' IS DISTINCT FROM current_row.tenant_id::text
       OR current_row.snapshot_json->'project'->>'id' IS DISTINCT FROM current_row.project_id::text
       OR current_row.snapshot_json->>'kind' IS DISTINCT FROM current_row.result_kind
       OR current_row.snapshot_json->>'state' IS DISTINCT FROM current_row.state
       OR current_row.snapshot_json->>'currency' IS DISTINCT FROM current_row.currency
       OR (current_row.snapshot_json->>'total_amount_minor')::bigint IS DISTINCT FROM current_row.total_amount_minor
       OR (current_row.snapshot_json->>'revision')::bigint IS DISTINCT FROM current_row.revision THEN
        RAISE EXCEPTION 'approval/audit result current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER approval_audit_results_current_consistency
    AFTER INSERT OR UPDATE ON approval_audit_results DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION ensure_approval_audit_result_current_matches_latest_revision();
CREATE CONSTRAINT TRIGGER approval_audit_result_revisions_current_consistency
    AFTER INSERT ON approval_audit_result_revisions DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION ensure_approval_audit_result_current_matches_latest_revision();

CREATE FUNCTION reject_approval_audit_result_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'immutable approval/audit result record'; END;
$$;
CREATE TRIGGER approval_audit_results_no_delete BEFORE DELETE ON approval_audit_results FOR EACH ROW EXECUTE FUNCTION reject_approval_audit_result_immutable_change();
CREATE TRIGGER approval_audit_result_revisions_immutable BEFORE UPDATE OR DELETE ON approval_audit_result_revisions FOR EACH ROW EXECUTE FUNCTION reject_approval_audit_result_immutable_change();
CREATE TRIGGER approval_audit_result_commands_immutable BEFORE UPDATE OR DELETE ON approval_audit_result_commands FOR EACH ROW EXECUTE FUNCTION reject_approval_audit_result_immutable_change();
CREATE TRIGGER approval_audit_result_audit_immutable BEFORE UPDATE OR DELETE ON approval_audit_result_audit_events FOR EACH ROW EXECUTE FUNCTION reject_approval_audit_result_immutable_change();
