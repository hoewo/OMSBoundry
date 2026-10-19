-- Customer Overlay only. No universal metadata store or cross-app table.
CREATE TABLE reference_oa_contract_drafts (
    contract_id uuid PRIMARY KEY REFERENCES contracts(id),
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    chain_type text NOT NULL CHECK (chain_type IN ('client_to_qualification', 'qualification_to_our_company')),
    revision bigint NOT NULL CHECK (revision > 0),
    snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot) = 'object'),
    updated_at timestamptz NOT NULL
);
CREATE TABLE reference_oa_contract_draft_commands (
    command_id uuid PRIMARY KEY,
    contract_id uuid NOT NULL REFERENCES reference_oa_contract_drafts(contract_id),
    tenant_id uuid NOT NULL,
    actor_id text NOT NULL,
    intent_sha256 text NOT NULL CHECK (length(intent_sha256) = 64),
    revision bigint NOT NULL CHECK (revision > 0),
    snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot) = 'object'),
    created_at timestamptz NOT NULL,
    UNIQUE (contract_id, revision)
);
CREATE INDEX reference_oa_contract_drafts_project_idx ON reference_oa_contract_drafts(tenant_id, project_id);
CREATE FUNCTION protect_reference_oa_contract_draft_history() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'reference OA contract draft history is append-only';
END;
$$;
CREATE TRIGGER reference_oa_contract_draft_commands_immutable BEFORE UPDATE OR DELETE
ON reference_oa_contract_draft_commands FOR EACH ROW EXECUTE FUNCTION protect_reference_oa_contract_draft_history();
