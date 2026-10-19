ALTER TABLE contract_evidence_versions
    ALTER COLUMN storage_provider DROP NOT NULL,
    ALTER COLUMN storage_key DROP NOT NULL,
    ADD COLUMN content_status text NOT NULL DEFAULT 'available'
        CHECK (content_status IN ('available', 'missing')),
    ADD CONSTRAINT contract_evidence_versions_content_shape_check CHECK (
        (
            content_status = 'available'
            AND storage_provider = 'local'
            AND storage_key IS NOT NULL
        )
        OR
        (
            content_status = 'missing'
            AND storage_provider IS NULL
            AND storage_key IS NULL
        )
    );

CREATE TABLE contract_restore_operations (
    command_id uuid PRIMARY KEY,
    ledger_sha256 character(64) NOT NULL UNIQUE CHECK (
        ledger_sha256 ~ '^[0-9a-f]{64}$'
    ),
    archive_format text NOT NULL CHECK (
        archive_format = 'omsbizfoundry.contract-ledger/contract-export-v1'
    ),
    source_generated_at timestamptz NOT NULL,
    contract_id uuid NOT NULL UNIQUE REFERENCES contracts(id) ON DELETE RESTRICT,
    missing_evidence_versions bigint NOT NULL CHECK (missing_evidence_versions >= 0),
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
    restored_at timestamptz NOT NULL
);

CREATE TRIGGER contract_restore_operations_immutable
BEFORE UPDATE OR DELETE ON contract_restore_operations
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();
