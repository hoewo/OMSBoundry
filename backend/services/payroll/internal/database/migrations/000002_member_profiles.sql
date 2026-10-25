CREATE TABLE compensation_profiles (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    person_id uuid NOT NULL,
    person_display_name text NOT NULL,
    person_source_revision bigint NOT NULL CHECK (person_source_revision >= 1),
    effective_month char(7) NOT NULL CHECK (effective_month ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    fixed_salary_minor bigint NOT NULL CHECK (fixed_salary_minor >= 0),
    social_insurance_base_minor bigint NOT NULL CHECK (social_insurance_base_minor >= 0),
    housing_fund_base_minor bigint NOT NULL CHECK (housing_fund_base_minor >= 0),
    currency char(3) NOT NULL CHECK (currency = 'CNY'),
    reason text NOT NULL CHECK (reason=btrim(reason) AND char_length(reason) BETWEEN 1 AND 500),
    revision bigint NOT NULL CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id),
    UNIQUE (tenant_id,person_id)
);

CREATE TABLE compensation_profile_revisions (
    tenant_id uuid NOT NULL,
    profile_id uuid NOT NULL,
    person_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    command_id uuid NOT NULL,
    event_kind text NOT NULL CHECK (event_kind IN ('created','revised')),
    person_display_name text NOT NULL,
    person_source_revision bigint NOT NULL CHECK (person_source_revision >= 1),
    effective_month char(7) NOT NULL,
    fixed_salary_minor bigint NOT NULL,
    social_insurance_base_minor bigint NOT NULL,
    housing_fund_base_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    reason text NOT NULL,
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,profile_id,revision),
    UNIQUE (tenant_id,command_id),
    FOREIGN KEY (tenant_id,profile_id) REFERENCES compensation_profiles(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX compensation_profile_effective_idx ON compensation_profile_revisions(tenant_id,person_id,effective_month DESC,revision DESC);

CREATE TABLE payment_profiles (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    person_id uuid NOT NULL,
    person_display_name text NOT NULL,
    person_source_revision bigint NOT NULL CHECK (person_source_revision >= 1),
    account_name text NOT NULL CHECK (account_name=btrim(account_name) AND char_length(account_name) BETWEEN 1 AND 100),
    bank_name text NOT NULL CHECK (bank_name=btrim(bank_name) AND char_length(bank_name) BETWEEN 1 AND 160),
    account_number text NOT NULL CHECK (account_number ~ '^[0-9]{4,32}$'),
    status text NOT NULL CHECK (status IN ('active','inactive')),
    effective_month char(7) NOT NULL CHECK (effective_month ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    reason text NOT NULL CHECK (reason=btrim(reason) AND char_length(reason) BETWEEN 1 AND 500),
    revision bigint NOT NULL CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (tenant_id,id),
    UNIQUE (tenant_id,person_id)
);

CREATE TABLE payment_profile_revisions (
    tenant_id uuid NOT NULL,
    profile_id uuid NOT NULL,
    person_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    command_id uuid NOT NULL,
    event_kind text NOT NULL CHECK (event_kind IN ('created','revised')),
    person_display_name text NOT NULL,
    person_source_revision bigint NOT NULL CHECK (person_source_revision >= 1),
    account_name text NOT NULL,
    bank_name text NOT NULL,
    account_number text NOT NULL,
    status text NOT NULL,
    effective_month char(7) NOT NULL,
    reason text NOT NULL,
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id,profile_id,revision),
    UNIQUE (tenant_id,command_id),
    FOREIGN KEY (tenant_id,profile_id) REFERENCES payment_profiles(tenant_id,id) ON DELETE RESTRICT
);
CREATE INDEX payment_profile_effective_idx ON payment_profile_revisions(tenant_id,person_id,effective_month DESC,revision DESC);

CREATE TABLE contribution_schemes (
    id uuid PRIMARY KEY,
    tenant_id uuid NULL,
    scheme_kind text NOT NULL CHECK (scheme_kind IN ('standard','custom')),
    source_scheme_id uuid NULL,
    source_scheme_revision bigint NULL,
    title text NOT NULL CHECK (title=btrim(title) AND char_length(title) BETWEEN 1 AND 160),
    region_code text NOT NULL CHECK (region_code=btrim(region_code) AND char_length(region_code) BETWEEN 2 AND 32),
    region_name text NOT NULL CHECK (region_name=btrim(region_name) AND char_length(region_name) BETWEEN 1 AND 100),
    effective_month char(7) NOT NULL CHECK (effective_month ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    authority_source text NOT NULL CHECK (authority_source=btrim(authority_source) AND char_length(authority_source) BETWEEN 1 AND 500),
    authority_verified_at timestamptz NOT NULL,
    components jsonb NOT NULL CHECK (jsonb_typeof(components)='array' AND jsonb_array_length(components)=6),
    reason text NOT NULL CHECK (reason=btrim(reason) AND char_length(reason) BETWEEN 1 AND 500),
    revision bigint NOT NULL CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK ((scheme_kind='standard' AND tenant_id IS NULL AND source_scheme_id IS NULL AND source_scheme_revision IS NULL)
        OR (scheme_kind='custom' AND tenant_id IS NOT NULL AND source_scheme_id IS NOT NULL AND source_scheme_revision >= 1)),
    UNIQUE NULLS NOT DISTINCT (tenant_id,id)
);
CREATE INDEX contribution_schemes_lookup_idx ON contribution_schemes(scheme_kind,tenant_id,region_code,effective_month DESC,id);

CREATE TABLE contribution_scheme_revisions (
    scheme_id uuid NOT NULL,
    tenant_id uuid NULL,
    revision bigint NOT NULL CHECK (revision >= 1),
    command_id uuid NOT NULL,
    scheme_kind text NOT NULL,
    source_scheme_id uuid NULL,
    source_scheme_revision bigint NULL,
    title text NOT NULL,
    region_code text NOT NULL,
    region_name text NOT NULL,
    effective_month char(7) NOT NULL,
    authority_source text NOT NULL,
    authority_verified_at timestamptz NOT NULL,
    components jsonb NOT NULL,
    reason text NOT NULL,
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (scheme_id,revision),
    UNIQUE (command_id)
);

CREATE TABLE payroll_profile_audit_events (
    id uuid PRIMARY KEY,
    tenant_id uuid NULL,
    resource_type text NOT NULL CHECK (resource_type IN ('compensation_profile','payment_profile','contribution_scheme')),
    resource_id uuid NOT NULL,
    person_id uuid NULL,
    command_id uuid NULL,
    source_revision bigint NOT NULL CHECK (source_revision >= 0),
    result_revision bigint NOT NULL CHECK (result_revision >= source_revision),
    action text NOT NULL,
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details)='object'),
    occurred_at timestamptz NOT NULL
);
CREATE INDEX payroll_profile_audit_timeline_idx ON payroll_profile_audit_events(tenant_id,occurred_at DESC,id);

