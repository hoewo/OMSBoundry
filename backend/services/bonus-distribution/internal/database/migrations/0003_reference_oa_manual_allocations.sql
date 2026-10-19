-- Explicit customer overlay. Existing confirmed-source Pool tables are untouched.
CREATE TABLE reference_oa_bonus_allocations (
 tenant_id uuid NOT NULL, id uuid NOT NULL, batch_id uuid NOT NULL,
 project_id uuid NOT NULL, scope text NOT NULL CHECK(scope IN ('business','production')),
 state text NOT NULL CHECK(state IN ('recorded','withdrawn')),
 revision bigint NOT NULL CHECK(revision>=1), snapshot jsonb NOT NULL,
 PRIMARY KEY(tenant_id,id),
 CHECK(jsonb_typeof(snapshot)='object'),
 CHECK(snapshot ?& ARRAY['tenant_id','id','batch_id','project','scope','state','revision','currency',
   'base_amount_minor','person','assignment','method','ratio_basis_points','fixed_amount_minor',
   'amount_minor','occurred_on','description','created_by','updated_by','created_at','updated_at']),
 CHECK(jsonb_typeof(snapshot->'project')='object' AND (snapshot->'project') ?& ARRAY['id','revision','code','display_name','digest']),
 CHECK(jsonb_typeof(snapshot->'person')='object' AND (snapshot->'person') ?& ARRAY['id','revision','display_name','digest']),
 CHECK(jsonb_typeof(snapshot->'assignment')='object' AND (snapshot->'assignment') ?& ARRAY['id','revision','project_id','person_id','role_key','digest']),
 CHECK(jsonb_typeof(snapshot->'revision')='number' AND jsonb_typeof(snapshot->'base_amount_minor')='number'
   AND jsonb_typeof(snapshot->'ratio_basis_points')='number' AND jsonb_typeof(snapshot->'fixed_amount_minor')='number'
   AND jsonb_typeof(snapshot->'amount_minor')='number'),
 CHECK(jsonb_typeof(snapshot->'occurred_on')='string' AND snapshot->>'occurred_on' ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'
   AND jsonb_typeof(snapshot->'description')='string' AND length(snapshot->>'description')<=1000),
 CHECK((snapshot->'project'->>'revision')::bigint>0 AND (snapshot->'person'->>'revision')::bigint>0
   AND (snapshot->'assignment'->>'revision')::bigint>0),
 CHECK(snapshot->>'tenant_id'=tenant_id::text AND snapshot->>'id'=id::text
   AND snapshot->>'batch_id'=batch_id::text AND snapshot->'project'->>'id'=project_id::text
   AND snapshot->>'scope'=scope AND snapshot->>'state'=state
   AND (snapshot->>'revision')::bigint=revision AND snapshot->>'currency'='CNY'),
 CHECK((snapshot->>'amount_minor')::bigint>0 AND (snapshot->>'base_amount_minor')::bigint>=0),
 CHECK(snapshot->'assignment'->>'project_id'=project_id::text
   AND snapshot->'assignment'->>'person_id'=snapshot->'person'->>'id'),
 CHECK((scope='business' AND snapshot->'assignment'->>'role_key' IN ('business_owner','commercial_contributor'))
   OR (scope='production' AND snapshot->'assignment'->>'role_key' IN ('production_owner','participant'))),
 CHECK((snapshot->>'method'='ratio' AND (snapshot->>'base_amount_minor')::bigint>0
     AND (snapshot->>'ratio_basis_points')::bigint BETWEEN 1 AND 10000
     AND (snapshot->>'fixed_amount_minor')::bigint=0
     AND (snapshot->>'amount_minor')::numeric=floor(((snapshot->>'base_amount_minor')::numeric
       *(snapshot->>'ratio_basis_points')::numeric+5000)/10000))
   OR (snapshot->>'method'='fixed' AND (snapshot->>'ratio_basis_points')::bigint=0
     AND (snapshot->>'fixed_amount_minor')::bigint=(snapshot->>'amount_minor')::bigint))
);
CREATE UNIQUE INDEX reference_oa_bonus_batch_assignment
 ON reference_oa_bonus_allocations(tenant_id,batch_id,((snapshot->'assignment'->>'id')));
