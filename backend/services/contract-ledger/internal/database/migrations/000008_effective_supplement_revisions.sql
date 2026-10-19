DROP TRIGGER contract_revisions_immutable ON contract_revisions;

ALTER TABLE contract_revisions
    RENAME COLUMN activated_at TO recorded_at;

ALTER TABLE contract_revisions
    DROP CONSTRAINT contract_revisions_revision_check,
    ALTER COLUMN activation_command_id DROP NOT NULL,
    ALTER COLUMN source_draft_version DROP NOT NULL;

ALTER TABLE contract_revisions
    ADD CONSTRAINT contract_revisions_revision_positive
        CHECK (revision >= 1),
    ADD CONSTRAINT contract_revisions_source_shape
        CHECK (
            (
                revision = 1
                AND activation_command_id IS NOT NULL
                AND source_draft_version IS NOT NULL
            )
            OR
            (
                revision >= 2
                AND activation_command_id IS NULL
                AND source_draft_version IS NULL
            )
        );

CREATE TABLE contract_instruments (
    id uuid PRIMARY KEY,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    instrument_type text NOT NULL CHECK (
        instrument_type IN ('original_contract', 'supplemental_agreement')
    ),
    instrument_number text NOT NULL CHECK (
        instrument_number = btrim(instrument_number)
        AND char_length(instrument_number) BETWEEN 1 AND 100
    ),
    title text NOT NULL CHECK (
        title = btrim(title)
        AND char_length(title) BETWEEN 1 AND 200
    ),
    change_summary text NOT NULL CHECK (
        change_summary = btrim(change_summary)
        AND char_length(change_summary) BETWEEN 1 AND 1000
    ),
    signed_on date NOT NULL,
    effective_on date NOT NULL,
    effective_order bigint NOT NULL CHECK (effective_order >= 1),
    amount_effect text NOT NULL CHECK (
        amount_effect IN ('original', 'adjustment', 'restatement', 'unchanged')
    ),
    amount_value numeric(24,6) NULL,
    source_command_id uuid NOT NULL UNIQUE,
    resulting_revision bigint NOT NULL CHECK (resulting_revision >= 1),
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
    CHECK (effective_order = resulting_revision),
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
    )
);

CREATE UNIQUE INDEX contract_instruments_number_unique
ON contract_instruments (contract_id, lower(instrument_number));

CREATE UNIQUE INDEX contract_instruments_effective_order_unique
ON contract_instruments (contract_id, effective_order);

CREATE INDEX contract_instruments_timeline_idx
ON contract_instruments (contract_id, effective_order, id);

INSERT INTO contract_instruments (
    id, contract_id, instrument_type, instrument_number, title,
    change_summary, signed_on, effective_on, effective_order,
    amount_effect, amount_value, source_command_id, resulting_revision,
    actor_kind, actor_id, actor_display_name, recorded_at
)
SELECT
    md5('original_contract_instrument:' || r.contract_id::text)::uuid,
    r.contract_id,
    'original_contract',
    r.contract_number,
    r.title,
    '原合同激活',
    r.signed_on,
    r.effective_on,
    1,
    'original',
    r.total_amount,
    r.activation_command_id,
    1,
    r.actor_kind,
    r.actor_id,
    COALESCE(a.actor_display_name, r.actor_id),
    r.recorded_at
FROM contract_revisions r
LEFT JOIN audit_entries a
    ON a.action = 'contract_activated'
    AND a.command_id = r.activation_command_id
WHERE r.revision = 1;

ALTER TABLE contract_revisions
    ADD COLUMN source_instrument_id uuid NULL;

UPDATE contract_revisions r
SET source_instrument_id = i.id
FROM contract_instruments i
WHERE i.contract_id = r.contract_id
    AND i.resulting_revision = r.revision;

ALTER TABLE contract_revisions
    ALTER COLUMN source_instrument_id SET NOT NULL,
    ADD CONSTRAINT contract_revisions_source_instrument_fkey
        FOREIGN KEY (source_instrument_id)
        REFERENCES contract_instruments(id)
        ON DELETE RESTRICT,
    ADD CONSTRAINT contract_revisions_source_instrument_unique
        UNIQUE (source_instrument_id);

CREATE TABLE contract_change_commands (
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
    change_summary text NOT NULL CHECK (
        change_summary = btrim(change_summary)
        AND char_length(change_summary) BETWEEN 1 AND 1000
    ),
    signed_on date NOT NULL,
    effective_on date NOT NULL,
    effective_order bigint NOT NULL CHECK (effective_order >= 2),
    amount_effect text NOT NULL CHECK (
        amount_effect IN ('adjustment', 'restatement', 'unchanged')
    ),
    amount_value numeric(24,6) NULL,
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
    CHECK (effective_order = resulting_revision),
    CHECK (
        (amount_effect = 'adjustment' AND amount_value IS NOT NULL AND amount_value <> 0)
        OR (amount_effect = 'restatement' AND amount_value IS NOT NULL AND amount_value >= 0)
        OR (amount_effect = 'unchanged' AND amount_value IS NULL)
    )
);

CREATE INDEX contract_change_commands_contract_idx
ON contract_change_commands (contract_id, resulting_revision, command_id);

CREATE TRIGGER contract_instruments_immutable
BEFORE UPDATE OR DELETE ON contract_instruments
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_change_commands_immutable
BEFORE UPDATE OR DELETE ON contract_change_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_revisions_immutable
BEFORE UPDATE OR DELETE ON contract_revisions
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();
