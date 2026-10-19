ALTER TABLE bonus_distribution_commands
    DROP CONSTRAINT bonus_distribution_commands_response_type_check;

ALTER TABLE bonus_distribution_commands
    ADD CONSTRAINT bonus_distribution_commands_response_type_check
    CHECK (response_type IN ('bonus_pool','bonus_supersession','bonus_payable_suggestion'));

CREATE TABLE bonus_payable_suggestions (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    allocation_id uuid NOT NULL,
    allocation_revision bigint NOT NULL CHECK (allocation_revision >= 1),
    allocation_digest text NOT NULL CHECK (allocation_digest ~ '^sha256:[0-9a-f]{64}$'),
    state text NOT NULL CHECK (state IN ('target_unavailable','proposed','accepted','rejected','withdrawn')),
    revision bigint NOT NULL CHECK (revision >= 1),
    target_revision bigint NOT NULL CHECK (target_revision >= 0),
    request_digest text NOT NULL CHECK (request_digest ~ '^sha256:[0-9a-f]{64}$'),
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,id)
);

CREATE UNIQUE INDEX bonus_payable_suggestions_active_source_idx
ON bonus_payable_suggestions(tenant_id,allocation_id,allocation_revision)
WHERE state <> 'withdrawn';

CREATE INDEX bonus_payable_suggestions_project_idx
ON bonus_payable_suggestions(tenant_id,project_id,id);

CREATE TABLE bonus_payable_suggestion_revisions (
    tenant_id uuid NOT NULL,
    suggestion_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    event_kind text NOT NULL CHECK (event_kind IN ('proposed','submitted','synchronized','accepted','rejected','withdrawn')),
    command_id uuid NOT NULL,
    snapshot jsonb NOT NULL,
    reason text NOT NULL,
    actor_ref uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,suggestion_id,revision)
);

CREATE FUNCTION guard_bonus_payable_suggestion_projection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'bonus payable suggestion cannot be physically deleted';
    END IF;
    IF NEW.revision <> OLD.revision + 1 THEN
        RAISE EXCEPTION 'bonus payable suggestion requires next revision';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM bonus_payable_suggestion_revisions r
        WHERE r.tenant_id=NEW.tenant_id AND r.suggestion_id=NEW.id
          AND r.revision=NEW.revision AND r.snapshot=NEW.snapshot
    ) THEN
        RAISE EXCEPTION 'matching bonus payable suggestion revision is required';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER bonus_payable_suggestion_revisions_immutable
BEFORE UPDATE OR DELETE ON bonus_payable_suggestion_revisions
FOR EACH ROW EXECUTE FUNCTION reject_bonus_append_only_mutation();

CREATE TRIGGER bonus_payable_suggestions_guard
BEFORE UPDATE OR DELETE ON bonus_payable_suggestions
FOR EACH ROW EXECUTE FUNCTION guard_bonus_payable_suggestion_projection();
