CREATE TABLE contract_activation_commands (
    command_id uuid PRIMARY KEY,
    contract_id uuid NOT NULL UNIQUE REFERENCES contracts(id) ON DELETE RESTRICT,
    expected_draft_version bigint NOT NULL CHECK (expected_draft_version >= 1),
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND char_length(actor_kind) BETWEEN 1 AND 32
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    resulting_revision bigint NOT NULL CHECK (resulting_revision = 1),
    occurred_at timestamptz NOT NULL
);

CREATE TABLE contract_revisions (
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    revision bigint NOT NULL CHECK (revision = 1),
    activation_command_id uuid NOT NULL UNIQUE REFERENCES contract_activation_commands(command_id) ON DELETE RESTRICT,
    source_draft_version bigint NOT NULL CHECK (source_draft_version >= 1),
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
    total_amount numeric(24,6) NULL CHECK (total_amount IS NULL OR total_amount >= 0),
    signed_on date NOT NULL,
    effective_on date NOT NULL,
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND char_length(actor_kind) BETWEEN 1 AND 32
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    activated_at timestamptz NOT NULL,
    PRIMARY KEY (contract_id, revision)
);

CREATE TABLE contract_revision_parties (
    contract_id uuid NOT NULL,
    revision bigint NOT NULL,
    position bigint NOT NULL CHECK (position >= 1),
    source_party_link_id uuid NOT NULL,
    source_party_id uuid NOT NULL,
    source_party_revision bigint NOT NULL CHECK (source_party_revision >= 1),
    party_kind text NOT NULL CHECK (party_kind IN ('organization', 'individual')),
    legal_name text NOT NULL CHECK (
        legal_name = btrim(legal_name)
        AND char_length(legal_name) BETWEEN 1 AND 200
    ),
    registration_identifier text NULL CHECK (
        registration_identifier IS NULL
        OR (
            registration_identifier = btrim(registration_identifier)
            AND char_length(registration_identifier) BETWEEN 1 AND 100
        )
    ),
    role text NOT NULL CHECK (
        role = btrim(role)
        AND char_length(role) BETWEEN 1 AND 100
    ),
    PRIMARY KEY (contract_id, revision, position),
    UNIQUE (contract_id, revision, source_party_link_id),
    FOREIGN KEY (contract_id, revision)
        REFERENCES contract_revisions(contract_id, revision)
        ON DELETE RESTRICT
);

CREATE TABLE contract_audit_entries (
    id uuid PRIMARY KEY,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    event_type text NOT NULL CHECK (
        event_type = btrim(event_type)
        AND char_length(event_type) BETWEEN 1 AND 100
    ),
    command_id uuid NOT NULL UNIQUE REFERENCES contract_activation_commands(command_id) ON DELETE RESTRICT,
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND char_length(actor_kind) BETWEEN 1 AND 32
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    occurred_at timestamptz NOT NULL,
    source_draft_version bigint NOT NULL CHECK (source_draft_version >= 1),
    resulting_revision bigint NOT NULL CHECK (resulting_revision >= 1)
);

CREATE INDEX contract_audit_entries_contract_idx
ON contract_audit_entries (contract_id, occurred_at, id);

CREATE FUNCTION reject_immutable_contract_fact_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION '% is immutable', TG_TABLE_NAME
        USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER contract_activation_commands_immutable
BEFORE UPDATE OR DELETE ON contract_activation_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_revisions_immutable
BEFORE UPDATE OR DELETE ON contract_revisions
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_revision_parties_immutable
BEFORE UPDATE OR DELETE ON contract_revision_parties
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_audit_entries_immutable
BEFORE UPDATE OR DELETE ON contract_audit_entries
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();
