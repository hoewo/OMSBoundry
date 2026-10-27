-- Platform system administrators are IAM users, but they are not organization
-- administrators and must not inherit any organization business capability.
ALTER TABLE subjects
    ADD COLUMN system_administrator boolean NOT NULL DEFAULT false;

ALTER TABLE subjects
    ADD CONSTRAINT subjects_user_level_check CHECK (
        NOT (system_administrator AND administrator)
    );

-- The configured bootstrap account in the current D1 installation is `admin`.
-- Promote that stable account during the migration and invalidate its old
-- browser sessions and organization-scoped grants.
UPDATE subjects
SET system_administrator = true,
    administrator = false,
    revision = revision + 1,
    session_generation = session_generation + 1,
    updated_at = clock_timestamp()
WHERE username = 'admin';

DELETE FROM capability_grants
WHERE subject_id IN (SELECT id FROM subjects WHERE system_administrator);
