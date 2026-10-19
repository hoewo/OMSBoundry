ALTER TABLE contracts
    DROP CONSTRAINT contracts_status_check,
    DROP CONSTRAINT contracts_check1;

ALTER TABLE contracts
    ADD CONSTRAINT contracts_status_check
        CHECK (status IN ('draft', 'active', 'terminated', 'closed', 'cancelled')),
    ADD CONSTRAINT contracts_lifecycle_revision_check
        CHECK (
            (status IN ('draft', 'cancelled') AND current_revision IS NULL)
            OR
            (status IN ('active', 'terminated', 'closed') AND current_revision IS NOT NULL)
        );

ALTER TABLE contract_instruments
    DROP CONSTRAINT contract_instruments_instrument_type_check,
    DROP CONSTRAINT contract_instruments_check1;

ALTER TABLE contract_instruments
    ADD CONSTRAINT contract_instruments_instrument_type_check
        CHECK (
            instrument_type IN (
                'original_contract',
                'supplemental_agreement',
                'termination_instrument'
            )
        ),
    ADD CONSTRAINT contract_instruments_type_payload_check
        CHECK (
            (
                instrument_type = 'original_contract'
                AND effective_order = 1
                AND resulting_revision = 1
                AND amount_effect = 'original'
                AND (amount_value IS NULL OR amount_value >= 0)
            )
            OR
            (
                instrument_type = 'supplemental_agreement'
                AND effective_order >= 2
                AND resulting_revision >= 2
                AND (
                    (amount_effect = 'adjustment' AND amount_value IS NOT NULL AND amount_value <> 0)
                    OR (amount_effect = 'restatement' AND amount_value IS NOT NULL AND amount_value >= 0)
                    OR (amount_effect = 'unchanged' AND amount_value IS NULL)
                )
            )
            OR
            (
                instrument_type = 'termination_instrument'
                AND effective_order >= 2
                AND resulting_revision >= 2
                AND amount_effect = 'unchanged'
                AND amount_value IS NULL
            )
        );

CREATE TABLE contract_termination_commands (
    command_id uuid PRIMARY KEY,
    instrument_id uuid NOT NULL UNIQUE REFERENCES contract_instruments(id) ON DELETE RESTRICT,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    expected_revision bigint NOT NULL CHECK (expected_revision >= 1),
    instrument_number text NOT NULL CHECK (
        instrument_number = btrim(instrument_number)
        AND char_length(instrument_number) BETWEEN 1 AND 100
    ),
    title text NOT NULL CHECK (
        title = btrim(title)
        AND char_length(title) BETWEEN 1 AND 200
    ),
    termination_reason text NOT NULL CHECK (
        termination_reason = btrim(termination_reason)
        AND char_length(termination_reason) BETWEEN 1 AND 1000
    ),
    signed_on date NOT NULL,
    effective_on date NOT NULL,
    effective_order bigint NOT NULL CHECK (effective_order >= 2),
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
    resulting_revision bigint NOT NULL CHECK (
        resulting_revision = expected_revision + 1
    ),
    occurred_at timestamptz NOT NULL,
    CHECK (effective_order = resulting_revision)
);

CREATE INDEX contract_termination_commands_contract_idx
ON contract_termination_commands (contract_id, resulting_revision, command_id);

CREATE TRIGGER contract_termination_commands_immutable
BEFORE UPDATE OR DELETE ON contract_termination_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();
