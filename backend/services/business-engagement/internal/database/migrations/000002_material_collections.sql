-- Preserve all existing material/history/command rows. Legacy put/remove keep
-- their replace-all/remove-all semantics; additive commands mutate one member.
DROP INDEX business_engagement_materials_active_key;

CREATE UNIQUE INDEX business_engagement_materials_active_version_key
    ON business_engagement_materials
       (tenant_id, engagement_id, requirement_key, evidence_file_id, evidence_version)
    WHERE superseded_at IS NULL;

ALTER TABLE business_engagement_history_events
    DROP CONSTRAINT business_engagement_history_events_event_type_check;
ALTER TABLE business_engagement_history_events
    ADD CONSTRAINT business_engagement_history_events_event_type_check
    CHECK (event_type IN ('engagement_created','applicability_changed','method_selected',
                         'material_put','material_remove','engagement_cancelled',
                         'material_append','material_remove_one'));
