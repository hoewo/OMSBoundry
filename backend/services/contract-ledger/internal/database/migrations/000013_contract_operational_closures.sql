ALTER TABLE contract_settlements
ADD CONSTRAINT contract_settlements_contract_id_id_unique
UNIQUE (contract_id, id);

CREATE TABLE contract_closures (
    id uuid PRIMARY KEY,
    contract_id uuid NOT NULL UNIQUE REFERENCES contracts(id) ON DELETE RESTRICT,
    source_revision bigint NOT NULL CHECK (source_revision >= 1),
    source_settlement_sequence bigint NOT NULL CHECK (source_settlement_sequence >= 0),
    settlement_disposition text NOT NULL CHECK (settlement_disposition IN ('final_recorded', 'not_required')),
    final_settlement_id uuid NULL,
    closed_on date NOT NULL,
    reason text NOT NULL CHECK (
        reason = btrim(reason)
        AND char_length(reason) BETWEEN 1 AND 1000
    ),
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND char_length(actor_kind) BETWEEN 1 AND 32
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    actor_display_name text NOT NULL CHECK (
        actor_display_name = btrim(actor_display_name)
        AND char_length(actor_display_name) BETWEEN 1 AND 100
    ),
    recorded_at timestamptz NOT NULL,
    CONSTRAINT contract_closures_revision_fk
        FOREIGN KEY (contract_id, source_revision)
        REFERENCES contract_revisions(contract_id, revision) ON DELETE RESTRICT,
    CONSTRAINT contract_closures_final_settlement_fk
        FOREIGN KEY (contract_id, final_settlement_id)
        REFERENCES contract_settlements(contract_id, id) ON DELETE RESTRICT,
    CONSTRAINT contract_closures_settlement_disposition_consistency_check CHECK (
        (settlement_disposition = 'final_recorded' AND final_settlement_id IS NOT NULL)
        OR
        (settlement_disposition = 'not_required' AND final_settlement_id IS NULL)
    )
);

CREATE TABLE contract_closure_commands (
    command_id uuid PRIMARY KEY,
    closure_id uuid NOT NULL UNIQUE REFERENCES contract_closures(id) ON DELETE RESTRICT,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    expected_contract_revision bigint NOT NULL CHECK (expected_contract_revision >= 1),
    expected_settlement_sequence bigint NOT NULL CHECK (expected_settlement_sequence >= 0),
    settlement_disposition text NOT NULL CHECK (settlement_disposition IN ('final_recorded', 'not_required')),
    closed_on date NOT NULL,
    reason text NOT NULL CHECK (
        reason = btrim(reason)
        AND char_length(reason) BETWEEN 1 AND 1000
    ),
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND char_length(actor_kind) BETWEEN 1 AND 32
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    actor_display_name text NOT NULL CHECK (
        actor_display_name = btrim(actor_display_name)
        AND char_length(actor_display_name) BETWEEN 1 AND 100
    ),
    occurred_at timestamptz NOT NULL
);

CREATE TRIGGER contract_closures_immutable
BEFORE UPDATE OR DELETE ON contract_closures
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_closure_commands_immutable
BEFORE UPDATE OR DELETE ON contract_closure_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();
