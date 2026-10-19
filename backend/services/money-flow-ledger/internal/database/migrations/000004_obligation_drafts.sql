CREATE TABLE obligations (
    id uuid PRIMARY KEY,
    direction text NOT NULL CHECK (direction IN ('receivable', 'payable')),
    title text NOT NULL CHECK (
        title = btrim(title)
        AND char_length(title) BETWEEN 1 AND 200
    ),
    ledger_owner_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    creditor_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    creditor_party_revision bigint NOT NULL CHECK (creditor_party_revision >= 1),
    creditor_kind text NOT NULL CHECK (creditor_kind IN ('organization', 'individual')),
    creditor_legal_name text NOT NULL CHECK (
        creditor_legal_name = btrim(creditor_legal_name)
        AND char_length(creditor_legal_name) BETWEEN 1 AND 200
    ),
    creditor_registration_identifier text NULL CHECK (
        creditor_registration_identifier = btrim(creditor_registration_identifier)
        AND char_length(creditor_registration_identifier) BETWEEN 1 AND 100
    ),
    debtor_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    debtor_party_revision bigint NOT NULL CHECK (debtor_party_revision >= 1),
    debtor_kind text NOT NULL CHECK (debtor_kind IN ('organization', 'individual')),
    debtor_legal_name text NOT NULL CHECK (
        debtor_legal_name = btrim(debtor_legal_name)
        AND char_length(debtor_legal_name) BETWEEN 1 AND 200
    ),
    debtor_registration_identifier text NULL CHECK (
        debtor_registration_identifier = btrim(debtor_registration_identifier)
        AND char_length(debtor_registration_identifier) BETWEEN 1 AND 100
    ),
    original_amount numeric(24,6) NOT NULL CHECK (original_amount > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    due_on date NOT NULL,
    source_kind text NOT NULL CHECK (source_kind = 'manual'),
    manual_source_reference text NOT NULL CHECK (
        manual_source_reference = btrim(manual_source_reference)
        AND char_length(manual_source_reference) BETWEEN 1 AND 200
    ),
    status text NOT NULL CHECK (status IN ('draft', 'open', 'closed', 'cancelled')),
    draft_version bigint NOT NULL DEFAULT 1 CHECK (draft_version >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (creditor_party_id <> debtor_party_id),
    CHECK (
        (direction = 'receivable' AND ledger_owner_party_id = creditor_party_id)
        OR (direction = 'payable' AND ledger_owner_party_id = debtor_party_id)
    ),
    CHECK (updated_at >= created_at)
);

CREATE INDEX obligations_created_id_idx
ON obligations (created_at DESC, id DESC);

CREATE INDEX obligations_status_due_id_idx
ON obligations (status, due_on, id);

CREATE TABLE obligation_create_commands (
    command_id uuid PRIMARY KEY,
    obligation_id uuid NOT NULL UNIQUE REFERENCES obligations(id) ON DELETE RESTRICT,
    payload_hash bytea NOT NULL CHECK (octet_length(payload_hash) = 32),
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    actor_display_name text NOT NULL CHECK (
        actor_display_name = btrim(actor_display_name)
        AND char_length(actor_display_name) BETWEEN 1 AND 100
    ),
    created_at timestamptz NOT NULL
);

CREATE TRIGGER obligation_create_commands_immutable
BEFORE UPDATE OR DELETE ON obligation_create_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
