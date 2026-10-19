CREATE TABLE party_directory_runtime_projections (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    runtime_transition_sequence bigint NOT NULL REFERENCES party_runtime_transitions(sequence) ON DELETE RESTRICT,
    source_application text NOT NULL CHECK (source_application = 'party-directory'),
    local_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    local_party_revision bigint NOT NULL CHECK (local_party_revision = 1),
    target_party_id uuid NOT NULL,
    target_party_revision bigint NOT NULL CHECK (target_party_revision >= 1),
    target_kind text NOT NULL CHECK (target_kind IN ('organization', 'individual')),
    target_legal_name text NOT NULL CHECK (length(btrim(target_legal_name)) BETWEEN 1 AND 200),
    target_registration_identifier text CHECK (
        target_registration_identifier IS NULL
        OR length(btrim(target_registration_identifier)) BETWEEN 1 AND 100
    ),
    reviewed_by_subject_id uuid NOT NULL,
    reviewed_by_display_name text NOT NULL CHECK (length(btrim(reviewed_by_display_name)) BETWEEN 1 AND 100),
    review_reason text NOT NULL CHECK (length(btrim(review_reason)) BETWEEN 1 AND 1000),
    target_verified_at timestamptz NOT NULL,
    recorded_at timestamptz NOT NULL,
    request_sha256 text NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
    PRIMARY KEY (tenant_id, target_party_id, target_party_revision),
    UNIQUE (tenant_id, command_id),
    UNIQUE (tenant_id, local_party_id)
);

CREATE INDEX party_directory_runtime_projections_local_idx
ON party_directory_runtime_projections (local_party_id);

CREATE FUNCTION validate_party_directory_runtime_projection_insert() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    local_snapshot parties%ROWTYPE;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM party_runtime_transitions t
        WHERE t.sequence = NEW.runtime_transition_sequence
          AND t.tenant_id = NEW.tenant_id
          AND t.action = 'cutover'
          AND t.sequence = (
              SELECT max(latest.sequence)
              FROM party_runtime_transitions latest
              WHERE latest.tenant_id = NEW.tenant_id
          )
    ) THEN
        RAISE EXCEPTION 'Party Directory runtime projection requires the current directory cutover state'
            USING ERRCODE = '55000';
    END IF;

    SELECT * INTO local_snapshot FROM parties WHERE id = NEW.local_party_id;
    IF NOT FOUND
       OR local_snapshot.revision <> NEW.local_party_revision
       OR local_snapshot.kind <> NEW.target_kind
       OR local_snapshot.legal_name <> NEW.target_legal_name
       OR local_snapshot.registration_identifier IS DISTINCT FROM NEW.target_registration_identifier THEN
        RAISE EXCEPTION 'Party Directory runtime projection local snapshot does not match its verified target'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER party_directory_runtime_projection_validate
BEFORE INSERT ON party_directory_runtime_projections
FOR EACH ROW EXECUTE FUNCTION validate_party_directory_runtime_projection_insert();

CREATE FUNCTION reject_party_directory_runtime_projection_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'Party Directory runtime projections are immutable' USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER party_directory_runtime_projections_immutable
BEFORE UPDATE OR DELETE ON party_directory_runtime_projections
FOR EACH ROW EXECUTE FUNCTION reject_party_directory_runtime_projection_change();

CREATE FUNCTION reject_projected_contract_party_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM party_directory_runtime_projections projection
        WHERE projection.local_party_id = OLD.id
    ) THEN
        RAISE EXCEPTION 'Party Directory projected Contract Party snapshots are immutable' USING ERRCODE = '55000';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER projected_contract_parties_immutable
BEFORE UPDATE OR DELETE ON parties
FOR EACH ROW EXECUTE FUNCTION reject_projected_contract_party_change();
