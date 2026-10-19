CREATE INDEX contracts_status_created_id_idx
ON contracts (status, created_at DESC, id DESC);

CREATE INDEX contracts_currency_created_id_idx
ON contracts (currency, created_at DESC, id DESC);
