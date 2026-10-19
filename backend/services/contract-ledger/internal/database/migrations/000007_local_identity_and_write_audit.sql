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
        object_type IN ('party', 'contract', 'local_user')
    ),
    object_id uuid NOT NULL,
    action text NOT NULL CHECK (
        action = btrim(action)
        AND char_length(action) BETWEEN 1 AND 100
    ),
    command_id uuid NULL,
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND char_length(actor_kind) BETWEEN 1 AND 32
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

CREATE TRIGGER audit_entries_immutable
BEFORE UPDATE OR DELETE ON audit_entries
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

INSERT INTO audit_entries (
    id, object_type, object_id, action, command_id,
    actor_kind, actor_id, actor_display_name,
    occurred_at, summary, details
)
SELECT
    md5('party_created:' || id::text)::uuid,
    'party', id, 'party_created', NULL,
    'legacy_unknown', 'actor-not-recorded-before-cl-m1-006', '旧数据（未记录操作者）',
    created_at, '创建主体，revision 1',
    jsonb_build_object('party_revision', revision)
FROM parties;

INSERT INTO audit_entries (
    id, object_type, object_id, action, command_id,
    actor_kind, actor_id, actor_display_name,
    occurred_at, summary, details
)
SELECT
    md5('contract_draft_created:' || id::text)::uuid,
    'contract', id, 'contract_draft_created', command_id,
    'legacy_unknown', 'actor-not-recorded-before-cl-m1-006', '旧数据（未记录操作者）',
    created_at, '创建合同草稿，草稿版本 1',
    jsonb_build_object('resulting_draft_version', 1)
FROM contracts;

INSERT INTO audit_entries (
    id, object_type, object_id, action, command_id,
    actor_kind, actor_id, actor_display_name,
    occurred_at, summary, details
)
SELECT
    md5('contract_draft_party_' || action || ':' || command_id::text)::uuid,
    'contract', contract_id,
    CASE action
        WHEN 'add' THEN 'contract_draft_party_added'
        ELSE 'contract_draft_party_removed'
    END,
    command_id,
    'legacy_unknown', 'actor-not-recorded-before-cl-m1-006', '旧数据（未记录操作者）',
    created_at,
    CASE action
        WHEN 'add' THEN '添加签约主体角色：' || role || '，草稿版本 ' || resulting_draft_version::text
        ELSE '移除签约主体角色：' || role || '，草稿版本 ' || resulting_draft_version::text
    END,
    jsonb_build_object(
        'party_id', party_id,
        'party_link_id', party_link_id,
        'role', role,
        'expected_draft_version', expected_draft_version,
        'resulting_draft_version', resulting_draft_version
    )
FROM contract_draft_party_commands;

INSERT INTO audit_entries (
    id, object_type, object_id, action, command_id,
    actor_kind, actor_id, actor_display_name,
    occurred_at, summary, details
)
SELECT
    md5('contract_draft_' || action || ':' || command_id::text)::uuid,
    'contract', contract_id,
    CASE action
        WHEN 'update' THEN 'contract_draft_updated'
        ELSE 'contract_draft_cancelled'
    END,
    command_id,
    'legacy_unknown', 'actor-not-recorded-before-cl-m1-006', '旧数据（未记录操作者）',
    created_at,
    CASE action
        WHEN 'update' THEN '更新合同草稿：版本 ' || expected_draft_version::text || ' → ' || resulting_draft_version::text
        ELSE '取消合同草稿：版本 ' || expected_draft_version::text || ' → ' || resulting_draft_version::text
    END,
    jsonb_build_object(
        'expected_draft_version', expected_draft_version,
        'resulting_draft_version', resulting_draft_version
    )
FROM contract_draft_change_commands;

INSERT INTO audit_entries (
    id, object_type, object_id, action, command_id,
    actor_kind, actor_id, actor_display_name,
    occurred_at, summary, details
)
SELECT
    id,
    'contract', contract_id, event_type, command_id,
    actor_kind, actor_id, actor_id,
    occurred_at,
    '激活合同：草稿版本 ' || source_draft_version::text || ' → revision ' || resulting_revision::text,
    jsonb_build_object(
        'source_draft_version', source_draft_version,
        'resulting_revision', resulting_revision
    )
FROM contract_audit_entries;

