CREATE TABLE invoice_recognition_proposals (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    evidence_file_id uuid NOT NULL,
    evidence_version bigint NOT NULL CHECK (evidence_version>=1),
    proposal jsonb NOT NULL CHECK (jsonb_typeof(proposal)='object'),
    status text NOT NULL CHECK (status IN ('needs_review','confirmed')),
    revision bigint NOT NULL CHECK (revision>=1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id)
);
CREATE INDEX invoice_recognition_proposals_timeline_idx ON invoice_recognition_proposals(tenant_id,created_at DESC,id);

CREATE TABLE invoice_recognition_proposal_revisions (
    tenant_id uuid NOT NULL,
    proposal_id uuid NOT NULL,
    revision bigint NOT NULL,
    command_id uuid NOT NULL,
    event_kind text NOT NULL CHECK (event_kind IN ('recognized','confirmed')),
    snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot)='object'),
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,proposal_id,revision),
    FOREIGN KEY (tenant_id,proposal_id) REFERENCES invoice_recognition_proposals(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE invoice_recognition_confirmations (
    tenant_id uuid NOT NULL,
    proposal_id uuid NOT NULL,
    proposal_revision bigint NOT NULL,
    confirmation jsonb NOT NULL CHECK (jsonb_typeof(confirmation)='object'),
    confirmed_by_subject uuid NOT NULL,
    confirmed_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,proposal_id),
    FOREIGN KEY (tenant_id,proposal_id) REFERENCES invoice_recognition_proposals(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE invoice_recognition_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    response_kind text NOT NULL CHECK (response_kind IN ('proposal','confirmation')),
    response_json jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,command_id)
);

CREATE TABLE invoice_recognition_audit_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    proposal_id uuid NOT NULL,
    action text NOT NULL CHECK (action IN ('invoice_recognized','invoice_recognition_confirmed')),
    source_revision bigint NOT NULL,
    result_revision bigint NOT NULL,
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL
);
CREATE INDEX invoice_recognition_audit_timeline_idx ON invoice_recognition_audit_events(tenant_id,occurred_at DESC,id);

CREATE FUNCTION reject_invoice_recognition_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'immutable invoice recognition record'; END; $$;
CREATE TRIGGER invoice_recognition_proposal_revisions_immutable BEFORE UPDATE OR DELETE ON invoice_recognition_proposal_revisions FOR EACH ROW EXECUTE FUNCTION reject_invoice_recognition_immutable_change();
CREATE TRIGGER invoice_recognition_confirmations_immutable BEFORE UPDATE OR DELETE ON invoice_recognition_confirmations FOR EACH ROW EXECUTE FUNCTION reject_invoice_recognition_immutable_change();
CREATE TRIGGER invoice_recognition_commands_immutable BEFORE UPDATE OR DELETE ON invoice_recognition_commands FOR EACH ROW EXECUTE FUNCTION reject_invoice_recognition_immutable_change();
CREATE TRIGGER invoice_recognition_audit_immutable BEFORE UPDATE OR DELETE ON invoice_recognition_audit_events FOR EACH ROW EXECUTE FUNCTION reject_invoice_recognition_immutable_change();
