CREATE TABLE persons (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    reference_code text NULL,
    display_name text NOT NULL CHECK (display_name = btrim(display_name) AND char_length(display_name) BETWEEN 1 AND 100),
    status text NOT NULL CHECK (status IN ('active', 'inactive')),
    iam_subject_id uuid NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id, id),
    CHECK (reference_code IS NULL OR (reference_code = btrim(reference_code) AND char_length(reference_code) BETWEEN 1 AND 64))
);
CREATE UNIQUE INDEX persons_reference_code_unique ON persons (tenant_id, lower(reference_code)) WHERE reference_code IS NOT NULL;
CREATE UNIQUE INDEX persons_iam_subject_unique ON persons (tenant_id, iam_subject_id) WHERE iam_subject_id IS NOT NULL;
CREATE INDEX persons_tenant_name_idx ON persons (tenant_id, lower(display_name), id);

CREATE TABLE person_revisions (
    tenant_id uuid NOT NULL,
    person_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    event_kind text NOT NULL CHECK (event_kind IN ('created', 'profile_changed', 'status_changed', 'iam_subject_changed')),
    reference_code text NULL,
    display_name text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'inactive')),
    iam_subject_id uuid NULL,
    occurred_at timestamptz NOT NULL,
    actor_kind text NOT NULL CHECK (actor_kind IN ('iam_subject', 'template_subject')),
    actor_id uuid NOT NULL,
    actor_display_name text NOT NULL,
    PRIMARY KEY (tenant_id, person_id, revision),
    FOREIGN KEY (tenant_id, person_id) REFERENCES persons(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE definitions (
    tenant_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('group_type', 'membership_role', 'capability')),
    key text NOT NULL CHECK (key ~ '^[a-z][a-z0-9_-]{1,62}[a-z0-9]$'),
    display_name text NOT NULL CHECK (display_name = btrim(display_name) AND char_length(display_name) BETWEEN 1 AND 100),
    description text NOT NULL DEFAULT '' CHECK (description = btrim(description) AND char_length(description) <= 500),
    status text NOT NULL CHECK (status IN ('active', 'inactive')),
    revision bigint NOT NULL CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, kind, key)
);

CREATE TABLE definition_revisions (
    tenant_id uuid NOT NULL,
    kind text NOT NULL,
    key text NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    event_kind text NOT NULL CHECK (event_kind IN ('created', 'changed')),
    display_name text NOT NULL,
    description text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'inactive')),
    occurred_at timestamptz NOT NULL,
    actor_kind text NOT NULL CHECK (actor_kind IN ('iam_subject', 'template_subject')),
    actor_id uuid NOT NULL,
    actor_display_name text NOT NULL,
    PRIMARY KEY (tenant_id, kind, key, revision),
    FOREIGN KEY (tenant_id, kind, key) REFERENCES definitions(tenant_id, kind, key) ON DELETE RESTRICT
);

CREATE TABLE groups (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    group_type_key text NOT NULL,
    name text NOT NULL CHECK (name = btrim(name) AND char_length(name) BETWEEN 1 AND 120),
    purpose text NOT NULL DEFAULT '' CHECK (purpose = btrim(purpose) AND char_length(purpose) <= 1000),
    status text NOT NULL CHECK (status IN ('active', 'archived')),
    revision bigint NOT NULL CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, name)
);
CREATE INDEX groups_tenant_status_idx ON groups (tenant_id, status, lower(name), id);

CREATE TABLE group_revisions (
    tenant_id uuid NOT NULL,
    group_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    event_kind text NOT NULL CHECK (event_kind IN ('created', 'profile_changed', 'member_person_changed', 'member_joined', 'member_role_changed', 'member_ended', 'capability_assigned', 'capability_ended', 'archived')),
    group_type_key text NOT NULL,
    name text NOT NULL,
    purpose text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'archived')),
    occurred_at timestamptz NOT NULL,
    actor_kind text NOT NULL CHECK (actor_kind IN ('iam_subject', 'template_subject')),
    actor_id uuid NOT NULL,
    actor_display_name text NOT NULL,
    PRIMARY KEY (tenant_id, group_id, revision),
    FOREIGN KEY (tenant_id, group_id) REFERENCES groups(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE memberships (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    group_id uuid NOT NULL,
    person_id uuid NOT NULL,
    role_key text NOT NULL,
    start_date date NOT NULL,
    end_date date NULL CHECK (end_date IS NULL OR end_date >= start_date),
    status text NOT NULL CHECK (status IN ('active', 'ended')),
    revision bigint NOT NULL CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, group_id) REFERENCES groups(tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, person_id) REFERENCES persons(tenant_id, id) ON DELETE RESTRICT,
    CHECK ((status = 'active' AND end_date IS NULL) OR (status = 'ended' AND end_date IS NOT NULL))
);
CREATE UNIQUE INDEX memberships_one_active_person_group ON memberships (tenant_id, group_id, person_id) WHERE status = 'active';
CREATE INDEX memberships_group_history_idx ON memberships (tenant_id, group_id, status, start_date, id);

