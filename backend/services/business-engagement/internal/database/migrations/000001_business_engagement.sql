CREATE TABLE business_engagements (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    project_source_revision bigint NOT NULL CHECK (project_source_revision > 0),
    applicability text NOT NULL CHECK (applicability IN ('undetermined', 'applicable', 'not_applicable')),
    engagement_method text CHECK (engagement_method IS NULL OR engagement_method IN ('comparison', 'competitive_negotiation', 'tender')),
    material_policy_version integer NOT NULL DEFAULT 1 CHECK (material_policy_version > 0),
    status text NOT NULL CHECK (status IN ('draft', 'collecting_evidence', 'ready', 'not_applicable', 'cancelled')),
    revision bigint NOT NULL CHECK (revision > 0),
    not_applicable_reason text,
    cancellation_reason text,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, project_id),
    CHECK ((applicability = 'not_applicable') = (not_applicable_reason IS NOT NULL)),
    CHECK (not_applicable_reason IS NULL OR (not_applicable_reason = btrim(not_applicable_reason) AND char_length(not_applicable_reason) BETWEEN 1 AND 1000)),
    CHECK ((status = 'cancelled') = (cancellation_reason IS NOT NULL)),
    CHECK (cancellation_reason IS NULL OR (cancellation_reason = btrim(cancellation_reason) AND char_length(cancellation_reason) BETWEEN 1 AND 1000))
);

CREATE INDEX business_engagements_tenant_updated_idx
    ON business_engagements (tenant_id, updated_at DESC, id DESC);

CREATE TABLE business_engagement_materials (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    engagement_id uuid NOT NULL REFERENCES business_engagements(id),
    requirement_key text NOT NULL CHECK (requirement_key IN ('tender_document', 'bid_document', 'award_document')),
    evidence_file_id uuid NOT NULL,
    evidence_version integer NOT NULL CHECK (evidence_version > 0),
    evidence_source_revision bigint NOT NULL CHECK (evidence_source_revision > 0),
    display_snapshot text NOT NULL,
    created_engagement_revision bigint NOT NULL CHECK (created_engagement_revision > 0),
    superseded_engagement_revision bigint CHECK (superseded_engagement_revision IS NULL OR superseded_engagement_revision > created_engagement_revision),
    created_at timestamptz NOT NULL,
    superseded_at timestamptz,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, engagement_id) REFERENCES business_engagements(tenant_id, id) ON DELETE RESTRICT,
    CHECK (display_snapshot = btrim(display_snapshot) AND char_length(display_snapshot) BETWEEN 1 AND 255),
    CHECK ((superseded_engagement_revision IS NULL) = (superseded_at IS NULL))
);

CREATE UNIQUE INDEX business_engagement_materials_active_key
    ON business_engagement_materials (tenant_id, engagement_id, requirement_key)
    WHERE superseded_at IS NULL;

CREATE INDEX business_engagement_materials_history_idx
    ON business_engagement_materials (tenant_id, engagement_id, requirement_key, created_engagement_revision);

CREATE TABLE business_engagement_history_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    engagement_id uuid NOT NULL REFERENCES business_engagements(id),
    engagement_revision bigint NOT NULL CHECK (engagement_revision > 0),
    event_sequence bigint NOT NULL CHECK (event_sequence > 0),
    event_type text NOT NULL CHECK (event_type IN ('engagement_created','applicability_changed','method_selected','material_put','material_remove','engagement_cancelled')),
    command_id uuid NOT NULL,
    actor_subject uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    payload jsonb NOT NULL,
    UNIQUE (tenant_id, engagement_id, event_sequence),
    UNIQUE (tenant_id, command_id, event_type),
    FOREIGN KEY (tenant_id, engagement_id) REFERENCES business_engagements(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX business_engagement_history_lookup_idx
    ON business_engagement_history_events (tenant_id, engagement_id, event_sequence);

CREATE TABLE business_engagement_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    response_json jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, command_id)
);

CREATE FUNCTION reject_business_engagement_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'immutable business engagement record'; END;
$$;

CREATE TRIGGER business_engagement_commands_immutable
    BEFORE UPDATE OR DELETE ON business_engagement_commands
    FOR EACH ROW EXECUTE FUNCTION reject_business_engagement_immutable_change();

CREATE TRIGGER business_engagement_history_immutable
    BEFORE UPDATE OR DELETE ON business_engagement_history_events
    FOR EACH ROW EXECUTE FUNCTION reject_business_engagement_immutable_change();
