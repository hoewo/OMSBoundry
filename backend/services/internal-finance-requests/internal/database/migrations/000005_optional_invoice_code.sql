DROP INDEX IF EXISTS finance_invoice_number_idx;

CREATE INDEX finance_invoice_number_idx
ON finance_invoice_fingerprints(tenant_id, invoice_code, invoice_number)
WHERE invoice_number <> '';
