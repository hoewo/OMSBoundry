CREATE TABLE parties (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('organization', 'individual')),
    legal_name text NOT NULL CHECK (char_length(btrim(legal_name)) BETWEEN 1 AND 200),
    registration_identifier text NULL CHECK (
        registration_identifier IS NULL
        OR char_length(btrim(registration_identifier)) BETWEEN 1 AND 100
    ),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at)
);

CREATE INDEX parties_legal_name_id_idx ON parties (legal_name, id);

