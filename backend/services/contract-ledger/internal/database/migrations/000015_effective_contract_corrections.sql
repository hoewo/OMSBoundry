ALTER TABLE contract_instruments
    DROP CONSTRAINT contract_instruments_instrument_type_check,
    DROP CONSTRAINT contract_instruments_type_payload_check,
    DROP CONSTRAINT contract_instruments_amount_effect_check;

ALTER TABLE contract_instruments
    ADD CONSTRAINT contract_instruments_instrument_type_check
        CHECK (
            instrument_type IN (
                'original_contract',
                'supplemental_agreement',
                'termination_instrument',
                'correction_instrument'
            )
        ),
    ADD CONSTRAINT contract_instruments_amount_effect_check
        CHECK (
            amount_effect IN ('original', 'adjustment', 'restatement', 'unchanged', 'corrected')
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
            OR
            (
                instrument_type = 'correction_instrument'
                AND effective_order >= 2
                AND resulting_revision >= 2
                AND amount_effect = 'corrected'
                AND (amount_value IS NULL OR amount_value >= 0)
            )
        );

CREATE TABLE contract_correction_commands (
    command_id uuid PRIMARY KEY,
    instrument_id uuid NOT NULL UNIQUE REFERENCES contract_instruments(id) ON DELETE RESTRICT,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    expected_status text NOT NULL CHECK (expected_status IN ('active', 'terminated', 'closed')),
    expected_revision bigint NOT NULL CHECK (expected_revision >= 1),
    instrument_number text NOT NULL CHECK (
        instrument_number = btrim(instrument_number)
        AND char_length(instrument_number) BETWEEN 1 AND 100
    ),
    instrument_title text NOT NULL CHECK (
        instrument_title = btrim(instrument_title)
        AND char_length(instrument_title) BETWEEN 1 AND 200
    ),
    correction_reason text NOT NULL CHECK (
        correction_reason = btrim(correction_reason)
        AND char_length(correction_reason) BETWEEN 1 AND 1000
    ),
    instrument_signed_on date NOT NULL,
    instrument_effective_on date NOT NULL,
    effective_order bigint NOT NULL CHECK (effective_order >= 2),
    corrected_title text NOT NULL CHECK (
        corrected_title = btrim(corrected_title)
        AND char_length(corrected_title) BETWEEN 1 AND 200
    ),
    corrected_category text NULL CHECK (
        corrected_category IS NULL
        OR (
            corrected_category = btrim(corrected_category)
            AND char_length(corrected_category) BETWEEN 1 AND 100
        )
    ),
    corrected_total_amount numeric(24,6) NULL CHECK (
        corrected_total_amount IS NULL OR corrected_total_amount >= 0
    ),
    corrected_signed_on date NOT NULL,
    corrected_effective_on date NOT NULL,
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
    CONSTRAINT contract_correction_commands_revision_order_check
        CHECK (effective_order = resulting_revision),
    CONSTRAINT contract_correction_commands_expected_revision_fk
        FOREIGN KEY (contract_id, expected_revision)
        REFERENCES contract_revisions(contract_id, revision) ON DELETE RESTRICT,
    CONSTRAINT contract_correction_commands_resulting_revision_fk
        FOREIGN KEY (contract_id, resulting_revision)
        REFERENCES contract_revisions(contract_id, revision) ON DELETE RESTRICT
);

CREATE INDEX contract_correction_commands_contract_idx
ON contract_correction_commands (contract_id, resulting_revision, command_id);

CREATE TRIGGER contract_correction_commands_immutable
BEFORE UPDATE OR DELETE ON contract_correction_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();
