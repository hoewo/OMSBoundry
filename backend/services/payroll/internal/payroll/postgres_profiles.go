package payroll

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) UpsertCompensation(ctx context.Context, actor Actor, in UpsertCompensationProfileInput, person PersonReference, requestHash string) (CompensationProfile, error) {
	tx, err := s.begin(ctx, actor.TenantID, in.CommandID)
	if err != nil {
		return CompensationProfile{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, err := replayValue[CompensationProfile](ctx, tx, actor, in.CommandID, requestHash); err != nil || found {
		return value, err
	}
	current, err := loadCompensation(ctx, tx, actor.TenantID, in.PersonID, "FOR UPDATE")
	creating := errors.Is(err, ErrProfileNotFound)
	if err != nil && !creating {
		return CompensationProfile{}, err
	}
	if (creating && in.ExpectedRevision != 0) || (!creating && current.Revision != in.ExpectedRevision) {
		return CompensationProfile{}, ErrRevisionConflict
	}
	now, sourceRevision := time.Now().UTC(), int64(0)
	if creating {
		current = CompensationProfile{ID: newUUID(), TenantID: actor.TenantID, PersonID: in.PersonID, CreatedAt: now}
		current.Revision = 1
	} else {
		sourceRevision = current.Revision
		current.Revision++
	}
	current.PersonDisplayName, current.PersonSourceRevision, current.EffectiveMonth = person.DisplayName, person.Revision, in.EffectiveMonth
	current.FixedSalaryMinor, current.SocialInsuranceBaseMinor, current.HousingFundBaseMinor = in.FixedSalaryMinor, in.SocialInsuranceBaseMinor, in.HousingFundBaseMinor
	current.Currency, current.Reason, current.UpdatedAt = "CNY", in.Reason, now
	if creating {
		_, err = tx.Exec(ctx, `INSERT INTO compensation_profiles(id,tenant_id,person_id,person_display_name,person_source_revision,effective_month,fixed_salary_minor,social_insurance_base_minor,housing_fund_base_minor,currency,reason,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`, current.ID, current.TenantID, current.PersonID, current.PersonDisplayName, current.PersonSourceRevision, current.EffectiveMonth, current.FixedSalaryMinor, current.SocialInsuranceBaseMinor, current.HousingFundBaseMinor, current.Currency, current.Reason, current.Revision, now)
	} else {
		_, err = tx.Exec(ctx, `UPDATE compensation_profiles SET person_display_name=$4,person_source_revision=$5,effective_month=$6,fixed_salary_minor=$7,social_insurance_base_minor=$8,housing_fund_base_minor=$9,reason=$10,revision=$11,updated_at=$12 WHERE tenant_id=$1 AND person_id=$2 AND revision=$3`, current.TenantID, current.PersonID, in.ExpectedRevision, current.PersonDisplayName, current.PersonSourceRevision, current.EffectiveMonth, current.FixedSalaryMinor, current.SocialInsuranceBaseMinor, current.HousingFundBaseMinor, current.Reason, current.Revision, now)
	}
	if err != nil {
		return CompensationProfile{}, err
	}
	event := "revised"
	if creating {
		event = "created"
	}
	_, err = tx.Exec(ctx, `INSERT INTO compensation_profile_revisions(tenant_id,profile_id,person_id,revision,command_id,event_kind,person_display_name,person_source_revision,effective_month,fixed_salary_minor,social_insurance_base_minor,housing_fund_base_minor,currency,reason,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, current.TenantID, current.ID, current.PersonID, current.Revision, in.CommandID, event, current.PersonDisplayName, current.PersonSourceRevision, current.EffectiveMonth, current.FixedSalaryMinor, current.SocialInsuranceBaseMinor, current.HousingFundBaseMinor, current.Currency, current.Reason, actor.ID, actor.CredentialID, now)
	if err != nil {
		return CompensationProfile{}, err
	}
	if err = insertProfileAudit(ctx, tx, actor, "compensation_profile", current.ID, current.PersonID, in.CommandID, sourceRevision, current.Revision, "compensation_profile_"+event, map[string]any{"effective_month": current.EffectiveMonth}); err != nil {
		return CompensationProfile{}, err
	}
	if err = recordValue(ctx, tx, actor, in.CommandID, requestHash, current, now); err != nil {
		return CompensationProfile{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CompensationProfile{}, err
	}
	return current, nil
}

func (s *PostgresStore) GetCompensation(ctx context.Context, tenant, person string) (CompensationProfile, error) {
	return loadCompensation(ctx, s.pool, tenant, person, "")
}
func (s *PostgresStore) ListCompensations(ctx context.Context, tenant string) ([]CompensationProfile, error) {
	rows, err := s.pool.Query(ctx, compensationSelect+` WHERE tenant_id=$1 ORDER BY lower(person_display_name),person_id`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []CompensationProfile{}
	for rows.Next() {
		v, e := scanCompensation(rows)
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

const compensationSelect = `SELECT id::text,tenant_id::text,person_id::text,person_display_name,person_source_revision,effective_month,fixed_salary_minor,social_insurance_base_minor,housing_fund_base_minor,currency,reason,revision,created_at,updated_at FROM compensation_profiles`

func loadCompensation(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, person, suffix string) (CompensationProfile, error) {
	v, e := scanCompensation(q.QueryRow(ctx, compensationSelect+` WHERE tenant_id=$1 AND person_id=$2 `+suffix, tenant, person))
	if errors.Is(e, pgx.ErrNoRows) {
		return CompensationProfile{}, ErrProfileNotFound
	}
	return v, e
}
func scanCompensation(row scanner) (CompensationProfile, error) {
	var v CompensationProfile
	e := row.Scan(&v.ID, &v.TenantID, &v.PersonID, &v.PersonDisplayName, &v.PersonSourceRevision, &v.EffectiveMonth, &v.FixedSalaryMinor, &v.SocialInsuranceBaseMinor, &v.HousingFundBaseMinor, &v.Currency, &v.Reason, &v.Revision, &v.CreatedAt, &v.UpdatedAt)
	return v, e
}

func (s *PostgresStore) UpsertPayment(ctx context.Context, actor Actor, in UpsertPaymentProfileInput, person PersonReference, requestHash string) (PaymentProfile, error) {
	tx, err := s.begin(ctx, actor.TenantID, in.CommandID)
	if err != nil {
		return PaymentProfile{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, e := replayValue[PaymentProfile](ctx, tx, actor, in.CommandID, requestHash); e != nil || found {
		return value, e
	}
	current, err := loadPayment(ctx, tx, actor.TenantID, in.PersonID, "FOR UPDATE")
	creating := errors.Is(err, ErrProfileNotFound)
	if err != nil && !creating {
		return PaymentProfile{}, err
	}
	if (creating && in.ExpectedRevision != 0) || (!creating && current.Revision != in.ExpectedRevision) {
		return PaymentProfile{}, ErrRevisionConflict
	}
	now, sourceRevision := time.Now().UTC(), int64(0)
	if creating {
		current = PaymentProfile{ID: newUUID(), TenantID: actor.TenantID, PersonID: in.PersonID, CreatedAt: now, Revision: 1}
	} else {
		sourceRevision = current.Revision
		current.Revision++
	}
	current.PersonDisplayName, current.PersonSourceRevision = person.DisplayName, person.Revision
	current.AccountName, current.BankName, current.accountNumber = in.AccountName, in.BankName, in.AccountNumber
	current.MaskedAccountNumber = maskAccount(in.AccountNumber)
	current.Status, current.EffectiveMonth, current.Reason, current.UpdatedAt = in.Status, in.EffectiveMonth, in.Reason, now
	if creating {
		_, err = tx.Exec(ctx, `INSERT INTO payment_profiles(id,tenant_id,person_id,person_display_name,person_source_revision,account_name,bank_name,account_number,status,effective_month,reason,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`, current.ID, current.TenantID, current.PersonID, current.PersonDisplayName, current.PersonSourceRevision, current.AccountName, current.BankName, current.accountNumber, current.Status, current.EffectiveMonth, current.Reason, current.Revision, now)
	} else {
		_, err = tx.Exec(ctx, `UPDATE payment_profiles SET person_display_name=$4,person_source_revision=$5,account_name=$6,bank_name=$7,account_number=$8,status=$9,effective_month=$10,reason=$11,revision=$12,updated_at=$13 WHERE tenant_id=$1 AND person_id=$2 AND revision=$3`, current.TenantID, current.PersonID, in.ExpectedRevision, current.PersonDisplayName, current.PersonSourceRevision, current.AccountName, current.BankName, current.accountNumber, current.Status, current.EffectiveMonth, current.Reason, current.Revision, now)
	}
	if err != nil {
		return PaymentProfile{}, err
	}
	event := "revised"
	if creating {
		event = "created"
	}
	_, err = tx.Exec(ctx, `INSERT INTO payment_profile_revisions(tenant_id,profile_id,person_id,revision,command_id,event_kind,person_display_name,person_source_revision,account_name,bank_name,account_number,status,effective_month,reason,actor_subject_id,credential_id,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, current.TenantID, current.ID, current.PersonID, current.Revision, in.CommandID, event, current.PersonDisplayName, current.PersonSourceRevision, current.AccountName, current.BankName, current.accountNumber, current.Status, current.EffectiveMonth, current.Reason, actor.ID, actor.CredentialID, now)
	if err != nil {
		return PaymentProfile{}, err
	}
	if err = insertProfileAudit(ctx, tx, actor, "payment_profile", current.ID, current.PersonID, in.CommandID, sourceRevision, current.Revision, "payment_profile_"+event, map[string]any{"masked_account_number": current.MaskedAccountNumber, "effective_month": current.EffectiveMonth, "status": current.Status}); err != nil {
		return PaymentProfile{}, err
	}
	if err = recordValue(ctx, tx, actor, in.CommandID, requestHash, current, now); err != nil {
		return PaymentProfile{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PaymentProfile{}, err
	}
	return current, nil
}
func (s *PostgresStore) GetPayment(ctx context.Context, tenant, person string) (PaymentProfile, error) {
	return loadPayment(ctx, s.pool, tenant, person, "")
}
func (s *PostgresStore) ListPayments(ctx context.Context, tenant string) ([]PaymentProfile, error) {
	rows, e := s.pool.Query(ctx, paymentSelect+` WHERE tenant_id=$1 ORDER BY lower(person_display_name),person_id`, tenant)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	values := []PaymentProfile{}
	for rows.Next() {
		v, e := scanPayment(rows)
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}
func (s *PostgresStore) ListPaymentHistory(ctx context.Context, tenant, person string) ([]PaymentProfile, error) {
	rows, e := s.pool.Query(ctx, `SELECT profile_id::text,tenant_id::text,person_id::text,person_display_name,person_source_revision,account_name,bank_name,account_number,status,effective_month,reason,revision,occurred_at,occurred_at FROM payment_profile_revisions WHERE tenant_id=$1 AND person_id=$2 ORDER BY revision`, tenant, person)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	values := []PaymentProfile{}
	for rows.Next() {
		v, e := scanPayment(rows)
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

const paymentSelect = `SELECT id::text,tenant_id::text,person_id::text,person_display_name,person_source_revision,account_name,bank_name,account_number,status,effective_month,reason,revision,created_at,updated_at FROM payment_profiles`

func loadPayment(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenant, person, suffix string) (PaymentProfile, error) {
	v, e := scanPayment(q.QueryRow(ctx, paymentSelect+` WHERE tenant_id=$1 AND person_id=$2 `+suffix, tenant, person))
	if errors.Is(e, pgx.ErrNoRows) {
		return PaymentProfile{}, ErrProfileNotFound
	}
	return v, e
}
func scanPayment(row scanner) (PaymentProfile, error) {
	var v PaymentProfile
	e := row.Scan(&v.ID, &v.TenantID, &v.PersonID, &v.PersonDisplayName, &v.PersonSourceRevision, &v.AccountName, &v.BankName, &v.accountNumber, &v.Status, &v.EffectiveMonth, &v.Reason, &v.Revision, &v.CreatedAt, &v.UpdatedAt)
	v.MaskedAccountNumber = maskAccount(v.accountNumber)
	return v, e
}
func maskAccount(value string) string {
	if len(value) <= 4 {
		return "****" + value
	}
	return "**** **** **** " + value[len(value)-4:]
}
func (s *PostgresStore) ReadSensitivePayment(ctx context.Context, actor Actor, person, purpose string) (SensitivePaymentProfile, error) {
	tx, e := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return SensitivePaymentProfile{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, e := loadPayment(ctx, tx, actor.TenantID, person, "")
	if e != nil {
		return SensitivePaymentProfile{}, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO payroll_sensitive_account_access(id,tenant_id,payment_profile_id,payment_profile_revision,person_id,actor_subject_id,credential_id,purpose,result,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'allowed',$9)`, newUUID(), actor.TenantID, value.ID, value.Revision, value.PersonID, actor.ID, actor.CredentialID, purpose, time.Now().UTC())
	if e != nil {
		return SensitivePaymentProfile{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return SensitivePaymentProfile{}, e
	}
	return SensitivePaymentProfile{PaymentProfile: value, AccountNumber: value.accountNumber}, nil
}

func (s *PostgresStore) CreateStandardScheme(ctx context.Context, actor Actor, in CreateStandardSchemeInput, requestHash string) (ContributionScheme, error) {
	tx, e := s.begin(ctx, actor.TenantID, in.CommandID)
	if e != nil {
		return ContributionScheme{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, e := replayValue[ContributionScheme](ctx, tx, actor, in.CommandID, requestHash); e != nil || found {
		return value, e
	}
	now := time.Now().UTC()
	value := ContributionScheme{ID: newUUID(), SchemeKind: "standard", Title: in.Title, RegionCode: in.RegionCode, RegionName: in.RegionName, EffectiveMonth: in.EffectiveMonth, AuthoritySource: in.AuthoritySource, AuthorityVerifiedAt: in.AuthorityVerifiedAt.UTC(), Components: in.Components, Reason: in.Reason, Revision: 1, CreatedAt: now, UpdatedAt: now}
	raw, _ := json.Marshal(value.Components)
	_, e = tx.Exec(ctx, `INSERT INTO contribution_schemes(id,tenant_id,scheme_kind,title,region_code,region_name,effective_month,authority_source,authority_verified_at,components,reason,revision,created_at,updated_at) VALUES($1,NULL,'standard',$2,$3,$4,$5,$6,$7,$8,$9,1,$10,$10)`, value.ID, value.Title, value.RegionCode, value.RegionName, value.EffectiveMonth, value.AuthoritySource, value.AuthorityVerifiedAt, raw, value.Reason, now)
	if e != nil {
		return ContributionScheme{}, e
	}
	if e = insertSchemeRevision(ctx, tx, actor, value, in.CommandID, raw); e != nil {
		return ContributionScheme{}, e
	}
	if e = insertProfileAudit(ctx, tx, actor, "contribution_scheme", value.ID, "", in.CommandID, 0, 1, "standard_scheme_created", map[string]any{"region_code": value.RegionCode, "effective_month": value.EffectiveMonth, "authority_source": value.AuthoritySource}); e != nil {
		return ContributionScheme{}, e
	}
	if e = recordValue(ctx, tx, actor, in.CommandID, requestHash, value, now); e != nil {
		return ContributionScheme{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return ContributionScheme{}, e
	}
	return value, nil
}
func (s *PostgresStore) CreateCustomScheme(ctx context.Context, actor Actor, in CreateCustomSchemeInput, requestHash string) (ContributionScheme, error) {
	tx, e := s.begin(ctx, actor.TenantID, in.CommandID)
	if e != nil {
		return ContributionScheme{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, e := replayValue[ContributionScheme](ctx, tx, actor, in.CommandID, requestHash); e != nil || found {
		return value, e
	}
	source, e := loadSchemeRevision(ctx, tx, in.SourceSchemeID, in.SourceSchemeRevision)
	if e != nil || source.SchemeKind != "standard" {
		return ContributionScheme{}, ErrProfileNotFound
	}
	components := in.Components
	if len(components) == 0 {
		components = source.Components
	}
	now := time.Now().UTC()
	value := ContributionScheme{ID: newUUID(), TenantID: actor.TenantID, SchemeKind: "custom", SourceSchemeID: source.ID, SourceSchemeRevision: source.Revision, Title: in.Title, RegionCode: source.RegionCode, RegionName: source.RegionName, EffectiveMonth: in.EffectiveMonth, AuthoritySource: source.AuthoritySource, AuthorityVerifiedAt: source.AuthorityVerifiedAt, Components: components, Reason: in.Reason, Revision: 1, CreatedAt: now, UpdatedAt: now}
	raw, _ := json.Marshal(value.Components)
	_, e = tx.Exec(ctx, `INSERT INTO contribution_schemes(id,tenant_id,scheme_kind,source_scheme_id,source_scheme_revision,title,region_code,region_name,effective_month,authority_source,authority_verified_at,components,reason,revision,created_at,updated_at) VALUES($1,$2,'custom',$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,1,$13,$13)`, value.ID, value.TenantID, value.SourceSchemeID, value.SourceSchemeRevision, value.Title, value.RegionCode, value.RegionName, value.EffectiveMonth, value.AuthoritySource, value.AuthorityVerifiedAt, raw, value.Reason, now)
	if e != nil {
		return ContributionScheme{}, e
	}
	if e = insertSchemeRevision(ctx, tx, actor, value, in.CommandID, raw); e != nil {
		return ContributionScheme{}, e
	}
	if e = insertProfileAudit(ctx, tx, actor, "contribution_scheme", value.ID, "", in.CommandID, 0, 1, "custom_scheme_created", map[string]any{"source_scheme_id": value.SourceSchemeID, "source_scheme_revision": value.SourceSchemeRevision}); e != nil {
		return ContributionScheme{}, e
	}
	if e = recordValue(ctx, tx, actor, in.CommandID, requestHash, value, now); e != nil {
		return ContributionScheme{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return ContributionScheme{}, e
	}
	return value, nil
}
func (s *PostgresStore) ReviseCustomScheme(ctx context.Context, actor Actor, schemeID string, in ReviseCustomSchemeInput, requestHash string) (ContributionScheme, error) {
	tx, e := s.begin(ctx, actor.TenantID, in.CommandID)
	if e != nil {
		return ContributionScheme{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if value, found, e := replayValue[ContributionScheme](ctx, tx, actor, in.CommandID, requestHash); e != nil || found {
		return value, e
	}
	current, e := scanScheme(tx.QueryRow(ctx, schemeSelect+` WHERE id=$1 AND tenant_id=$2 AND scheme_kind='custom' FOR UPDATE`, schemeID, actor.TenantID))
	if errors.Is(e, pgx.ErrNoRows) {
		return ContributionScheme{}, ErrProfileNotFound
	}
	if e != nil {
		return ContributionScheme{}, e
	}
	if current.Revision != in.ExpectedRevision {
		return ContributionScheme{}, ErrRevisionConflict
	}
	now, sourceRevision := time.Now().UTC(), current.Revision
	current.Title, current.EffectiveMonth, current.Components, current.Reason = in.Title, in.EffectiveMonth, in.Components, in.Reason
	current.Revision, current.UpdatedAt = sourceRevision+1, now
	raw, e := json.Marshal(current.Components)
	if e != nil {
		return ContributionScheme{}, e
	}
	result, e := tx.Exec(ctx, `UPDATE contribution_schemes SET title=$4,effective_month=$5,components=$6,reason=$7,revision=$8,updated_at=$9 WHERE id=$1 AND tenant_id=$2 AND revision=$3 AND scheme_kind='custom'`, current.ID, actor.TenantID, in.ExpectedRevision, current.Title, current.EffectiveMonth, raw, current.Reason, current.Revision, now)
	if e != nil {
		return ContributionScheme{}, e
	}
	if result.RowsAffected() != 1 {
		return ContributionScheme{}, ErrRevisionConflict
	}
	if e = insertSchemeRevision(ctx, tx, actor, current, in.CommandID, raw); e != nil {
		return ContributionScheme{}, e
	}
	if e = insertProfileAudit(ctx, tx, actor, "contribution_scheme", current.ID, "", in.CommandID, sourceRevision, current.Revision, "custom_scheme_revised", map[string]any{"source_scheme_id": current.SourceSchemeID, "source_scheme_revision": current.SourceSchemeRevision, "effective_month": current.EffectiveMonth}); e != nil {
		return ContributionScheme{}, e
	}
	if e = recordValue(ctx, tx, actor, in.CommandID, requestHash, current, now); e != nil {
		return ContributionScheme{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return ContributionScheme{}, e
	}
	return current, nil
}
func (s *PostgresStore) ListSchemes(ctx context.Context, tenant string) ([]ContributionScheme, error) {
	rows, e := s.pool.Query(ctx, schemeSelect+` WHERE tenant_id IS NULL OR tenant_id=$1 ORDER BY scheme_kind DESC,region_code,effective_month DESC,id`, tenant)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	values := []ContributionScheme{}
	for rows.Next() {
		v, e := scanScheme(rows)
		if e != nil {
			return nil, e
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

const schemeSelect = `SELECT id::text,COALESCE(tenant_id::text,''),scheme_kind,COALESCE(source_scheme_id::text,''),COALESCE(source_scheme_revision,0),title,region_code,region_name,effective_month,authority_source,authority_verified_at,components,reason,revision,created_at,updated_at FROM contribution_schemes`

func scanScheme(row scanner) (ContributionScheme, error) {
	var v ContributionScheme
	var raw []byte
	e := row.Scan(&v.ID, &v.TenantID, &v.SchemeKind, &v.SourceSchemeID, &v.SourceSchemeRevision, &v.Title, &v.RegionCode, &v.RegionName, &v.EffectiveMonth, &v.AuthoritySource, &v.AuthorityVerifiedAt, &raw, &v.Reason, &v.Revision, &v.CreatedAt, &v.UpdatedAt)
	if e == nil {
		e = json.Unmarshal(raw, &v.Components)
	}
	return v, e
}
func loadSchemeRevision(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string, revision int64) (ContributionScheme, error) {
	var v ContributionScheme
	var raw []byte
	e := q.QueryRow(ctx, `SELECT scheme_id::text,COALESCE(tenant_id::text,''),scheme_kind,COALESCE(source_scheme_id::text,''),COALESCE(source_scheme_revision,0),title,region_code,region_name,effective_month,authority_source,authority_verified_at,components,reason,revision,occurred_at,occurred_at FROM contribution_scheme_revisions WHERE scheme_id=$1 AND revision=$2`, id, revision).Scan(&v.ID, &v.TenantID, &v.SchemeKind, &v.SourceSchemeID, &v.SourceSchemeRevision, &v.Title, &v.RegionCode, &v.RegionName, &v.EffectiveMonth, &v.AuthoritySource, &v.AuthorityVerifiedAt, &raw, &v.Reason, &v.Revision, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return ContributionScheme{}, ErrProfileNotFound
	}
	if e == nil {
		e = json.Unmarshal(raw, &v.Components)
	}
	return v, e
}
func insertSchemeRevision(ctx context.Context, tx pgx.Tx, actor Actor, v ContributionScheme, command string, raw []byte) error {
	_, e := tx.Exec(ctx, `INSERT INTO contribution_scheme_revisions(scheme_id,tenant_id,revision,command_id,scheme_kind,source_scheme_id,source_scheme_revision,title,region_code,region_name,effective_month,authority_source,authority_verified_at,components,reason,actor_subject_id,credential_id,occurred_at) VALUES($1,NULLIF($2,'')::uuid,$3,$4,$5,NULLIF($6,'')::uuid,NULLIF($7,0),$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, v.ID, v.TenantID, v.Revision, command, v.SchemeKind, v.SourceSchemeID, v.SourceSchemeRevision, v.Title, v.RegionCode, v.RegionName, v.EffectiveMonth, v.AuthoritySource, v.AuthorityVerifiedAt, raw, v.Reason, actor.ID, actor.CredentialID, v.UpdatedAt)
	return e
}

func replayValue[T any](ctx context.Context, tx pgx.Tx, actor Actor, command, requestHash string) (T, bool, error) {
	var zero T
	var stored, storedActor string
	var raw []byte
	e := tx.QueryRow(ctx, `SELECT request_hash,actor_subject_id::text,response_json FROM payroll_commands WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, command).Scan(&stored, &storedActor, &raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return zero, false, nil
	}
	if e != nil {
		return zero, false, e
	}
	if stored != requestHash || storedActor != actor.ID {
		return zero, false, ErrCommandConflict
	}
	var value T
	if e = json.Unmarshal(raw, &value); e != nil {
		return zero, false, e
	}
	return value, true, nil
}
func recordValue(ctx context.Context, tx pgx.Tx, actor Actor, command, requestHash string, value any, at time.Time) error {
	raw, e := json.Marshal(value)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO payroll_commands(tenant_id,command_id,request_hash,actor_subject_id,response_json,created_at) VALUES($1,$2,$3,$4,$5,$6)`, actor.TenantID, command, requestHash, actor.ID, raw, at)
	return e
}
func insertProfileAudit(ctx context.Context, tx pgx.Tx, actor Actor, resourceType, resourceID, personID, command string, source, result int64, action string, details any) error {
	raw, e := json.Marshal(details)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO payroll_profile_audit_events(id,tenant_id,resource_type,resource_id,person_id,command_id,source_revision,result_revision,action,actor_subject_id,credential_id,details,occurred_at) VALUES($1,NULLIF($2,'')::uuid,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8,$9,$10,$11,$12,$13)`, newUUID(), actor.TenantID, resourceType, resourceID, personID, command, source, result, action, actor.ID, actor.CredentialID, raw, time.Now().UTC())
	return e
}

var _ ProfileStore = (*PostgresStore)(nil)
