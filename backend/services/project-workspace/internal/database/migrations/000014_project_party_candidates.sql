CREATE TABLE project_party_candidates (
    tenant_id uuid NOT NULL,
    party_id uuid NOT NULL,
    role_key text NOT NULL CHECK (role_key IN ('client', 'contractor')),
    source_revision bigint NOT NULL CHECK (source_revision >= 1),
    registration_command_id uuid NOT NULL,
    registered_by uuid NOT NULL,
    registered_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, party_id, role_key),
    UNIQUE (tenant_id, registration_command_id)
);

CREATE INDEX project_party_candidates_tenant_role_idx
    ON project_party_candidates (tenant_id, role_key, registered_at DESC, party_id);

CREATE TRIGGER project_party_candidates_immutable
    BEFORE UPDATE OR DELETE ON project_party_candidates
    FOR EACH ROW EXECUTE FUNCTION reject_project_immutable_change();
