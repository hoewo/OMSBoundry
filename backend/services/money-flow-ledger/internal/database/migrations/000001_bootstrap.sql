CREATE TABLE app_metadata (
    key text PRIMARY KEY,
    value text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

INSERT INTO app_metadata (key, value)
VALUES ('service', 'money-flow-ledger')
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = clock_timestamp();

