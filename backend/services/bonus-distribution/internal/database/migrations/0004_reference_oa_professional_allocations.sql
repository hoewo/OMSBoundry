-- Explicit v3 consumer. No mutation of v2 snapshots, commands or Pool facts.
DO $$
DECLARE old_name text; matches integer;
BEGIN
 SELECT count(*),min(conname) INTO matches,old_name FROM pg_constraint
 WHERE conrelid='reference_oa_bonus_allocations'::regclass AND contype='c'
 AND pg_get_constraintdef(oid) LIKE '%role_key%commercial_contributor%'
 AND pg_get_constraintdef(oid) LIKE '%production_owner%participant%';
 IF matches<>1 THEN RAISE EXCEPTION 'expected exactly one v2 manual assignment policy'; END IF;
 EXECUTE format('ALTER TABLE reference_oa_bonus_allocations DROP CONSTRAINT %I',old_name);
END $$;
ALTER TABLE reference_oa_bonus_allocations ADD CONSTRAINT reference_oa_bonus_assignment_policy_v3 CHECK (
 coalesce(
  (NOT snapshot ?| ARRAY['assignment_source','profile','discipline'] AND
   ((scope='business' AND snapshot->'assignment'->>'role_key' IN ('business_owner','commercial_contributor'))
    OR (scope='production' AND snapshot->'assignment'->>'role_key' IN ('production_owner','participant'))))
  OR
  (snapshot->>'assignment_source'='reference-oa-production-personnel-v1'
   AND snapshot->>'profile'='reference-oa-bonus-v3' AND scope='production'
   AND snapshot->'assignment'->>'role_key' IN ('designer','participant','reviewer','auditor','approver','estimator')
   AND (
    (snapshot->'assignment'->>'role_key' IN ('approver','estimator') AND NOT snapshot ? 'discipline')
    OR (snapshot->'assignment'->>'role_key' IN ('designer','participant','reviewer','auditor')
     AND jsonb_typeof(snapshot->'discipline')='object'
     AND (snapshot->'discipline') ?& ARRAY['id','revision','display_name']
     AND jsonb_typeof(snapshot->'discipline'->'id')='string'
     AND jsonb_typeof(snapshot->'discipline'->'revision')='number'
     AND jsonb_typeof(snapshot->'discipline'->'display_name')='string'
     AND snapshot->'discipline'->>'id' ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
     AND (snapshot->'discipline'->>'revision')::bigint>0
     AND length(trim(snapshot->'discipline'->>'display_name'))>0)
   )),false)
);
CREATE UNIQUE INDEX reference_oa_bonus_batch_source_assignment
 ON reference_oa_bonus_allocations(tenant_id,batch_id,(coalesce(snapshot->>'assignment_source','')),((snapshot->'assignment'->>'id')));
DROP INDEX reference_oa_bonus_batch_assignment;
