ALTER TABLE design_work_items DROP CONSTRAINT design_work_items_type_definition_check;
ALTER TABLE design_work_items ADD CONSTRAINT design_work_items_type_definition_check
    CHECK ((type_key IN ('design-v1','consultation-v1') AND type_definition_version=1)
        OR (type_key='reference-oa-stage-set-v1' AND type_definition_version IN (2,3)));

CREATE TABLE production_definition_upgrades (
    tenant_id uuid NOT NULL,
    work_item_id uuid NOT NULL,
    from_version bigint NOT NULL,
    to_version bigint NOT NULL,
    upgraded_revision bigint NOT NULL CHECK (upgraded_revision >= 2),
    command_id uuid NOT NULL,
    actor_subject uuid NOT NULL,
    upgraded_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,work_item_id,to_version),
    UNIQUE (tenant_id,command_id),
    FOREIGN KEY (tenant_id,work_item_id) REFERENCES design_work_items(tenant_id,id) ON DELETE RESTRICT,
    CHECK (from_version=2 AND to_version=3)
);
CREATE TRIGGER production_definition_upgrades_immutable BEFORE UPDATE OR DELETE ON production_definition_upgrades FOR EACH ROW EXECUTE FUNCTION reject_design_immutable_change();

DROP TRIGGER production_definition_identity ON design_work_items;
DROP FUNCTION guard_production_definition_identity();
CREATE FUNCTION guard_production_definition_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.tenant_id,NEW.project_id,NEW.type_key) IS DISTINCT FROM ROW(OLD.tenant_id,OLD.project_id,OLD.type_key) THEN
        RAISE EXCEPTION 'immutable production definition identity';
    END IF;
    IF NEW.type_definition_version IS DISTINCT FROM OLD.type_definition_version AND NOT (
        OLD.type_key='reference-oa-stage-set-v1' AND OLD.type_definition_version=2 AND NEW.type_definition_version=3
        AND NEW.revision=OLD.revision+1
        AND EXISTS (SELECT 1 FROM production_definition_upgrades u WHERE u.tenant_id=NEW.tenant_id AND u.work_item_id=NEW.id AND u.from_version=2 AND u.to_version=3 AND u.upgraded_revision=NEW.revision)
    ) THEN
        RAISE EXCEPTION 'immutable production definition identity';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER production_definition_identity BEFORE UPDATE ON design_work_items FOR EACH ROW EXECUTE FUNCTION guard_production_definition_identity();

CREATE OR REPLACE FUNCTION guard_production_stage_truth() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.tenant_id::text||':'||NEW.project_id::text,0));
    IF NEW.status<>'cancelled' AND EXISTS (
        SELECT 1 FROM design_work_items w WHERE w.tenant_id=NEW.tenant_id
        AND w.project_id=NEW.project_id AND w.id<>NEW.id AND w.status<>'cancelled'
        AND (NEW.type_key='reference-oa-stage-set-v1' OR w.type_key='reference-oa-stage-set-v1')
    ) THEN
        RAISE EXCEPTION 'parallel production stage truth' USING ERRCODE='23505', CONSTRAINT='design_work_items_active_project_type_idx';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION guard_production_stage_material_insert() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE item design_work_items%ROWTYPE; ref production_stage_materials%ROWTYPE; n integer;
BEGIN
    SELECT * INTO item FROM design_work_items WHERE tenant_id=NEW.tenant_id AND id=NEW.work_item_id FOR UPDATE;
    IF NOT FOUND OR item.type_key<>'reference-oa-stage-set-v1' OR item.type_definition_version NOT IN (2,3) OR item.status='cancelled' THEN
        RAISE EXCEPTION 'invalid material collection owner';
    END IF;
    IF TG_TABLE_NAME='production_stage_material_endings' THEN
        SELECT * INTO ref FROM production_stage_materials WHERE tenant_id=NEW.tenant_id AND work_item_id=NEW.work_item_id AND stage_key=NEW.stage_key AND id=NEW.reference_id;
        IF NOT FOUND OR NEW.ended_revision<>item.revision OR NEW.ended_revision<=ref.created_revision THEN
            RAISE EXCEPTION 'invalid material ending revision';
        END IF;
    ELSE
        IF NEW.created_revision<>item.revision THEN RAISE EXCEPTION 'invalid material revision'; END IF;
        IF TG_TABLE_NAME='production_stage_materials' THEN
            SELECT count(*) INTO n FROM production_stage_materials m WHERE m.tenant_id=NEW.tenant_id AND m.work_item_id=NEW.work_item_id AND m.stage_key=NEW.stage_key AND m.kind=NEW.kind AND NOT EXISTS (SELECT 1 FROM production_stage_material_endings e WHERE e.reference_id=m.id);
            IF n>=100 THEN RAISE EXCEPTION 'material collection limit'; END IF;
            IF EXISTS (SELECT 1 FROM production_stage_materials m WHERE m.tenant_id=NEW.tenant_id AND m.work_item_id=NEW.work_item_id AND m.stage_key=NEW.stage_key AND m.kind=NEW.kind AND m.file_id=NEW.file_id AND m.file_version=NEW.file_version AND NOT EXISTS (SELECT 1 FROM production_stage_material_endings e WHERE e.reference_id=m.id)) THEN
                RAISE EXCEPTION 'duplicate active material';
            END IF;
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

ALTER TABLE design_history_events DROP CONSTRAINT design_history_events_event_type_check;
ALTER TABLE design_history_events ADD CONSTRAINT design_history_events_event_type_check CHECK (event_type IN ('work_item_created','owner_set','deliverable_registered','stage_not_applicable','stage_applicability_restored','stage_completed','stage_reopened','work_item_cancelled','stage_materials_registered','stage_material_withdrawn','definition_upgraded'));
