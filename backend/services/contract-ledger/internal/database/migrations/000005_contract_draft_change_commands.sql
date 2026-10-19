CREATE TABLE contract_draft_change_commands (
    command_id uuid PRIMARY KEY,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    action text NOT NULL CHECK (action IN ('update', 'cancel')),
    expected_draft_version bigint NOT NULL CHECK (expected_draft_version >= 1),
    resulting_draft_version bigint NOT NULL CHECK (
        resulting_draft_version = expected_draft_version + 1
    ),
    number_namespace text NULL CHECK (
        number_namespace IS NULL
        OR (
            number_namespace = btrim(number_namespace)
            AND char_length(number_namespace) BETWEEN 1 AND 64
        )
    ),
    contract_number text NULL CHECK (
        contract_number IS NULL
        OR (
            contract_number = btrim(contract_number)
            AND char_length(contract_number) BETWEEN 1 AND 100
        )
    ),
    title text NULL CHECK (
        title IS NULL
        OR (
            title = btrim(title)
            AND char_length(title) BETWEEN 1 AND 200
        )
    ),
    category text NULL CHECK (
        category IS NULL
        OR (
            category = btrim(category)
            AND char_length(category) BETWEEN 1 AND 100
        )
    ),
    currency character(3) NULL CHECK (
        currency IS NULL OR currency ~ '^[A-Z]{3}$'
    ),
    total_amount numeric(24,6) NULL CHECK (total_amount IS NULL OR total_amount >= 0),
    signed_on date NULL,
    effective_on date NULL,
    cancellation_reason text NULL CHECK (
        cancellation_reason IS NULL
        OR (
            cancellation_reason = btrim(cancellation_reason)
            AND char_length(cancellation_reason) BETWEEN 1 AND 500
        )
    ),
    created_at timestamptz NOT NULL,
    CHECK (
        (
            action = 'update'
            AND number_namespace IS NOT NULL
            AND contract_number IS NOT NULL
            AND title IS NOT NULL
            AND currency IS NOT NULL
            AND cancellation_reason IS NULL
        )
        OR (
            action = 'cancel'
            AND number_namespace IS NULL
            AND contract_number IS NULL
            AND title IS NULL
            AND category IS NULL
            AND currency IS NULL
            AND total_amount IS NULL
            AND signed_on IS NULL
            AND effective_on IS NULL
            AND cancellation_reason IS NOT NULL
        )
    )
);

CREATE UNIQUE INDEX contract_draft_change_single_cancel
ON contract_draft_change_commands (contract_id)
WHERE action = 'cancel';

CREATE INDEX contract_draft_change_commands_contract_idx
ON contract_draft_change_commands (contract_id, created_at, command_id);
