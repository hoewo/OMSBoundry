CREATE TABLE file_version_cleanup_attempts (
    tenant_id uuid NOT NULL,
    file_id uuid NOT NULL,
    version integer NOT NULL CHECK (version >= 1),
    status text NOT NULL CHECK (status IN ('pending','failed','succeeded','protected')),
    attempts integer NOT NULL CHECK (attempts >= 1),
    last_error text NOT NULL DEFAULT '' CHECK (char_length(last_error) <= 2000),
    first_attempted_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,file_id,version),
    FOREIGN KEY (tenant_id,file_id,version) REFERENCES file_versions(tenant_id,file_id,version) ON DELETE RESTRICT
);
CREATE INDEX file_version_cleanup_status_idx
    ON file_version_cleanup_attempts(status,updated_at,tenant_id,file_id,version);
