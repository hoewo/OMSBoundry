ALTER TABLE finance_requests
    ADD COLUMN archived_at timestamptz NULL,
    ADD COLUMN deleted_at timestamptz NULL,
    ADD CONSTRAINT finance_request_retention_order CHECK (deleted_at IS NULL OR archived_at IS NULL OR deleted_at >= archived_at);

ALTER TABLE finance_request_revisions DROP CONSTRAINT finance_request_revisions_event_kind_check;
ALTER TABLE finance_request_revisions ADD CONSTRAINT finance_request_revisions_event_kind_check
    CHECK (event_kind IN ('created','draft_updated','submitted','withdrawn','payment_recorded','restored'));
ALTER TABLE finance_request_audit_events DROP CONSTRAINT finance_request_audit_events_action_check;
ALTER TABLE finance_request_audit_events ADD CONSTRAINT finance_request_audit_events_action_check
    CHECK (action IN ('finance_request_created','finance_request_draft_updated','finance_request_submitted','finance_request_withdrawn','finance_request_payment_recorded','finance_request_draft_restored'));

CREATE TABLE finance_request_retention_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    request_id uuid NOT NULL,
    action text NOT NULL CHECK (action IN ('archived','restored','deleted')),
    command_id uuid NULL,
    actor_subject_id uuid NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    FOREIGN KEY (tenant_id,request_id) REFERENCES finance_requests(tenant_id,id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX finance_request_retention_command_uniq
    ON finance_request_retention_events(tenant_id,command_id)
    WHERE command_id IS NOT NULL;
CREATE INDEX finance_request_retention_lookup_idx ON finance_requests(tenant_id,deleted_at,archived_at,updated_at DESC,id);
CREATE TRIGGER finance_request_retention_events_immutable
BEFORE UPDATE OR DELETE ON finance_request_retention_events
FOR EACH ROW EXECUTE FUNCTION reject_finance_request_immutable_change();
