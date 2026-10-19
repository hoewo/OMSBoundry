-- Optional, typed customer facts; generic CostEntry and its constraints unchanged.
CREATE TABLE reference_oa_cost_entries (
    tenant_id uuid NOT NULL, id uuid NOT NULL, project_id uuid NOT NULL,
    scope text NOT NULL CHECK(scope IN ('business','production')),
    state text NOT NULL CHECK(state IN ('recorded','withdrawn')),
    revision bigint NOT NULL CHECK(revision > 0), snapshot jsonb NOT NULL,
    PRIMARY KEY(tenant_id,id),
    CHECK(jsonb_typeof(snapshot)='object' AND snapshot ?& ARRAY['id','tenant_id','project','scope','state','revision','currency','amount_minor','category','created_by','created_at','updated_by','updated_at','evidence']),
    CHECK(jsonb_typeof(snapshot->'project')='object' AND snapshot->'project' ?& ARRAY['id','revision','code','display_name']),
    CHECK(jsonb_typeof(snapshot->'amount_minor')='number' AND jsonb_typeof(snapshot->'evidence')='array'),
    CHECK(jsonb_typeof(snapshot->'id')='string' AND jsonb_typeof(snapshot->'tenant_id')='string' AND jsonb_typeof(snapshot->'scope')='string' AND jsonb_typeof(snapshot->'state')='string'),
    CHECK(jsonb_typeof(snapshot->'revision')='number' AND jsonb_typeof(snapshot->'project'->'revision')='number'),
    CHECK(jsonb_typeof(snapshot->'project'->'id')='string' AND jsonb_typeof(snapshot->'project'->'code')='string' AND jsonb_typeof(snapshot->'project'->'display_name')='string'),
    CHECK(jsonb_typeof(snapshot->'currency')='string' AND jsonb_typeof(snapshot->'category')='string' AND jsonb_typeof(snapshot->'created_by')='string' AND jsonb_typeof(snapshot->'updated_by')='string' AND jsonb_typeof(snapshot->'created_at')='string' AND jsonb_typeof(snapshot->'updated_at')='string'),
    CHECK(snapshot->>'id'=id::text AND snapshot->>'tenant_id'=tenant_id::text),
    CHECK(snapshot->'project'->>'id'=project_id::text AND snapshot->>'scope'=scope),
    CHECK(snapshot->>'state'=state AND (snapshot->>'revision')::bigint=revision),
    CHECK(snapshot->>'currency'='CNY' AND (snapshot->>'amount_minor')::bigint>=0),
    CHECK((snapshot->'project'->>'revision')::bigint>0 AND char_length(snapshot->'project'->>'display_name')>0),
    CHECK(snapshot->>'category' IN ('vehicle','accommodation','transport','expert_fee','bid_signup_fee','bid_service_fee','other')),
    CHECK(scope='business' OR snapshot->>'category' NOT IN ('bid_signup_fee','bid_service_fee'))
);
CREATE INDEX reference_oa_cost_project_scope ON reference_oa_cost_entries(tenant_id,project_id,scope,id);
CREATE TABLE reference_oa_cost_revisions (
    tenant_id uuid NOT NULL, cost_entry_id uuid NOT NULL, revision bigint NOT NULL,
    command_id uuid NOT NULL, operation text NOT NULL CHECK(operation IN ('create','correct','withdraw')),
    reason text NOT NULL, snapshot jsonb NOT NULL,
    PRIMARY KEY(tenant_id,cost_entry_id,revision),
    FOREIGN KEY(tenant_id,cost_entry_id) REFERENCES reference_oa_cost_entries(tenant_id,id),
    CHECK(snapshot ?& ARRAY['revision','id','tenant_id']),
    CHECK(jsonb_typeof(snapshot->'revision')='number' AND jsonb_typeof(snapshot->'id')='string' AND jsonb_typeof(snapshot->'tenant_id')='string'),
    CHECK((operation='create' AND revision=1 AND reason='') OR (operation<>'create' AND revision>1 AND char_length(btrim(reason)) BETWEEN 1 AND 500)),
    CHECK((snapshot->>'revision')::bigint=revision AND snapshot->>'id'=cost_entry_id::text AND snapshot->>'tenant_id'=tenant_id::text)
);
CREATE TABLE reference_oa_cost_commands (
    tenant_id uuid NOT NULL, command_id uuid NOT NULL, actor_id uuid NOT NULL,
    payload_hash text NOT NULL CHECK(payload_hash ~ '^[a-f0-9]{64}$'),
    response jsonb NOT NULL, PRIMARY KEY(tenant_id,command_id)
);
CREATE TABLE reference_oa_cost_audit (
    tenant_id uuid NOT NULL, command_id uuid NOT NULL, cost_entry_id uuid NOT NULL,
    revision bigint NOT NULL, actor_id uuid NOT NULL, credential_id uuid NOT NULL,
    operation text NOT NULL, reason text NOT NULL, occurred_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,command_id),
    FOREIGN KEY(tenant_id,command_id) REFERENCES reference_oa_cost_commands(tenant_id,command_id),
    FOREIGN KEY(tenant_id,cost_entry_id,revision) REFERENCES reference_oa_cost_revisions(tenant_id,cost_entry_id,revision)
);
CREATE FUNCTION guard_reference_oa_cost_head() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.state='withdrawn' OR NEW.revision<>OLD.revision+1 OR
       NEW.id<>OLD.id OR NEW.tenant_id<>OLD.tenant_id OR NEW.project_id<>OLD.project_id OR NEW.scope<>OLD.scope OR
       NEW.snapshot->'project' IS DISTINCT FROM OLD.snapshot->'project' OR
       NEW.snapshot->>'created_by' IS DISTINCT FROM OLD.snapshot->>'created_by' OR
       NEW.snapshot->>'created_at' IS DISTINCT FROM OLD.snapshot->>'created_at' THEN
        RAISE EXCEPTION 'reference OA cost identity, side and frozen project cannot change';
    END IF;
    IF NEW.state='withdrawn' AND
       (NEW.snapshot - ARRAY['state','revision','updated_by','updated_at']) IS DISTINCT FROM
       (OLD.snapshot - ARRAY['state','revision','updated_by','updated_at']) THEN
        RAISE EXCEPTION 'withdrawal must retain the original cost fields and files';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER reference_oa_cost_head_guard BEFORE UPDATE ON reference_oa_cost_entries FOR EACH ROW EXECUTE FUNCTION guard_reference_oa_cost_head();
CREATE TRIGGER reference_oa_cost_head_no_delete BEFORE DELETE ON reference_oa_cost_entries FOR EACH ROW EXECUTE FUNCTION reject_cost_outsourcing_mutation();
CREATE TRIGGER reference_oa_cost_revisions_immutable BEFORE UPDATE OR DELETE ON reference_oa_cost_revisions FOR EACH ROW EXECUTE FUNCTION reject_cost_outsourcing_mutation();
CREATE TRIGGER reference_oa_cost_commands_immutable BEFORE UPDATE OR DELETE ON reference_oa_cost_commands FOR EACH ROW EXECUTE FUNCTION reject_cost_outsourcing_mutation();
CREATE TRIGGER reference_oa_cost_audit_immutable BEFORE UPDATE OR DELETE ON reference_oa_cost_audit FOR EACH ROW EXECUTE FUNCTION reject_cost_outsourcing_mutation();
