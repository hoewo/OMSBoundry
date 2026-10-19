ALTER TABLE obligations
    DROP CONSTRAINT obligations_source_check;

ALTER TABLE obligations
    ADD CONSTRAINT obligations_source_check CHECK (
        (
            source_kind = 'manual'
            AND manual_source_reference IS NOT NULL
            AND manual_source_reference = btrim(manual_source_reference)
            AND char_length(manual_source_reference) BETWEEN 1 AND 200
            AND source_application IS NULL
            AND source_resource_type IS NULL
            AND source_resource_id IS NULL
            AND source_revision IS NULL
            AND source_digest IS NULL
        )
        OR
        (
            source_kind = 'cost_outsourcing_payable_suggestion'
            AND manual_source_reference IS NULL
            AND source_application = 'cost-outsourcing'
            AND source_resource_type IN ('cost_entry', 'outsourcing_engagement')
            AND source_resource_id IS NOT NULL
            AND source_revision >= 1
            AND source_digest ~ '^sha256:[0-9a-f]{64}$'
        )
        OR
        (
            source_kind = 'bonus_payable_suggestion'
            AND manual_source_reference IS NULL
            AND source_application = 'bonus-distribution'
            AND source_resource_type = 'bonus_allocation'
            AND source_resource_id IS NOT NULL
            AND source_revision >= 1
            AND source_digest ~ '^sha256:[0-9a-f]{64}$'
        )
    );

ALTER TABLE obligation_revisions
    DROP CONSTRAINT obligation_revisions_source_check;

ALTER TABLE obligation_revisions
    ADD CONSTRAINT obligation_revisions_source_check CHECK (
        (
            source_kind = 'manual'
            AND manual_source_reference IS NOT NULL
            AND manual_source_reference = btrim(manual_source_reference)
            AND char_length(manual_source_reference) BETWEEN 1 AND 200
            AND source_application IS NULL
            AND source_resource_type IS NULL
            AND source_resource_id IS NULL
            AND source_fact_revision IS NULL
            AND source_digest IS NULL
        )
        OR
        (
            source_kind = 'cost_outsourcing_payable_suggestion'
            AND manual_source_reference IS NULL
            AND source_application = 'cost-outsourcing'
            AND source_resource_type IN ('cost_entry', 'outsourcing_engagement')
            AND source_resource_id IS NOT NULL
            AND source_fact_revision >= 1
            AND source_digest ~ '^sha256:[0-9a-f]{64}$'
        )
        OR
        (
            source_kind = 'bonus_payable_suggestion'
            AND manual_source_reference IS NULL
            AND source_application = 'bonus-distribution'
            AND source_resource_type = 'bonus_allocation'
            AND source_resource_id IS NOT NULL
            AND source_fact_revision >= 1
            AND source_digest ~ '^sha256:[0-9a-f]{64}$'
        )
    );

ALTER TABLE payable_suggestion_intakes
    DROP CONSTRAINT payable_suggestion_intakes_source_application_check,
    DROP CONSTRAINT payable_suggestion_intakes_source_resource_type_check;

ALTER TABLE payable_suggestion_intakes
    ADD CONSTRAINT payable_suggestion_intakes_source_pair_check CHECK (
        (source_application = 'cost-outsourcing' AND source_resource_type IN ('cost_entry','outsourcing_engagement'))
        OR
        (source_application = 'bonus-distribution' AND source_resource_type = 'bonus_allocation')
    ),
    ADD COLUMN ledger_owner_source_application text NOT NULL DEFAULT 'party-directory',
    ADD COLUMN ledger_owner_source_resource_type text NOT NULL DEFAULT 'party',
    ADD COLUMN counterparty_source_application text NOT NULL DEFAULT 'party-directory',
    ADD COLUMN counterparty_source_resource_type text NOT NULL DEFAULT 'party';

CREATE TABLE payable_party_source_mappings (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL UNIQUE,
    source_application text NOT NULL CHECK (source_application = 'people-groups'),
    source_resource_type text NOT NULL CHECK (source_resource_type = 'person'),
    source_resource_id uuid NOT NULL,
    source_revision bigint NOT NULL CHECK (source_revision >= 1),
    source_digest text NOT NULL CHECK (source_digest ~ '^sha256:[0-9a-f]{64}$'),
    local_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    local_party_revision bigint NOT NULL CHECK (local_party_revision >= 1),
    local_party_kind text NOT NULL CHECK (local_party_kind = 'individual'),
    local_party_legal_name text NOT NULL CHECK (length(btrim(local_party_legal_name)) BETWEEN 1 AND 200),
    reviewed_by_actor_kind text NOT NULL CHECK (reviewed_by_actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'),
    reviewed_by_actor_id text NOT NULL CHECK (length(btrim(reviewed_by_actor_id)) BETWEEN 1 AND 100),
    reviewed_by_actor_display_name text NOT NULL CHECK (length(btrim(reviewed_by_actor_display_name)) BETWEEN 1 AND 100),
    review_reason text NOT NULL CHECK (length(btrim(review_reason)) BETWEEN 1 AND 1000),
    recorded_at timestamptz NOT NULL,
    request_sha256 char(64) NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
    PRIMARY KEY (tenant_id,source_application,source_resource_type,source_resource_id,source_revision)
);

CREATE FUNCTION reject_payable_party_source_mapping_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'payable party source mappings are immutable' USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER payable_party_source_mappings_immutable
BEFORE UPDATE OR DELETE ON payable_party_source_mappings
FOR EACH ROW EXECUTE FUNCTION reject_payable_party_source_mapping_change();
