CREATE TABLE capability_resource_grants (
    subject_id uuid NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    audience text NOT NULL CHECK (audience ~ '^[a-z][a-z0-9-]{2,63}$'),
    action text NOT NULL CHECK (action ~ '^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$'),
    resource_type text NOT NULL CHECK (resource_type ~ '^[a-z][a-z0-9_-]{0,63}$'),
    resource_id text NOT NULL CHECK (resource_id = btrim(resource_id) AND char_length(resource_id) BETWEEN 1 AND 128),
    granted_by uuid NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    granted_at timestamptz NOT NULL,
    PRIMARY KEY (subject_id, audience, action, resource_type, resource_id),
    CHECK (action LIKE audience || '.%')
);

CREATE INDEX capability_resource_grants_subject_audience_idx
    ON capability_resource_grants (subject_id, audience, action, resource_type, resource_id);
