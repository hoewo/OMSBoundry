ALTER TABLE capability_grants
    ADD COLUMN source_kind text NOT NULL DEFAULT 'manual',
    ADD CONSTRAINT capability_grants_source_kind_check CHECK (
        source_kind IN ('manual', 'administrator_profile')
    );

CREATE INDEX capability_grants_active_source_idx
    ON capability_grants (subject_id, source_kind, audience, action)
    WHERE revoked_at IS NULL;
