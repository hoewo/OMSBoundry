CREATE TABLE cash_transactions (
    id uuid PRIMARY KEY,
    type text NOT NULL CHECK (type IN ('receipt', 'payment')),
    description text NOT NULL CHECK (
        description = btrim(description)
        AND char_length(description) BETWEEN 1 AND 200
    ),
    ledger_owner_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    payer_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    payer_party_revision bigint NOT NULL CHECK (payer_party_revision >= 1),
    payer_kind text NOT NULL CHECK (payer_kind IN ('organization', 'individual')),
    payer_legal_name text NOT NULL CHECK (
        payer_legal_name = btrim(payer_legal_name)
        AND char_length(payer_legal_name) BETWEEN 1 AND 200
    ),
    payer_registration_identifier text NULL CHECK (
        payer_registration_identifier = btrim(payer_registration_identifier)
        AND char_length(payer_registration_identifier) BETWEEN 1 AND 100
    ),
    payee_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    payee_party_revision bigint NOT NULL CHECK (payee_party_revision >= 1),
    payee_kind text NOT NULL CHECK (payee_kind IN ('organization', 'individual')),
    payee_legal_name text NOT NULL CHECK (
        payee_legal_name = btrim(payee_legal_name)
        AND char_length(payee_legal_name) BETWEEN 1 AND 200
    ),
    payee_registration_identifier text NULL CHECK (
        payee_registration_identifier = btrim(payee_registration_identifier)
        AND char_length(payee_registration_identifier) BETWEEN 1 AND 100
    ),
    amount numeric(24,6) NOT NULL CHECK (amount > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    occurred_on date NOT NULL,
    account_reference text NOT NULL CHECK (
        account_reference = btrim(account_reference)
        AND char_length(account_reference) BETWEEN 1 AND 100
    ),
    source_kind text NOT NULL CHECK (source_kind = 'manual'),
    manual_source_reference text NOT NULL CHECK (
        manual_source_reference = btrim(manual_source_reference)
        AND char_length(manual_source_reference) BETWEEN 1 AND 200
    ),
    external_transaction_reference text NULL CHECK (
        external_transaction_reference = btrim(external_transaction_reference)
        AND char_length(external_transaction_reference) BETWEEN 1 AND 100
    ),
    recorded_by_actor_kind text NOT NULL CHECK (
        recorded_by_actor_kind = btrim(recorded_by_actor_kind)
        AND recorded_by_actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    recorded_by_actor_id text NOT NULL CHECK (
        recorded_by_actor_id = btrim(recorded_by_actor_id)
        AND char_length(recorded_by_actor_id) BETWEEN 1 AND 100
    ),
    recorded_by_actor_display_name text NOT NULL CHECK (
        recorded_by_actor_display_name = btrim(recorded_by_actor_display_name)
        AND char_length(recorded_by_actor_display_name) BETWEEN 1 AND 100
    ),
    recorded_at timestamptz NOT NULL,
    CHECK (payer_party_id <> payee_party_id),
    CHECK (
        (type = 'receipt' AND ledger_owner_party_id = payee_party_id)
        OR (type = 'payment' AND ledger_owner_party_id = payer_party_id)
    )
);

CREATE INDEX cash_transactions_recorded_id_idx
ON cash_transactions (recorded_at DESC, id DESC);

CREATE INDEX cash_transactions_type_occurred_id_idx
ON cash_transactions (type, occurred_on DESC, id DESC);

CREATE UNIQUE INDEX cash_transactions_external_reference_unique
ON cash_transactions (
    ledger_owner_party_id,
    lower(account_reference),
    source_kind,
    lower(external_transaction_reference)
)
WHERE external_transaction_reference IS NOT NULL;

CREATE TABLE cash_transaction_create_commands (
    command_id uuid PRIMARY KEY,
    cash_transaction_id uuid NOT NULL UNIQUE REFERENCES cash_transactions(id) ON DELETE RESTRICT,
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

CREATE TRIGGER cash_transactions_immutable
BEFORE UPDATE OR DELETE ON cash_transactions
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER cash_transaction_create_commands_immutable
BEFORE UPDATE OR DELETE ON cash_transaction_create_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
