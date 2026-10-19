CREATE TABLE internal_transfers (
    id uuid PRIMARY KEY,
    description text NOT NULL CHECK (
        description = btrim(description)
        AND char_length(description) BETWEEN 1 AND 200
    ),
    ledger_owner_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    ledger_owner_party_revision bigint NOT NULL CHECK (ledger_owner_party_revision >= 1),
    ledger_owner_kind text NOT NULL CHECK (ledger_owner_kind IN ('organization', 'individual')),
    ledger_owner_legal_name text NOT NULL CHECK (
        ledger_owner_legal_name = btrim(ledger_owner_legal_name)
        AND char_length(ledger_owner_legal_name) BETWEEN 1 AND 200
    ),
    ledger_owner_registration_identifier text NULL CHECK (
        ledger_owner_registration_identifier = btrim(ledger_owner_registration_identifier)
        AND char_length(ledger_owner_registration_identifier) BETWEEN 1 AND 100
    ),
    amount numeric(24,6) NOT NULL CHECK (amount > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    occurred_on date NOT NULL,
    source_account_reference text NOT NULL CHECK (
        source_account_reference = btrim(source_account_reference)
        AND char_length(source_account_reference) BETWEEN 1 AND 100
    ),
    destination_account_reference text NOT NULL CHECK (
        destination_account_reference = btrim(destination_account_reference)
        AND char_length(destination_account_reference) BETWEEN 1 AND 100
    ),
    source_kind text NOT NULL CHECK (source_kind = 'manual'),
    manual_source_reference text NOT NULL CHECK (
        manual_source_reference = btrim(manual_source_reference)
        AND char_length(manual_source_reference) BETWEEN 1 AND 200
    ),
    external_transfer_reference text NULL CHECK (
        external_transfer_reference = btrim(external_transfer_reference)
        AND char_length(external_transfer_reference) BETWEEN 1 AND 100
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
    CHECK (lower(source_account_reference) <> lower(destination_account_reference))
);

CREATE INDEX internal_transfers_recorded_id_idx
ON internal_transfers (recorded_at DESC, id DESC);

CREATE INDEX internal_transfers_occurred_id_idx
ON internal_transfers (occurred_on DESC, id DESC);

CREATE UNIQUE INDEX internal_transfers_external_reference_unique
ON internal_transfers (
    ledger_owner_party_id,
    lower(source_account_reference),
    lower(destination_account_reference),
    source_kind,
    lower(external_transfer_reference)
)
WHERE external_transfer_reference IS NOT NULL;

CREATE TABLE internal_transfer_create_commands (
    command_id uuid PRIMARY KEY,
    internal_transfer_id uuid NOT NULL UNIQUE REFERENCES internal_transfers(id) ON DELETE RESTRICT,
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

CREATE TRIGGER internal_transfers_immutable
BEFORE UPDATE OR DELETE ON internal_transfers
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER internal_transfer_create_commands_immutable
BEFORE UPDATE OR DELETE ON internal_transfer_create_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
