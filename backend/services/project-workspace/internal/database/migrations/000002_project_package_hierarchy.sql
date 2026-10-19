ALTER TABLE projects
    ADD COLUMN node_type text NOT NULL DEFAULT 'project',
    ADD COLUMN parent_project_id uuid NULL,
    ADD COLUMN sort_order integer NOT NULL DEFAULT 0;

ALTER TABLE projects
    DROP CONSTRAINT projects_project_number_check,
    DROP CONSTRAINT projects_tenant_id_project_number_normalized_key,
    ADD CONSTRAINT projects_node_type_check CHECK (node_type IN ('project', 'package')),
    ADD CONSTRAINT projects_project_number_check CHECK (
        project_number = btrim(project_number)
        AND char_length(project_number) <= 64
        AND (node_type = 'package' OR char_length(project_number) >= 1)
    ),
    ADD CONSTRAINT projects_parent_shape_check CHECK (
        (node_type = 'package' AND parent_project_id IS NULL)
        OR node_type = 'project'
    ),
    ADD CONSTRAINT projects_sort_order_check CHECK (sort_order >= 0),
    ADD CONSTRAINT projects_parent_fk FOREIGN KEY (tenant_id, parent_project_id)
        REFERENCES projects(tenant_id, id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX projects_tenant_number_active_idx
    ON projects (tenant_id, project_number_normalized)
    WHERE project_number_normalized <> '';
CREATE INDEX projects_parent_idx
    ON projects (tenant_id, parent_project_id, sort_order, created_at, id)
    WHERE parent_project_id IS NOT NULL;

CREATE FUNCTION enforce_project_parent_is_package() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    parent_type text;
BEGIN
    IF NEW.parent_project_id IS NOT NULL THEN
        SELECT node_type INTO parent_type
        FROM projects
        WHERE tenant_id = NEW.tenant_id AND id = NEW.parent_project_id;
        IF parent_type IS DISTINCT FROM 'package' THEN
            RAISE EXCEPTION 'project parent must be a package' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER projects_parent_package_guard
    BEFORE INSERT OR UPDATE OF tenant_id, node_type, parent_project_id ON projects
    FOR EACH ROW EXECUTE FUNCTION enforce_project_parent_is_package();

ALTER TABLE project_history_events
    DROP CONSTRAINT project_history_events_event_type_check,
    ADD CONSTRAINT project_history_events_event_type_check CHECK (event_type IN (
        'project_created', 'project_details_replaced', 'project_initiation_pending',
        'project_activated', 'project_cancelled', 'assignment_started', 'assignment_ended',
        'resource_linked', 'resource_unlinked', 'project_attached_to_package',
        'project_wrapped_as_package', 'package_child_attached', 'package_child_detached',
        'project_package_conversion_rolled_back'
    ));

CREATE TABLE project_package_conversions (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    operation text NOT NULL CHECK (operation IN ('attach_existing', 'wrap_as_package')),
    project_id uuid NOT NULL,
    package_id uuid NOT NULL,
    previous_parent_project_id uuid NULL,
    previous_sort_order integer NOT NULL CHECK (previous_sort_order >= 0),
    created_package boolean NOT NULL,
    created_project_revision bigint NOT NULL CHECK (created_project_revision >= 1),
    created_package_revision bigint NOT NULL CHECK (created_package_revision >= 1),
    status text NOT NULL CHECK (status IN ('active', 'rolled_back')),
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    rolled_back_by uuid NULL,
    rolled_back_at timestamptz NULL,
    rollback_reason text NULL CHECK (rollback_reason IS NULL OR (rollback_reason = btrim(rollback_reason) AND char_length(rollback_reason) BETWEEN 1 AND 1000)),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, package_id) REFERENCES projects(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, previous_parent_project_id) REFERENCES projects(tenant_id, id) ON DELETE RESTRICT,
    CHECK (
        (status = 'active' AND rolled_back_by IS NULL AND rolled_back_at IS NULL AND rollback_reason IS NULL)
        OR (status = 'rolled_back' AND rolled_back_by IS NOT NULL AND rolled_back_at IS NOT NULL AND rollback_reason IS NOT NULL)
    )
);
CREATE UNIQUE INDEX project_package_conversions_active_project_idx
    ON project_package_conversions (tenant_id, project_id)
    WHERE status = 'active';
CREATE INDEX project_package_conversions_package_idx
    ON project_package_conversions (tenant_id, package_id, created_at, id);

CREATE TRIGGER project_package_conversions_immutable
    BEFORE DELETE ON project_package_conversions
    FOR EACH ROW EXECUTE FUNCTION reject_project_immutable_change();
