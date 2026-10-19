ALTER TABLE iam_audit_entries
    DROP CONSTRAINT iam_audit_entries_actor_kind_check,
    ADD CONSTRAINT iam_audit_entries_actor_kind_check CHECK (actor_kind IN ('bootstrap_configuration', 'iam_subject', 'activation_token', 'service_identity'));

CREATE TABLE workload_assertion_nonces (
    assertion_id uuid PRIMARY KEY,
    workload_id text NOT NULL CHECK (workload_id ~ '^[a-z][a-z0-9-]{2,63}$'),
    expires_at timestamptz NOT NULL,
    used_at timestamptz NOT NULL,
    CHECK (expires_at > used_at - interval '31 seconds')
);

CREATE INDEX workload_assertion_nonces_expiry_idx ON workload_assertion_nonces (expires_at);
