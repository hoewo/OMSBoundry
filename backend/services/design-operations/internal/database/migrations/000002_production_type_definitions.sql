ALTER TABLE design_work_items
    DROP CONSTRAINT design_work_items_type_key_check,
    DROP CONSTRAINT design_work_items_type_definition_version_check,
    DROP CONSTRAINT design_work_items_current_stage_key_check;

ALTER TABLE design_work_items
    ADD CONSTRAINT design_work_items_type_definition_check CHECK (
        (type_key = 'design-v1' AND type_definition_version = 1)
        OR (type_key = 'consultation-v1' AND type_definition_version = 1)
    ),
    ADD CONSTRAINT design_work_items_current_stage_key_check CHECK (
        current_stage_key IS NULL
        OR current_stage_key IN ('scheme','preliminary-design','construction-drawing','construction-support','report')
    );

ALTER TABLE design_stages
    DROP CONSTRAINT design_stages_stage_key_check,
    DROP CONSTRAINT design_stages_sequence_check;

ALTER TABLE design_stages
    ADD CONSTRAINT design_stages_stage_key_check CHECK (
        stage_key IN ('scheme','preliminary-design','construction-drawing','construction-support','report')
    ),
    ADD CONSTRAINT design_stages_sequence_check CHECK (sequence IN (10,20,30,40));
