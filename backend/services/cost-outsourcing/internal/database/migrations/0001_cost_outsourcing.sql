CREATE TABLE cost_entries (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    state text NOT NULL CHECK (state IN ('recorded','withdrawn')),
    currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    revision bigint NOT NULL CHECK (revision > 0),
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,id)
);

CREATE INDEX cost_entries_project_idx ON cost_entries(tenant_id,project_id,id);

CREATE TABLE cost_entry_revisions (
    tenant_id uuid NOT NULL,
    cost_entry_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    event_kind text NOT NULL CHECK (event_kind IN ('recorded','corrected','withdrawn')),
    command_id uuid NOT NULL,
    snapshot jsonb NOT NULL,
    reason text NOT NULL,
    actor_ref uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,cost_entry_id,revision)
);

CREATE TABLE outsourcing_engagements (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    provider_party_id uuid NOT NULL,
    state text NOT NULL CHECK (state IN ('recorded','withdrawn')),
    currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    revision bigint NOT NULL CHECK (revision > 0),
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,id)
);

CREATE INDEX outsourcing_engagements_project_idx ON outsourcing_engagements(tenant_id,project_id,id);

CREATE TABLE outsourcing_engagement_revisions (
    tenant_id uuid NOT NULL,
    engagement_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    event_kind text NOT NULL CHECK (event_kind IN ('recorded','corrected','withdrawn')),
    command_id uuid NOT NULL,
    snapshot jsonb NOT NULL,
    reason text NOT NULL,
    actor_ref uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,engagement_id,revision)
);

CREATE TABLE outsourcing_assessment_revisions (
    tenant_id uuid NOT NULL,
    engagement_id uuid NOT NULL,
    assessment_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    engagement_revision bigint NOT NULL CHECK (engagement_revision > 0),
    command_id uuid NOT NULL,
    rule_key text NOT NULL CHECK (rule_key = 'reference-oa-five-point-v1'),
    score integer NOT NULL CHECK (score BETWEEN 1 AND 5),
    reason text NOT NULL,
    actor_ref uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    snapshot jsonb NOT NULL,
    PRIMARY KEY (tenant_id,engagement_id,revision),
    UNIQUE (tenant_id,assessment_id,revision)
);

CREATE TABLE cost_outsourcing_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    actor_ref uuid NOT NULL,
    request_hash char(64) NOT NULL,
    response_type text NOT NULL CHECK (response_type IN ('cost_entry','outsourcing_engagement','outsourcing_assessment')),
    response jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,command_id)
);

CREATE TABLE cost_outsourcing_audit_events (
    tenant_id uuid NOT NULL,
    event_id uuid NOT NULL,
    aggregate_type text NOT NULL CHECK (aggregate_type IN ('cost_entry','outsourcing_engagement','outsourcing_assessment')),
    aggregate_id uuid NOT NULL,
    aggregate_revision bigint NOT NULL CHECK (aggregate_revision > 0),
    command_id uuid NOT NULL,
    actor_ref uuid NOT NULL,
    event_kind text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,event_id)
);

CREATE FUNCTION reject_cost_outsourcing_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME;
END;
$$;

CREATE FUNCTION guard_cost_current_projection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '% cannot be physically deleted', TG_TABLE_NAME;
    END IF;
    IF NEW.revision <> OLD.revision + 1 THEN
        RAISE EXCEPTION 'current projection requires next revision';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM cost_entry_revisions r
        WHERE r.tenant_id = NEW.tenant_id AND r.cost_entry_id = NEW.id
          AND r.revision = NEW.revision AND r.snapshot = NEW.snapshot
    ) THEN
        RAISE EXCEPTION 'matching cost revision is required';
    END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION guard_outsourcing_current_projection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '% cannot be physically deleted', TG_TABLE_NAME;
    END IF;
    IF NEW.revision <> OLD.revision + 1 THEN
        RAISE EXCEPTION 'current projection requires next revision';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM outsourcing_engagement_revisions r
        WHERE r.tenant_id = NEW.tenant_id AND r.engagement_id = NEW.id
          AND r.revision = NEW.revision AND r.snapshot = NEW.snapshot
    ) THEN
        RAISE EXCEPTION 'matching engagement revision is required';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER cost_entry_revisions_immutable BEFORE UPDATE OR DELETE ON cost_entry_revisions FOR EACH ROW EXECUTE FUNCTION reject_cost_outsourcing_mutation();
CREATE TRIGGER outsourcing_engagement_revisions_immutable BEFORE UPDATE OR DELETE ON outsourcing_engagement_revisions FOR EACH ROW EXECUTE FUNCTION reject_cost_outsourcing_mutation();
CREATE TRIGGER outsourcing_assessment_revisions_immutable BEFORE UPDATE OR DELETE ON outsourcing_assessment_revisions FOR EACH ROW EXECUTE FUNCTION reject_cost_outsourcing_mutation();
CREATE TRIGGER cost_outsourcing_commands_immutable BEFORE UPDATE OR DELETE ON cost_outsourcing_commands FOR EACH ROW EXECUTE FUNCTION reject_cost_outsourcing_mutation();
CREATE TRIGGER cost_outsourcing_audit_events_immutable BEFORE UPDATE OR DELETE ON cost_outsourcing_audit_events FOR EACH ROW EXECUTE FUNCTION reject_cost_outsourcing_mutation();
CREATE TRIGGER cost_entries_guard BEFORE UPDATE OR DELETE ON cost_entries FOR EACH ROW EXECUTE FUNCTION guard_cost_current_projection();
CREATE TRIGGER outsourcing_engagements_guard BEFORE UPDATE OR DELETE ON outsourcing_engagements FOR EACH ROW EXECUTE FUNCTION guard_outsourcing_current_projection();
