ALTER TABLE design_work_items DROP CONSTRAINT design_work_items_type_definition_check;
ALTER TABLE design_work_items ADD CONSTRAINT design_work_items_type_definition_check
    CHECK ((type_key IN ('design-v1','consultation-v1') AND type_definition_version=1)
        OR (type_key='reference-oa-stage-set-v1' AND type_definition_version IN (2,3,4,5)));

ALTER TABLE design_work_items DROP CONSTRAINT design_work_items_current_stage_key_check;
ALTER TABLE design_work_items ADD CONSTRAINT design_work_items_current_stage_key_check
    CHECK (current_stage_key IS NULL OR current_stage_key IN ('scheme','preliminary-design','construction-drawing','construction-support','report','change','completion'));

ALTER TABLE design_stages DROP CONSTRAINT design_stages_stage_key_check;
ALTER TABLE design_stages DROP CONSTRAINT design_stages_sequence_check;
ALTER TABLE design_stages ADD CONSTRAINT design_stages_stage_key_check
    CHECK (stage_key IN ('scheme','preliminary-design','construction-drawing','construction-support','report','change','completion'));
ALTER TABLE design_stages ADD CONSTRAINT design_stages_sequence_check CHECK (sequence IN (10,20,30,40,50));

ALTER TABLE production_definition_upgrades DROP CONSTRAINT production_definition_upgrades_step_check;
ALTER TABLE production_definition_upgrades ADD CONSTRAINT production_definition_upgrades_step_check
    CHECK ((from_version=2 AND to_version=3)
        OR (from_version=3 AND to_version=4)
        OR (from_version=4 AND to_version=5));

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
          OR (OLD.type_definition_version=4 AND NEW.type_definition_version=5))
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
DECLARE item design_work_items%ROWTYPE; ref production_stage_materials%ROWTYPE; n integer;
BEGIN
    SELECT * INTO item FROM design_work_items WHERE tenant_id=NEW.tenant_id AND id=NEW.work_item_id FOR UPDATE;
    IF NOT FOUND OR item.type_key<>'reference-oa-stage-set-v1' OR item.type_definition_version NOT IN (2,3,4,5) OR item.status='cancelled' THEN
        RAISE EXCEPTION 'invalid material collection owner';
    END IF;
    IF TG_TABLE_NAME='production_stage_material_endings' THEN
        SELECT * INTO ref FROM production_stage_materials WHERE tenant_id=NEW.tenant_id AND work_item_id=NEW.work_item_id AND stage_key=NEW.stage_key AND id=NEW.reference_id;
        IF NOT FOUND OR NEW.ended_revision<>item.revision OR NEW.ended_revision<=ref.created_revision THEN
            RAISE EXCEPTION 'invalid material ending revision';
        END IF;
    ELSE
        IF NEW.created_revision<>item.revision THEN RAISE EXCEPTION 'invalid material revision'; END IF;
		IF TG_TABLE_NAME='production_stage_scores' AND NEW.stage_key IN ('change','completion') THEN
			RAISE EXCEPTION 'auxiliary stage score not allowed';
		END IF;
        IF TG_TABLE_NAME='production_stage_materials' THEN
			IF NEW.stage_key IN ('change','completion') AND NEW.kind<>'main-file' THEN
				RAISE EXCEPTION 'auxiliary stage review not allowed';
			END IF;
			IF NEW.stage_key='change' AND lower(NEW.display_snapshot) !~ '[.](pdf|doc|docx|xls|xlsx|png|jpg|jpeg|dwg|dxf)$' THEN
				RAISE EXCEPTION 'invalid change file type';
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
