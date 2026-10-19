CREATE TABLE projects (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_number text NOT NULL CHECK (project_number = btrim(project_number) AND char_length(project_number) BETWEEN 1 AND 64),
    project_number_normalized text NOT NULL,
    name text NOT NULL CHECK (name = btrim(name) AND char_length(name) BETWEEN 1 AND 200),
    summary text NOT NULL DEFAULT '' CHECK (summary = btrim(summary) AND char_length(summary) <= 2000),
    planned_start_date date NULL,
    planned_end_date date NULL,
    project_type_keys text[] NOT NULL CHECK (cardinality(project_type_keys) BETWEEN 1 AND 16),
    status text NOT NULL CHECK (status IN ('draft', 'pending_completion', 'active', 'cancelled')),
    revision bigint NOT NULL CHECK (revision >= 1),
    cancellation_reason text NULL CHECK (cancellation_reason IS NULL OR (cancellation_reason = btrim(cancellation_reason) AND char_length(cancellation_reason) BETWEEN 1 AND 1000)),
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, project_number_normalized),
    CHECK (planned_end_date IS NULL OR planned_start_date IS NULL OR planned_end_date >= planned_start_date),
    CHECK ((status = 'cancelled' AND cancellation_reason IS NOT NULL) OR (status <> 'cancelled' AND cancellation_reason IS NULL))
);
CREATE INDEX projects_tenant_status_idx ON projects (tenant_id, status, created_at DESC, id DESC);

CREATE TABLE project_assignments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    subject_kind text NOT NULL CHECK (subject_kind IN ('person', 'group')),
    subject_id uuid NOT NULL,
    source_revision bigint NOT NULL CHECK (source_revision >= 1),
    display_snapshot text NOT NULL CHECK (display_snapshot = btrim(display_snapshot) AND char_length(display_snapshot) BETWEEN 1 AND 200),
    role_key text NOT NULL CHECK (role_key IN ('business_owner', 'production_owner', 'participant')),
    effective_from timestamptz NOT NULL,
    ended_at timestamptz NULL,
    end_reason text NULL CHECK (end_reason IS NULL OR (end_reason = btrim(end_reason) AND char_length(end_reason) BETWEEN 1 AND 500)),
    created_project_revision bigint NOT NULL CHECK (created_project_revision >= 1),
    ended_project_revision bigint NULL CHECK (ended_project_revision IS NULL OR ended_project_revision > created_project_revision),
    created_at timestamptz NOT NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id) ON DELETE RESTRICT,
    CHECK ((ended_at IS NULL AND end_reason IS NULL AND ended_project_revision IS NULL) OR (ended_at IS NOT NULL AND end_reason IS NOT NULL AND ended_project_revision IS NOT NULL))
);
CREATE UNIQUE INDEX project_assignments_active_owner_idx ON project_assignments (tenant_id, project_id, role_key) WHERE ended_at IS NULL AND role_key IN ('business_owner', 'production_owner');
CREATE UNIQUE INDEX project_assignments_active_subject_role_idx ON project_assignments (tenant_id, project_id, subject_kind, subject_id, role_key) WHERE ended_at IS NULL;
CREATE INDEX project_assignments_project_idx ON project_assignments (tenant_id, project_id, created_at, id);

CREATE TABLE project_resource_links (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    target_service text NOT NULL CHECK (target_service IN ('contract-ledger', 'money-flow-ledger', 'evidence-service', 'design-operations')),
    resource_type text NOT NULL CHECK (resource_type IN ('contract', 'obligation', 'transaction', 'evidence-file', 'design-work-item')),
    resource_id uuid NOT NULL,
    source_revision bigint NULL CHECK (source_revision IS NULL OR source_revision >= 1),
    relation_key text NOT NULL CHECK (relation_key ~ '^[a-z][a-z0-9-]{0,63}$'),
    note text NOT NULL DEFAULT '' CHECK (note = btrim(note) AND char_length(note) <= 500),
    created_project_revision bigint NOT NULL CHECK (created_project_revision >= 1),
    unlinked_project_revision bigint NULL CHECK (unlinked_project_revision IS NULL OR unlinked_project_revision > created_project_revision),
    linked_at timestamptz NOT NULL,
    unlinked_at timestamptz NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id) ON DELETE RESTRICT,
    CHECK ((unlinked_at IS NULL AND unlinked_project_revision IS NULL) OR (unlinked_at IS NOT NULL AND unlinked_project_revision IS NOT NULL))
);
CREATE UNIQUE INDEX project_resource_links_active_idx ON project_resource_links (tenant_id, project_id, target_service, resource_type, resource_id, relation_key) WHERE unlinked_at IS NULL;
CREATE INDEX project_resource_links_target_idx ON project_resource_links (tenant_id, target_service, resource_type, resource_id);

CREATE TABLE project_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    response_status integer NOT NULL CHECK (response_status BETWEEN 200 AND 599),
    response_json jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, command_id)
);

CREATE TABLE project_history_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    project_revision bigint NOT NULL CHECK (project_revision >= 1),
    event_sequence bigint NOT NULL CHECK (event_sequence >= 1),
    event_type text NOT NULL CHECK (event_type IN ('project_created', 'project_details_replaced', 'project_initiation_pending', 'project_activated', 'project_cancelled', 'assignment_started', 'assignment_ended', 'resource_linked', 'resource_unlinked')),
    command_id uuid NOT NULL,
    actor_subject uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    payload jsonb NOT NULL,
    UNIQUE (tenant_id, project_id, project_revision, event_sequence),
    UNIQUE (tenant_id, command_id, event_type),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX project_history_page_idx ON project_history_events (tenant_id, project_id, project_revision DESC, event_sequence DESC, id DESC);

CREATE FUNCTION reject_project_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'immutable project record';
END;
$$;

CREATE TRIGGER project_commands_immutable BEFORE UPDATE OR DELETE ON project_commands FOR EACH ROW EXECUTE FUNCTION reject_project_immutable_change();
CREATE TRIGGER project_history_immutable BEFORE UPDATE OR DELETE ON project_history_events FOR EACH ROW EXECUTE FUNCTION reject_project_immutable_change();
