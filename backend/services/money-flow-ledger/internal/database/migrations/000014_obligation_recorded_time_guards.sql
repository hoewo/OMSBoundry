CREATE FUNCTION obligation_latest_recorded_fact_at(target_obligation_id uuid)
RETURNS timestamptz
LANGUAGE sql
STABLE
AS $$
    SELECT max(event_at)
    FROM (
        SELECT recorded_at AS event_at
        FROM obligation_revisions
        WHERE obligation_id = target_obligation_id

        UNION ALL

        SELECT adjusted_at
        FROM obligation_amount_adjustments
        WHERE obligation_id = target_obligation_id

        UNION ALL

        SELECT applied_at
        FROM allocations
        WHERE obligation_id = target_obligation_id

        UNION ALL

        SELECT reversed_at
        FROM allocation_reversals
        WHERE obligation_id = target_obligation_id

        UNION ALL

        SELECT recorded_at
        FROM obligation_remissions
        WHERE obligation_id = target_obligation_id

        UNION ALL

        SELECT closed_at
        FROM obligation_closures
        WHERE obligation_id = target_obligation_id
    ) facts;
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM obligation_revisions child
        JOIN obligation_revisions source
          ON source.obligation_id = child.obligation_id
         AND source.revision = child.source_revision
        WHERE child.recorded_at < source.recorded_at
    ) OR EXISTS (
        SELECT 1
        FROM obligation_amount_adjustments adjustment
        JOIN obligation_revisions source
          ON source.obligation_id = adjustment.obligation_id
         AND source.revision = adjustment.source_revision
        JOIN obligation_revisions result
          ON result.obligation_id = adjustment.obligation_id
         AND result.revision = adjustment.resulting_revision
        WHERE adjustment.adjusted_at < source.recorded_at
           OR result.recorded_at IS DISTINCT FROM adjustment.adjusted_at
    ) OR EXISTS (
        SELECT 1
        FROM allocations allocation
        JOIN obligation_revisions revision
          ON revision.obligation_id = allocation.obligation_id
         AND revision.revision = allocation.obligation_revision
        WHERE allocation.applied_at < revision.recorded_at
    ) OR EXISTS (
        SELECT 1
        FROM allocation_reversals reversal
        JOIN allocations allocation ON allocation.id = reversal.allocation_id
        WHERE reversal.reversed_at < allocation.applied_at
    ) OR EXISTS (
        SELECT 1
        FROM obligation_remissions remission
        JOIN obligation_revisions revision
          ON revision.obligation_id = remission.obligation_id
         AND revision.revision = remission.obligation_revision
        WHERE remission.recorded_at < revision.recorded_at
    ) OR EXISTS (
        SELECT 1
        FROM obligation_closures closure
        JOIN obligation_revisions revision
          ON revision.obligation_id = closure.obligation_id
         AND revision.revision = closure.obligation_revision
        WHERE closure.closed_at < revision.recorded_at
    ) THEN
        RAISE EXCEPTION 'existing obligation facts contain causally inverted recorded times'
            USING ERRCODE = '23514';
    END IF;

    IF EXISTS (
        WITH event_times AS (
            SELECT obligation_id, recorded_at AS event_at FROM obligation_revisions
            UNION
            SELECT obligation_id, adjusted_at FROM obligation_amount_adjustments
            UNION
            SELECT obligation_id, applied_at FROM allocations
            UNION
            SELECT obligation_id, reversed_at FROM allocation_reversals
            UNION
            SELECT obligation_id, recorded_at FROM obligation_remissions
            UNION
            SELECT obligation_id, closed_at FROM obligation_closures
        ), historical_prefixes AS (
            SELECT
                event.obligation_id,
                event.event_at,
                revision.revision,
                revision.effective_amount,
                COALESCE(allocation_totals.net_allocated, 0) AS net_allocated,
                COALESCE(remission_totals.remitted, 0) AS remitted,
                closure.id AS closure_id
            FROM event_times event
            LEFT JOIN LATERAL (
                SELECT r.revision, r.effective_amount
                FROM obligation_revisions r
                WHERE r.obligation_id = event.obligation_id
                  AND r.recorded_at <= event.event_at
                ORDER BY r.revision DESC
                LIMIT 1
            ) revision ON true
            LEFT JOIN LATERAL (
                SELECT COALESCE(SUM(
                    allocation.amount
                    - CASE
                        WHEN reversal.reversed_at <= event.event_at THEN reversal.amount
                        ELSE 0
                      END
                ), 0) AS net_allocated
                FROM allocations allocation
                LEFT JOIN allocation_reversals reversal
                  ON reversal.allocation_id = allocation.id
                WHERE allocation.obligation_id = event.obligation_id
                  AND allocation.applied_at <= event.event_at
            ) allocation_totals ON true
            LEFT JOIN LATERAL (
                SELECT COALESCE(SUM(remission.amount), 0) AS remitted
                FROM obligation_remissions remission
                WHERE remission.obligation_id = event.obligation_id
                  AND remission.recorded_at <= event.event_at
            ) remission_totals ON true
            LEFT JOIN LATERAL (
                SELECT closure.id
                FROM obligation_closures closure
                WHERE closure.obligation_id = event.obligation_id
                  AND closure.closed_at <= event.event_at
            ) closure ON true
        )
        SELECT 1
        FROM historical_prefixes prefix
        WHERE prefix.revision IS NULL
           OR prefix.net_allocated < 0
           OR prefix.effective_amount - prefix.net_allocated - prefix.remitted < 0
           OR (
                prefix.closure_id IS NOT NULL
                AND prefix.effective_amount - prefix.net_allocated - prefix.remitted <> 0
           )
    ) THEN
        RAISE EXCEPTION 'existing obligation facts cannot form valid recorded-time historical prefixes'
            USING ERRCODE = '23514';
    END IF;
