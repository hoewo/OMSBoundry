CREATE TABLE obligation_cash_expectations (
    id uuid PRIMARY KEY,
    obligation_id uuid NOT NULL REFERENCES obligations(id) ON DELETE RESTRICT,
    sequence bigint NOT NULL CHECK (sequence >= 1),
    action text NOT NULL CHECK (action IN ('set', 'clear')),
    expected_on date,
    obligation_revision bigint NOT NULL CHECK (obligation_revision >= 1),
    source_kind text NOT NULL CHECK (source_kind = 'manual'),
    expectation_reason text NOT NULL CHECK (
        expectation_reason = btrim(expectation_reason)
        AND char_length(expectation_reason) BETWEEN 1 AND 200
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
    CHECK (
        (action = 'set' AND expected_on IS NOT NULL)
        OR (action = 'clear' AND expected_on IS NULL)
    ),
    UNIQUE (obligation_id, sequence),
    UNIQUE (id, obligation_id, sequence),
    FOREIGN KEY (obligation_id, obligation_revision)
        REFERENCES obligation_revisions(obligation_id, revision)
        ON DELETE RESTRICT
);

CREATE INDEX obligation_cash_expectations_current_idx
ON obligation_cash_expectations (obligation_id, sequence DESC);

CREATE INDEX obligation_cash_expectations_schedule_idx
ON obligation_cash_expectations (expected_on, obligation_id)
WHERE action = 'set';

CREATE TABLE obligation_cash_expectation_commands (
    command_id uuid PRIMARY KEY,
    obligation_cash_expectation_id uuid NOT NULL UNIQUE,
    obligation_id uuid NOT NULL,
    expectation_sequence bigint NOT NULL CHECK (expectation_sequence >= 1),
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
    FOREIGN KEY (
        obligation_cash_expectation_id,
        obligation_id,
        expectation_sequence
    ) REFERENCES obligation_cash_expectations(id, obligation_id, sequence)
        ON DELETE RESTRICT
);

CREATE FUNCTION validate_obligation_cash_expectation_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    obligation_status text;
    obligation_current_revision bigint;
    current_effective_amount numeric(24,6);
    current_net_allocated numeric(24,6);
    current_total_remitted numeric(24,6);
    previous_sequence bigint;
    previous_action text;
    previous_recorded_at timestamptz;
BEGIN
    SELECT status, current_revision
    INTO obligation_status, obligation_current_revision
    FROM obligations
    WHERE id = NEW.obligation_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'cash expectation obligation does not exist'
            USING ERRCODE = '23503';
    END IF;
    IF obligation_status <> 'open' THEN
        RAISE EXCEPTION 'cash expectation requires open obligation'
            USING ERRCODE = '23514';
    END IF;
    IF obligation_current_revision IS DISTINCT FROM NEW.obligation_revision THEN
        RAISE EXCEPTION 'cash expectation obligation revision is stale'
            USING ERRCODE = '23514';
    END IF;

    SELECT sequence, action, recorded_at
    INTO previous_sequence, previous_action, previous_recorded_at
    FROM obligation_cash_expectations
    WHERE obligation_id = NEW.obligation_id
    ORDER BY sequence DESC
    LIMIT 1;

    IF NEW.sequence IS DISTINCT FROM COALESCE(previous_sequence, 0) + 1 THEN
        RAISE EXCEPTION 'cash expectation sequence must be continuous'
            USING ERRCODE = '23514';
    END IF;
    IF previous_recorded_at IS NOT NULL AND NEW.recorded_at < previous_recorded_at THEN
        RAISE EXCEPTION 'cash expectation recorded time cannot move backwards'
            USING ERRCODE = '23514';
    END IF;
    IF NEW.action = 'clear' AND previous_action IS DISTINCT FROM 'set' THEN
        RAISE EXCEPTION 'cash expectation clear requires an active set fact'
            USING ERRCODE = '23514';
    END IF;

    IF NEW.action = 'set' THEN
        SELECT revision.effective_amount
        INTO current_effective_amount
        FROM obligation_revisions revision
        WHERE revision.obligation_id = NEW.obligation_id
          AND revision.revision = NEW.obligation_revision;

        SELECT COALESCE(SUM(allocation.amount - COALESCE(reversal.amount, 0)), 0)
        INTO current_net_allocated
        FROM allocations allocation
        LEFT JOIN allocation_reversals reversal ON reversal.allocation_id = allocation.id
        WHERE allocation.obligation_id = NEW.obligation_id;

        SELECT COALESCE(SUM(remission.amount), 0)
        INTO current_total_remitted
        FROM obligation_remissions remission
        WHERE remission.obligation_id = NEW.obligation_id;

        IF current_effective_amount - current_net_allocated - current_total_remitted <= 0 THEN
            RAISE EXCEPTION 'cash expectation set requires positive outstanding amount'
                USING ERRCODE = '23514';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER obligation_cash_expectations_validate_insert
BEFORE INSERT ON obligation_cash_expectations
FOR EACH ROW EXECUTE FUNCTION validate_obligation_cash_expectation_insert();

CREATE TRIGGER obligation_cash_expectations_immutable
BEFORE UPDATE OR DELETE ON obligation_cash_expectations
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();

CREATE TRIGGER obligation_cash_expectation_commands_immutable
BEFORE UPDATE OR DELETE ON obligation_cash_expectation_commands
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
