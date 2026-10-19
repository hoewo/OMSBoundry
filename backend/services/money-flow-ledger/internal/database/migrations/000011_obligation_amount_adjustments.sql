ALTER TABLE obligation_revisions
RENAME COLUMN original_amount TO effective_amount;

ALTER TABLE obligation_revisions
RENAME COLUMN confirmed_by_actor_kind TO recorded_by_actor_kind;

ALTER TABLE obligation_revisions
RENAME COLUMN confirmed_by_actor_id TO recorded_by_actor_id;

ALTER TABLE obligation_revisions
RENAME COLUMN confirmed_by_actor_display_name TO recorded_by_actor_display_name;

ALTER TABLE obligation_revisions
RENAME COLUMN confirmed_at TO recorded_at;

ALTER TABLE obligation_revisions
DROP CONSTRAINT obligation_revisions_obligation_id_source_draft_version_key;

ALTER TABLE obligation_revisions
ALTER COLUMN source_draft_version DROP NOT NULL;

ALTER TABLE obligation_revisions
ADD COLUMN revision_kind text NOT NULL DEFAULT 'confirmation',
ADD COLUMN source_revision bigint NULL;

ALTER TABLE obligation_revisions
ALTER COLUMN revision_kind DROP DEFAULT;

ALTER TABLE obligation_revisions
ADD CONSTRAINT obligation_revisions_kind_source_check
CHECK (
    (
        revision_kind = 'confirmation'
        AND revision = 1
        AND source_draft_version IS NOT NULL
        AND source_revision IS NULL
    )
    OR
    (
        revision_kind = 'amount_adjustment'
        AND revision >= 2
        AND source_draft_version IS NULL
        AND source_revision = revision - 1
    )
),
ADD CONSTRAINT obligation_revisions_source_revision_fk
FOREIGN KEY (obligation_id, source_revision)
REFERENCES obligation_revisions(obligation_id, revision)
ON DELETE RESTRICT
DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE obligation_amount_adjustments (
    id uuid PRIMARY KEY,
    obligation_id uuid NOT NULL REFERENCES obligations(id) ON DELETE RESTRICT,
    source_revision bigint NOT NULL CHECK (source_revision >= 1),
    resulting_revision bigint NOT NULL CHECK (resulting_revision = source_revision + 1),
    adjustment_type text NOT NULL CHECK (adjustment_type IN ('increase', 'decrease')),
    adjustment_amount numeric(24,6) NOT NULL CHECK (adjustment_amount > 0),
    previous_effective_amount numeric(24,6) NOT NULL CHECK (previous_effective_amount > 0),
    resulting_effective_amount numeric(24,6) NOT NULL CHECK (resulting_effective_amount > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    source_kind text NOT NULL CHECK (source_kind = 'manual'),
    adjustment_reason text NOT NULL CHECK (
        adjustment_reason = btrim(adjustment_reason)
        AND char_length(adjustment_reason) BETWEEN 1 AND 200
    ),
    adjusted_by_actor_kind text NOT NULL CHECK (
        adjusted_by_actor_kind = btrim(adjusted_by_actor_kind)
        AND adjusted_by_actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    adjusted_by_actor_id text NOT NULL CHECK (
        adjusted_by_actor_id = btrim(adjusted_by_actor_id)
        AND char_length(adjusted_by_actor_id) BETWEEN 1 AND 100
    ),
    adjusted_by_actor_display_name text NOT NULL CHECK (
        adjusted_by_actor_display_name = btrim(adjusted_by_actor_display_name)
        AND char_length(adjusted_by_actor_display_name) BETWEEN 1 AND 100
    ),
    adjusted_at timestamptz NOT NULL,
    UNIQUE (obligation_id, source_revision),
    UNIQUE (obligation_id, resulting_revision),
    FOREIGN KEY (obligation_id, source_revision)
        REFERENCES obligation_revisions(obligation_id, revision)
        ON DELETE RESTRICT,
    FOREIGN KEY (obligation_id, resulting_revision)
        REFERENCES obligation_revisions(obligation_id, revision)
        ON DELETE RESTRICT
        DEFERRABLE INITIALLY DEFERRED,
    CHECK (
        (adjustment_type = 'increase'
            AND resulting_effective_amount = previous_effective_amount + adjustment_amount)
        OR
        (adjustment_type = 'decrease'
            AND resulting_effective_amount = previous_effective_amount - adjustment_amount)
    )
);

CREATE INDEX obligation_amount_adjustments_obligation_adjusted_id_idx
ON obligation_amount_adjustments (obligation_id, adjusted_at, id);

CREATE TABLE obligation_amount_adjustment_commands (
    command_id uuid PRIMARY KEY,
    obligation_amount_adjustment_id uuid NOT NULL UNIQUE
        REFERENCES obligation_amount_adjustments(id) ON DELETE RESTRICT,
    obligation_id uuid NOT NULL,
    expected_current_revision bigint NOT NULL CHECK (expected_current_revision >= 1),
    resulting_revision bigint NOT NULL CHECK (resulting_revision = expected_current_revision + 1),
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
    adjusted_at timestamptz NOT NULL,
    FOREIGN KEY (obligation_id, resulting_revision)
        REFERENCES obligation_revisions(obligation_id, revision)
        ON DELETE RESTRICT
);

CREATE FUNCTION validate_obligation_amount_adjustment_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    obligation_status text;
    obligation_current_revision bigint;
    source_amount numeric(24,6);
    source_currency text;
    net_allocated numeric(24,6);
    expected_result numeric;
BEGIN
    SELECT status, current_revision
    INTO obligation_status, obligation_current_revision
    FROM obligations
    WHERE id = NEW.obligation_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'obligation amount adjustment source does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF obligation_status <> 'open' THEN
        RAISE EXCEPTION 'obligation amount adjustment requires open obligation'
            USING ERRCODE = '23514';
    END IF;
    IF obligation_current_revision IS DISTINCT FROM NEW.source_revision THEN
        RAISE EXCEPTION 'obligation amount adjustment source revision is stale'
            USING ERRCODE = '23514';
    END IF;

    SELECT effective_amount, currency
    INTO source_amount, source_currency
    FROM obligation_revisions
    WHERE obligation_id = NEW.obligation_id
      AND revision = NEW.source_revision;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'obligation amount adjustment source revision does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF NEW.previous_effective_amount IS DISTINCT FROM source_amount
       OR NEW.currency IS DISTINCT FROM source_currency THEN
        RAISE EXCEPTION 'obligation amount adjustment source facts do not match revision'
            USING ERRCODE = '23514';
    END IF;

    IF NEW.adjustment_type = 'increase' THEN
        expected_result := source_amount + NEW.adjustment_amount;
    ELSE
        expected_result := source_amount - NEW.adjustment_amount;
    END IF;
    IF NEW.resulting_effective_amount IS DISTINCT FROM expected_result THEN
        RAISE EXCEPTION 'obligation amount adjustment result is invalid'
            USING ERRCODE = '23514';
    END IF;
    IF NEW.resulting_effective_amount <= 0 THEN
        RAISE EXCEPTION 'obligation amount adjustment result must be positive'
            USING ERRCODE = '23514';
    END IF;

    SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
    INTO net_allocated
    FROM allocations a
    LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
    WHERE a.obligation_id = NEW.obligation_id;

    IF NEW.resulting_effective_amount < net_allocated THEN
        RAISE EXCEPTION 'obligation amount adjustment result is below net allocated amount'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER obligation_amount_adjustments_validate_insert
BEFORE INSERT ON obligation_amount_adjustments
FOR EACH ROW EXECUTE FUNCTION validate_obligation_amount_adjustment_insert();

CREATE FUNCTION validate_obligation_revision_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    source obligation_revisions%ROWTYPE;
    adjustment obligation_amount_adjustments%ROWTYPE;
BEGIN
    IF NEW.revision_kind = 'confirmation' THEN
        IF NEW.revision <> 1
           OR NEW.source_draft_version IS NULL
           OR NEW.source_revision IS NOT NULL THEN
            RAISE EXCEPTION 'confirmation obligation revision metadata is invalid'
                USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;

    SELECT * INTO source
    FROM obligation_revisions
    WHERE obligation_id = NEW.obligation_id
      AND revision = NEW.source_revision;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'adjustment obligation revision source does not exist'
            USING ERRCODE = '23503';
    END IF;

    SELECT * INTO adjustment
    FROM obligation_amount_adjustments
    WHERE obligation_id = NEW.obligation_id
      AND resulting_revision = NEW.revision;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'adjustment obligation revision has no adjustment fact'
            USING ERRCODE = '23514';
    END IF;

    IF adjustment.source_revision IS DISTINCT FROM NEW.source_revision
       OR adjustment.resulting_effective_amount IS DISTINCT FROM NEW.effective_amount
       OR NEW.direction IS DISTINCT FROM source.direction
       OR NEW.title IS DISTINCT FROM source.title
       OR NEW.ledger_owner_party_id IS DISTINCT FROM source.ledger_owner_party_id
       OR NEW.creditor_party_id IS DISTINCT FROM source.creditor_party_id
       OR NEW.creditor_party_revision IS DISTINCT FROM source.creditor_party_revision
       OR NEW.creditor_kind IS DISTINCT FROM source.creditor_kind
       OR NEW.creditor_legal_name IS DISTINCT FROM source.creditor_legal_name
       OR NEW.creditor_registration_identifier IS DISTINCT FROM source.creditor_registration_identifier
       OR NEW.debtor_party_id IS DISTINCT FROM source.debtor_party_id
       OR NEW.debtor_party_revision IS DISTINCT FROM source.debtor_party_revision
       OR NEW.debtor_kind IS DISTINCT FROM source.debtor_kind
       OR NEW.debtor_legal_name IS DISTINCT FROM source.debtor_legal_name
       OR NEW.debtor_registration_identifier IS DISTINCT FROM source.debtor_registration_identifier
       OR NEW.currency IS DISTINCT FROM source.currency
       OR NEW.due_on IS DISTINCT FROM source.due_on
       OR NEW.source_kind IS DISTINCT FROM source.source_kind
       OR NEW.manual_source_reference IS DISTINCT FROM source.manual_source_reference
       OR NEW.recorded_by_actor_kind IS DISTINCT FROM adjustment.adjusted_by_actor_kind
       OR NEW.recorded_by_actor_id IS DISTINCT FROM adjustment.adjusted_by_actor_id
       OR NEW.recorded_by_actor_display_name IS DISTINCT FROM adjustment.adjusted_by_actor_display_name
       OR NEW.recorded_at IS DISTINCT FROM adjustment.adjusted_at THEN
        RAISE EXCEPTION 'adjustment obligation revision facts must match source and adjustment'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER obligation_revisions_validate_insert
BEFORE INSERT ON obligation_revisions
FOR EACH ROW EXECUTE FUNCTION validate_obligation_revision_insert();

CREATE FUNCTION validate_obligation_current_revision_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.current_revision IS NOT DISTINCT FROM OLD.current_revision THEN
        RETURN NEW;
    END IF;

    IF OLD.current_revision IS NULL
       AND OLD.status = 'draft'
       AND NEW.status = 'open'
       AND NEW.current_revision = 1
       AND EXISTS (
           SELECT 1
           FROM obligation_confirmation_commands
           WHERE obligation_id = NEW.id
             AND resulting_revision = NEW.current_revision
       ) THEN
        RETURN NEW;
    END IF;

    IF OLD.status = 'open'
       AND NEW.status = 'open'
       AND NEW.current_revision = OLD.current_revision + 1
       AND EXISTS (
           SELECT 1
           FROM obligation_amount_adjustments
           WHERE obligation_id = NEW.id
             AND source_revision = OLD.current_revision
             AND resulting_revision = NEW.current_revision
       ) THEN
        RETURN NEW;
    END IF;

    RAISE EXCEPTION 'obligation current revision transition is not backed by a valid fact'
        USING ERRCODE = '23514';
END;
$$;

CREATE TRIGGER obligations_validate_current_revision_update
BEFORE UPDATE OF current_revision ON obligations
FOR EACH ROW EXECUTE FUNCTION validate_obligation_current_revision_update();

CREATE OR REPLACE FUNCTION validate_allocation_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    cash_type text;
    cash_ledger_owner uuid;
    cash_payer uuid;
    cash_payee uuid;
    cash_amount numeric(24,6);
    cash_currency text;
    obligation_status text;
    obligation_current_revision bigint;
    obligation_direction text;
    obligation_ledger_owner uuid;
    obligation_creditor uuid;
    obligation_debtor uuid;
    obligation_amount numeric(24,6);
    obligation_currency text;
    existing_cash_allocated numeric(24,6);
    existing_obligation_allocated numeric(24,6);
BEGIN
    SELECT
        type,
        ledger_owner_party_id,
        payer_party_id,
        payee_party_id,
        amount,
        currency
    INTO
        cash_type,
        cash_ledger_owner,
        cash_payer,
        cash_payee,
        cash_amount,
        cash_currency
    FROM cash_transactions
    WHERE id = NEW.cash_transaction_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'allocation cash transaction does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM cash_transaction_reversals
        WHERE cash_transaction_id = NEW.cash_transaction_id
    ) THEN
        RAISE EXCEPTION 'allocation cash transaction is reversed'
            USING ERRCODE = '23514';
    END IF;

    SELECT status, current_revision
    INTO obligation_status, obligation_current_revision
    FROM obligations
    WHERE id = NEW.obligation_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'allocation obligation does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF obligation_status <> 'open' THEN
        RAISE EXCEPTION 'allocation obligation must be open'
            USING ERRCODE = '23514';
    END IF;
    IF obligation_current_revision IS DISTINCT FROM NEW.obligation_revision THEN
        RAISE EXCEPTION 'allocation obligation revision is stale'
            USING ERRCODE = '23514';
    END IF;

    SELECT
        direction,
        ledger_owner_party_id,
        creditor_party_id,
        debtor_party_id,
        effective_amount,
        currency
    INTO
        obligation_direction,
        obligation_ledger_owner,
        obligation_creditor,
        obligation_debtor,
        obligation_amount,
        obligation_currency
    FROM obligation_revisions
    WHERE obligation_id = NEW.obligation_id
      AND revision = NEW.obligation_revision;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'allocation obligation revision does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF NEW.currency <> cash_currency OR NEW.currency <> obligation_currency THEN
        RAISE EXCEPTION 'allocation currency must match cash and obligation'
            USING ERRCODE = '23514';
    END IF;
    IF cash_ledger_owner <> obligation_ledger_owner THEN
        RAISE EXCEPTION 'allocation ledger owner must match'
            USING ERRCODE = '23514';
    END IF;
    IF cash_payer <> obligation_debtor OR cash_payee <> obligation_creditor THEN
        RAISE EXCEPTION 'allocation payer and payee must match obligation parties'
            USING ERRCODE = '23514';
    END IF;
    IF (obligation_direction = 'receivable' AND cash_type <> 'receipt')
       OR (obligation_direction = 'payable' AND cash_type <> 'payment') THEN
        RAISE EXCEPTION 'allocation cash and obligation directions must match'
            USING ERRCODE = '23514';
    END IF;

    SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
    INTO existing_cash_allocated
    FROM allocations a
    LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
    WHERE a.cash_transaction_id = NEW.cash_transaction_id;

    IF existing_cash_allocated + NEW.amount > cash_amount THEN
        RAISE EXCEPTION 'allocation exceeds cash unallocated amount'
            USING ERRCODE = '23514';
    END IF;

    SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
    INTO existing_obligation_allocated
    FROM allocations a
    LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
    WHERE a.obligation_id = NEW.obligation_id;

    IF existing_obligation_allocated + NEW.amount > obligation_amount THEN
        RAISE EXCEPTION 'allocation exceeds obligation outstanding amount'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER obligation_amount_adjustments_immutable
BEFORE UPDATE OR DELETE ON obligation_amount_adjustments
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER obligation_amount_adjustment_commands_immutable
BEFORE UPDATE OR DELETE ON obligation_amount_adjustment_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
