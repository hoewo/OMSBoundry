CREATE TABLE api_keys (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    subject_id uuid NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    person_id uuid NOT NULL,
    person_revision bigint NOT NULL CHECK (person_revision >= 1),
    label text NOT NULL CHECK (label = btrim(label) AND char_length(label) BETWEEN 1 AND 80),
    prefix text NOT NULL UNIQUE CHECK (prefix ~ '^bfk_[A-Za-z0-9_-]{10}$'),
    secret_verifier text NOT NULL CHECK (secret_verifier LIKE '$2%'),
    status text NOT NULL CHECK (status IN ('active', 'revoked')),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL CHECK (expires_at > created_at),
    last_used_at timestamptz NULL,
    revoked_at timestamptz NULL,
    revoked_by uuid NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    CHECK ((status = 'active' AND revoked_at IS NULL AND revoked_by IS NULL)
        OR (status = 'revoked' AND revoked_at IS NOT NULL AND revoked_by IS NOT NULL))
);
CREATE INDEX api_keys_subject_timeline_idx ON api_keys (tenant_id, subject_id, created_at DESC, id DESC);
CREATE INDEX api_keys_active_expiry_idx ON api_keys (tenant_id, subject_id, expires_at) WHERE status = 'active';

CREATE TABLE api_key_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    api_key_id uuid NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    event_kind text NOT NULL CHECK (event_kind IN ('created', 'revoked')),
    actor_subject_id uuid NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    occurred_at timestamptz NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details) = 'object')
);
CREATE INDEX api_key_events_timeline_idx ON api_key_events (tenant_id, api_key_id, occurred_at DESC, id DESC);

CREATE TABLE open_api_usage_records (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    subject_id uuid NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    person_id uuid NOT NULL,
    api_key_id uuid NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    key_prefix text NOT NULL CHECK (key_prefix ~ '^bfk_[A-Za-z0-9_-]{10}$'),
    action text NOT NULL CHECK (action ~ '^open-api\.[a-z0-9.-]+$'),
    request_id text NOT NULL CHECK (request_id = btrim(request_id) AND char_length(request_id) BETWEEN 8 AND 128),
    source_ip_masked text NOT NULL CHECK (source_ip_masked = btrim(source_ip_masked) AND char_length(source_ip_masked) BETWEEN 1 AND 80),
    result_code text NOT NULL CHECK (result_code = btrim(result_code) AND char_length(result_code) BETWEEN 1 AND 80),
    http_status integer NOT NULL CHECK (http_status BETWEEN 0 AND 599),
    occurred_at timestamptz NOT NULL,
    completed_at timestamptz NULL,
    CONSTRAINT open_api_usage_request_unique UNIQUE (tenant_id, request_id)
);
CREATE INDEX open_api_usage_self_timeline_idx ON open_api_usage_records (tenant_id, subject_id, occurred_at DESC, id DESC);
CREATE INDEX open_api_usage_tenant_timeline_idx ON open_api_usage_records (tenant_id, occurred_at DESC, id DESC);

CREATE TABLE open_api_usage_cleanup_runs (
    id uuid PRIMARY KEY,
    cutoff timestamptz NOT NULL,
    deleted_count bigint NOT NULL CHECK (deleted_count >= 0),
    operator_id text NOT NULL CHECK (operator_id = btrim(operator_id) AND char_length(operator_id) BETWEEN 1 AND 128),
    occurred_at timestamptz NOT NULL,
    CHECK (cutoff <= occurred_at - interval '180 days')
);
CREATE INDEX open_api_usage_cleanup_timeline_idx ON open_api_usage_cleanup_runs (occurred_at DESC, id DESC);

CREATE FUNCTION reject_api_key_event_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'API key events are immutable';
END;
$$;

CREATE TRIGGER api_key_events_immutable
BEFORE UPDATE OR DELETE ON api_key_events
FOR EACH ROW EXECUTE FUNCTION reject_api_key_event_change();

CREATE FUNCTION reject_open_api_usage_identity_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id <> OLD.id
       OR NEW.tenant_id <> OLD.tenant_id
       OR NEW.subject_id <> OLD.subject_id
       OR NEW.person_id <> OLD.person_id
       OR NEW.api_key_id <> OLD.api_key_id
       OR NEW.key_prefix <> OLD.key_prefix
       OR NEW.action <> OLD.action
       OR NEW.request_id <> OLD.request_id
       OR NEW.source_ip_masked <> OLD.source_ip_masked
       OR NEW.occurred_at <> OLD.occurred_at THEN
        RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'open API usage identity is immutable';
    END IF;
    IF OLD.completed_at IS NOT NULL THEN
        RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'completed open API usage is immutable';
    END IF;
    IF NEW.completed_at IS NULL OR NEW.http_status = 0 THEN
        RAISE EXCEPTION USING ERRCODE = '23514', MESSAGE = 'open API usage completion is incomplete';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER open_api_usage_guard
BEFORE UPDATE ON open_api_usage_records
FOR EACH ROW EXECUTE FUNCTION reject_open_api_usage_identity_change();

CREATE TRIGGER open_api_usage_cleanup_runs_immutable
BEFORE UPDATE OR DELETE ON open_api_usage_cleanup_runs
FOR EACH ROW EXECUTE FUNCTION reject_iam_audit_change();
