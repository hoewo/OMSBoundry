ALTER TABLE project_history_events
    DROP CONSTRAINT project_history_events_event_type_check,
    ADD CONSTRAINT project_history_events_event_type_check CHECK (event_type IN (
        'project_created', 'project_details_replaced', 'project_initiation_pending',
        'project_activated', 'project_cancelled', 'assignment_started', 'assignment_ended',
        'resource_linked', 'resource_unlinked', 'project_attached_to_package',
        'project_wrapped_as_package', 'package_child_attached', 'package_child_detached',
        'project_package_conversion_rolled_back', 'completion_profile_confirmed'
    ));

CREATE TABLE project_completion_confirmations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    profile_key text NOT NULL CHECK (profile_key = 'reference-oa-c5-v1'),
    observed_project_revision bigint NOT NULL CHECK (observed_project_revision >= 1),
    confirmed_project_revision bigint NOT NULL CHECK (confirmed_project_revision = observed_project_revision + 1),
    sources_json jsonb NOT NULL CHECK (jsonb_typeof(sources_json) = 'array' AND jsonb_array_length(sources_json) = 2),
    command_id uuid NOT NULL,
    confirmed_by uuid NOT NULL,
    confirmed_at timestamptz NOT NULL,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, command_id),
    UNIQUE (tenant_id, project_id, confirmed_project_revision),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX project_completion_confirmations_latest_idx
    ON project_completion_confirmations (tenant_id, project_id, profile_key, confirmed_project_revision DESC, id DESC);

CREATE TRIGGER project_completion_confirmations_immutable
    BEFORE UPDATE OR DELETE ON project_completion_confirmations
    FOR EACH ROW EXECUTE FUNCTION reject_project_immutable_change();
