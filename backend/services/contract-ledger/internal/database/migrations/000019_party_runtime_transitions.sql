CREATE TABLE party_runtime_transitions (
    sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    action text NOT NULL CHECK (action IN ('cutover', 'rollback')),
    migration_batch_id uuid NOT NULL,
    prior_sequence bigint REFERENCES party_runtime_transitions(sequence) ON DELETE RESTRICT,
    reviewed_by_subject_id uuid NOT NULL,
    reviewed_by_display_name text NOT NULL CHECK (length(btrim(reviewed_by_display_name)) BETWEEN 1 AND 100),
    reason text NOT NULL CHECK (length(btrim(reason)) BETWEEN 1 AND 1000),
    mapped_party_count integer NOT NULL CHECK (mapped_party_count >= 1),
    referenced_party_count integer NOT NULL CHECK (referenced_party_count >= 0),
    request_sha256 text NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL,
    UNIQUE (tenant_id, command_id)
);

CREATE INDEX party_runtime_transitions_current_idx
ON party_runtime_transitions (tenant_id, sequence DESC);

CREATE FUNCTION reject_party_runtime_transition_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'party runtime transitions are immutable' USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER party_runtime_transitions_immutable
BEFORE UPDATE OR DELETE ON party_runtime_transitions
FOR EACH ROW EXECUTE FUNCTION reject_party_runtime_transition_change();

CREATE FUNCTION reject_mapping_for_transitioned_batch() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM party_runtime_transitions
        WHERE tenant_id = NEW.tenant_id AND migration_batch_id = NEW.migration_batch_id
    ) THEN
        RAISE EXCEPTION 'party migration batch is sealed by a runtime transition' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER party_migration_mapping_batch_sealed
BEFORE INSERT ON party_migration_mappings
FOR EACH ROW EXECUTE FUNCTION reject_mapping_for_transitioned_batch();
