CREATE TABLE invoice_registrations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    status text NOT NULL CHECK (status IN ('recorded','registration_invalidated')),
    issued_on date NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    issuer_party jsonb NOT NULL CHECK (jsonb_typeof(issuer_party)='object'),
    recipient_party jsonb NOT NULL CHECK (jsonb_typeof(recipient_party)='object'),
    source_refs jsonb NOT NULL CHECK (jsonb_typeof(source_refs)='array' AND jsonb_array_length(source_refs)>0),
    evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(evidence_refs)='array'),
    note text NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id),
    CHECK (char_length(note) <= 2000),
    CHECK (issuer_party->>'id' <> recipient_party->>'id')
);
CREATE INDEX invoice_registrations_tenant_issued_idx ON invoice_registrations(tenant_id,issued_on,id);
CREATE INDEX invoice_registrations_tenant_status_idx ON invoice_registrations(tenant_id,status,updated_at,id);

CREATE TABLE invoice_registration_revisions (
    tenant_id uuid NOT NULL,
    invoice_registration_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    event_kind text NOT NULL CHECK (event_kind IN ('recorded','details_corrected','evidence_changed','registration_invalidated')),
    command_id uuid NOT NULL,
    status text NOT NULL CHECK (status IN ('recorded','registration_invalidated')),
    issued_on date NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    issuer_party jsonb NOT NULL CHECK (jsonb_typeof(issuer_party)='object'),
    recipient_party jsonb NOT NULL CHECK (jsonb_typeof(recipient_party)='object'),
    source_refs jsonb NOT NULL CHECK (jsonb_typeof(source_refs)='array' AND jsonb_array_length(source_refs)>0),
    evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(evidence_refs)='array'),
    note text NOT NULL,
    reason text NOT NULL,
    actor_ref text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,invoice_registration_id,revision),
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,invoice_registration_id) REFERENCES invoice_registrations(tenant_id,id) ON DELETE RESTRICT,
    CHECK (char_length(note) <= 2000),
    CHECK (reason=btrim(reason) AND char_length(reason) BETWEEN 1 AND 1000),
    CHECK (actor_ref=btrim(actor_ref) AND char_length(actor_ref) BETWEEN 1 AND 256),
    CHECK (issuer_party->>'id' <> recipient_party->>'id')
);

CREATE TABLE invoice_registration_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    actor_subject uuid NOT NULL,
    response_json jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,command_id)
);

CREATE TABLE invoice_registration_audit_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    invoice_registration_id uuid NOT NULL,
    command_id uuid NOT NULL,
    source_revision bigint NOT NULL CHECK (source_revision >= 0),
    result_revision bigint NOT NULL CHECK (result_revision > source_revision),
    action text NOT NULL CHECK (action IN ('invoice_registration_recorded','invoice_registration_corrected','invoice_registration_evidence_changed','invoice_registration_invalidated')),
    reason text NOT NULL,
    actor_subject uuid NOT NULL,
    actor_kind text NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,invoice_registration_id,result_revision) REFERENCES invoice_registration_revisions(tenant_id,invoice_registration_id,revision) ON DELETE RESTRICT
);

CREATE FUNCTION ensure_invoice_registration_current_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    current_row invoice_registrations%ROWTYPE;
    latest_row invoice_registration_revisions%ROWTYPE;
    target_tenant uuid;
    target_registration uuid;
BEGIN
    target_tenant := NEW.tenant_id;
    target_registration := COALESCE((to_jsonb(NEW)->>'id')::uuid, (to_jsonb(NEW)->>'invoice_registration_id')::uuid);
    SELECT * INTO current_row FROM invoice_registrations WHERE tenant_id=target_tenant AND id=target_registration;
    SELECT * INTO latest_row FROM invoice_registration_revisions WHERE tenant_id=target_tenant AND invoice_registration_id=target_registration ORDER BY revision DESC LIMIT 1;
    IF NOT FOUND OR current_row.revision IS DISTINCT FROM latest_row.revision
       OR current_row.status IS DISTINCT FROM latest_row.status
       OR current_row.issued_on IS DISTINCT FROM latest_row.issued_on
       OR current_row.amount_minor IS DISTINCT FROM latest_row.amount_minor
       OR current_row.currency IS DISTINCT FROM latest_row.currency
       OR current_row.issuer_party IS DISTINCT FROM latest_row.issuer_party
       OR current_row.recipient_party IS DISTINCT FROM latest_row.recipient_party
       OR current_row.source_refs IS DISTINCT FROM latest_row.source_refs
       OR current_row.evidence_refs IS DISTINCT FROM latest_row.evidence_refs
       OR current_row.note IS DISTINCT FROM latest_row.note
       OR current_row.created_at IS DISTINCT FROM latest_row.created_at
       OR current_row.updated_at IS DISTINCT FROM latest_row.updated_at THEN
        RAISE EXCEPTION 'invoice registration current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER invoice_registrations_current_consistency
    AFTER INSERT OR UPDATE ON invoice_registrations DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION ensure_invoice_registration_current_matches_latest_revision();
CREATE CONSTRAINT TRIGGER invoice_registration_revisions_current_consistency
    AFTER INSERT ON invoice_registration_revisions DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION ensure_invoice_registration_current_matches_latest_revision();

CREATE FUNCTION reject_invoice_registration_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'immutable invoice registration record'; END;
$$;
CREATE TRIGGER invoice_registration_revisions_immutable BEFORE UPDATE OR DELETE ON invoice_registration_revisions FOR EACH ROW EXECUTE FUNCTION reject_invoice_registration_immutable_change();
CREATE TRIGGER invoice_registration_commands_immutable BEFORE UPDATE OR DELETE ON invoice_registration_commands FOR EACH ROW EXECUTE FUNCTION reject_invoice_registration_immutable_change();
CREATE TRIGGER invoice_registration_audit_immutable BEFORE UPDATE OR DELETE ON invoice_registration_audit_events FOR EACH ROW EXECUTE FUNCTION reject_invoice_registration_immutable_change();
