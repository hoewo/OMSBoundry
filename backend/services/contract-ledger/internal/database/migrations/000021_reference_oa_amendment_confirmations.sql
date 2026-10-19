-- A typed mapping owned by Contract, never an external business table.
CREATE TABLE reference_oa_amendment_confirmations (
    command_id uuid PRIMARY KEY,
    contract_id uuid NOT NULL REFERENCES reference_oa_contract_drafts(contract_id),
    tenant_id uuid NOT NULL,
    amendment_id uuid NOT NULL,
    overlay_revision bigint NOT NULL CHECK (overlay_revision > 0),
    contract_revision bigint NOT NULL CHECK (contract_revision > 1),
    instrument_id uuid NOT NULL REFERENCES contract_instruments(id),
    actor_id text NOT NULL,
    intent_sha256 text NOT NULL CHECK (length(intent_sha256) = 64),
    receipt jsonb NOT NULL CHECK (jsonb_typeof(receipt) = 'object'),
    recorded_at timestamptz NOT NULL,
    UNIQUE (contract_id, amendment_id),
    UNIQUE (contract_id, contract_revision)
);
CREATE TRIGGER reference_oa_amendment_confirmations_immutable BEFORE UPDATE OR DELETE
ON reference_oa_amendment_confirmations FOR EACH ROW EXECUTE FUNCTION protect_reference_oa_contract_draft_history();
