-- A login account owns the credential once.  Each organization keeps an
-- independent IAM Subject so status, administrator profile, grants, sessions
-- and People references remain tenant scoped.
CREATE TABLE login_accounts (
    id uuid PRIMARY KEY,
    login_identifier text NOT NULL UNIQUE,
    password_hash text NULL,
    failed_login_attempts integer NOT NULL DEFAULT 0 CHECK (failed_login_attempts >= 0),
    locked_until timestamptz NULL,
    password_changed_at timestamptz NULL,
    primary_subject_id uuid NOT NULL UNIQUE,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT login_accounts_identifier_normalized CHECK (
        login_identifier = lower(btrim(login_identifier))
        AND (
            login_identifier ~ '^[a-z0-9][a-z0-9._-]{2,63}$'
            OR login_identifier ~ '^1[3-9][0-9]{9}$'
            OR (
                char_length(login_identifier) <= 254
                AND login_identifier ~ '^[a-z0-9.!#$%&''*+/=?^_`{|}~-]+@[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$'
            )
        )
    )
);

INSERT INTO login_accounts(
    id, login_identifier, password_hash, failed_login_attempts, locked_until,
    password_changed_at, primary_subject_id, created_at, updated_at
)
SELECT
    id, username, password_hash, failed_login_attempts, locked_until,
    password_changed_at, id, created_at, updated_at
FROM subjects;

ALTER TABLE subjects ADD COLUMN account_id uuid NULL;
UPDATE subjects SET account_id = id;
ALTER TABLE subjects
    ALTER COLUMN account_id SET NOT NULL,
    ADD CONSTRAINT subjects_account_fk FOREIGN KEY (account_id) REFERENCES login_accounts(id) ON DELETE RESTRICT,
    ADD CONSTRAINT subjects_tenant_account_unique UNIQUE (tenant_id, account_id);

ALTER TABLE login_accounts
    ADD CONSTRAINT login_accounts_primary_subject_fk
    FOREIGN KEY (primary_subject_id) REFERENCES subjects(id) ON DELETE RESTRICT
    DEFERRABLE INITIALLY DEFERRED;

-- A normalized identifier is globally unique at the account layer.  Subjects
-- may repeat it in different tenants, while the original tenant-local unique
-- constraint continues to reject duplicate membership in one organization.
DROP INDEX subjects_global_username_unique;

CREATE INDEX subjects_account_memberships_idx
    ON subjects (account_id, status, created_at, id);
