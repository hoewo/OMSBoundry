CREATE TABLE files (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    created_by_subject uuid NOT NULL,
    PRIMARY KEY (id),
    UNIQUE (tenant_id, id)
);

CREATE TABLE upload_sessions (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    file_id uuid NOT NULL,
    version integer NOT NULL CHECK (version = 1),
    provider text NOT NULL CHECK (provider IN ('local', 'aliyun-oss')),
    bucket text NOT NULL DEFAULT '',
    object_key text NOT NULL CHECK (object_key = btrim(object_key) AND char_length(object_key) BETWEEN 1 AND 1024),
    file_name text NOT NULL CHECK (file_name = btrim(file_name) AND char_length(file_name) BETWEEN 1 AND 512),
    expected_size bigint NOT NULL CHECK (expected_size > 0),
    declared_media_type text NOT NULL DEFAULT '' CHECK (char_length(declared_media_type) <= 255),
    declared_sha256 text NULL CHECK (declared_sha256 IS NULL OR declared_sha256 ~ '^[0-9a-f]{64}$'),
    client_context jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(client_context) = 'object'),
    status text NOT NULL CHECK (status IN ('pending', 'completing', 'completed', 'aborted', 'expired', 'cleanup_pending', 'cleaned')),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    completed_at timestamptz NULL,
    aborted_at timestamptz NULL,
    abort_reason text NOT NULL DEFAULT '' CHECK (char_length(abort_reason) <= 64),
    cleanup_attempts integer NOT NULL DEFAULT 0 CHECK (cleanup_attempts >= 0),
    last_cleanup_error text NOT NULL DEFAULT '' CHECK (char_length(last_cleanup_error) <= 2000),
    created_by_subject uuid NOT NULL,
    created_by_name text NOT NULL CHECK (created_by_name = btrim(created_by_name) AND char_length(created_by_name) BETWEEN 1 AND 100),
    PRIMARY KEY (id),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, file_id, version),
    UNIQUE (provider, bucket, object_key),
    FOREIGN KEY (tenant_id, file_id) REFERENCES files(tenant_id, id) ON DELETE RESTRICT,
    CHECK (expires_at > created_at),
    CHECK ((status = 'completed' AND completed_at IS NOT NULL) OR status <> 'completed')
);
CREATE INDEX upload_sessions_cleanup_idx ON upload_sessions (status, expires_at, updated_at, id);

CREATE TABLE file_versions (
    tenant_id uuid NOT NULL,
    file_id uuid NOT NULL,
    version integer NOT NULL CHECK (version = 1),
    status text NOT NULL CHECK (status = 'available'),
    file_name text NOT NULL CHECK (file_name = btrim(file_name) AND char_length(file_name) BETWEEN 1 AND 512),
    size_bytes bigint NOT NULL CHECK (size_bytes > 0),
    media_type text NOT NULL CHECK (media_type = btrim(media_type) AND char_length(media_type) BETWEEN 1 AND 255),
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    provider text NOT NULL CHECK (provider IN ('local', 'aliyun-oss')),
    bucket text NOT NULL DEFAULT '',
    object_key text NOT NULL CHECK (object_key = btrim(object_key) AND char_length(object_key) BETWEEN 1 AND 1024),
    created_at timestamptz NOT NULL,
    created_by_subject uuid NOT NULL,
    created_by_name text NOT NULL CHECK (created_by_name = btrim(created_by_name) AND char_length(created_by_name) BETWEEN 1 AND 100),
    PRIMARY KEY (tenant_id, file_id, version),
    UNIQUE (provider, bucket, object_key),
    FOREIGN KEY (tenant_id, file_id) REFERENCES files(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE idempotency_commands (
    tenant_id uuid NOT NULL,
    subject_id uuid NOT NULL,
    action text NOT NULL CHECK (action ~ '^[a-z][a-z0-9_.-]{2,99}$'),
    idempotency_key text NOT NULL CHECK (idempotency_key = btrim(idempotency_key) AND char_length(idempotency_key) BETWEEN 1 AND 128),
    payload_hash bytea NOT NULL CHECK (octet_length(payload_hash) = 32),
    resource_id uuid NOT NULL,
    result_json jsonb NULL CHECK (result_json IS NULL OR jsonb_typeof(result_json) = 'object'),
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, subject_id, action, idempotency_key)
);

CREATE TABLE evidence_audit_entries (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    action text NOT NULL CHECK (action = btrim(action) AND char_length(action) BETWEEN 1 AND 100),
    object_type text NOT NULL CHECK (object_type = btrim(object_type) AND char_length(object_type) BETWEEN 1 AND 64),
    object_id text NOT NULL CHECK (object_id = btrim(object_id) AND char_length(object_id) BETWEEN 1 AND 128),
    actor_kind text NOT NULL CHECK (actor_kind IN ('iam_subject', 'template_subject', 'system')),
    actor_id uuid NULL,
    actor_display_name text NOT NULL CHECK (actor_display_name = btrim(actor_display_name) AND char_length(actor_display_name) BETWEEN 1 AND 100),
    outcome text NOT NULL CHECK (outcome IN ('succeeded', 'failed')),
    provider text NOT NULL DEFAULT '' CHECK (char_length(provider) <= 32),
    occurred_at timestamptz NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details) = 'object'),
    CHECK ((actor_kind = 'system' AND actor_id IS NULL) OR (actor_kind <> 'system' AND actor_id IS NOT NULL))
);
CREATE INDEX evidence_audit_timeline_idx ON evidence_audit_entries (tenant_id, occurred_at DESC, id DESC);

CREATE FUNCTION reject_evidence_immutable_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'Evidence immutable fact cannot be changed';
END;
$$;

CREATE TRIGGER file_versions_immutable BEFORE UPDATE OR DELETE ON file_versions FOR EACH ROW EXECUTE FUNCTION reject_evidence_immutable_change();
CREATE TRIGGER idempotency_commands_immutable BEFORE UPDATE OR DELETE ON idempotency_commands FOR EACH ROW EXECUTE FUNCTION reject_evidence_immutable_change();
CREATE TRIGGER evidence_audit_immutable BEFORE UPDATE OR DELETE ON evidence_audit_entries FOR EACH ROW EXECUTE FUNCTION reject_evidence_immutable_change();
