ALTER TABLE persons
    ADD COLUMN department text NOT NULL DEFAULT '',
    ADD COLUMN position_title text NOT NULL DEFAULT '',
    ADD COLUMN status_effective_date date NOT NULL DEFAULT CURRENT_DATE;

ALTER TABLE persons
    ADD CONSTRAINT persons_department_check CHECK (department = btrim(department) AND char_length(department) <= 100),
    ADD CONSTRAINT persons_position_title_check CHECK (position_title = btrim(position_title) AND char_length(position_title) <= 100);

ALTER TABLE person_revisions
    ADD COLUMN department text NOT NULL DEFAULT '',
    ADD COLUMN position_title text NOT NULL DEFAULT '',
    ADD COLUMN status_effective_date date NOT NULL DEFAULT CURRENT_DATE;

ALTER TABLE person_revisions DISABLE TRIGGER person_revisions_immutable;
UPDATE person_revisions SET status_effective_date = occurred_at::date;
ALTER TABLE person_revisions ENABLE TRIGGER person_revisions_immutable;

UPDATE persons
SET status_effective_date = CASE WHEN status = 'inactive' THEN updated_at::date ELSE created_at::date END;

CREATE TABLE person_employment_events (
    tenant_id uuid NOT NULL,
    person_id uuid NOT NULL,
    sequence bigint NOT NULL CHECK (sequence >= 1),
    person_revision bigint NOT NULL CHECK (person_revision >= 1),
    event_kind text NOT NULL CHECK (event_kind IN ('hired', 'offboarded', 'rehired')),
    effective_date date NOT NULL,
    reason text NOT NULL DEFAULT '' CHECK (reason = btrim(reason) AND char_length(reason) <= 500),
    department text NOT NULL DEFAULT '' CHECK (department = btrim(department) AND char_length(department) <= 100),
    position_title text NOT NULL DEFAULT '' CHECK (position_title = btrim(position_title) AND char_length(position_title) <= 100),
    occurred_at timestamptz NOT NULL,
    actor_kind text NOT NULL CHECK (actor_kind IN ('iam_subject', 'template_subject')),
    actor_id uuid NOT NULL,
    actor_display_name text NOT NULL,
    PRIMARY KEY (tenant_id, person_id, sequence),
    UNIQUE (tenant_id, person_id, person_revision),
    FOREIGN KEY (tenant_id, person_id) REFERENCES persons(tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX person_employment_events_timeline_idx ON person_employment_events(tenant_id, person_id, effective_date, sequence);

INSERT INTO person_employment_events(
    tenant_id, person_id, sequence, person_revision, event_kind, effective_date, reason,
    department, position_title, occurred_at, actor_kind, actor_id, actor_display_name
)
SELECT tenant_id, person_id, 1, revision, 'hired', occurred_at::date, '历史成员迁移',
       department, position_title, occurred_at, actor_kind, actor_id, actor_display_name
FROM person_revisions
WHERE revision = 1;

INSERT INTO person_employment_events(
    tenant_id, person_id, sequence, person_revision, event_kind, effective_date, reason,
    department, position_title, occurred_at, actor_kind, actor_id, actor_display_name
)
SELECT p.tenant_id, p.id, 2, p.revision, 'offboarded', p.updated_at::date, '历史停用状态迁移',
       p.department, p.position_title, p.updated_at, r.actor_kind, r.actor_id, r.actor_display_name
FROM persons p
JOIN person_revisions r
  ON r.tenant_id = p.tenant_id AND r.person_id = p.id AND r.revision = p.revision
WHERE p.status = 'inactive' AND p.revision > 1;

CREATE TRIGGER person_employment_events_immutable
    BEFORE UPDATE OR DELETE ON person_employment_events
    FOR EACH ROW EXECUTE FUNCTION reject_people_groups_history_change();
