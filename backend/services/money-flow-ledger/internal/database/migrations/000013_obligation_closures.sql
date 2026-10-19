CREATE TABLE obligation_closures (
    id uuid PRIMARY KEY,
    obligation_id uuid NOT NULL UNIQUE REFERENCES obligations(id) ON DELETE RESTRICT,
    obligation_revision bigint NOT NULL CHECK (obligation_revision >= 1),
    balance_sequence bigint NOT NULL CHECK (balance_sequence >= 1),
    recorded_effective_amount numeric(24,6) NOT NULL CHECK (recorded_effective_amount > 0),
    net_allocated_amount numeric(24,6) NOT NULL CHECK (net_allocated_amount >= 0),
    remitted_amount numeric(24,6) NOT NULL CHECK (remitted_amount >= 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    settlement_basis text NOT NULL CHECK (settlement_basis IN ('allocated', 'remitted', 'mixed')),
    source_kind text NOT NULL CHECK (source_kind = 'manual'),
    closure_reason text NOT NULL CHECK (
        closure_reason = btrim(closure_reason)
        AND char_length(closure_reason) BETWEEN 1 AND 200
    ),
    closed_by_actor_kind text NOT NULL CHECK (
        closed_by_actor_kind = btrim(closed_by_actor_kind)
        AND closed_by_actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    closed_by_actor_id text NOT NULL CHECK (
        closed_by_actor_id = btrim(closed_by_actor_id)
        AND char_length(closed_by_actor_id) BETWEEN 1 AND 100
    ),
    closed_by_actor_display_name text NOT NULL CHECK (
        closed_by_actor_display_name = btrim(closed_by_actor_display_name)
        AND char_length(closed_by_actor_display_name) BETWEEN 1 AND 100
    ),
    closed_at timestamptz NOT NULL,
    CHECK (net_allocated_amount + remitted_amount = recorded_effective_amount),
    CHECK (
        (settlement_basis = 'allocated' AND net_allocated_amount > 0 AND remitted_amount = 0)
        OR (settlement_basis = 'remitted' AND net_allocated_amount = 0 AND remitted_amount > 0)
        OR (settlement_basis = 'mixed' AND net_allocated_amount > 0 AND remitted_amount > 0)
    ),
    UNIQUE (id, obligation_id, obligation_revision, balance_sequence),
    FOREIGN KEY (obligation_id, obligation_revision)
        REFERENCES obligation_revisions(obligation_id, revision)
        ON DELETE RESTRICT
);

CREATE INDEX obligation_closures_closed_id_idx
ON obligation_closures (closed_at DESC, id DESC);

CREATE TABLE obligation_closure_commands (
    command_id uuid PRIMARY KEY,
    obligation_closure_id uuid NOT NULL,
    obligation_id uuid NOT NULL,
    expected_current_revision bigint NOT NULL CHECK (expected_current_revision >= 1),
    expected_balance_sequence bigint NOT NULL CHECK (expected_balance_sequence >= 1),
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
    closed_at timestamptz NOT NULL,
    UNIQUE (obligation_closure_id),
    FOREIGN KEY (
        obligation_closure_id,
        obligation_id,
        expected_current_revision,
        expected_balance_sequence
    ) REFERENCES obligation_closures(id, obligation_id, obligation_revision, balance_sequence)
        ON DELETE RESTRICT
);

CREATE FUNCTION validate_obligation_closure_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    obligation_status text;
    obligation_current_revision bigint;
    effective_amount numeric(24,6);
    obligation_currency text;
    net_allocated numeric(24,6);
    total_remitted numeric(24,6);
    current_balance_sequence bigint;
    expected_basis text;
BEGIN
    SELECT status, current_revision
    INTO obligation_status, obligation_current_revision
    FROM obligations
    WHERE id = NEW.obligation_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'obligation closure source does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF obligation_status <> 'open' THEN
        RAISE EXCEPTION 'obligation closure requires open obligation'
            USING ERRCODE = '23514';
    END IF;
    IF obligation_current_revision IS DISTINCT FROM NEW.obligation_revision THEN
        RAISE EXCEPTION 'obligation closure revision is stale'
            USING ERRCODE = '23514';
    END IF;

    SELECT r.effective_amount, r.currency
    INTO effective_amount, obligation_currency
    FROM obligation_revisions r
    WHERE r.obligation_id = NEW.obligation_id
      AND r.revision = NEW.obligation_revision;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'obligation closure revision does not exist'
            USING ERRCODE = '23503';
    END IF;

    SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
    INTO net_allocated
    FROM allocations a
    LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
    WHERE a.obligation_id = NEW.obligation_id;

    SELECT COALESCE(SUM(r.amount), 0)
    INTO total_remitted
    FROM obligation_remissions r
    WHERE r.obligation_id = NEW.obligation_id;

    SELECT
        (SELECT count(*) FROM allocations a WHERE a.obligation_id = NEW.obligation_id)
        + (SELECT count(*) FROM allocation_reversals ar WHERE ar.obligation_id = NEW.obligation_id)
        + (SELECT count(*) FROM obligation_remissions r WHERE r.obligation_id = NEW.obligation_id)
    INTO current_balance_sequence;

    IF net_allocated > 0 AND total_remitted = 0 THEN
        expected_basis := 'allocated';
    ELSIF net_allocated = 0 AND total_remitted > 0 THEN
        expected_basis := 'remitted';
    ELSIF net_allocated > 0 AND total_remitted > 0 THEN
        expected_basis := 'mixed';
    ELSE
        expected_basis := NULL;
    END IF;

    IF effective_amount IS DISTINCT FROM net_allocated + total_remitted THEN
        RAISE EXCEPTION 'obligation closure requires zero outstanding amount'
            USING ERRCODE = '23514';
    END IF;
    IF NEW.balance_sequence IS DISTINCT FROM current_balance_sequence THEN
        RAISE EXCEPTION 'obligation closure balance sequence is stale'
            USING ERRCODE = '23514';
    END IF;
    IF NEW.recorded_effective_amount IS DISTINCT FROM effective_amount
       OR NEW.net_allocated_amount IS DISTINCT FROM net_allocated
       OR NEW.remitted_amount IS DISTINCT FROM total_remitted
       OR NEW.currency IS DISTINCT FROM obligation_currency
       OR NEW.settlement_basis IS DISTINCT FROM expected_basis THEN
        RAISE EXCEPTION 'obligation closure frozen facts do not match current balance'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER obligation_closures_validate_insert
BEFORE INSERT ON obligation_closures
FOR EACH ROW EXECUTE FUNCTION validate_obligation_closure_insert();

CREATE FUNCTION validate_obligation_closure_status_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    closure_time timestamptz;
BEGIN
    IF OLD.status = 'closed' AND NEW.status <> 'closed' THEN
        RAISE EXCEPTION 'closed obligation cannot be reopened'
            USING ERRCODE = '55000';
    END IF;

    IF NEW.status = 'closed' AND OLD.status <> 'closed' THEN
        IF OLD.status <> 'open' THEN
            RAISE EXCEPTION 'only open obligation can be closed'
                USING ERRCODE = '23514';
        END IF;

        SELECT closed_at
        INTO closure_time
        FROM obligation_closures
        WHERE obligation_id = NEW.id;

        IF NOT FOUND THEN
            RAISE EXCEPTION 'closed obligation requires closure fact'
                USING ERRCODE = '23514';
        END IF;
        IF NEW.updated_at IS DISTINCT FROM closure_time THEN
            RAISE EXCEPTION 'closed obligation timestamp must match closure fact'
                USING ERRCODE = '23514';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER obligations_validate_closure_status_update
BEFORE UPDATE OF status ON obligations
FOR EACH ROW EXECUTE FUNCTION validate_obligation_closure_status_update();

CREATE FUNCTION require_closed_obligation_for_closure()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    obligation_status text;
    obligation_current_revision bigint;
    obligation_updated_at timestamptz;
BEGIN
    SELECT status, current_revision, updated_at
    INTO obligation_status, obligation_current_revision, obligation_updated_at
    FROM obligations
    WHERE id = NEW.obligation_id;

    IF obligation_status IS DISTINCT FROM 'closed'
       OR obligation_current_revision IS DISTINCT FROM NEW.obligation_revision
       OR obligation_updated_at IS DISTINCT FROM NEW.closed_at THEN
        RAISE EXCEPTION 'obligation closure and closed status must commit together'
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;

CREATE CONSTRAINT TRIGGER obligation_closures_require_closed_status
AFTER INSERT ON obligation_closures
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION require_closed_obligation_for_closure();

CREATE OR REPLACE FUNCTION validate_allocation_reversal_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    original_cash_transaction_id uuid;
    original_obligation_id uuid;
    original_obligation_revision bigint;
    original_amount numeric(24,6);
    original_currency text;
    obligation_status text;
BEGIN
    PERFORM 1
    FROM cash_transactions
    WHERE id = NEW.cash_transaction_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'allocation reversal cash transaction does not exist'
            USING ERRCODE = '23503';
    END IF;

    SELECT status
    INTO obligation_status
    FROM obligations
    WHERE id = NEW.obligation_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'allocation reversal obligation does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF obligation_status <> 'open' THEN
        RAISE EXCEPTION 'allocation reversal obligation must be open'
            USING ERRCODE = '23514';
    END IF;

    SELECT
        cash_transaction_id,
        obligation_id,
        obligation_revision,
        amount,
        currency
    INTO
        original_cash_transaction_id,
        original_obligation_id,
        original_obligation_revision,
        original_amount,
        original_currency
    FROM allocations
    WHERE id = NEW.allocation_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'allocation reversal allocation does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF NEW.cash_transaction_id IS DISTINCT FROM original_cash_transaction_id
       OR NEW.obligation_id IS DISTINCT FROM original_obligation_id
       OR NEW.obligation_revision IS DISTINCT FROM original_obligation_revision
       OR NEW.amount IS DISTINCT FROM original_amount
       OR NEW.currency IS DISTINCT FROM original_currency THEN
        RAISE EXCEPTION 'allocation reversal facts must match original allocation'
            USING ERRCODE = '23514';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM allocation_reversals
        WHERE allocation_id = NEW.allocation_id
    ) THEN
        RAISE EXCEPTION 'allocation already has a reversal'
            USING ERRCODE = '23505';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER obligation_closures_immutable
BEFORE UPDATE OR DELETE ON obligation_closures
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER obligation_closure_commands_immutable
BEFORE UPDATE OR DELETE ON obligation_closure_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
