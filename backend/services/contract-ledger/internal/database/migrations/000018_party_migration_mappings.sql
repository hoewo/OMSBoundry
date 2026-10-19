CREATE TABLE party_migration_mappings (
    tenant_id uuid NOT NULL,
    migration_batch_id uuid NOT NULL,
    command_id uuid NOT NULL,
    source_application text NOT NULL CHECK (source_application = 'contract-ledger'),
    local_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    local_party_revision bigint NOT NULL CHECK (local_party_revision >= 1),
    local_snapshot_sha256 text NOT NULL CHECK (local_snapshot_sha256 ~ '^[0-9a-f]{64}$'),
    decision text NOT NULL CHECK (decision IN ('create_directory_party', 'map_existing_directory_party')),
    target_party_id uuid NOT NULL,
    target_party_revision bigint NOT NULL CHECK (target_party_revision >= 1),
    target_kind text NOT NULL CHECK (target_kind IN ('organization', 'individual')),
    target_legal_name text NOT NULL CHECK (length(btrim(target_legal_name)) BETWEEN 1 AND 200),
    target_registration_identifier text,
    reviewed_by_subject_id uuid NOT NULL,
    reviewed_by_display_name text NOT NULL CHECK (length(btrim(reviewed_by_display_name)) BETWEEN 1 AND 100),
    review_reason text NOT NULL CHECK (length(btrim(review_reason)) BETWEEN 1 AND 1000),
    target_verified_at timestamptz NOT NULL,
    recorded_at timestamptz NOT NULL,
    request_sha256 text NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
    PRIMARY KEY (tenant_id, migration_batch_id, local_party_id),
    UNIQUE (tenant_id, command_id)
);

CREATE INDEX party_migration_mappings_target_idx
ON party_migration_mappings (tenant_id, target_party_id, target_party_revision);

CREATE FUNCTION reject_party_migration_mapping_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'party migration mappings are immutable' USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER party_migration_mappings_immutable
BEFORE UPDATE OR DELETE ON party_migration_mappings
FOR EACH ROW EXECUTE FUNCTION reject_party_migration_mapping_change();
