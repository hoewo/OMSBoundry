-- Login does not ask the user to choose a tenant in v1, so a normalized login
-- name must resolve to exactly one subject across the shared IAM runtime.
CREATE UNIQUE INDEX subjects_global_username_unique ON subjects (username);

-- Subject references consumed by People & Groups need a stable revision that
-- changes only when the externally visible account lifecycle changes.
ALTER TABLE subjects
    ADD COLUMN revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1);

ALTER TABLE iam_audit_entries
    DROP CONSTRAINT iam_audit_entries_actor_kind_check,
    ADD CONSTRAINT iam_audit_entries_actor_kind_check CHECK (
        actor_kind IN ('bootstrap_configuration', 'iam_subject', 'activation_token', 'service_identity', 'platform_operator')
    );
