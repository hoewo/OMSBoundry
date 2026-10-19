CREATE TABLE obligation_remissions (
    id uuid PRIMARY KEY,
    obligation_id uuid NOT NULL REFERENCES obligations(id) ON DELETE RESTRICT,
    obligation_revision bigint NOT NULL CHECK (obligation_revision >= 1),
    amount numeric(24,6) NOT NULL CHECK (amount > 0),
    recorded_effective_amount numeric(24,6) NOT NULL CHECK (recorded_effective_amount > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    source_kind text NOT NULL CHECK (source_kind = 'manual'),
    remission_reason text NOT NULL CHECK (
        remission_reason = btrim(remission_reason)
        AND char_length(remission_reason) BETWEEN 1 AND 200
    ),
    manual_source_reference text NOT NULL CHECK (
        manual_source_reference = btrim(manual_source_reference)
        AND char_length(manual_source_reference) BETWEEN 1 AND 200
    ),
    recorded_by_actor_kind text NOT NULL CHECK (
        recorded_by_actor_kind = btrim(recorded_by_actor_kind)
        AND recorded_by_actor_kind ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    recorded_by_actor_id text NOT NULL CHECK (
        recorded_by_actor_id = btrim(recorded_by_actor_id)
        AND char_length(recorded_by_actor_id) BETWEEN 1 AND 100
    ),
    recorded_by_actor_display_name text NOT NULL CHECK (
        recorded_by_actor_display_name = btrim(recorded_by_actor_display_name)
        AND char_length(recorded_by_actor_display_name) BETWEEN 1 AND 100
    ),
    recorded_at timestamptz NOT NULL,
    UNIQUE (id, obligation_id, obligation_revision),
    FOREIGN KEY (obligation_id, obligation_revision)
        REFERENCES obligation_revisions(obligation_id, revision)
        ON DELETE RESTRICT
);

CREATE INDEX obligation_remissions_obligation_recorded_id_idx
ON obligation_remissions (obligation_id, recorded_at, id);

CREATE TABLE obligation_remission_commands (
    command_id uuid PRIMARY KEY,
    obligation_remission_id uuid NOT NULL,
    obligation_id uuid NOT NULL,
    expected_current_revision bigint NOT NULL CHECK (expected_current_revision >= 1),
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
    recorded_at timestamptz NOT NULL,
    UNIQUE (obligation_remission_id),
    FOREIGN KEY (obligation_remission_id, obligation_id, expected_current_revision)
        REFERENCES obligation_remissions(id, obligation_id, obligation_revision)
        ON DELETE RESTRICT
);

CREATE FUNCTION validate_obligation_remission_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    obligation_status text;
    obligation_current_revision bigint;
    effective_amount numeric(24,6);
    obligation_currency text;
    net_allocated numeric(24,6);
    already_remitted numeric(24,6);
BEGIN
    SELECT status, current_revision
    INTO obligation_status, obligation_current_revision
    FROM obligations
    WHERE id = NEW.obligation_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'obligation remission source does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF obligation_status <> 'open' THEN
        RAISE EXCEPTION 'obligation remission requires open obligation'
            USING ERRCODE = '23514';
    END IF;
    IF obligation_current_revision IS DISTINCT FROM NEW.obligation_revision THEN
        RAISE EXCEPTION 'obligation remission revision is stale'
            USING ERRCODE = '23514';
    END IF;

    SELECT r.effective_amount, r.currency
    INTO effective_amount, obligation_currency
    FROM obligation_revisions r
    WHERE r.obligation_id = NEW.obligation_id
      AND r.revision = NEW.obligation_revision;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'obligation remission revision does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF NEW.recorded_effective_amount IS DISTINCT FROM effective_amount
       OR NEW.currency IS DISTINCT FROM obligation_currency THEN
        RAISE EXCEPTION 'obligation remission source facts do not match revision'
            USING ERRCODE = '23514';
    END IF;

    SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
    INTO net_allocated
    FROM allocations a
    LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
    WHERE a.obligation_id = NEW.obligation_id;

    SELECT COALESCE(SUM(r.amount), 0)
    INTO already_remitted
    FROM obligation_remissions r
    WHERE r.obligation_id = NEW.obligation_id;

    IF net_allocated + already_remitted + NEW.amount > effective_amount THEN
        RAISE EXCEPTION 'obligation remission exceeds outstanding amount'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER obligation_remissions_validate_insert
BEFORE INSERT ON obligation_remissions
FOR EACH ROW EXECUTE FUNCTION validate_obligation_remission_insert();

CREATE FUNCTION validate_allocation_remission_balance_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    effective_amount numeric(24,6);
    existing_allocated numeric(24,6);
    remitted_amount numeric(24,6);
BEGIN
    SELECT r.effective_amount
    INTO effective_amount
    FROM obligations o
    JOIN obligation_revisions r
      ON r.obligation_id = o.id AND r.revision = o.current_revision
    WHERE o.id = NEW.obligation_id
    FOR UPDATE OF o;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'allocation obligation does not exist'
            USING ERRCODE = '23503';
    END IF;

    SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
    INTO existing_allocated
    FROM allocations a
    LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
    WHERE a.obligation_id = NEW.obligation_id;

    SELECT COALESCE(SUM(r.amount), 0)
    INTO remitted_amount
    FROM obligation_remissions r
    WHERE r.obligation_id = NEW.obligation_id;

    IF existing_allocated + remitted_amount + NEW.amount > effective_amount THEN
        RAISE EXCEPTION 'allocation exceeds obligation outstanding after remissions'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER allocations_validate_remission_insert
BEFORE INSERT ON allocations
FOR EACH ROW EXECUTE FUNCTION validate_allocation_remission_balance_insert();

CREATE FUNCTION validate_adjustment_remission_floor_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    net_allocated numeric(24,6);
    remitted_amount numeric(24,6);
BEGIN
    PERFORM id
    FROM obligations
    WHERE id = NEW.obligation_id
    FOR UPDATE;

    SELECT COALESCE(SUM(a.amount - COALESCE(ar.amount, 0)), 0)
    INTO net_allocated
    FROM allocations a
    LEFT JOIN allocation_reversals ar ON ar.allocation_id = a.id
    WHERE a.obligation_id = NEW.obligation_id;

    SELECT COALESCE(SUM(r.amount), 0)
    INTO remitted_amount
    FROM obligation_remissions r
    WHERE r.obligation_id = NEW.obligation_id;

    IF NEW.resulting_effective_amount < net_allocated + remitted_amount THEN
        RAISE EXCEPTION 'obligation amount adjustment result is below allocated and remitted amount'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER obligation_amount_adjustments_validate_remission_floor
BEFORE INSERT ON obligation_amount_adjustments
FOR EACH ROW EXECUTE FUNCTION validate_adjustment_remission_floor_insert();

CREATE TRIGGER obligation_remissions_immutable
BEFORE UPDATE OR DELETE ON obligation_remissions
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER obligation_remission_commands_immutable
BEFORE UPDATE OR DELETE ON obligation_remission_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