END;
$$;

CREATE FUNCTION validate_obligation_recorded_time_monotonic()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    value jsonb := to_jsonb(NEW);
    target_obligation_id uuid;
    event_at timestamptz;
    latest_event_at timestamptz;
BEGIN
    target_obligation_id := (value ->> 'obligation_id')::uuid;
    event_at := CASE TG_TABLE_NAME
        WHEN 'obligation_revisions' THEN (value ->> 'recorded_at')::timestamptz
        WHEN 'obligation_amount_adjustments' THEN (value ->> 'adjusted_at')::timestamptz
        WHEN 'allocations' THEN (value ->> 'applied_at')::timestamptz
        WHEN 'allocation_reversals' THEN (value ->> 'reversed_at')::timestamptz
        WHEN 'obligation_remissions' THEN (value ->> 'recorded_at')::timestamptz
        WHEN 'obligation_closures' THEN (value ->> 'closed_at')::timestamptz
        ELSE NULL
    END;

    IF target_obligation_id IS NULL OR event_at IS NULL THEN
        RAISE EXCEPTION 'obligation recorded-time guard received unsupported fact'
            USING ERRCODE = '23514';
    END IF;

    PERFORM 1
    FROM obligations
    WHERE id = target_obligation_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'obligation recorded-time guard source does not exist'
            USING ERRCODE = '23503';
    END IF;

    latest_event_at := obligation_latest_recorded_fact_at(target_obligation_id);
    IF latest_event_at IS NOT NULL AND event_at < latest_event_at THEN
        RAISE EXCEPTION 'obligation fact recorded time cannot precede an existing fact'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER zz_obligation_revisions_recorded_time_monotonic
BEFORE INSERT ON obligation_revisions
FOR EACH ROW EXECUTE FUNCTION validate_obligation_recorded_time_monotonic();

CREATE TRIGGER zz_obligation_amount_adjustments_recorded_time_monotonic
BEFORE INSERT ON obligation_amount_adjustments
FOR EACH ROW EXECUTE FUNCTION validate_obligation_recorded_time_monotonic();

CREATE TRIGGER zz_allocations_recorded_time_monotonic
BEFORE INSERT ON allocations
FOR EACH ROW EXECUTE FUNCTION validate_obligation_recorded_time_monotonic();

CREATE TRIGGER zz_allocation_reversals_recorded_time_monotonic
BEFORE INSERT ON allocation_reversals
FOR EACH ROW EXECUTE FUNCTION validate_obligation_recorded_time_monotonic();

CREATE TRIGGER zz_obligation_remissions_recorded_time_monotonic
BEFORE INSERT ON obligation_remissions
FOR EACH ROW EXECUTE FUNCTION validate_obligation_recorded_time_monotonic();

CREATE TRIGGER zz_obligation_closures_recorded_time_monotonic
BEFORE INSERT ON obligation_closures
FOR EACH ROW EXECUTE FUNCTION validate_obligation_recorded_time_monotonic();

CREATE INDEX obligation_revisions_recorded_revision_idx
ON obligation_revisions (obligation_id, recorded_at DESC, revision DESC);
