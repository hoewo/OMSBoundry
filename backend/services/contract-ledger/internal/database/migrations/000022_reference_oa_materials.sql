-- New explicit external-file references; legacy embedded Evidence is untouched.
CREATE TABLE reference_oa_materials (
  contract_id uuid PRIMARY KEY REFERENCES reference_oa_contract_drafts(contract_id),
  tenant_id uuid NOT NULL,
  revision bigint NOT NULL CHECK (revision > 0),
  snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot) = 'object')
);
CREATE TABLE reference_oa_material_commands (
  command_id uuid PRIMARY KEY,
  contract_id uuid NOT NULL REFERENCES reference_oa_materials(contract_id),
  tenant_id uuid NOT NULL,
  actor_id text NOT NULL,
  intent_sha256 text NOT NULL CHECK (length(intent_sha256) = 64),
  revision bigint NOT NULL CHECK (revision > 0),
  receipt jsonb NOT NULL CHECK (jsonb_typeof(receipt) = 'object'),
  recorded_at timestamptz NOT NULL,
  UNIQUE (contract_id, revision)
);
CREATE TRIGGER reference_oa_material_commands_immutable BEFORE UPDATE OR DELETE
ON reference_oa_material_commands FOR EACH ROW EXECUTE FUNCTION protect_reference_oa_contract_draft_history();
