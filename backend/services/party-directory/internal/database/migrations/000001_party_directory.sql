CREATE TABLE parties (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('organization','individual')),
    legal_name text NOT NULL,
    legal_name_normalized text NOT NULL,
    registration_identifier text,
    registration_identifier_normalized text,
    status text NOT NULL CHECK (status IN ('active','inactive')),
    revision bigint NOT NULL CHECK (revision > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id),
    CHECK (legal_name=btrim(legal_name) AND char_length(legal_name) BETWEEN 1 AND 200),
    CHECK (legal_name_normalized=lower(regexp_replace(btrim(legal_name),'[[:space:]]+',' ','g'))),
    CHECK (registration_identifier IS NULL OR (registration_identifier=btrim(registration_identifier) AND char_length(registration_identifier) BETWEEN 1 AND 100)),
    CHECK ((registration_identifier IS NULL)=(registration_identifier_normalized IS NULL)),
    CHECK (registration_identifier_normalized IS NULL OR registration_identifier_normalized=lower(btrim(registration_identifier)))
);
CREATE INDEX parties_tenant_name_idx ON parties(tenant_id,legal_name_normalized,legal_name,id);
CREATE INDEX parties_tenant_registration_idx ON parties(tenant_id,registration_identifier_normalized,id);

CREATE TABLE party_revisions (
    tenant_id uuid NOT NULL,
    party_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    event_kind text NOT NULL CHECK (event_kind IN ('created','profile_updated','status_changed')),
    command_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('organization','individual')),
    legal_name text NOT NULL,
    registration_identifier text,
    status text NOT NULL CHECK (status IN ('active','inactive')),
    reason text NOT NULL,
    actor_ref text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,party_id,revision),
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,party_id) REFERENCES parties(tenant_id,id) ON DELETE RESTRICT,
    CHECK (legal_name=btrim(legal_name) AND char_length(legal_name) BETWEEN 1 AND 200),
    CHECK (registration_identifier IS NULL OR (registration_identifier=btrim(registration_identifier) AND char_length(registration_identifier) BETWEEN 1 AND 100)),
    CHECK (reason=btrim(reason) AND char_length(reason) <= 1000),
    CHECK (actor_ref=btrim(actor_ref) AND char_length(actor_ref) BETWEEN 1 AND 256)
);

CREATE TABLE party_commands (
    tenant_id uuid NOT NULL,
    command_id uuid NOT NULL,
    request_hash text NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    actor_subject uuid NOT NULL,
    response_json jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY(tenant_id,command_id)
);

CREATE TABLE party_audit_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    party_id uuid NOT NULL,
    command_id uuid NOT NULL,
    source_revision bigint NOT NULL CHECK (source_revision >= 0),
    result_revision bigint NOT NULL CHECK (result_revision > source_revision),
    action text NOT NULL CHECK (action IN ('party_created','party_profile_updated','party_status_changed')),
    reason text NOT NULL,
    actor_subject uuid NOT NULL,
    actor_kind text NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    UNIQUE(tenant_id,command_id),
    FOREIGN KEY(tenant_id,party_id,result_revision) REFERENCES party_revisions(tenant_id,party_id,revision) ON DELETE RESTRICT
);

CREATE FUNCTION ensure_party_current_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    current_row parties%ROWTYPE;
    latest_row party_revisions%ROWTYPE;
    target_tenant uuid;
    target_party uuid;
BEGIN
    target_tenant := NEW.tenant_id;
    target_party := COALESCE((to_jsonb(NEW)->>'id')::uuid, (to_jsonb(NEW)->>'party_id')::uuid);
    SELECT * INTO current_row FROM parties WHERE tenant_id=target_tenant AND id=target_party;
    SELECT * INTO latest_row FROM party_revisions WHERE tenant_id=target_tenant AND party_id=target_party ORDER BY revision DESC LIMIT 1;
    IF NOT FOUND OR current_row.revision IS DISTINCT FROM latest_row.revision
       OR current_row.kind IS DISTINCT FROM latest_row.kind
       OR current_row.legal_name IS DISTINCT FROM latest_row.legal_name
       OR current_row.registration_identifier IS DISTINCT FROM latest_row.registration_identifier
       OR current_row.status IS DISTINCT FROM latest_row.status THEN
        RAISE EXCEPTION 'party current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER parties_current_consistency
    AFTER INSERT OR UPDATE ON parties DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION ensure_party_current_matches_latest_revision();

CREATE CONSTRAINT TRIGGER party_revisions_current_consistency
    AFTER INSERT ON party_revisions DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION ensure_party_current_matches_latest_revision();

CREATE FUNCTION reject_party_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'immutable party record'; END;
$$;
CREATE TRIGGER party_revisions_immutable BEFORE UPDATE OR DELETE ON party_revisions FOR EACH ROW EXECUTE FUNCTION reject_party_immutable_change();
CREATE TRIGGER party_commands_immutable BEFORE UPDATE OR DELETE ON party_commands FOR EACH ROW EXECUTE FUNCTION reject_party_immutable_change();
CREATE TRIGGER party_audit_events_immutable BEFORE UPDATE OR DELETE ON party_audit_events FOR EACH ROW EXECUTE FUNCTION reject_party_immutable_change();
