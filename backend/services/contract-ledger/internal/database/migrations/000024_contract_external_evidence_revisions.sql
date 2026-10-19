CREATE TABLE IF NOT EXISTS contract_external_evidence_revisions (
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    tenant_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    command_id uuid NOT NULL UNIQUE,
    intent_sha256 bytea NOT NULL CHECK (octet_length(intent_sha256) = 32),
    evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(evidence_refs) = 'array'),
    reason text NOT NULL CHECK (btrim(reason) <> ''),
    actor_kind text NOT NULL CHECK (btrim(actor_kind) <> ''),
    actor_id text NOT NULL CHECK (btrim(actor_id) <> ''),
    actor_display_name text NOT NULL,
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (contract_id, revision)
);

CREATE INDEX IF NOT EXISTS contract_external_evidence_revisions_latest_idx
    ON contract_external_evidence_revisions(contract_id, revision DESC);

CREATE OR REPLACE FUNCTION reject_contract_external_evidence_revision_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'contract external evidence revisions are immutable';
END;
$$;

DROP TRIGGER IF EXISTS contract_external_evidence_revisions_immutable ON contract_external_evidence_revisions;
CREATE TRIGGER contract_external_evidence_revisions_immutable
BEFORE UPDATE OR DELETE ON contract_external_evidence_revisions
FOR EACH ROW EXECUTE FUNCTION reject_contract_external_evidence_revision_mutation();