CREATE TABLE payroll_sensitive_account_access (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    payment_profile_id uuid NOT NULL,
    payment_profile_revision bigint NOT NULL,
    person_id uuid NOT NULL,
    actor_subject_id uuid NOT NULL,
    credential_id text NOT NULL,
    purpose text NOT NULL CHECK (purpose=btrim(purpose) AND char_length(purpose) BETWEEN 1 AND 300),
    result text NOT NULL CHECK (result IN ('allowed','denied')),
    occurred_at timestamptz NOT NULL
);
CREATE INDEX payroll_sensitive_account_access_idx ON payroll_sensitive_account_access(tenant_id,person_id,occurred_at DESC,id);

CREATE FUNCTION ensure_compensation_profile_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE current_row compensation_profiles%ROWTYPE; latest_row compensation_profile_revisions%ROWTYPE; target_tenant uuid; target_profile uuid;
BEGIN
    target_tenant := NEW.tenant_id;
    target_profile := COALESCE((to_jsonb(NEW)->>'id')::uuid,(to_jsonb(NEW)->>'profile_id')::uuid);
    SELECT * INTO current_row FROM compensation_profiles WHERE tenant_id=target_tenant AND id=target_profile;
    SELECT * INTO latest_row FROM compensation_profile_revisions WHERE tenant_id=target_tenant AND profile_id=target_profile ORDER BY revision DESC LIMIT 1;
    IF NOT FOUND OR current_row.revision IS DISTINCT FROM latest_row.revision OR current_row.person_id IS DISTINCT FROM latest_row.person_id
       OR current_row.person_display_name IS DISTINCT FROM latest_row.person_display_name
       OR current_row.person_source_revision IS DISTINCT FROM latest_row.person_source_revision
       OR current_row.effective_month IS DISTINCT FROM latest_row.effective_month OR current_row.fixed_salary_minor IS DISTINCT FROM latest_row.fixed_salary_minor
       OR current_row.social_insurance_base_minor IS DISTINCT FROM latest_row.social_insurance_base_minor OR current_row.housing_fund_base_minor IS DISTINCT FROM latest_row.housing_fund_base_minor
       OR current_row.currency IS DISTINCT FROM latest_row.currency OR current_row.reason IS DISTINCT FROM latest_row.reason THEN
        RAISE EXCEPTION 'compensation profile current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;
CREATE CONSTRAINT TRIGGER compensation_profiles_current_consistency AFTER INSERT OR UPDATE ON compensation_profiles DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_compensation_profile_matches_latest_revision();
CREATE CONSTRAINT TRIGGER compensation_profile_revisions_current_consistency AFTER INSERT ON compensation_profile_revisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_compensation_profile_matches_latest_revision();

