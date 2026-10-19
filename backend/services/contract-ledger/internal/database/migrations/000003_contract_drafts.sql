CREATE TABLE contracts (
    id uuid PRIMARY KEY,
    command_id uuid NOT NULL UNIQUE,
    number_namespace text NOT NULL CHECK (
        number_namespace = btrim(number_namespace)
        AND char_length(number_namespace) BETWEEN 1 AND 64
    ),
    contract_number text NOT NULL CHECK (
        contract_number = btrim(contract_number)
        AND char_length(contract_number) BETWEEN 1 AND 100
    ),
    title text NOT NULL CHECK (
        title = btrim(title)
        AND char_length(title) BETWEEN 1 AND 200
    ),
    category text NULL CHECK (
        category IS NULL
        OR (
            category = btrim(category)
            AND char_length(category) BETWEEN 1 AND 100
        )
    ),
    currency character(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    total_amount numeric(24,6) NULL CHECK (total_amount >= 0),
    signed_on date NULL,
    effective_on date NULL,
    status text NOT NULL CHECK (status IN ('draft', 'active', 'closed', 'cancelled')),
    draft_version bigint NOT NULL DEFAULT 1 CHECK (draft_version >= 1),
    current_revision bigint NULL CHECK (current_revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at),
    CHECK (
        (status IN ('draft', 'cancelled') AND current_revision IS NULL)
        OR (status IN ('active', 'closed') AND current_revision IS NOT NULL)
    )
);

CREATE UNIQUE INDEX contracts_number_scope_unique
ON contracts (lower(number_namespace), lower(contract_number));

CREATE INDEX contracts_created_at_id_idx
ON contracts (created_at DESC, id DESC);
