CREATE TABLE contract_evidence_documents (
    id uuid PRIMARY KEY,
    contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    target_type text NOT NULL CHECK (target_type IN ('contract', 'instrument', 'settlement', 'closure')),
    target_id uuid NOT NULL,
    title text NOT NULL CHECK (
        title = btrim(title)
        AND char_length(title) BETWEEN 1 AND 200
    ),
    purpose text NOT NULL CHECK (
        purpose = btrim(purpose)
        AND char_length(purpose) BETWEEN 1 AND 500
    ),
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND char_length(actor_kind) BETWEEN 1 AND 32
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    actor_display_name text NOT NULL CHECK (
        actor_display_name = btrim(actor_display_name)
        AND char_length(actor_display_name) BETWEEN 1 AND 100
    ),
    created_at timestamptz NOT NULL,
    CONSTRAINT contract_evidence_documents_contract_id_id_unique UNIQUE (contract_id, id),
    CONSTRAINT contract_evidence_documents_contract_target_check CHECK (
        target_type <> 'contract' OR target_id = contract_id
    )
);

CREATE INDEX contract_evidence_documents_contract_timeline_idx
ON contract_evidence_documents (contract_id, created_at, id);

CREATE FUNCTION validate_contract_evidence_target()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.target_type = 'contract' THEN
        IF NEW.target_id <> NEW.contract_id THEN
            RAISE EXCEPTION USING
                ERRCODE = '23503',
                CONSTRAINT = 'contract_evidence_documents_target_fk',
                MESSAGE = 'contract evidence target does not belong to contract';
        END IF;
    ELSIF NEW.target_type = 'instrument' THEN
        IF NOT EXISTS (
            SELECT 1 FROM contract_instruments
            WHERE id = NEW.target_id AND contract_id = NEW.contract_id
        ) THEN
            RAISE EXCEPTION USING
                ERRCODE = '23503',
                CONSTRAINT = 'contract_evidence_documents_target_fk',
                MESSAGE = 'contract evidence target does not belong to contract';
        END IF;
    ELSIF NEW.target_type = 'settlement' THEN
        IF NOT EXISTS (
            SELECT 1 FROM contract_settlements
            WHERE id = NEW.target_id AND contract_id = NEW.contract_id
        ) THEN
            RAISE EXCEPTION USING
                ERRCODE = '23503',
                CONSTRAINT = 'contract_evidence_documents_target_fk',
                MESSAGE = 'contract evidence target does not belong to contract';
        END IF;
    ELSIF NEW.target_type = 'closure' THEN
        IF NOT EXISTS (
            SELECT 1 FROM contract_closures
            WHERE id = NEW.target_id AND contract_id = NEW.contract_id
        ) THEN
            RAISE EXCEPTION USING
                ERRCODE = '23503',
                CONSTRAINT = 'contract_evidence_documents_target_fk',
                MESSAGE = 'contract evidence target does not belong to contract';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER contract_evidence_documents_target
BEFORE INSERT ON contract_evidence_documents
FOR EACH ROW EXECUTE FUNCTION validate_contract_evidence_target();

CREATE TABLE contract_evidence_versions (
    id uuid PRIMARY KEY,
    document_id uuid NOT NULL REFERENCES contract_evidence_documents(id) ON DELETE RESTRICT,
    version bigint NOT NULL CHECK (version >= 1),
    original_filename text NOT NULL CHECK (
        original_filename = btrim(original_filename)
        AND char_length(original_filename) BETWEEN 1 AND 255
        AND original_filename !~ '[[:cntrl:]/\\]'
    ),
    detected_media_type text NOT NULL CHECK (
        detected_media_type = btrim(detected_media_type)
        AND char_length(detected_media_type) BETWEEN 1 AND 200
    ),
    size_bytes bigint NOT NULL CHECK (size_bytes >= 1),
    sha256 character(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    storage_provider text NOT NULL CHECK (storage_provider = 'local'),
    storage_key text NOT NULL UNIQUE CHECK (
        storage_key ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
    ),
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND char_length(actor_kind) BETWEEN 1 AND 32
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    actor_display_name text NOT NULL CHECK (
        actor_display_name = btrim(actor_display_name)
        AND char_length(actor_display_name) BETWEEN 1 AND 100
    ),
    recorded_at timestamptz NOT NULL,
    CONSTRAINT contract_evidence_versions_sequence_unique UNIQUE (document_id, version),
    CONSTRAINT contract_evidence_versions_command_fk_unique UNIQUE (document_id, version, id)
);

CREATE TABLE contract_evidence_commands (
    command_id uuid PRIMARY KEY,
    action text NOT NULL CHECK (action IN ('create_document', 'add_version')),
    contract_id uuid NOT NULL,
    document_id uuid NOT NULL,
    version_id uuid NOT NULL UNIQUE,
    expected_latest_version bigint NOT NULL CHECK (expected_latest_version >= 0),
    resulting_version bigint NOT NULL CHECK (resulting_version = expected_latest_version + 1),
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND char_length(actor_kind) BETWEEN 1 AND 32
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    actor_display_name text NOT NULL CHECK (
        actor_display_name = btrim(actor_display_name)
        AND char_length(actor_display_name) BETWEEN 1 AND 100
    ),
    occurred_at timestamptz NOT NULL,
    CONSTRAINT contract_evidence_commands_document_fk
        FOREIGN KEY (contract_id, document_id)
        REFERENCES contract_evidence_documents(contract_id, id) ON DELETE RESTRICT,
    CONSTRAINT contract_evidence_commands_version_fk
        FOREIGN KEY (document_id, resulting_version, version_id)
        REFERENCES contract_evidence_versions(document_id, version, id) ON DELETE RESTRICT,
    CONSTRAINT contract_evidence_commands_action_version_check CHECK (
        (action = 'create_document' AND expected_latest_version = 0 AND resulting_version = 1)
        OR
        (action = 'add_version' AND expected_latest_version >= 1)
    )
);

CREATE TRIGGER contract_evidence_documents_immutable
BEFORE UPDATE OR DELETE ON contract_evidence_documents
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_evidence_versions_immutable
BEFORE UPDATE OR DELETE ON contract_evidence_versions
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_evidence_commands_immutable
BEFORE UPDATE OR DELETE ON contract_evidence_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();
