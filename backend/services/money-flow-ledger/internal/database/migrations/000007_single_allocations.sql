CREATE TABLE allocations (
    id uuid PRIMARY KEY,
    cash_transaction_id uuid NOT NULL REFERENCES cash_transactions(id) ON DELETE RESTRICT,
    obligation_id uuid NOT NULL,
    obligation_revision bigint NOT NULL CHECK (obligation_revision >= 1),
    amount numeric(24,6) NOT NULL CHECK (amount > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    source_kind text NOT NULL CHECK (source_kind = 'manual'),
    manual_source_reference text NOT NULL CHECK (
        manual_source_reference = btrim(manual_source_reference)
        AND char_length(manual_source_reference) BETWEEN 1 AND 200
    ),
    applied_by_actor_kind text NOT NULL CHECK (
        applied_by_actor_kind = btrim(applied_by_actor_kind)
        AND applied_by_actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    applied_by_actor_id text NOT NULL CHECK (
        applied_by_actor_id = btrim(applied_by_actor_id)
        AND char_length(applied_by_actor_id) BETWEEN 1 AND 100
    ),
    applied_by_actor_display_name text NOT NULL CHECK (
        applied_by_actor_display_name = btrim(applied_by_actor_display_name)
        AND char_length(applied_by_actor_display_name) BETWEEN 1 AND 100
    ),
    applied_at timestamptz NOT NULL,
    FOREIGN KEY (obligation_id, obligation_revision)
        REFERENCES obligation_revisions(obligation_id, revision)
        ON DELETE RESTRICT
);

CREATE INDEX allocations_applied_id_idx
ON allocations (applied_at DESC, id DESC);

CREATE INDEX allocations_cash_applied_id_idx
ON allocations (cash_transaction_id, applied_at, id);

CREATE INDEX allocations_obligation_applied_id_idx
ON allocations (obligation_id, applied_at, id);

CREATE TABLE allocation_apply_commands (
    command_id uuid PRIMARY KEY,
    allocation_id uuid NOT NULL UNIQUE REFERENCES allocations(id) ON DELETE RESTRICT,
    cash_transaction_id uuid NOT NULL REFERENCES cash_transactions(id) ON DELETE RESTRICT,
    obligation_id uuid NOT NULL,
    expected_obligation_revision bigint NOT NULL CHECK (expected_obligation_revision >= 1),
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
    applied_at timestamptz NOT NULL,
    FOREIGN KEY (obligation_id, expected_obligation_revision)
        REFERENCES obligation_revisions(obligation_id, revision)
        ON DELETE RESTRICT
);

CREATE FUNCTION validate_allocation_insert()
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
        original_amount,
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

    SELECT COALESCE(SUM(amount), 0)
    INTO existing_cash_allocated
    FROM allocations
    WHERE cash_transaction_id = NEW.cash_transaction_id;

    IF existing_cash_allocated + NEW.amount > cash_amount THEN
        RAISE EXCEPTION 'allocation exceeds cash unallocated amount'
            USING ERRCODE = '23514';
    END IF;

    SELECT COALESCE(SUM(amount), 0)
    INTO existing_obligation_allocated
    FROM allocations
    WHERE obligation_id = NEW.obligation_id;

    IF existing_obligation_allocated + NEW.amount > obligation_amount THEN
        RAISE EXCEPTION 'allocation exceeds obligation outstanding amount'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER allocations_validate_insert
BEFORE INSERT ON allocations
FOR EACH ROW EXECUTE FUNCTION validate_allocation_insert();

CREATE TRIGGER allocations_immutable
BEFORE UPDATE OR DELETE ON allocations
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER allocation_apply_commands_immutable
BEFORE UPDATE OR DELETE ON allocation_apply_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
