ALTER TABLE capability_grants
    ADD COLUMN revoked_at timestamptz NULL,
    ADD COLUMN revoked_by uuid NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    ADD CONSTRAINT capability_grants_revocation_pair CHECK ((revoked_at IS NULL) = (revoked_by IS NULL));

ALTER TABLE capability_resource_grants
    ADD COLUMN revoked_at timestamptz NULL,
    ADD COLUMN revoked_by uuid NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    ADD CONSTRAINT capability_resource_grants_revocation_pair CHECK ((revoked_at IS NULL) = (revoked_by IS NULL));

CREATE INDEX capability_grants_active_subject_audience_idx
    ON capability_grants (subject_id, audience, action)
    WHERE revoked_at IS NULL;

CREATE INDEX capability_resource_grants_active_subject_audience_idx
    ON capability_resource_grants (subject_id, audience, action, resource_type, resource_id)
    WHERE revoked_at IS NULL;
