ALTER TABLE projects
    ADD COLUMN client_party_id uuid NULL,
    ADD COLUMN client_party_source_revision bigint NULL,
    ADD COLUMN client_party_kind text NULL,
    ADD COLUMN client_party_legal_name text NULL,
    ADD CONSTRAINT projects_client_party_complete_chk CHECK (
        (client_party_id IS NULL
            AND client_party_source_revision IS NULL
            AND client_party_kind IS NULL
            AND client_party_legal_name IS NULL)
        OR
        (node_type = 'project'
            AND client_party_id IS NOT NULL
            AND client_party_source_revision >= 1
            AND client_party_kind IN ('organization', 'individual')
            AND client_party_legal_name = btrim(client_party_legal_name)
            AND char_length(client_party_legal_name) BETWEEN 1 AND 200)
    );

CREATE INDEX projects_client_party_idx
    ON projects (tenant_id, client_party_id)
    WHERE client_party_id IS NOT NULL;
