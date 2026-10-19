ALTER TABLE upload_sessions
    DROP CONSTRAINT upload_sessions_version_check,
    ADD CONSTRAINT upload_sessions_version_positive CHECK (version >= 1);

ALTER TABLE file_versions
    DROP CONSTRAINT file_versions_version_check,
    ADD CONSTRAINT file_versions_version_positive CHECK (version >= 1);
