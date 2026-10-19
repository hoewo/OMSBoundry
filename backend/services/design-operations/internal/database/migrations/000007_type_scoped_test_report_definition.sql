ALTER TABLE design_work_items DROP CONSTRAINT design_work_items_type_definition_check;
ALTER TABLE design_work_items ADD CONSTRAINT design_work_items_type_definition_check
    CHECK ((type_key IN ('design-v1','consultation-v1') AND type_definition_version=1)
        OR (type_key='reference-oa-stage-set-v1' AND type_definition_version IN (2,3,4,5,6)));

ALTER TABLE design_work_items DROP CONSTRAINT design_work_items_current_stage_key_check;
ALTER TABLE design_work_items ADD CONSTRAINT design_work_items_current_stage_key_check
    CHECK (current_stage_key IS NULL OR current_stage_key IN ('scheme','preliminary-design','construction-drawing','construction-support','report','change','completion','test'));

ALTER TABLE design_stages DROP CONSTRAINT design_stages_stage_key_check;
ALTER TABLE design_stages DROP CONSTRAINT design_stages_sequence_check;
ALTER TABLE design_stages ADD CONSTRAINT design_stages_stage_key_check
    CHECK (stage_key IN ('scheme','preliminary-design','construction-drawing','construction-support','report','change','completion','test'));
ALTER TABLE design_stages ADD CONSTRAINT design_stages_sequence_check CHECK (sequence IN (10,20,30,40,50,60,70));

ALTER TABLE production_definition_upgrades DROP CONSTRAINT production_definition_upgrades_step_check;
ALTER TABLE production_definition_upgrades ADD CONSTRAINT production_definition_upgrades_step_check
    CHECK ((from_version=2 AND to_version=3)
        OR (from_version=3 AND to_version=4)
        OR (from_version=4 AND to_version=5)
        OR (from_version=5 AND to_version=6));

-- Production freezes only the Project type keys needed to explain stage
-- applicability at the verified Project revision. Project remains the owner of
-- current project types; this snapshot is immutable supporting evidence.
CREATE TABLE production_project_type_snapshots (
    tenant_id uuid NOT NULL,
    work_item_id uuid NOT NULL,
    project_id uuid NOT NULL,
    project_source_revision bigint NOT NULL CHECK (project_source_revision >= 1),
    project_type_key text NOT NULL CHECK (project_type_key IN ('design','consultation','survey','mapping')),
    captured_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,work_item_id,project_type_key),
    FOREIGN KEY (tenant_id,work_item_id) REFERENCES design_work_items(tenant_id,id) ON DELETE RESTRICT
);
CREATE TRIGGER production_project_type_snapshots_immutable
    BEFORE UPDATE OR DELETE ON production_project_type_snapshots
    FOR EACH ROW EXECUTE FUNCTION reject_design_immutable_change();

CREATE FUNCTION guard_production_v6_project_type_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.type_key='reference-oa-stage-set-v1' AND NEW.type_definition_version=6 AND NOT EXISTS (
        SELECT 1 FROM production_project_type_snapshots s
        WHERE s.tenant_id=NEW.tenant_id AND s.work_item_id=NEW.id
          AND s.project_id=NEW.project_id
          AND s.project_source_revision=NEW.project_source_revision
    ) THEN
        RAISE EXCEPTION 'missing production project type snapshot';
    END IF;
    RETURN NULL;
END;
$$;
CREATE CONSTRAINT TRIGGER production_v6_project_type_snapshot_required
    AFTER INSERT OR UPDATE OF type_definition_version ON design_work_items
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
    EXECUTE FUNCTION guard_production_v6_project_type_snapshot();

