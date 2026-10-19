CREATE TABLE parties (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('organization', 'individual')),
    legal_name text NOT NULL CHECK (
        legal_name = btrim(legal_name)
        AND char_length(legal_name) BETWEEN 1 AND 200
    ),
    registration_identifier text NULL CHECK (
        registration_identifier = btrim(registration_identifier)
        AND char_length(registration_identifier) BETWEEN 1 AND 100
    ),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at)
);

CREATE INDEX parties_legal_name_id_idx
ON parties (lower(legal_name) COLLATE "C", legal_name COLLATE "C", id);

CREATE TABLE party_create_commands (
    command_id uuid PRIMARY KEY,
    party_id uuid NOT NULL UNIQUE REFERENCES parties(id) ON DELETE RESTRICT,
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

CREATE INDEX party_create_commands_party_idx
ON party_create_commands (party_id);

CREATE TRIGGER party_create_commands_immutable
BEFORE UPDATE OR DELETE ON party_create_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
