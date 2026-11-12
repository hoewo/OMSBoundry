-- The login account remembers the last organization membership that
-- successfully received a session. Login still validates the membership as
-- active before selecting it, then falls back to the primary (or another
-- active) membership and self-heals this pointer when a new session is made.
ALTER TABLE login_accounts ADD COLUMN last_used_subject_id uuid NULL;

ALTER TABLE subjects
    ADD CONSTRAINT subjects_id_account_unique UNIQUE (id, account_id);

ALTER TABLE login_accounts
    ADD CONSTRAINT login_accounts_last_used_subject_account_fk
    FOREIGN KEY (last_used_subject_id, id) REFERENCES subjects(id, account_id) ON DELETE RESTRICT
    DEFERRABLE INITIALLY DEFERRED;

UPDATE login_accounts
SET last_used_subject_id = primary_subject_id;
