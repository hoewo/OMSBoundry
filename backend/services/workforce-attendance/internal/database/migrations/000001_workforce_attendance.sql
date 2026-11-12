CREATE TABLE attendance_policies (
    tenant_id uuid NOT NULL,
    policy_key text NOT NULL CHECK (policy_key=btrim(policy_key) AND char_length(policy_key) BETWEEN 1 AND 80),
    period char(7) NOT NULL CHECK (period ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    basis text NOT NULL CHECK (basis IN ('calendar_days','scheduled_workdays')),
    scheduled_dates jsonb NOT NULL CHECK (jsonb_typeof(scheduled_dates)='array'),
    reason text NOT NULL CHECK (reason=btrim(reason) AND char_length(reason) BETWEEN 1 AND 500),
    revision bigint NOT NULL CHECK (revision >= 1),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,policy_key)
);
CREATE TABLE attendance_policy_revisions (
    tenant_id uuid NOT NULL, policy_key text NOT NULL, revision bigint NOT NULL,
    command_id uuid NOT NULL, snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot)='object'),
    actor_subject_id uuid NOT NULL, credential_id text NOT NULL, occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,policy_key,revision), UNIQUE (tenant_id,command_id),
    FOREIGN KEY (tenant_id,policy_key) REFERENCES attendance_policies(tenant_id,policy_key) ON DELETE RESTRICT
);

CREATE TABLE attendance_records (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, person_id uuid NOT NULL, work_date date NOT NULL,
    status text NOT NULL CHECK (status IN ('present','absent')), units bigint NOT NULL CHECK (units BETWEEN 1 AND 10000),
    note text NOT NULL DEFAULT '' CHECK (note=btrim(note) AND char_length(note)<=500), revision bigint NOT NULL CHECK (revision>=1),
    updated_at timestamptz NOT NULL, UNIQUE (tenant_id,id), UNIQUE (tenant_id,person_id,work_date)
);
CREATE INDEX attendance_records_period_idx ON attendance_records(tenant_id,work_date,person_id);
CREATE TABLE attendance_record_revisions (
    tenant_id uuid NOT NULL, record_id uuid NOT NULL, revision bigint NOT NULL, command_id uuid NOT NULL,
    snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot)='object'), actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL, occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,record_id,revision), UNIQUE (tenant_id,command_id),
    FOREIGN KEY (tenant_id,record_id) REFERENCES attendance_records(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE leave_records (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, person_id uuid NOT NULL, from_date date NOT NULL, through_date date NOT NULL,
    leave_type text NOT NULL CHECK (leave_type=btrim(leave_type) AND char_length(leave_type) BETWEEN 1 AND 40),
    units_per_day bigint NOT NULL CHECK (units_per_day BETWEEN 1 AND 10000), paid_ratio_bps bigint NOT NULL CHECK (paid_ratio_bps BETWEEN 0 AND 10000),
    reason text NOT NULL CHECK (reason=btrim(reason) AND char_length(reason) BETWEEN 1 AND 500), revision bigint NOT NULL CHECK (revision>=1),
    updated_at timestamptz NOT NULL, CHECK (through_date>=from_date), UNIQUE (tenant_id,id)
);
CREATE INDEX leave_records_period_idx ON leave_records(tenant_id,from_date,through_date,person_id);
CREATE TABLE leave_record_revisions (
    tenant_id uuid NOT NULL, leave_id uuid NOT NULL, revision bigint NOT NULL, command_id uuid NOT NULL,
    snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot)='object'), actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL, occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,leave_id,revision), UNIQUE (tenant_id,command_id),
    FOREIGN KEY (tenant_id,leave_id) REFERENCES leave_records(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE attendance_monthly_summaries (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, person_id uuid NOT NULL, period char(7) NOT NULL,
    summary jsonb NOT NULL CHECK (jsonb_typeof(summary)='object'), revision bigint NOT NULL CHECK (revision>=1),
    created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id), UNIQUE (tenant_id,person_id,period)
);
CREATE INDEX attendance_monthly_summaries_period_idx ON attendance_monthly_summaries(tenant_id,period,person_id);
CREATE TABLE attendance_monthly_summary_revisions (
    tenant_id uuid NOT NULL, summary_id uuid NOT NULL, revision bigint NOT NULL, command_id uuid NOT NULL,
    snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot)='object'), actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL, occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,summary_id,revision), UNIQUE (tenant_id,command_id),
    FOREIGN KEY (tenant_id,summary_id) REFERENCES attendance_monthly_summaries(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE attendance_commands (
    tenant_id uuid NOT NULL, command_id uuid NOT NULL, request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    response_kind text NOT NULL, response_json jsonb NOT NULL, created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,command_id)
);
CREATE TABLE attendance_audit_events (
    id uuid PRIMARY KEY, tenant_id uuid NOT NULL, resource_type text NOT NULL,
    resource_id text NOT NULL, action text NOT NULL, source_revision bigint NOT NULL,
    result_revision bigint NOT NULL, actor_subject_id uuid NOT NULL, credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL
);
CREATE INDEX attendance_audit_timeline_idx ON attendance_audit_events(tenant_id,occurred_at DESC,id);

CREATE FUNCTION reject_attendance_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'immutable attendance record'; END; $$;
CREATE TRIGGER attendance_policy_revisions_immutable BEFORE UPDATE OR DELETE ON attendance_policy_revisions FOR EACH ROW EXECUTE FUNCTION reject_attendance_immutable_change();
CREATE TRIGGER attendance_record_revisions_immutable BEFORE UPDATE OR DELETE ON attendance_record_revisions FOR EACH ROW EXECUTE FUNCTION reject_attendance_immutable_change();
CREATE TRIGGER leave_record_revisions_immutable BEFORE UPDATE OR DELETE ON leave_record_revisions FOR EACH ROW EXECUTE FUNCTION reject_attendance_immutable_change();
CREATE TRIGGER attendance_summary_revisions_immutable BEFORE UPDATE OR DELETE ON attendance_monthly_summary_revisions FOR EACH ROW EXECUTE FUNCTION reject_attendance_immutable_change();
CREATE TRIGGER attendance_commands_immutable BEFORE UPDATE OR DELETE ON attendance_commands FOR EACH ROW EXECUTE FUNCTION reject_attendance_immutable_change();
CREATE TRIGGER attendance_audit_immutable BEFORE UPDATE OR DELETE ON attendance_audit_events FOR EACH ROW EXECUTE FUNCTION reject_attendance_immutable_change();
