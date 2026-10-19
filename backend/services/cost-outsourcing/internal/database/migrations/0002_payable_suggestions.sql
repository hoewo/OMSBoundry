ALTER TABLE cost_outsourcing_commands DROP CONSTRAINT cost_outsourcing_commands_response_type_check;
ALTER TABLE cost_outsourcing_commands ADD CONSTRAINT cost_outsourcing_commands_response_type_check
CHECK (response_type IN ('cost_entry','outsourcing_engagement','outsourcing_assessment','payable_suggestion'));

ALTER TABLE cost_outsourcing_audit_events DROP CONSTRAINT cost_outsourcing_audit_events_aggregate_type_check;
ALTER TABLE cost_outsourcing_audit_events ADD CONSTRAINT cost_outsourcing_audit_events_aggregate_type_check
CHECK (aggregate_type IN ('cost_entry','outsourcing_engagement','outsourcing_assessment','payable_suggestion'));

CREATE TABLE payable_suggestions (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    project_id uuid NOT NULL,
    source_type text NOT NULL CHECK (source_type IN ('cost_entry','outsourcing_engagement')),
    source_id uuid NOT NULL,
    source_revision bigint NOT NULL CHECK (source_revision >= 1),
    state text NOT NULL CHECK (state IN ('proposed','accepted','rejected','withdrawn','target_unavailable')),
    revision bigint NOT NULL CHECK (revision >= 1),
    target_revision bigint NOT NULL DEFAULT 0 CHECK (target_revision >= 0),
    request_digest text NOT NULL CHECK (request_digest ~ '^sha256:[0-9a-f]{64}$'),
    snapshot jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,id)
);

CREATE INDEX payable_suggestions_project_idx ON payable_suggestions(tenant_id,project_id,id);
CREATE UNIQUE INDEX payable_suggestions_source_active_idx
ON payable_suggestions(tenant_id,source_type,source_id,source_revision)
WHERE state <> 'withdrawn';

CREATE TABLE payable_suggestion_revisions (
    tenant_id uuid NOT NULL,
    suggestion_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    event_kind text NOT NULL CHECK (event_kind IN ('proposed','submitted','target_unavailable','accepted','rejected','withdrawn','synchronized')),
    command_id uuid NOT NULL,
    snapshot jsonb NOT NULL,
    reason text NOT NULL,
    actor_ref uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,suggestion_id,revision)
);

CREATE FUNCTION guard_payable_suggestion_projection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'payable suggestion cannot be physically deleted';
    END IF;
    IF NEW.revision <> OLD.revision + 1 THEN
        RAISE EXCEPTION 'payable suggestion requires next revision';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM payable_suggestion_revisions r
        WHERE r.tenant_id=NEW.tenant_id AND r.suggestion_id=NEW.id
          AND r.revision=NEW.revision AND r.snapshot=NEW.snapshot
    ) THEN
        RAISE EXCEPTION 'matching payable suggestion revision is required';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER payable_suggestion_revisions_immutable
BEFORE UPDATE OR DELETE ON payable_suggestion_revisions
FOR EACH ROW EXECUTE FUNCTION reject_cost_outsourcing_mutation();

CREATE TRIGGER payable_suggestions_guard
BEFORE UPDATE OR DELETE ON payable_suggestions
FOR EACH ROW EXECUTE FUNCTION guard_payable_suggestion_projection();
