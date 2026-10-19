ALTER TABLE obligations
ADD COLUMN current_revision bigint NULL;

CREATE TABLE obligation_revisions (
    obligation_id uuid NOT NULL REFERENCES obligations(id) ON DELETE RESTRICT,
    revision bigint NOT NULL CHECK (revision >= 1),
    direction text NOT NULL CHECK (direction IN ('receivable', 'payable')),
    title text NOT NULL CHECK (
        title = btrim(title)
        AND char_length(title) BETWEEN 1 AND 200
    ),
    ledger_owner_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    creditor_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    creditor_party_revision bigint NOT NULL CHECK (creditor_party_revision >= 1),
    creditor_kind text NOT NULL CHECK (creditor_kind IN ('organization', 'individual')),
    creditor_legal_name text NOT NULL CHECK (
        creditor_legal_name = btrim(creditor_legal_name)
        AND char_length(creditor_legal_name) BETWEEN 1 AND 200
    ),
    creditor_registration_identifier text NULL CHECK (
        creditor_registration_identifier = btrim(creditor_registration_identifier)
        AND char_length(creditor_registration_identifier) BETWEEN 1 AND 100
    ),
    debtor_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    debtor_party_revision bigint NOT NULL CHECK (debtor_party_revision >= 1),
    debtor_kind text NOT NULL CHECK (debtor_kind IN ('organization', 'individual')),
    debtor_legal_name text NOT NULL CHECK (
        debtor_legal_name = btrim(debtor_legal_name)
        AND char_length(debtor_legal_name) BETWEEN 1 AND 200
    ),
    debtor_registration_identifier text NULL CHECK (
        debtor_registration_identifier = btrim(debtor_registration_identifier)
        AND char_length(debtor_registration_identifier) BETWEEN 1 AND 100
    ),
    original_amount numeric(24,6) NOT NULL CHECK (original_amount > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    due_on date NOT NULL,
    source_kind text NOT NULL CHECK (source_kind = 'manual'),
    manual_source_reference text NOT NULL CHECK (
        manual_source_reference = btrim(manual_source_reference)
        AND char_length(manual_source_reference) BETWEEN 1 AND 200
    ),
    source_draft_version bigint NOT NULL CHECK (source_draft_version >= 1),
    confirmed_by_actor_kind text NOT NULL CHECK (
        confirmed_by_actor_kind = btrim(confirmed_by_actor_kind)
        AND confirmed_by_actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    confirmed_by_actor_id text NOT NULL CHECK (
        confirmed_by_actor_id = btrim(confirmed_by_actor_id)
        AND char_length(confirmed_by_actor_id) BETWEEN 1 AND 100
    ),
    confirmed_by_actor_display_name text NOT NULL CHECK (
        confirmed_by_actor_display_name = btrim(confirmed_by_actor_display_name)
        AND char_length(confirmed_by_actor_display_name) BETWEEN 1 AND 100
    ),
    confirmed_at timestamptz NOT NULL,
    PRIMARY KEY (obligation_id, revision),
    UNIQUE (obligation_id, source_draft_version),
    CHECK (creditor_party_id <> debtor_party_id),
    CHECK (
        (direction = 'receivable' AND ledger_owner_party_id = creditor_party_id)
        OR (direction = 'payable' AND ledger_owner_party_id = debtor_party_id)
    )
);

ALTER TABLE obligations
ADD CONSTRAINT obligations_current_revision_fk
FOREIGN KEY (id, current_revision)
REFERENCES obligation_revisions(obligation_id, revision)
ON DELETE RESTRICT
DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE obligations
ADD CONSTRAINT obligations_status_current_revision_check
CHECK (
    (status IN ('draft', 'cancelled') AND current_revision IS NULL)
    OR (status IN ('open', 'closed') AND current_revision IS NOT NULL)
);

CREATE TABLE obligation_confirmation_commands (
    command_id uuid PRIMARY KEY,
    obligation_id uuid NOT NULL,
    expected_draft_version bigint NOT NULL CHECK (expected_draft_version >= 1),
    resulting_revision bigint NOT NULL CHECK (resulting_revision >= 1),
    payload_hash bytea NOT NULL CHECK (octet_length(payload_hash) = 32),
    actor_kind text NOT NULL CHECK (
        actor_kind = btrim(actor_kind)
        AND actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    actor_id text NOT NULL CHECK (
        actor_id = btrim(actor_id)
        AND char_length(actor_id) BETWEEN 1 AND 100
    ),
    actor_display_name text NOT NULL CHECK (
        actor_display_name = btrim(actor_display_name)
        AND char_length(actor_display_name) BETWEEN 1 AND 100
    ),
    confirmed_at timestamptz NOT NULL,
    UNIQUE (obligation_id, resulting_revision),
    FOREIGN KEY (obligation_id, resulting_revision)
        REFERENCES obligation_revisions(obligation_id, revision)
        ON DELETE RESTRICT
);

CREATE TRIGGER obligation_revisions_immutable
BEFORE UPDATE OR DELETE ON obligation_revisions
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER obligation_confirmation_commands_immutable
BEFORE UPDATE OR DELETE ON obligation_confirmation_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
