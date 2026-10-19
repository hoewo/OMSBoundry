CREATE TABLE contract_settlements (
    id uuid PRIMARY KEY,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    source_revision bigint NOT NULL CHECK (source_revision >= 1),
    settlement_sequence bigint NOT NULL CHECK (settlement_sequence >= 1),
    settlement_kind text NOT NULL CHECK (settlement_kind IN ('progress', 'final')),
    statement_number text NOT NULL CHECK (
        statement_number = btrim(statement_number)
        AND char_length(statement_number) BETWEEN 1 AND 100
    ),
    title text NOT NULL CHECK (
        title = btrim(title)
        AND char_length(title) BETWEEN 1 AND 200
    ),
    total_amount numeric(24,6) NOT NULL CHECK (total_amount >= 0),
    currency character(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    confirmed_on date NOT NULL,
    basis text NOT NULL CHECK (
        basis = btrim(basis)
        AND char_length(basis) BETWEEN 1 AND 1000
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
    CONSTRAINT contract_settlements_revision_fk
        FOREIGN KEY (contract_id, source_revision)
        REFERENCES contract_revisions(contract_id, revision) ON DELETE RESTRICT,
    CONSTRAINT contract_settlements_sequence_unique
        UNIQUE (contract_id, settlement_sequence)
);

CREATE UNIQUE INDEX contract_settlements_number_unique
ON contract_settlements (contract_id, lower(statement_number));

CREATE UNIQUE INDEX contract_settlements_final_unique
ON contract_settlements (contract_id)
WHERE settlement_kind = 'final';

CREATE INDEX contract_settlements_timeline_idx
ON contract_settlements (contract_id, settlement_sequence DESC);

CREATE TABLE contract_settlement_commands (
    command_id uuid PRIMARY KEY,
    settlement_id uuid NOT NULL UNIQUE REFERENCES contract_settlements(id) ON DELETE RESTRICT,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    expected_contract_revision bigint NOT NULL CHECK (expected_contract_revision >= 1),
    expected_settlement_sequence bigint NOT NULL CHECK (expected_settlement_sequence >= 0),
    settlement_kind text NOT NULL CHECK (settlement_kind IN ('progress', 'final')),
    statement_number text NOT NULL CHECK (
        statement_number = btrim(statement_number)
        AND char_length(statement_number) BETWEEN 1 AND 100
    ),
    title text NOT NULL CHECK (
        title = btrim(title)
        AND char_length(title) BETWEEN 1 AND 200
    ),
    total_amount numeric(24,6) NOT NULL CHECK (total_amount >= 0),
    confirmed_on date NOT NULL,
    basis text NOT NULL CHECK (
        basis = btrim(basis)
        AND char_length(basis) BETWEEN 1 AND 1000
    ),
    confirming_party_positions bigint[] NOT NULL CHECK (
        array_length(confirming_party_positions, 1) >= 1
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
    resulting_settlement_sequence bigint NOT NULL CHECK (
        resulting_settlement_sequence = expected_settlement_sequence + 1
    ),
    occurred_at timestamptz NOT NULL
);

CREATE TABLE contract_settlement_parties (
    settlement_id uuid NOT NULL REFERENCES contract_settlements(id) ON DELETE RESTRICT,
    position bigint NOT NULL CHECK (position >= 1),
    source_party_link_id uuid NOT NULL,
    source_party_id uuid NOT NULL,
    source_party_revision bigint NOT NULL CHECK (source_party_revision >= 1),
    party_kind text NOT NULL,
    legal_name text NOT NULL,
    registration_identifier text NULL,
    role text NOT NULL,
    PRIMARY KEY (settlement_id, position)
);

CREATE TRIGGER contract_settlements_immutable
BEFORE UPDATE OR DELETE ON contract_settlements
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_settlement_commands_immutable
BEFORE UPDATE OR DELETE ON contract_settlement_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_settlement_parties_immutable
BEFORE UPDATE OR DELETE ON contract_settlement_parties
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();
