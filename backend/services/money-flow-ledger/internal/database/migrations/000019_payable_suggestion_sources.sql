ALTER TABLE obligations
    DROP CONSTRAINT obligations_source_kind_check,
    DROP CONSTRAINT obligations_manual_source_reference_check;

ALTER TABLE obligations
    ALTER COLUMN manual_source_reference DROP NOT NULL,
    ADD COLUMN source_application text,
    ADD COLUMN source_resource_type text,
    ADD COLUMN source_resource_id uuid,
    ADD COLUMN source_revision bigint,
    ADD COLUMN source_digest text;

ALTER TABLE obligations
    ADD CONSTRAINT obligations_source_check CHECK (
        (
            source_kind = 'manual'
            AND manual_source_reference IS NOT NULL
            AND manual_source_reference = btrim(manual_source_reference)
            AND char_length(manual_source_reference) BETWEEN 1 AND 200
            AND source_application IS NULL
            AND source_resource_type IS NULL
            AND source_resource_id IS NULL
            AND source_revision IS NULL
            AND source_digest IS NULL
        )
        OR
        (
            source_kind = 'cost_outsourcing_payable_suggestion'
            AND manual_source_reference IS NULL
            AND source_application = 'cost-outsourcing'
            AND source_resource_type IN ('cost_entry', 'outsourcing_engagement')
            AND source_resource_id IS NOT NULL
            AND source_revision >= 1
            AND source_digest ~ '^sha256:[0-9a-f]{64}$'
        )
    );

ALTER TABLE obligation_revisions
    DROP CONSTRAINT obligation_revisions_source_kind_check,
    DROP CONSTRAINT obligation_revisions_manual_source_reference_check;

ALTER TABLE obligation_revisions
    ALTER COLUMN manual_source_reference DROP NOT NULL,
    ADD COLUMN source_application text,
    ADD COLUMN source_resource_type text,
    ADD COLUMN source_resource_id uuid,
    ADD COLUMN source_fact_revision bigint,
    ADD COLUMN source_digest text;

ALTER TABLE obligation_revisions
    ADD CONSTRAINT obligation_revisions_source_check CHECK (
        (
            source_kind = 'manual'
            AND manual_source_reference IS NOT NULL
            AND manual_source_reference = btrim(manual_source_reference)
            AND char_length(manual_source_reference) BETWEEN 1 AND 200
            AND source_application IS NULL
            AND source_resource_type IS NULL
            AND source_resource_id IS NULL
            AND source_fact_revision IS NULL
            AND source_digest IS NULL
        )
        OR
        (
            source_kind = 'cost_outsourcing_payable_suggestion'
            AND manual_source_reference IS NULL
            AND source_application = 'cost-outsourcing'
            AND source_resource_type IN ('cost_entry', 'outsourcing_engagement')
            AND source_resource_id IS NOT NULL
            AND source_fact_revision >= 1
            AND source_digest ~ '^sha256:[0-9a-f]{64}$'
        )
    );

CREATE TABLE payable_suggestion_intakes (
    tenant_id uuid NOT NULL,
    suggestion_id uuid NOT NULL,
    request_digest text NOT NULL CHECK (request_digest ~ '^sha256:[0-9a-f]{64}$'),
    source_application text NOT NULL CHECK (source_application = 'cost-outsourcing'),
    source_resource_type text NOT NULL CHECK (source_resource_type IN ('cost_entry','outsourcing_engagement')),
    source_resource_id uuid NOT NULL,
    source_revision bigint NOT NULL CHECK (source_revision >= 1),
    project_id uuid NOT NULL,
    ledger_owner_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    counterparty_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    ledger_owner_directory_party_id uuid NOT NULL,
    ledger_owner_directory_party_revision bigint NOT NULL CHECK (ledger_owner_directory_party_revision >= 1),
    counterparty_directory_party_id uuid NOT NULL,
    counterparty_directory_party_revision bigint NOT NULL CHECK (counterparty_directory_party_revision >= 1),
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    due_on date NOT NULL,
    purpose text NOT NULL CHECK (purpose = btrim(purpose) AND char_length(purpose) BETWEEN 1 AND 1000),
    status text NOT NULL CHECK (status IN ('pending','accepted','rejected','withdrawn')),
    revision bigint NOT NULL CHECK (revision >= 1),
    obligation_id uuid REFERENCES obligations(id) ON DELETE RESTRICT,
    obligation_revision bigint,
    decision_reason text,
    submitted_by_actor_kind text NOT NULL,
    submitted_by_actor_id text NOT NULL,
    decided_by_actor_kind text,
    decided_by_actor_id text,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    snapshot jsonb NOT NULL,
    PRIMARY KEY (tenant_id,suggestion_id),
    CHECK (ledger_owner_party_id <> counterparty_party_id),
    CHECK (
        (status = 'accepted' AND obligation_id IS NOT NULL AND obligation_revision >= 1 AND decided_by_actor_id IS NOT NULL)
        OR (status IN ('rejected','withdrawn') AND obligation_id IS NULL AND obligation_revision IS NULL AND decided_by_actor_id IS NOT NULL)
        OR (status = 'pending' AND obligation_id IS NULL AND obligation_revision IS NULL AND decided_by_actor_id IS NULL)
    )
);

CREATE TABLE payable_suggestion_intake_revisions (
    tenant_id uuid NOT NULL,
    suggestion_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    command_id uuid NOT NULL,
    event_kind text NOT NULL CHECK (event_kind IN ('submitted','accepted','rejected','withdrawn')),
    request_digest text NOT NULL CHECK (request_digest ~ '^sha256:[0-9a-f]{64}$'),
    command_digest text NOT NULL CHECK (command_digest ~ '^sha256:[0-9a-f]{64}$'),
    snapshot jsonb NOT NULL,
    actor_kind text NOT NULL,
    actor_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,suggestion_id,revision),
    UNIQUE (tenant_id,command_id)
);

CREATE FUNCTION reject_payable_suggestion_intake_history_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'payable suggestion intake history is immutable' USING ERRCODE = '55000';
END;
$$;

CREATE FUNCTION guard_payable_suggestion_intake_projection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'payable suggestion intake cannot be physically deleted' USING ERRCODE = '55000';
    END IF;
    IF NEW.revision <> OLD.revision + 1 THEN
        RAISE EXCEPTION 'payable suggestion intake requires next revision' USING ERRCODE = '55000';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM payable_suggestion_intake_revisions r
        WHERE r.tenant_id=NEW.tenant_id AND r.suggestion_id=NEW.suggestion_id
          AND r.revision=NEW.revision AND r.request_digest=NEW.request_digest
          AND r.snapshot=NEW.snapshot
    ) THEN
        RAISE EXCEPTION 'matching payable suggestion intake revision is required' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER payable_suggestion_intake_revisions_immutable
BEFORE UPDATE OR DELETE ON payable_suggestion_intake_revisions
FOR EACH ROW EXECUTE FUNCTION reject_payable_suggestion_intake_history_change();

CREATE TRIGGER payable_suggestion_intakes_guard
BEFORE UPDATE OR DELETE ON payable_suggestion_intakes
FOR EACH ROW EXECUTE FUNCTION guard_payable_suggestion_intake_projection();
