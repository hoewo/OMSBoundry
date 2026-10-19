-- Dispatch can precede formal numbering and type selection. No existing facts
-- are rewritten; activation still requires types and an ordinary project number.
ALTER TABLE projects
    DROP CONSTRAINT projects_project_number_check,
    DROP CONSTRAINT projects_project_type_keys_check,
    ADD CONSTRAINT projects_project_number_check CHECK (
        project_number = btrim(project_number)
        AND char_length(project_number) <= 64
        AND (node_type = 'package' OR status <> 'active' OR char_length(project_number) >= 1)
    ),
    ADD CONSTRAINT projects_project_type_keys_check CHECK (
        cardinality(project_type_keys) BETWEEN 0 AND 16
        AND (status <> 'active' OR cardinality(project_type_keys) >= 1)
    );
