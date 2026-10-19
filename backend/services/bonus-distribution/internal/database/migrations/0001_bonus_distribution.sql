CREATE TABLE bonus_pools (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    bonus_type text NOT NULL CHECK (bonus_type IN ('business','production')),
    state text NOT NULL CHECK (state IN ('draft','published','superseded','withdrawn')),
    currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    unallocated_minor bigint NOT NULL CHECK (unallocated_minor >= 0 AND unallocated_minor <= amount_minor),
    revision bigint NOT NULL CHECK (revision > 0),
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,id)
);

CREATE INDEX bonus_pools_project_idx ON bonus_pools(tenant_id,project_id,bonus_type,id);

CREATE TABLE bonus_pool_revisions (
    tenant_id uuid NOT NULL,
    pool_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    event_kind text NOT NULL CHECK (event_kind IN ('drafted','published','superseded','withdrawn')),
    command_id uuid NOT NULL,
    snapshot jsonb NOT NULL,
    reason text NOT NULL,
    actor_ref uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,pool_id,revision)
);

CREATE TABLE bonus_allocations (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    pool_id uuid NOT NULL,
    project_id uuid NOT NULL,
    bonus_type text NOT NULL CHECK (bonus_type IN ('business','production')),
    person_id uuid NOT NULL,
    assignment_id uuid NOT NULL,
    state text NOT NULL CHECK (state IN ('draft','published','superseded','withdrawn')),
    currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    ratio_basis_points integer NOT NULL CHECK (ratio_basis_points > 0 AND ratio_basis_points <= 10000),
    pool_revision bigint NOT NULL CHECK (pool_revision > 0),
    revision bigint NOT NULL CHECK (revision > 0),
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,id),
    UNIQUE (tenant_id,pool_id,person_id)
);

CREATE INDEX bonus_allocations_pool_idx ON bonus_allocations(tenant_id,pool_id,id);

CREATE TABLE bonus_allocation_revisions (
    tenant_id uuid NOT NULL,
    allocation_id uuid NOT NULL,
    pool_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    event_kind text NOT NULL CHECK (event_kind IN ('drafted','published','superseded','withdrawn')),
    command_id uuid NOT NULL,
    snapshot jsonb NOT NULL,
    reason text NOT NULL,
    actor_ref uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,allocation_id,revision)
);

CREATE TABLE bonus_distribution_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    actor_ref uuid NOT NULL,
    request_hash char(64) NOT NULL,
    response_type text NOT NULL CHECK (response_type IN ('bonus_pool','bonus_supersession')),
    response jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,command_id)
);

CREATE TABLE bonus_distribution_audit_events (
    tenant_id uuid NOT NULL,
    event_id uuid NOT NULL,
    aggregate_type text NOT NULL CHECK (aggregate_type = 'bonus_pool'),
    aggregate_id uuid NOT NULL,
    aggregate_revision bigint NOT NULL CHECK (aggregate_revision > 0),
    command_id uuid NOT NULL,
    actor_ref uuid NOT NULL,
    event_kind text NOT NULL CHECK (event_kind IN ('drafted','published','superseded','withdrawn')),
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,event_id)
);

CREATE FUNCTION reject_bonus_append_only_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME;
END;
$$;

CREATE FUNCTION guard_bonus_pool_projection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'bonus pool cannot be physically deleted';
    END IF;
    IF NEW.revision <> OLD.revision + 1 THEN
        RAISE EXCEPTION 'bonus pool requires next revision';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM bonus_pool_revisions r
        WHERE r.tenant_id=NEW.tenant_id AND r.pool_id=NEW.id
          AND r.revision=NEW.revision AND r.snapshot=NEW.snapshot
    ) THEN
        RAISE EXCEPTION 'matching bonus pool revision is required';
    END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION guard_bonus_allocation_projection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'bonus allocation cannot be physically deleted';
    END IF;
    IF NEW.revision <> OLD.revision + 1 THEN
        RAISE EXCEPTION 'bonus allocation requires next revision';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM bonus_allocation_revisions r
        WHERE r.tenant_id=NEW.tenant_id AND r.allocation_id=NEW.id
          AND r.revision=NEW.revision AND r.snapshot=NEW.snapshot
    ) THEN
        RAISE EXCEPTION 'matching bonus allocation revision is required';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER bonus_pool_revisions_immutable BEFORE UPDATE OR DELETE ON bonus_pool_revisions FOR EACH ROW EXECUTE FUNCTION reject_bonus_append_only_mutation();
CREATE TRIGGER bonus_allocation_revisions_immutable BEFORE UPDATE OR DELETE ON bonus_allocation_revisions FOR EACH ROW EXECUTE FUNCTION reject_bonus_append_only_mutation();
CREATE TRIGGER bonus_distribution_commands_immutable BEFORE UPDATE OR DELETE ON bonus_distribution_commands FOR EACH ROW EXECUTE FUNCTION reject_bonus_append_only_mutation();
CREATE TRIGGER bonus_distribution_audit_events_immutable BEFORE UPDATE OR DELETE ON bonus_distribution_audit_events FOR EACH ROW EXECUTE FUNCTION reject_bonus_append_only_mutation();
CREATE TRIGGER bonus_pools_guard BEFORE UPDATE OR DELETE ON bonus_pools FOR EACH ROW EXECUTE FUNCTION guard_bonus_pool_projection();
CREATE TRIGGER bonus_allocations_guard BEFORE UPDATE OR DELETE ON bonus_allocations FOR EACH ROW EXECUTE FUNCTION guard_bonus_allocation_projection();
