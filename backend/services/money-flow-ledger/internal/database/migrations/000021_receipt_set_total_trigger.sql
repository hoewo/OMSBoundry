CREATE OR REPLACE FUNCTION validate_receipt_set_total()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    target_id uuid;
    expected_total bigint;
    item_count bigint;
    item_total numeric;
BEGIN
    IF TG_TABLE_NAME = 'receipt_sets' THEN
        target_id := NEW.id;
    ELSE
        target_id := NEW.receipt_set_id;
    END IF;

    SELECT total_amount_minor INTO expected_total
    FROM receipt_sets
    WHERE id = target_id;

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
