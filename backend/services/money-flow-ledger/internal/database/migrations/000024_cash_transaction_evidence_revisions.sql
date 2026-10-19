CREATE TABLE IF NOT EXISTS cash_transaction_evidence_revisions (
    cash_transaction_id uuid NOT NULL REFERENCES cash_transactions(id) ON DELETE RESTRICT,
    revision bigint NOT NULL CHECK (revision > 0),
    command_id uuid NOT NULL UNIQUE,
    intent_sha256 bytea NOT NULL CHECK (octet_length(intent_sha256) = 32),
    evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(evidence_refs) = 'array'),
    reason text NOT NULL CHECK (btrim(reason) <> ''),
    actor_kind text NOT NULL CHECK (btrim(actor_kind) <> ''),
    actor_id text NOT NULL CHECK (btrim(actor_id) <> ''),
    actor_display_name text NOT NULL,
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (cash_transaction_id, revision)
);

CREATE INDEX IF NOT EXISTS cash_transaction_evidence_revisions_latest_idx
    ON cash_transaction_evidence_revisions(cash_transaction_id, revision DESC);

CREATE OR REPLACE FUNCTION reject_cash_transaction_evidence_revision_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'cash transaction evidence revisions are immutable';
END;
$$;

DROP TRIGGER IF EXISTS cash_transaction_evidence_revisions_immutable ON cash_transaction_evidence_revisions;
CREATE TRIGGER cash_transaction_evidence_revisions_immutable
BEFORE UPDATE OR DELETE ON cash_transaction_evidence_revisions
FOR EACH ROW EXECUTE FUNCTION reject_cash_transaction_evidence_revision_mutation();