CREATE TABLE membership_revisions (
    tenant_id uuid NOT NULL,
    membership_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    event_kind text NOT NULL CHECK (event_kind IN ('joined', 'role_changed', 'ended')),
    group_id uuid NOT NULL,
    person_id uuid NOT NULL,
    role_key text NOT NULL,
    start_date date NOT NULL,
    end_date date NULL,
    status text NOT NULL CHECK (status IN ('active', 'ended')),
    group_revision bigint NOT NULL CHECK (group_revision >= 1),
    occurred_at timestamptz NOT NULL,
    actor_kind text NOT NULL CHECK (actor_kind IN ('iam_subject', 'template_subject')),
    actor_id uuid NOT NULL,
    actor_display_name text NOT NULL,
    PRIMARY KEY (tenant_id, membership_id, revision),
    FOREIGN KEY (tenant_id, membership_id) REFERENCES memberships(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE group_capability_assignments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    group_id uuid NOT NULL,
    capability_key text NOT NULL,
    note text NOT NULL DEFAULT '' CHECK (note = btrim(note) AND char_length(note) <= 500),
    start_date date NOT NULL,
    end_date date NULL CHECK (end_date IS NULL OR end_date >= start_date),
    status text NOT NULL CHECK (status IN ('active', 'ended')),
    revision bigint NOT NULL CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, group_id) REFERENCES groups(tenant_id, id) ON DELETE RESTRICT,
    CHECK ((status = 'active' AND end_date IS NULL) OR (status = 'ended' AND end_date IS NOT NULL))
);
CREATE UNIQUE INDEX group_capability_one_active ON group_capability_assignments (tenant_id, group_id, capability_key) WHERE status = 'active';

CREATE TABLE group_capability_revisions (
    tenant_id uuid NOT NULL,
    assignment_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    event_kind text NOT NULL CHECK (event_kind IN ('assigned', 'ended')),
    group_id uuid NOT NULL,
    capability_key text NOT NULL,
    note text NOT NULL,
    start_date date NOT NULL,
    end_date date NULL,
    status text NOT NULL CHECK (status IN ('active', 'ended')),
    group_revision bigint NOT NULL CHECK (group_revision >= 1),
    occurred_at timestamptz NOT NULL,
    actor_kind text NOT NULL CHECK (actor_kind IN ('iam_subject', 'template_subject')),
    actor_id uuid NOT NULL,
    actor_display_name text NOT NULL,
    PRIMARY KEY (tenant_id, assignment_id, revision),
    FOREIGN KEY (tenant_id, assignment_id) REFERENCES group_capability_assignments(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE roster_snapshots (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    group_id uuid NOT NULL,
    group_revision bigint NOT NULL CHECK (group_revision >= 1),
    payload bytea NOT NULL CHECK (octet_length(payload) BETWEEN 2 AND 1048576),
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL,
    actor_kind text NOT NULL CHECK (actor_kind IN ('iam_subject', 'template_subject')),
    actor_id uuid NOT NULL,
    actor_display_name text NOT NULL,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, group_id, group_revision),
    FOREIGN KEY (tenant_id, group_id) REFERENCES groups(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE business_commands (
    tenant_id uuid NOT NULL,
    operation text NOT NULL CHECK (operation ~ '^[a-z][a-z0-9_.-]{2,99}$'),
    command_id uuid NOT NULL,
    fingerprint bytea NOT NULL CHECK (octet_length(fingerprint) = 32),
    result_type text NOT NULL,
    result_id text NOT NULL CHECK (result_id = btrim(result_id) AND char_length(result_id) BETWEEN 1 AND 128),
    result_revision bigint NOT NULL CHECK (result_revision >= 1),
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, operation, command_id)
);

CREATE TABLE people_groups_audit_entries (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    action text NOT NULL CHECK (action = btrim(action) AND char_length(action) BETWEEN 1 AND 100),
    object_type text NOT NULL CHECK (object_type = btrim(object_type) AND char_length(object_type) BETWEEN 1 AND 64),
    object_id text NOT NULL CHECK (object_id = btrim(object_id) AND char_length(object_id) BETWEEN 1 AND 128),
    actor_kind text NOT NULL CHECK (actor_kind IN ('iam_subject', 'template_subject')),
    actor_id uuid NOT NULL,
    actor_display_name text NOT NULL CHECK (actor_display_name = btrim(actor_display_name) AND char_length(actor_display_name) BETWEEN 1 AND 100),
    occurred_at timestamptz NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details) = 'object')
);
CREATE INDEX people_groups_audit_timeline_idx ON people_groups_audit_entries (tenant_id, occurred_at DESC, id DESC);

CREATE FUNCTION reject_people_groups_history_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'People & Groups history is immutable';
END;
$$;

CREATE TRIGGER person_revisions_immutable BEFORE UPDATE OR DELETE ON person_revisions FOR EACH ROW EXECUTE FUNCTION reject_people_groups_history_change();
CREATE TRIGGER definition_revisions_immutable BEFORE UPDATE OR DELETE ON definition_revisions FOR EACH ROW EXECUTE FUNCTION reject_people_groups_history_change();
CREATE TRIGGER group_revisions_immutable BEFORE UPDATE OR DELETE ON group_revisions FOR EACH ROW EXECUTE FUNCTION reject_people_groups_history_change();
CREATE TRIGGER membership_revisions_immutable BEFORE UPDATE OR DELETE ON membership_revisions FOR EACH ROW EXECUTE FUNCTION reject_people_groups_history_change();
CREATE TRIGGER group_capability_revisions_immutable BEFORE UPDATE OR DELETE ON group_capability_revisions FOR EACH ROW EXECUTE FUNCTION reject_people_groups_history_change();
CREATE TRIGGER roster_snapshots_immutable BEFORE UPDATE OR DELETE ON roster_snapshots FOR EACH ROW EXECUTE FUNCTION reject_people_groups_history_change();
CREATE TRIGGER business_commands_immutable BEFORE UPDATE OR DELETE ON business_commands FOR EACH ROW EXECUTE FUNCTION reject_people_groups_history_change();
CREATE TRIGGER people_groups_audit_immutable BEFORE UPDATE OR DELETE ON people_groups_audit_entries FOR EACH ROW EXECUTE FUNCTION reject_people_groups_history_change();