CREATE FUNCTION ensure_payment_profile_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE current_row payment_profiles%ROWTYPE; latest_row payment_profile_revisions%ROWTYPE; target_tenant uuid; target_profile uuid;
BEGIN
    target_tenant := NEW.tenant_id;
    target_profile := COALESCE((to_jsonb(NEW)->>'id')::uuid,(to_jsonb(NEW)->>'profile_id')::uuid);
    SELECT * INTO current_row FROM payment_profiles WHERE tenant_id=target_tenant AND id=target_profile;
    SELECT * INTO latest_row FROM payment_profile_revisions WHERE tenant_id=target_tenant AND profile_id=target_profile ORDER BY revision DESC LIMIT 1;
    IF NOT FOUND OR current_row.revision IS DISTINCT FROM latest_row.revision OR current_row.person_id IS DISTINCT FROM latest_row.person_id
       OR current_row.person_display_name IS DISTINCT FROM latest_row.person_display_name
       OR current_row.person_source_revision IS DISTINCT FROM latest_row.person_source_revision
       OR current_row.account_name IS DISTINCT FROM latest_row.account_name OR current_row.bank_name IS DISTINCT FROM latest_row.bank_name
       OR current_row.account_number IS DISTINCT FROM latest_row.account_number OR current_row.status IS DISTINCT FROM latest_row.status
       OR current_row.effective_month IS DISTINCT FROM latest_row.effective_month OR current_row.reason IS DISTINCT FROM latest_row.reason THEN
        RAISE EXCEPTION 'payment profile current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;
CREATE CONSTRAINT TRIGGER payment_profiles_current_consistency AFTER INSERT OR UPDATE ON payment_profiles DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_payment_profile_matches_latest_revision();
CREATE CONSTRAINT TRIGGER payment_profile_revisions_current_consistency AFTER INSERT ON payment_profile_revisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_payment_profile_matches_latest_revision();

CREATE FUNCTION ensure_contribution_scheme_matches_latest_revision() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE current_row contribution_schemes%ROWTYPE; latest_row contribution_scheme_revisions%ROWTYPE; target_scheme uuid;
BEGIN
    target_scheme := COALESCE((to_jsonb(NEW)->>'id')::uuid,(to_jsonb(NEW)->>'scheme_id')::uuid);
    SELECT * INTO current_row FROM contribution_schemes WHERE id=target_scheme;
    SELECT * INTO latest_row FROM contribution_scheme_revisions WHERE scheme_id=target_scheme ORDER BY revision DESC LIMIT 1;
    IF NOT FOUND OR current_row.revision IS DISTINCT FROM latest_row.revision OR current_row.tenant_id IS DISTINCT FROM latest_row.tenant_id
       OR current_row.scheme_kind IS DISTINCT FROM latest_row.scheme_kind OR current_row.source_scheme_id IS DISTINCT FROM latest_row.source_scheme_id
       OR current_row.source_scheme_revision IS DISTINCT FROM latest_row.source_scheme_revision OR current_row.title IS DISTINCT FROM latest_row.title
       OR current_row.region_code IS DISTINCT FROM latest_row.region_code OR current_row.region_name IS DISTINCT FROM latest_row.region_name
       OR current_row.effective_month IS DISTINCT FROM latest_row.effective_month OR current_row.components IS DISTINCT FROM latest_row.components
       OR current_row.authority_source IS DISTINCT FROM latest_row.authority_source
       OR current_row.authority_verified_at IS DISTINCT FROM latest_row.authority_verified_at OR current_row.reason IS DISTINCT FROM latest_row.reason THEN
        RAISE EXCEPTION 'contribution scheme current projection does not match latest revision';
    END IF;
    RETURN NEW;
END;
$$;
CREATE CONSTRAINT TRIGGER contribution_schemes_current_consistency AFTER INSERT OR UPDATE ON contribution_schemes DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_contribution_scheme_matches_latest_revision();
CREATE CONSTRAINT TRIGGER contribution_scheme_revisions_current_consistency AFTER INSERT ON contribution_scheme_revisions DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ensure_contribution_scheme_matches_latest_revision();

CREATE TRIGGER compensation_profile_revisions_immutable BEFORE UPDATE OR DELETE ON compensation_profile_revisions FOR EACH ROW EXECUTE FUNCTION reject_payroll_immutable_change();
CREATE TRIGGER payment_profile_revisions_immutable BEFORE UPDATE OR DELETE ON payment_profile_revisions FOR EACH ROW EXECUTE FUNCTION reject_payroll_immutable_change();
CREATE TRIGGER contribution_scheme_revisions_immutable BEFORE UPDATE OR DELETE ON contribution_scheme_revisions FOR EACH ROW EXECUTE FUNCTION reject_payroll_immutable_change();
CREATE TRIGGER payroll_profile_audit_immutable BEFORE UPDATE OR DELETE ON payroll_profile_audit_events FOR EACH ROW EXECUTE FUNCTION reject_payroll_immutable_change();
CREATE TRIGGER payroll_sensitive_account_access_immutable BEFORE UPDATE OR DELETE ON payroll_sensitive_account_access FOR EACH ROW EXECUTE FUNCTION reject_payroll_immutable_change();
