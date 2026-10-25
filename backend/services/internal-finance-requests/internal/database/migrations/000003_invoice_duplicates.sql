CREATE TABLE finance_invoice_fingerprints (
    tenant_id uuid NOT NULL,
    request_id uuid NOT NULL,
    request_revision bigint NOT NULL CHECK (request_revision >= 1),
    line_id uuid NOT NULL,
    evidence_sha256 text NOT NULL DEFAULT '' CHECK (evidence_sha256 = '' OR evidence_sha256 ~ '^[0-9a-f]{64}$'),
    invoice_code text NOT NULL DEFAULT '',
    invoice_number text NOT NULL DEFAULT '',
    issuer_name text NOT NULL,
    invoice_date date NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, request_id, request_revision, line_id, evidence_sha256),
    FOREIGN KEY (tenant_id, request_id, request_revision)
      REFERENCES finance_request_revisions(tenant_id, request_id, revision) ON DELETE RESTRICT
);
CREATE INDEX finance_invoice_hash_idx ON finance_invoice_fingerprints(tenant_id, evidence_sha256) WHERE evidence_sha256 <> '';
CREATE INDEX finance_invoice_number_idx ON finance_invoice_fingerprints(tenant_id, invoice_code, invoice_number) WHERE invoice_code <> '' AND invoice_number <> '';
CREATE INDEX finance_invoice_suspect_idx ON finance_invoice_fingerprints(tenant_id, issuer_name, invoice_date, amount_minor);

CREATE TRIGGER finance_invoice_fingerprints_immutable
BEFORE UPDATE OR DELETE ON finance_invoice_fingerprints
FOR EACH ROW EXECUTE FUNCTION reject_finance_request_immutable_change();
