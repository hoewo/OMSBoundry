CREATE TABLE receipt_sets (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    project_id uuid NOT NULL,
    status text NOT NULL CHECK (status = 'confirmed'),
    currency text NOT NULL CHECK (currency = 'CNY'),
    revision bigint NOT NULL CHECK (revision = 1),
    total_amount_minor bigint NOT NULL CHECK (total_amount_minor > 0),
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
    confirmed_at timestamptz NOT NULL
);

CREATE INDEX receipt_sets_tenant_project_confirmed_idx
ON receipt_sets (tenant_id, project_id, confirmed_at DESC, id DESC);

CREATE TABLE receipt_set_items (
    receipt_set_id uuid NOT NULL REFERENCES receipt_sets(id) ON DELETE RESTRICT,
    ordinal integer NOT NULL CHECK (ordinal >= 1),
    transaction_id uuid NOT NULL REFERENCES cash_transactions(id) ON DELETE RESTRICT,
    transaction_revision bigint NOT NULL CHECK (transaction_revision = 1),
    allocation_id uuid NOT NULL REFERENCES allocations(id) ON DELETE RESTRICT,
    allocation_revision bigint NOT NULL CHECK (allocation_revision = 1),
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency = 'CNY'),
    status text NOT NULL CHECK (status = 'confirmed'),
    PRIMARY KEY (receipt_set_id, ordinal),
    UNIQUE (receipt_set_id, allocation_id)
);

CREATE INDEX receipt_set_items_allocation_idx
ON receipt_set_items (allocation_id, receipt_set_id);

CREATE TABLE receipt_set_confirm_commands (
    command_id uuid PRIMARY KEY,
    receipt_set_id uuid NOT NULL UNIQUE REFERENCES receipt_sets(id) ON DELETE RESTRICT,
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
    confirmed_at timestamptz NOT NULL
);

CREATE FUNCTION validate_receipt_set_item_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    source_transaction_id uuid;
    source_amount numeric(24,6);
    source_currency text;
    source_type text;
    source_transaction_reversed boolean;
    source_allocation_reversed boolean;
BEGIN
    SELECT
        a.cash_transaction_id,
        a.amount,
        a.currency,
        ct.type,
        EXISTS (SELECT 1 FROM cash_transaction_reversals ctr WHERE ctr.cash_transaction_id = ct.id),
        EXISTS (SELECT 1 FROM allocation_reversals ar WHERE ar.allocation_id = a.id)
    INTO
        source_transaction_id,
        source_amount,
        source_currency,
        source_type,
        source_transaction_reversed,
        source_allocation_reversed
    FROM allocations a
    JOIN cash_transactions ct ON ct.id = a.cash_transaction_id
    WHERE a.id = NEW.allocation_id
    FOR SHARE OF a, ct;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'receipt set allocation does not exist' USING ERRCODE = '23503';
    END IF;
    IF NEW.transaction_id IS DISTINCT FROM source_transaction_id
       OR NEW.transaction_revision <> 1
       OR NEW.allocation_revision <> 1
       OR source_type <> 'receipt'
       OR source_transaction_reversed
       OR source_allocation_reversed
       OR source_currency <> 'CNY'
       OR NEW.currency <> source_currency
       OR trunc(source_amount, 2) IS DISTINCT FROM source_amount
       OR source_amount * 100 IS DISTINCT FROM NEW.amount_minor::numeric THEN
        RAISE EXCEPTION 'receipt set item does not match an active CNY receipt allocation'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER receipt_set_items_validate_insert
BEFORE INSERT ON receipt_set_items
FOR EACH ROW EXECUTE FUNCTION validate_receipt_set_item_insert();

CREATE FUNCTION validate_receipt_set_total()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_id uuid;
    expected_total bigint;
    item_count bigint;
    item_total numeric;
BEGIN
    target_id := CASE WHEN TG_TABLE_NAME = 'receipt_sets' THEN NEW.id ELSE NEW.receipt_set_id END;
    SELECT total_amount_minor INTO expected_total FROM receipt_sets WHERE id = target_id;
    SELECT count(*), COALESCE(sum(amount_minor), 0)
    INTO item_count, item_total
    FROM receipt_set_items
    WHERE receipt_set_id = target_id;
    IF expected_total IS NULL OR item_count < 1 OR item_total IS DISTINCT FROM expected_total::numeric THEN
        RAISE EXCEPTION 'receipt set total must equal its non-empty item sum' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER receipt_sets_total_consistent
AFTER INSERT ON receipt_sets
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_receipt_set_total();

CREATE CONSTRAINT TRIGGER receipt_set_items_total_consistent
AFTER INSERT ON receipt_set_items
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_receipt_set_total();

CREATE TRIGGER receipt_sets_immutable
BEFORE UPDATE OR DELETE ON receipt_sets
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER receipt_set_items_immutable
BEFORE UPDATE OR DELETE ON receipt_set_items
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER receipt_set_confirm_commands_immutable
BEFORE UPDATE OR DELETE ON receipt_set_confirm_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
