CREATE TABLE cash_transaction_reversals (
    id uuid PRIMARY KEY,
    cash_transaction_id uuid NOT NULL UNIQUE REFERENCES cash_transactions(id) ON DELETE RESTRICT,
    transaction_type text NOT NULL CHECK (transaction_type IN ('receipt', 'payment')),
    amount numeric(24,6) NOT NULL CHECK (amount > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    source_kind text NOT NULL CHECK (source_kind = 'manual'),
    reversal_reason text NOT NULL CHECK (
        reversal_reason = btrim(reversal_reason)
        AND char_length(reversal_reason) BETWEEN 1 AND 200
    ),
    reversed_by_actor_kind text NOT NULL CHECK (
        reversed_by_actor_kind = btrim(reversed_by_actor_kind)
        AND reversed_by_actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    reversed_by_actor_id text NOT NULL CHECK (
        reversed_by_actor_id = btrim(reversed_by_actor_id)
        AND char_length(reversed_by_actor_id) BETWEEN 1 AND 100
    ),
    reversed_by_actor_display_name text NOT NULL CHECK (
        reversed_by_actor_display_name = btrim(reversed_by_actor_display_name)
        AND char_length(reversed_by_actor_display_name) BETWEEN 1 AND 100
    ),
    reversed_at timestamptz NOT NULL
);

CREATE INDEX cash_transaction_reversals_reversed_id_idx
ON cash_transaction_reversals (reversed_at DESC, id DESC);

CREATE TABLE cash_transaction_reversal_create_commands (
    command_id uuid PRIMARY KEY,
    cash_transaction_reversal_id uuid NOT NULL UNIQUE REFERENCES cash_transaction_reversals(id) ON DELETE RESTRICT,
    cash_transaction_id uuid NOT NULL REFERENCES cash_transactions(id) ON DELETE RESTRICT,
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
    reversed_at timestamptz NOT NULL
);

CREATE FUNCTION validate_cash_transaction_reversal_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    original_type text;
    original_amount numeric(24,6);
    original_currency text;
    existing_allocated numeric(24,6);
BEGIN
    SELECT type, amount, currency
    INTO original_type, original_amount, original_currency
    FROM cash_transactions
    WHERE id = NEW.cash_transaction_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'cash transaction reversal source does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF NEW.transaction_type IS DISTINCT FROM original_type
       OR NEW.amount IS DISTINCT FROM original_amount
       OR NEW.currency IS DISTINCT FROM original_currency THEN
        RAISE EXCEPTION 'cash transaction reversal facts must match original transaction'
            USING ERRCODE = '23514';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM cash_transaction_reversals
        WHERE cash_transaction_id = NEW.cash_transaction_id
    ) THEN
        RAISE EXCEPTION 'cash transaction already has a reversal'
            USING ERRCODE = '23505';
    END IF;

    SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
    INTO existing_allocated
    FROM allocations a
    LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
    WHERE a.cash_transaction_id = NEW.cash_transaction_id;

    IF existing_allocated <> 0 THEN
        RAISE EXCEPTION 'cash transaction reversal requires zero net allocated amount'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER cash_transaction_reversals_validate_insert
BEFORE INSERT ON cash_transaction_reversals
FOR EACH ROW EXECUTE FUNCTION validate_cash_transaction_reversal_insert();

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

CREATE TRIGGER cash_transaction_reversals_immutable
BEFORE UPDATE OR DELETE ON cash_transaction_reversals
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER cash_transaction_reversal_create_commands_immutable
BEFORE UPDATE OR DELETE ON cash_transaction_reversal_create_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
