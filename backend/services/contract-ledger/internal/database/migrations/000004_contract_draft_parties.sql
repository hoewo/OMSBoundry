CREATE TABLE contract_draft_parties (
    id uuid PRIMARY KEY,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    role text NOT NULL CHECK (
        role = btrim(role)
        AND char_length(role) BETWEEN 1 AND 100
    ),
    added_at timestamptz NOT NULL,
    removed_at timestamptz NULL,
    CHECK (removed_at IS NULL OR removed_at >= added_at)
);

CREATE UNIQUE INDEX contract_draft_parties_active_unique
ON contract_draft_parties (contract_id, party_id, lower(role))
WHERE removed_at IS NULL;

CREATE INDEX contract_draft_parties_active_list_idx
ON contract_draft_parties (contract_id, added_at, id)
WHERE removed_at IS NULL;

CREATE TABLE contract_draft_party_commands (
    command_id uuid PRIMARY KEY,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    action text NOT NULL CHECK (action IN ('add', 'remove')),
    party_link_id uuid NOT NULL REFERENCES contract_draft_parties(id) ON DELETE CASCADE,
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    role text NOT NULL CHECK (
        role = btrim(role)
        AND char_length(role) BETWEEN 1 AND 100
    ),
    expected_draft_version bigint NOT NULL CHECK (expected_draft_version >= 1),
    resulting_draft_version bigint NOT NULL CHECK (
        resulting_draft_version = expected_draft_version + 1
    ),
    created_at timestamptz NOT NULL
);

CREATE INDEX contract_draft_party_commands_contract_idx
ON contract_draft_party_commands (contract_id, created_at, command_id);
