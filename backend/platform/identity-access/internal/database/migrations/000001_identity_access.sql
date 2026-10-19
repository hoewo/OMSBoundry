CREATE TABLE tenants (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (name = btrim(name) AND char_length(name) BETWEEN 1 AND 100),
    created_at timestamptz NOT NULL
);

CREATE TABLE subjects (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    username text NOT NULL,
    display_name text NOT NULL CHECK (display_name = btrim(display_name) AND char_length(display_name) BETWEEN 1 AND 100),
    password_hash text NULL,
    status text NOT NULL CHECK (status IN ('pending', 'active', 'disabled')),
    administrator boolean NOT NULL DEFAULT false,
    session_generation bigint NOT NULL DEFAULT 1 CHECK (session_generation >= 1),
    failed_login_attempts integer NOT NULL DEFAULT 0 CHECK (failed_login_attempts >= 0),
    locked_until timestamptz NULL,
    password_changed_at timestamptz NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT subjects_username_normalized CHECK (
        username = lower(btrim(username))
        AND username ~ '^[a-z0-9][a-z0-9._-]{2,63}$'
    ),
    CONSTRAINT subjects_username_unique UNIQUE (tenant_id, username),
    CONSTRAINT subjects_status_password_check CHECK (
        (status = 'pending' AND password_hash IS NULL AND password_changed_at IS NULL)
        OR (status IN ('active', 'disabled') AND password_hash IS NOT NULL AND password_changed_at IS NOT NULL)
    )
);

CREATE TABLE suite_sessions (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    subject_id uuid NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    session_generation bigint NOT NULL CHECK (session_generation >= 1),
    auth_time timestamptz NOT NULL,
    expires_at timestamptz NOT NULL CHECK (expires_at > auth_time),
    revoked_at timestamptz NULL
);
CREATE INDEX suite_sessions_subject_idx ON suite_sessions (subject_id, expires_at);

CREATE TABLE activation_tokens (
    id uuid PRIMARY KEY,
    subject_id uuid NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    kind text NOT NULL CHECK (kind IN ('invitation', 'password_reset')),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    used_at timestamptz NULL,
    created_by uuid NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    CHECK (expires_at > created_at)
);
CREATE INDEX activation_tokens_subject_idx ON activation_tokens (subject_id, kind, created_at DESC);

CREATE TABLE capability_grants (
    subject_id uuid NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    audience text NOT NULL CHECK (audience ~ '^[a-z][a-z0-9-]{2,63}$'),
    action text NOT NULL CHECK (action ~ '^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$'),
    scope_kind text NOT NULL CHECK (scope_kind = 'tenant'),
    granted_by uuid NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    granted_at timestamptz NOT NULL,
    PRIMARY KEY (subject_id, audience, action),
    CHECK (action LIKE audience || '.%')
);

CREATE TABLE iam_audit_entries (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    action text NOT NULL CHECK (action = btrim(action) AND char_length(action) BETWEEN 1 AND 100),
    object_type text NOT NULL CHECK (object_type = btrim(object_type) AND char_length(object_type) BETWEEN 1 AND 64),
    object_id text NOT NULL CHECK (object_id = btrim(object_id) AND char_length(object_id) BETWEEN 1 AND 128),
    actor_kind text NOT NULL CHECK (actor_kind IN ('bootstrap_configuration', 'iam_subject', 'activation_token')),
    actor_id text NOT NULL CHECK (actor_id = btrim(actor_id) AND char_length(actor_id) BETWEEN 1 AND 128),
    actor_display_name text NOT NULL CHECK (actor_display_name = btrim(actor_display_name) AND char_length(actor_display_name) BETWEEN 1 AND 100),
    occurred_at timestamptz NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details) = 'object')
);
CREATE INDEX iam_audit_timeline_idx ON iam_audit_entries (occurred_at DESC, id DESC);

CREATE FUNCTION reject_iam_audit_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'IAM audit entries are immutable';
END;
$$;

CREATE TRIGGER iam_audit_entries_immutable
BEFORE UPDATE OR DELETE ON iam_audit_entries
FOR EACH ROW EXECUTE FUNCTION reject_iam_audit_change();
