-- Bounded additional participation purpose. No old assignment is rewritten.
ALTER TABLE project_assignments DROP CONSTRAINT project_assignments_role_key_check;
ALTER TABLE project_assignments ADD CONSTRAINT project_assignments_role_key_check
    CHECK (role_key IN ('business_owner', 'production_owner', 'participant', 'commercial_contributor'));
ALTER TABLE project_assignments ADD CONSTRAINT project_assignments_commercial_person_check
    CHECK (role_key <> 'commercial_contributor' OR subject_kind = 'person');
