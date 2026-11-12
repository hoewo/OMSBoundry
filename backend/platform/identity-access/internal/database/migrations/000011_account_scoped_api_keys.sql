ALTER TABLE api_keys
    ALTER COLUMN person_id DROP NOT NULL,
    ALTER COLUMN person_revision DROP NOT NULL;

ALTER TABLE api_keys
    ADD CONSTRAINT api_keys_legacy_person_snapshot_pair_check
    CHECK (
        (person_id IS NULL AND person_revision IS NULL)
        OR (person_id IS NOT NULL AND person_revision IS NOT NULL AND person_revision >= 1)
    );

ALTER TABLE open_api_usage_records
    ALTER COLUMN person_id DROP NOT NULL;

CREATE OR REPLACE FUNCTION reject_open_api_usage_identity_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id <> OLD.id
       OR NEW.tenant_id <> OLD.tenant_id
       OR NEW.subject_id <> OLD.subject_id
       OR NEW.person_id IS DISTINCT FROM OLD.person_id
       OR NEW.api_key_id <> OLD.api_key_id
       OR NEW.key_prefix <> OLD.key_prefix
       OR NEW.action <> OLD.action
       OR NEW.request_id <> OLD.request_id
       OR NEW.source_ip_masked <> OLD.source_ip_masked
       OR NEW.occurred_at <> OLD.occurred_at THEN
        RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'open API usage identity is immutable';
    END IF;
    IF OLD.completed_at IS NOT NULL THEN
        RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'completed open API usage is immutable';
    END IF;
    IF NEW.completed_at IS NULL OR NEW.http_status = 0 THEN
        RAISE EXCEPTION USING ERRCODE = '23514', MESSAGE = 'open API usage completion is incomplete';
    END IF;
    RETURN NEW;
END;
$$;