DROP TRIGGER production_definition_identity ON design_work_items;
DROP FUNCTION guard_production_definition_identity();
CREATE FUNCTION guard_production_definition_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.tenant_id,NEW.project_id,NEW.type_key) IS DISTINCT FROM ROW(OLD.tenant_id,OLD.project_id,OLD.type_key) THEN
        RAISE EXCEPTION 'immutable production definition identity';
    END IF;
    IF NEW.type_definition_version IS DISTINCT FROM OLD.type_definition_version AND NOT (
        OLD.type_key='reference-oa-stage-set-v1'
        AND ((OLD.type_definition_version=2 AND NEW.type_definition_version=3)
          OR (OLD.type_definition_version=3 AND NEW.type_definition_version=4)
          OR (OLD.type_definition_version=4 AND NEW.type_definition_version=5)
          OR (OLD.type_definition_version=5 AND NEW.type_definition_version=6))
        AND NEW.revision=OLD.revision+1
        AND EXISTS (SELECT 1 FROM production_definition_upgrades u WHERE u.tenant_id=NEW.tenant_id AND u.work_item_id=NEW.id AND u.from_version=OLD.type_definition_version AND u.to_version=NEW.type_definition_version AND u.upgraded_revision=NEW.revision)
    ) THEN
        RAISE EXCEPTION 'immutable production definition identity';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER production_definition_identity BEFORE UPDATE ON design_work_items FOR EACH ROW EXECUTE FUNCTION guard_production_definition_identity();

CREATE OR REPLACE FUNCTION guard_production_stage_material_insert() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE item design_work_items%ROWTYPE; ref production_stage_materials%ROWTYPE; n integer; applicable boolean;
BEGIN
    SELECT * INTO item FROM design_work_items WHERE tenant_id=NEW.tenant_id AND id=NEW.work_item_id FOR UPDATE;
    IF NOT FOUND OR item.type_key<>'reference-oa-stage-set-v1' OR item.type_definition_version NOT IN (2,3,4,5,6) OR item.status='cancelled' THEN
        RAISE EXCEPTION 'invalid material collection owner';
    END IF;
    applicable := true;
    IF item.type_definition_version=6 THEN
        applicable := CASE
            WHEN NEW.stage_key='scheme' THEN true
            WHEN NEW.stage_key IN ('preliminary-design','construction-drawing','change','completion') THEN EXISTS (
                SELECT 1 FROM production_project_type_snapshots s WHERE s.tenant_id=NEW.tenant_id AND s.work_item_id=NEW.work_item_id AND s.project_type_key='design')
            WHEN NEW.stage_key='test' THEN EXISTS (
                SELECT 1 FROM production_project_type_snapshots s WHERE s.tenant_id=NEW.tenant_id AND s.work_item_id=NEW.work_item_id AND s.project_type_key='survey')
            WHEN NEW.stage_key='report' THEN EXISTS (
                SELECT 1 FROM production_project_type_snapshots s WHERE s.tenant_id=NEW.tenant_id AND s.work_item_id=NEW.work_item_id AND s.project_type_key IN ('consultation','survey','mapping'))
            ELSE false
        END;
    END IF;
    IF NOT applicable THEN RAISE EXCEPTION 'stage not applicable to project type snapshot'; END IF;
    IF TG_TABLE_NAME='production_stage_material_endings' THEN
        SELECT * INTO ref FROM production_stage_materials WHERE tenant_id=NEW.tenant_id AND work_item_id=NEW.work_item_id AND stage_key=NEW.stage_key AND id=NEW.reference_id;
        IF NOT FOUND OR NEW.ended_revision<>item.revision OR NEW.ended_revision<=ref.created_revision THEN
            RAISE EXCEPTION 'invalid material ending revision';
        END IF;
    ELSE
        IF NEW.created_revision<>item.revision THEN RAISE EXCEPTION 'invalid material revision'; END IF;
        IF TG_TABLE_NAME='production_stage_scores' AND NEW.stage_key IN ('change','completion','test','report') THEN
            RAISE EXCEPTION 'main-only stage score not allowed';
        END IF;
        IF TG_TABLE_NAME='production_stage_materials' THEN
            IF NEW.stage_key IN ('change','completion','test','report') AND NEW.kind<>'main-file' THEN
                RAISE EXCEPTION 'main-only stage review not allowed';
            END IF;
            IF NEW.stage_key IN ('change','test','report') AND lower(NEW.display_snapshot) !~ '[.](pdf|doc|docx|xls|xlsx|png|jpg|jpeg|dwg|dxf)$' THEN
                RAISE EXCEPTION 'invalid stage file type';
            END IF;
            IF NEW.stage_key='completion' AND lower(NEW.display_snapshot) !~ '[.](pdf|doc|docx|xls|xlsx|zip|rar|png|jpg|jpeg|dwg|dxf)$' THEN
                RAISE EXCEPTION 'invalid completion file type';
            END IF;
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