CREATE INDEX reference_oa_bonus_project ON reference_oa_bonus_allocations(tenant_id,project_id,scope,id);
CREATE TABLE reference_oa_bonus_commands (
 tenant_id uuid NOT NULL, command_id uuid NOT NULL, actor_id uuid NOT NULL,
 payload_hash text NOT NULL CHECK(payload_hash ~ '^[0-9a-f]{64}$'),
 response jsonb NOT NULL CHECK(jsonb_typeof(response)='array' AND jsonb_array_length(response) BETWEEN 1 AND 100),
 PRIMARY KEY(tenant_id,command_id)
);
CREATE TABLE reference_oa_bonus_revisions (
 tenant_id uuid NOT NULL, allocation_id uuid NOT NULL, revision bigint NOT NULL CHECK(revision>=1),
 command_id uuid NOT NULL, operation text NOT NULL CHECK(operation IN ('record','withdraw')),
 reason text NOT NULL, snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object'
   AND snapshot ?& ARRAY['tenant_id','id','revision','state']),
 PRIMARY KEY(tenant_id,allocation_id,revision),
 FOREIGN KEY(tenant_id,allocation_id) REFERENCES reference_oa_bonus_allocations(tenant_id,id) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(tenant_id,command_id) REFERENCES reference_oa_bonus_commands(tenant_id,command_id) DEFERRABLE INITIALLY DEFERRED,
 CHECK((operation='record' AND revision=1 AND reason='' AND snapshot->>'state'='recorded')
   OR (operation='withdraw' AND revision=2 AND length(trim(reason)) BETWEEN 1 AND 500 AND snapshot->>'state'='withdrawn')),
 CHECK(snapshot->>'tenant_id'=tenant_id::text AND snapshot->>'id'=allocation_id::text AND (snapshot->>'revision')::bigint=revision)
);
CREATE TABLE reference_oa_bonus_audit (
 tenant_id uuid NOT NULL, command_id uuid NOT NULL, allocation_id uuid NOT NULL, revision bigint NOT NULL,
 actor_id uuid NOT NULL, credential_id text NOT NULL CHECK(length(credential_id) BETWEEN 1 AND 256),
 operation text NOT NULL CHECK(operation IN ('record','withdraw')), reason text NOT NULL, occurred_at timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,command_id,allocation_id),
 FOREIGN KEY(tenant_id,allocation_id,revision) REFERENCES reference_oa_bonus_revisions(tenant_id,allocation_id,revision) DEFERRABLE INITIALLY DEFERRED
);
CREATE FUNCTION guard_reference_oa_bonus_head() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'manual bonus cannot be physically deleted'; END IF;
 IF OLD.state<>'recorded' OR NEW.state<>'withdrawn' OR NEW.revision<>OLD.revision+1
   OR (to_jsonb(NEW)-ARRAY['state','revision','snapshot'])<>(to_jsonb(OLD)-ARRAY['state','revision','snapshot'])
   OR (NEW.snapshot-ARRAY['state','revision','updated_by','updated_at'])<>(OLD.snapshot-ARRAY['state','revision','updated_by','updated_at']) THEN
   RAISE EXCEPTION 'manual bonus only permits append-only withdrawal';
 END IF;
 RETURN NEW;
END;
$$;
CREATE FUNCTION require_reference_oa_bonus_history() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM reference_oa_bonus_revisions r WHERE r.tenant_id=NEW.tenant_id
   AND r.allocation_id=NEW.id AND r.revision=NEW.revision AND r.snapshot=NEW.snapshot) THEN
   RAISE EXCEPTION 'manual bonus matching history required';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER reference_oa_bonus_head_guard BEFORE UPDATE OR DELETE ON reference_oa_bonus_allocations
 FOR EACH ROW EXECUTE FUNCTION guard_reference_oa_bonus_head();
CREATE CONSTRAINT TRIGGER reference_oa_bonus_history_required AFTER INSERT OR UPDATE ON reference_oa_bonus_allocations
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION require_reference_oa_bonus_history();
CREATE TRIGGER reference_oa_bonus_revisions_immutable BEFORE UPDATE OR DELETE ON reference_oa_bonus_revisions
 FOR EACH ROW EXECUTE FUNCTION reject_bonus_append_only_mutation();
CREATE TRIGGER reference_oa_bonus_commands_immutable BEFORE UPDATE OR DELETE ON reference_oa_bonus_commands
 FOR EACH ROW EXECUTE FUNCTION reject_bonus_append_only_mutation();
CREATE TRIGGER reference_oa_bonus_audit_immutable BEFORE UPDATE OR DELETE ON reference_oa_bonus_audit
 FOR EACH ROW EXECUTE FUNCTION reject_bonus_append_only_mutation();
