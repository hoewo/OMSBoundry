ALTER TABLE design_work_items DROP CONSTRAINT design_work_items_type_definition_check;
ALTER TABLE design_work_items ADD CONSTRAINT design_work_items_type_definition_check
    CHECK ((type_key IN ('design-v1','consultation-v1') AND type_definition_version=1)
        OR (type_key='reference-oa-stage-set-v1' AND type_definition_version=2));

-- A shared project-stage collection cannot coexist with old per-type writable
-- tasks. Serialize both creation directions; existing v1 facts remain intact.
CREATE FUNCTION guard_production_stage_truth() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.tenant_id::text||':'||NEW.project_id::text,0));
    IF NEW.status<>'cancelled' AND EXISTS (
        SELECT 1 FROM design_work_items w WHERE w.tenant_id=NEW.tenant_id
        AND w.project_id=NEW.project_id AND w.id<>NEW.id AND w.status<>'cancelled'
        AND (NEW.type_definition_version=2 OR w.type_definition_version=2)
    ) THEN
        RAISE EXCEPTION 'parallel production stage truth' USING ERRCODE='23505', CONSTRAINT='design_work_items_active_project_type_idx';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER production_stage_truth BEFORE INSERT OR UPDATE OF status ON design_work_items
    FOR EACH ROW EXECUTE FUNCTION guard_production_stage_truth();

CREATE FUNCTION guard_production_definition_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.tenant_id,NEW.project_id,NEW.type_key,NEW.type_definition_version)
       IS DISTINCT FROM ROW(OLD.tenant_id,OLD.project_id,OLD.type_key,OLD.type_definition_version) THEN
        RAISE EXCEPTION 'immutable production definition identity';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER production_definition_identity BEFORE UPDATE ON design_work_items
    FOR EACH ROW EXECUTE FUNCTION guard_production_definition_identity();

CREATE TABLE production_stage_materials (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    work_item_id uuid NOT NULL,
    stage_key text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('main-file','review-sheet')),
    file_id uuid NOT NULL,
    file_version integer NOT NULL CHECK (file_version >= 1),
    source_revision bigint NOT NULL CHECK (source_revision = file_version),
    display_snapshot text NOT NULL CHECK (display_snapshot=btrim(display_snapshot) AND char_length(display_snapshot) BETWEEN 1 AND 255),
    created_revision bigint NOT NULL CHECK (created_revision >= 2),
    batch_position integer NOT NULL CHECK (batch_position BETWEEN 1 AND 20),
    created_at timestamptz NOT NULL,
    UNIQUE (tenant_id,work_item_id,stage_key,id),
    UNIQUE (tenant_id,work_item_id,stage_key,kind,created_revision,batch_position),
    FOREIGN KEY (tenant_id,work_item_id,stage_key) REFERENCES design_stages(tenant_id,work_item_id,stage_key) ON DELETE RESTRICT
);
CREATE INDEX production_stage_materials_current_idx ON production_stage_materials(tenant_id,work_item_id,stage_key,kind,created_revision,batch_position,id);
CREATE TABLE production_stage_material_endings (
    reference_id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    work_item_id uuid NOT NULL,
    stage_key text NOT NULL,
    ended_revision bigint NOT NULL CHECK (ended_revision >= 3),
    reason text NOT NULL CHECK (reason=btrim(reason) AND char_length(reason) BETWEEN 1 AND 1000),
    ended_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id,work_item_id,stage_key,reference_id) REFERENCES production_stage_materials(tenant_id,work_item_id,stage_key,id) ON DELETE RESTRICT
);
CREATE TABLE production_stage_scores (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    work_item_id uuid NOT NULL,
    stage_key text NOT NULL,
    score text NOT NULL CHECK (char_length(score) BETWEEN 1 AND 128 AND score ~ '^[0-9]+([.][0-9]+)?$' AND score::numeric BETWEEN 0 AND 100),
    created_revision bigint NOT NULL CHECK (created_revision >= 2),
    created_at timestamptz NOT NULL,
    UNIQUE (tenant_id,work_item_id,stage_key,created_revision),
    FOREIGN KEY (tenant_id,work_item_id,stage_key) REFERENCES design_stages(tenant_id,work_item_id,stage_key) ON DELETE RESTRICT
);
CREATE TRIGGER production_stage_materials_immutable BEFORE UPDATE OR DELETE ON production_stage_materials FOR EACH ROW EXECUTE FUNCTION reject_design_immutable_change();
CREATE TRIGGER production_stage_material_endings_immutable BEFORE UPDATE OR DELETE ON production_stage_material_endings FOR EACH ROW EXECUTE FUNCTION reject_design_immutable_change();
CREATE TRIGGER production_stage_scores_immutable BEFORE UPDATE OR DELETE ON production_stage_scores FOR EACH ROW EXECUTE FUNCTION reject_design_immutable_change();

-- Parent row locking also protects direct SQL writers against duplicate active references.
CREATE FUNCTION guard_production_stage_material_insert() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE item design_work_items%ROWTYPE; ref production_stage_materials%ROWTYPE; n integer;
BEGIN
    SELECT * INTO item FROM design_work_items WHERE tenant_id=NEW.tenant_id AND id=NEW.work_item_id FOR UPDATE;
    IF NOT FOUND OR item.type_definition_version<>2 OR item.status='cancelled' THEN
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
CREATE TRIGGER production_stage_materials_guard BEFORE INSERT ON production_stage_materials FOR EACH ROW EXECUTE FUNCTION guard_production_stage_material_insert();
CREATE TRIGGER production_stage_material_endings_guard BEFORE INSERT ON production_stage_material_endings FOR EACH ROW EXECUTE FUNCTION guard_production_stage_material_insert();
CREATE TRIGGER production_stage_scores_guard BEFORE INSERT ON production_stage_scores FOR EACH ROW EXECUTE FUNCTION guard_production_stage_material_insert();

CREATE FUNCTION guard_legacy_production_deliverable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM design_work_items WHERE tenant_id=NEW.tenant_id AND id=NEW.work_item_id AND type_definition_version=1) THEN
        RAISE EXCEPTION 'legacy deliverable requires definition v1';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER design_deliverables_definition_guard BEFORE INSERT OR UPDATE ON design_deliverables FOR EACH ROW EXECUTE FUNCTION guard_legacy_production_deliverable();
ALTER TABLE design_history_events DROP CONSTRAINT design_history_events_event_type_check;
ALTER TABLE design_history_events ADD CONSTRAINT design_history_events_event_type_check CHECK (event_type IN ('work_item_created','owner_set','deliverable_registered','stage_not_applicable','stage_applicability_restored','stage_completed','stage_reopened','work_item_cancelled','stage_materials_registered','stage_material_withdrawn'));
