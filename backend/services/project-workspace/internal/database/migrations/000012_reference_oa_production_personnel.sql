-- Closed customer overlay only. Migration does not enroll any Project, change
-- Base roles, import People rows or register an HTTP/IAM capability.
CREATE TABLE reference_oa_production_discipline_versions (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    name text NOT NULL CHECK (name = btrim(name) AND char_length(name) BETWEEN 1 AND 200),
    description text NOT NULL CHECK (description = btrim(description) AND char_length(description) <= 1000),
    state text NOT NULL CHECK (state IN ('active','archived')),
    command_id uuid NOT NULL,
    actor_subject uuid NOT NULL,
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, id, revision),
    UNIQUE (tenant_id, command_id)
);
CREATE TABLE reference_oa_production_discipline_heads (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    revision bigint NOT NULL,
    name_normalized text NOT NULL,
    state text NOT NULL CHECK (state IN ('active','archived')),
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, id, revision) REFERENCES reference_oa_production_discipline_versions(tenant_id, id, revision) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX reference_oa_production_discipline_active_name_idx ON reference_oa_production_discipline_heads (tenant_id,name_normalized) WHERE state='active';
CREATE TRIGGER reference_oa_production_discipline_versions_immutable BEFORE UPDATE OR DELETE ON reference_oa_production_discipline_versions FOR EACH ROW EXECUTE FUNCTION reject_project_immutable_change();

CREATE TABLE reference_oa_production_profiles (
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    profile_key text NOT NULL CHECK (profile_key = 'reference-oa-production-personnel-v1'),
    participant_not_applicable boolean NOT NULL DEFAULT false,
    installed_project_revision bigint NOT NULL CHECK (installed_project_revision > 0),
    installed_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, project_id),
    FOREIGN KEY (tenant_id, project_id) REFERENCES projects(tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE reference_oa_production_assignments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    role_key text NOT NULL CHECK (role_key IN ('designer','participant','reviewer','auditor','approver','estimator')),
    person_id uuid NOT NULL,
    person_revision bigint NOT NULL CHECK (person_revision > 0),
    person_name text NOT NULL CHECK (btrim(person_name) <> '' AND char_length(person_name) <= 300),
    discipline_id uuid NULL,
    discipline_revision bigint NULL,
    discipline_name text NULL,
    created_project_revision bigint NOT NULL CHECK (created_project_revision > 0),
    started_at timestamptz NOT NULL,
    ended_project_revision bigint NULL,
    ended_at timestamptz NULL,
    end_reason text NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, project_id) REFERENCES reference_oa_production_profiles(tenant_id, project_id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, discipline_id, discipline_revision) REFERENCES reference_oa_production_discipline_versions(tenant_id, id, revision) ON DELETE RESTRICT,
    CHECK ((role_key IN ('designer','participant','reviewer','auditor') AND discipline_id IS NOT NULL AND discipline_revision IS NOT NULL AND discipline_name IS NOT NULL AND discipline_revision > 0 AND btrim(discipline_name) <> '' AND char_length(discipline_name) <= 200)
        OR (role_key IN ('approver','estimator') AND discipline_id IS NULL AND discipline_revision IS NULL AND discipline_name IS NULL)),
    CHECK ((ended_project_revision IS NULL AND ended_at IS NULL AND end_reason IS NULL)
        OR (ended_project_revision IS NOT NULL AND ended_at IS NOT NULL AND end_reason IS NOT NULL AND ended_project_revision > created_project_revision AND ended_at >= started_at AND btrim(end_reason) <> '' AND char_length(end_reason) <= 1000))
);
CREATE UNIQUE INDEX reference_oa_production_active_person_role_idx ON reference_oa_production_assignments
    (tenant_id, project_id, role_key, (coalesce(discipline_id::text, '')), person_id) WHERE ended_at IS NULL;
CREATE UNIQUE INDEX reference_oa_production_active_single_idx ON reference_oa_production_assignments
    (tenant_id, project_id, role_key, (coalesce(discipline_id::text, ''))) WHERE ended_at IS NULL AND role_key IN ('reviewer','auditor','approver','estimator');
CREATE INDEX reference_oa_production_project_idx ON reference_oa_production_assignments (tenant_id, project_id, started_at, id);

CREATE TABLE reference_oa_production_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    project_revision bigint NOT NULL CHECK (project_revision > 0),
    event_type text NOT NULL CHECK (event_type IN ('profile_installed','personnel_changed','participant_applicability_changed')),
    command_id uuid NOT NULL,
    actor_subject uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    UNIQUE (tenant_id, command_id),
    UNIQUE (tenant_id, project_id, project_revision),
    FOREIGN KEY (tenant_id, project_id) REFERENCES reference_oa_production_profiles(tenant_id, project_id) ON DELETE RESTRICT
);
CREATE TRIGGER reference_oa_production_events_immutable BEFORE UPDATE OR DELETE ON reference_oa_production_events FOR EACH ROW EXECUTE FUNCTION reject_project_immutable_change();

CREATE FUNCTION guard_reference_oa_production_assignment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'OA production assignment cannot be deleted'; END IF;
    IF OLD.ended_at IS NOT NULL OR NEW.ended_at IS NULL OR
       (to_jsonb(NEW) - ARRAY['ended_at','ended_project_revision','end_reason']) IS DISTINCT FROM
       (to_jsonb(OLD) - ARRAY['ended_at','ended_project_revision','end_reason']) THEN
        RAISE EXCEPTION 'OA production assignment only permits one append-only ending';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER reference_oa_production_assignment_lifecycle BEFORE UPDATE OR DELETE ON reference_oa_production_assignments FOR EACH ROW EXECUTE FUNCTION guard_reference_oa_production_assignment();

CREATE FUNCTION guard_reference_oa_production_profile() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'OA production profile cannot be deleted'; END IF;
    IF (to_jsonb(NEW) - 'participant_not_applicable') IS DISTINCT FROM (to_jsonb(OLD) - 'participant_not_applicable') THEN
        RAISE EXCEPTION 'OA production profile identity is immutable';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER reference_oa_production_profile_identity BEFORE UPDATE OR DELETE ON reference_oa_production_profiles FOR EACH ROW EXECUTE FUNCTION guard_reference_oa_production_profile();
