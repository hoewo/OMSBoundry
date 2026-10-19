CREATE TABLE contract_relations (
    id uuid PRIMARY KEY,
    source_contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    target_contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    relation_type text NOT NULL CHECK (relation_type IN (
        'funding_source_for',
        'entrustment_basis_for',
        'subcontract_of',
        'specific_under_framework',
        'supersedes',
        'references'
    )),
    source_revision bigint NOT NULL CHECK (source_revision >= 1),
    target_revision bigint NOT NULL CHECK (target_revision >= 1),
    effective_on date NULL,
    note text NULL CHECK (
        note IS NULL OR (
            note = btrim(note)
            AND char_length(note) BETWEEN 1 AND 500
        )
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
    CONSTRAINT contract_relations_distinct_contracts CHECK (source_contract_id <> target_contract_id),
    CONSTRAINT contract_relations_source_revision_fk
        FOREIGN KEY (source_contract_id, source_revision)
        REFERENCES contract_revisions(contract_id, revision) ON DELETE RESTRICT,
    CONSTRAINT contract_relations_target_revision_fk
        FOREIGN KEY (target_contract_id, target_revision)
        REFERENCES contract_revisions(contract_id, revision) ON DELETE RESTRICT,
    CONSTRAINT contract_relations_unique_edge
        UNIQUE (source_contract_id, target_contract_id, relation_type)
);

CREATE INDEX contract_relations_source_idx
ON contract_relations (source_contract_id, created_at, id);

CREATE INDEX contract_relations_target_idx
ON contract_relations (target_contract_id, created_at, id);

CREATE TABLE contract_relation_commands (
    command_id uuid PRIMARY KEY,
    relation_id uuid NOT NULL UNIQUE REFERENCES contract_relations(id) ON DELETE RESTRICT,
    source_contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    target_contract_id uuid NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
    relation_type text NOT NULL CHECK (relation_type IN (
        'funding_source_for',
        'entrustment_basis_for',
        'subcontract_of',
        'specific_under_framework',
        'supersedes',
        'references'
    )),
    expected_source_revision bigint NOT NULL CHECK (expected_source_revision >= 1),
    expected_target_revision bigint NOT NULL CHECK (expected_target_revision >= 1),
    effective_on date NULL,
    note text NULL CHECK (
        note IS NULL OR (
            note = btrim(note)
            AND char_length(note) BETWEEN 1 AND 500
        )
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
    occurred_at timestamptz NOT NULL,
    CONSTRAINT contract_relation_commands_distinct_contracts
        CHECK (source_contract_id <> target_contract_id)
);

CREATE TRIGGER contract_relations_immutable
BEFORE UPDATE OR DELETE ON contract_relations
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();

CREATE TRIGGER contract_relation_commands_immutable
BEFORE UPDATE OR DELETE ON contract_relation_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_contract_fact_change();
