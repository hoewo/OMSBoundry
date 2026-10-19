CREATE TABLE design_work_items (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    project_source_revision bigint NOT NULL CHECK (project_source_revision >= 1),
    type_key text NOT NULL CHECK (type_key = 'design-v1'),
    type_definition_version bigint NOT NULL CHECK (type_definition_version = 1),
    status text NOT NULL CHECK (status IN ('draft','in-progress','completed','cancelled')),
    revision bigint NOT NULL CHECK (revision >= 1),
    owner_assignment_id uuid NULL,
    owner_source_revision bigint NULL CHECK (owner_source_revision IS NULL OR owner_source_revision >= 1),
    current_stage_key text NULL CHECK (current_stage_key IS NULL OR current_stage_key IN ('scheme','preliminary-design','construction-drawing','construction-support')),
    cancellation_reason text NULL CHECK (cancellation_reason IS NULL OR (cancellation_reason=btrim(cancellation_reason) AND char_length(cancellation_reason) BETWEEN 1 AND 1000)),
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id),
    CHECK ((owner_assignment_id IS NULL) = (owner_source_revision IS NULL)),
    CHECK ((status='cancelled' AND cancellation_reason IS NOT NULL) OR (status<>'cancelled' AND cancellation_reason IS NULL))
);
CREATE UNIQUE INDEX design_work_items_active_project_type_idx ON design_work_items(tenant_id,project_id,type_key) WHERE status <> 'cancelled';
CREATE INDEX design_work_items_tenant_status_idx ON design_work_items(tenant_id,status,created_at DESC,id DESC);

CREATE TABLE design_stages (
    tenant_id uuid NOT NULL,
    work_item_id uuid NOT NULL,
    stage_key text NOT NULL CHECK (stage_key IN ('scheme','preliminary-design','construction-drawing','construction-support')),
    sequence integer NOT NULL CHECK (sequence IN (10,20,30,40)),
    status text NOT NULL CHECK (status IN ('pending','ready','completed','not-applicable')),
    not_applicable_reason text NULL CHECK (not_applicable_reason IS NULL OR (not_applicable_reason=btrim(not_applicable_reason) AND char_length(not_applicable_reason) BETWEEN 1 AND 1000)),
    completed_at timestamptz NULL,
    completed_work_item_revision bigint NULL CHECK (completed_work_item_revision IS NULL OR completed_work_item_revision >= 1),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,work_item_id,stage_key),
    UNIQUE (tenant_id,work_item_id,sequence),
    FOREIGN KEY (tenant_id,work_item_id) REFERENCES design_work_items(tenant_id,id) ON DELETE RESTRICT,
    CHECK ((status='not-applicable' AND not_applicable_reason IS NOT NULL) OR (status<>'not-applicable' AND not_applicable_reason IS NULL)),
    CHECK ((status='completed' AND completed_at IS NOT NULL AND completed_work_item_revision IS NOT NULL) OR (status<>'completed' AND completed_at IS NULL AND completed_work_item_revision IS NULL))
);
CREATE UNIQUE INDEX design_stages_single_ready_idx ON design_stages(tenant_id,work_item_id) WHERE status='ready';

CREATE TABLE design_deliverables (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    work_item_id uuid NOT NULL,
    stage_key text NOT NULL,
    requirement_key text NOT NULL CHECK (requirement_key='primary-output'),
    evidence_file_id uuid NOT NULL,
    evidence_version integer NOT NULL CHECK (evidence_version >= 1),
    evidence_source_revision bigint NOT NULL CHECK (evidence_source_revision >= 1),
    display_snapshot text NOT NULL CHECK (display_snapshot=btrim(display_snapshot) AND char_length(display_snapshot) BETWEEN 1 AND 255),
    created_work_item_revision bigint NOT NULL CHECK (created_work_item_revision >= 1),
    superseded_work_item_revision bigint NULL CHECK (superseded_work_item_revision IS NULL OR superseded_work_item_revision > created_work_item_revision),
    created_at timestamptz NOT NULL,
    superseded_at timestamptz NULL,
    UNIQUE (tenant_id,id),
    FOREIGN KEY (tenant_id,work_item_id,stage_key) REFERENCES design_stages(tenant_id,work_item_id,stage_key) ON DELETE RESTRICT,
    CHECK ((superseded_at IS NULL) = (superseded_work_item_revision IS NULL))
);
CREATE UNIQUE INDEX design_deliverables_active_slot_idx ON design_deliverables(tenant_id,work_item_id,stage_key,requirement_key) WHERE superseded_at IS NULL;
CREATE INDEX design_deliverables_work_item_idx ON design_deliverables(tenant_id,work_item_id,created_at,id);

CREATE TABLE design_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    response_status integer NOT NULL CHECK (response_status BETWEEN 200 AND 599),
    response_json jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,command_id)
);

CREATE TABLE design_history_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    work_item_id uuid NOT NULL,
    work_item_revision bigint NOT NULL CHECK (work_item_revision >= 1),
    event_sequence bigint NOT NULL CHECK (event_sequence >= 1),
    event_type text NOT NULL CHECK (event_type IN ('work_item_created','owner_set','deliverable_registered','stage_not_applicable','stage_applicability_restored','stage_completed','stage_reopened','work_item_cancelled')),
    command_id uuid NOT NULL,
    actor_subject uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    payload jsonb NOT NULL,
    UNIQUE (tenant_id,work_item_id,event_sequence),
    UNIQUE (tenant_id,command_id,event_type),
    FOREIGN KEY (tenant_id,work_item_id) REFERENCES design_work_items(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX design_history_page_idx ON design_history_events(tenant_id,work_item_id,event_sequence,id);

CREATE FUNCTION reject_design_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'immutable design record'; END;
$$;
CREATE TRIGGER design_commands_immutable BEFORE UPDATE OR DELETE ON design_commands FOR EACH ROW EXECUTE FUNCTION reject_design_immutable_change();
CREATE TRIGGER design_history_immutable BEFORE UPDATE OR DELETE ON design_history_events FOR EACH ROW EXECUTE FUNCTION reject_design_immutable_change();
