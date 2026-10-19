CREATE TABLE local_users (
    id uuid PRIMARY KEY,
    username text NOT NULL CHECK (
        username = lower(btrim(username))
        AND username ~ '^[a-z0-9][a-z0-9._-]{2,63}$'
    ),
    display_name text NOT NULL CHECK (
        display_name = btrim(display_name)
        AND char_length(display_name) BETWEEN 1 AND 100
    ),
    password_hash text NOT NULL CHECK (
        char_length(password_hash) BETWEEN 40 AND 100
    ),
    role text NOT NULL CHECK (role IN ('administrator', 'editor', 'viewer')),
    status text NOT NULL CHECK (status IN ('active', 'disabled')),
    session_generation bigint NOT NULL DEFAULT 1 CHECK (session_generation >= 1),
    failed_login_attempts integer NOT NULL DEFAULT 0 CHECK (failed_login_attempts >= 0),
    locked_until timestamptz NULL,
    password_changed_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at)
);

CREATE UNIQUE INDEX local_users_username_unique
ON local_users (lower(username));

CREATE INDEX local_users_role_status_idx
ON local_users (role, status, id);

CREATE TABLE local_sessions (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id uuid NOT NULL REFERENCES local_users(id) ON DELETE RESTRICT,
    session_generation bigint NOT NULL CHECK (session_generation >= 1),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz NULL,
    CHECK (expires_at > created_at),
    CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX local_sessions_user_idx
ON local_sessions (user_id, expires_at DESC);

CREATE INDEX local_sessions_expiry_idx
ON local_sessions (expires_at);

CREATE TABLE audit_entries (
    id uuid PRIMARY KEY,
    object_type text NOT NULL CHECK (
        object_type = btrim(object_type)
        AND object_type ~ '^[a-z][a-z0-9_]{0,63}$'
    ),
    object_id uuid NOT NULL,
    action text NOT NULL CHECK (
        action = btrim(action)
        AND char_length(action) BETWEEN 1 AND 100
    ),
    command_id uuid NULL,
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    actor_display_name text NOT NULL CHECK (
        actor_display_name = btrim(actor_display_name)
        AND char_length(actor_display_name) BETWEEN 1 AND 100
    ),
    occurred_at timestamptz NOT NULL,
    summary text NOT NULL CHECK (
        summary = btrim(summary)
        AND char_length(summary) BETWEEN 1 AND 500
    ),
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (
        jsonb_typeof(details) = 'object'
    )
);

CREATE UNIQUE INDEX audit_entries_action_command_unique
ON audit_entries (action, command_id)
WHERE command_id IS NOT NULL;

CREATE INDEX audit_entries_object_timeline_idx
ON audit_entries (object_type, object_id, occurred_at, id);

CREATE INDEX audit_entries_timeline_idx
ON audit_entries (occurred_at DESC, id DESC);

CREATE FUNCTION reject_immutable_money_flow_fact_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'immutable Money Flow fact cannot be changed: %', TG_TABLE_NAME
        USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER audit_entries_immutable
BEFORE UPDATE OR DELETE ON audit_entries
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
