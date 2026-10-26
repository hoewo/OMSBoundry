-- One v1 account still resolves to one tenant without a tenant picker, but its
-- primary login identifier may now be a phone number, an email address, or a
-- legacy username.  The existing username column remains the stable storage
-- field so deployed subjects and references do not need to be rewritten.
ALTER TABLE subjects
    DROP CONSTRAINT subjects_username_normalized;

ALTER TABLE subjects
    ADD CONSTRAINT subjects_login_identifier_normalized CHECK (
        username = lower(btrim(username))
        AND (
            username ~ '^[a-z0-9][a-z0-9._-]{2,63}$'
            OR username ~ '^1[3-9][0-9]{9}$'
            OR (
                char_length(username) <= 254
                AND username ~ '^[a-z0-9.!#$%&''*+/=?^_`{|}~-]+@[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$'
            )
        )
    );
