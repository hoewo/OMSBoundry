package payroll

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var accountNumberPattern = regexp.MustCompile(`^[0-9]{4,32}$`)
var sourceDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

var (
	ErrProfileNotFound      = errors.New("payroll profile not found")
	ErrReferenceInvalid     = errors.New("person reference is invalid")
	ErrReferenceUnavailable = errors.New("person reference is unavailable")
)

type PersonReference struct {
	ID, TenantID, ReferenceCode, DisplayName, Status, Digest string
	Revision                                                 int64
}

type PersonReferencePort interface {
	ValidatePerson(context.Context, string, string) (PersonReference, error)
	ResolvePersonForSubject(context.Context, string, string) (PersonReference, error)
}

type CompensationProfile struct {
	ID                       string    `json:"id"`
	TenantID                 string    `json:"tenant_id"`
	PersonID                 string    `json:"person_id"`
	PersonDisplayName        string    `json:"person_display_name"`
	PersonSourceRevision     int64     `json:"person_source_revision"`
	EffectiveMonth           string    `json:"effective_month"`
	FixedSalaryMinor         int64     `json:"fixed_salary_minor"`
	SocialInsuranceBaseMinor int64     `json:"social_insurance_base_minor"`
	HousingFundBaseMinor     int64     `json:"housing_fund_base_minor"`
	Currency                 string    `json:"currency"`
	Reason                   string    `json:"reason"`
	Revision                 int64     `json:"revision"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type UpsertCompensationProfileInput struct {
	CommandID                string `json:"command_id"`
	ExpectedRevision         int64  `json:"expected_revision"`
	PersonID                 string `json:"person_id"`
	EffectiveMonth           string `json:"effective_month"`
	FixedSalaryMinor         int64  `json:"fixed_salary_minor"`
	SocialInsuranceBaseMinor int64  `json:"social_insurance_base_minor"`
	HousingFundBaseMinor     int64  `json:"housing_fund_base_minor"`
	Reason                   string `json:"reason"`
}

type PaymentProfile struct {
	ID                   string    `json:"id"`
	TenantID             string    `json:"tenant_id"`
	PersonID             string    `json:"person_id"`
	PersonDisplayName    string    `json:"person_display_name"`
	PersonSourceRevision int64     `json:"person_source_revision"`
	AccountName          string    `json:"account_name"`
	BankName             string    `json:"bank_name"`
	MaskedAccountNumber  string    `json:"masked_account_number"`
	Status               string    `json:"status"`
	EffectiveMonth       string    `json:"effective_month"`
	Reason               string    `json:"reason"`
	Revision             int64     `json:"revision"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	accountNumber        string
}

type SensitivePaymentProfile struct {
	PaymentProfile
	AccountNumber string `json:"account_number"`
}

type UpsertPaymentProfileInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	PersonID         string `json:"person_id"`
	AccountName      string `json:"account_name"`
	BankName         string `json:"bank_name"`
	AccountNumber    string `json:"account_number"`
	Status           string `json:"status"`
	EffectiveMonth   string `json:"effective_month"`
	Reason           string `json:"reason"`
	Confirmed        bool   `json:"confirmed"`
}

type SensitiveAccountReadInput struct {
	Purpose string `json:"purpose"`
}

type ContributionComponent struct {
	Key                  string `json:"key"`
	BaseKind             string `json:"base_kind"`
	IndividualRateBPS    int64  `json:"individual_rate_bps"`
	EmployerRateBPS      int64  `json:"employer_rate_bps"`
	IndividualFixedMinor int64  `json:"individual_fixed_minor,omitempty"`
	EmployerFixedMinor   int64  `json:"employer_fixed_minor,omitempty"`
	BaseFloorMinor       int64  `json:"base_floor_minor"`
	BaseCapMinor         int64  `json:"base_cap_minor"`
	RoundingMode         string `json:"rounding_mode"`
}

type ContributionScheme struct {
	ID                   string                  `json:"id"`
	TenantID             string                  `json:"tenant_id,omitempty"`
	SchemeKind           string                  `json:"scheme_kind"`
	SourceSchemeID       string                  `json:"source_scheme_id,omitempty"`
	SourceSchemeRevision int64                   `json:"source_scheme_revision,omitempty"`
	Title                string                  `json:"title"`
	RegionCode           string                  `json:"region_code"`
	RegionName           string                  `json:"region_name"`
	EffectiveMonth       string                  `json:"effective_month"`
	AuthoritySource      string                  `json:"authority_source"`
	AuthorityVerifiedAt  time.Time               `json:"authority_verified_at"`
	Components           []ContributionComponent `json:"components"`
	Reason               string                  `json:"reason"`
	Revision             int64                   `json:"revision"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
}

type CreateStandardSchemeInput struct {
	CommandID           string                  `json:"command_id"`
	Title               string                  `json:"title"`
	RegionCode          string                  `json:"region_code"`
	RegionName          string                  `json:"region_name"`
	EffectiveMonth      string                  `json:"effective_month"`
	AuthoritySource     string                  `json:"authority_source"`
	AuthorityVerifiedAt time.Time               `json:"authority_verified_at"`
	Components          []ContributionComponent `json:"components"`
	Reason              string                  `json:"reason"`
}

type CreateCustomSchemeInput struct {
	CommandID            string                  `json:"command_id"`
	SourceSchemeID       string                  `json:"source_scheme_id"`
	SourceSchemeRevision int64                   `json:"source_scheme_revision"`
	Title                string                  `json:"title"`
	EffectiveMonth       string                  `json:"effective_month"`
	Components           []ContributionComponent `json:"components,omitempty"`
	Reason               string                  `json:"reason"`
}

type ReviseCustomSchemeInput struct {
	CommandID        string                  `json:"command_id"`
	ExpectedRevision int64                   `json:"expected_revision"`
	Title            string                  `json:"title"`
	EffectiveMonth   string                  `json:"effective_month"`
	Components       []ContributionComponent `json:"components"`
	Reason           string                  `json:"reason"`
}

type ProfileStore interface {
	UpsertCompensation(context.Context, Actor, UpsertCompensationProfileInput, PersonReference, string) (CompensationProfile, error)
	GetCompensation(context.Context, string, string) (CompensationProfile, error)
	ListCompensations(context.Context, string) ([]CompensationProfile, error)
	UpsertPayment(context.Context, Actor, UpsertPaymentProfileInput, PersonReference, string) (PaymentProfile, error)
	GetPayment(context.Context, string, string) (PaymentProfile, error)
	ListPayments(context.Context, string) ([]PaymentProfile, error)
	ListPaymentHistory(context.Context, string, string) ([]PaymentProfile, error)
	ReadSensitivePayment(context.Context, Actor, string, string) (SensitivePaymentProfile, error)
	CreateStandardScheme(context.Context, Actor, CreateStandardSchemeInput, string) (ContributionScheme, error)
	CreateCustomScheme(context.Context, Actor, CreateCustomSchemeInput, string) (ContributionScheme, error)
	ReviseCustomScheme(context.Context, Actor, string, ReviseCustomSchemeInput, string) (ContributionScheme, error)
	ListSchemes(context.Context, string) ([]ContributionScheme, error)
}

func (s *Service) profileStore() (ProfileStore, error) {
	store, ok := s.store.(ProfileStore)
	if !ok {
		return nil, fmt.Errorf("profile store is unavailable")
	}
	return store, nil
}

func (s *Service) UpsertCompensation(ctx context.Context, actor Actor, in UpsertCompensationProfileInput) (CompensationProfile, error) {
	if err := validateActor(actor); err != nil {
		return CompensationProfile{}, err
	}
	if !uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(in.CommandID))) || !uuidPattern.MatchString(strings.ToLower(strings.TrimSpace(in.PersonID))) || !periodPattern.MatchString(strings.TrimSpace(in.EffectiveMonth)) || in.ExpectedRevision < 0 || in.FixedSalaryMinor < 0 || in.FixedSalaryMinor > maxPayrollMinor || in.SocialInsuranceBaseMinor < 0 || in.SocialInsuranceBaseMinor > maxPayrollMinor || in.HousingFundBaseMinor < 0 || in.HousingFundBaseMinor > maxPayrollMinor {
		return CompensationProfile{}, fmt.Errorf("invalid compensation profile")
	}
	in.CommandID, in.PersonID, in.EffectiveMonth = strings.ToLower(strings.TrimSpace(in.CommandID)), strings.ToLower(strings.TrimSpace(in.PersonID)), strings.TrimSpace(in.EffectiveMonth)
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" || len([]rune(in.Reason)) > 500 {
		return CompensationProfile{}, fmt.Errorf("invalid compensation profile reason")
	}
	person, err := s.validatePerson(ctx, actor.TenantID, in.PersonID)
	if err != nil {
		return CompensationProfile{}, err
	}
	store, err := s.profileStore()
	if err != nil {
		return CompensationProfile{}, err
	}
	return store.UpsertCompensation(ctx, actor, in, person, hash(in))
}

func (s *Service) UpsertPayment(ctx context.Context, actor Actor, in UpsertPaymentProfileInput) (PaymentProfile, error) {
	if err := validateActor(actor); err != nil {
		return PaymentProfile{}, err
	}
	in.CommandID, in.PersonID = strings.ToLower(strings.TrimSpace(in.CommandID)), strings.ToLower(strings.TrimSpace(in.PersonID))
	in.AccountName, in.BankName, in.AccountNumber = strings.TrimSpace(in.AccountName), strings.TrimSpace(in.BankName), strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(in.AccountNumber), " ", ""), "-", "")
	in.Status, in.EffectiveMonth, in.Reason = strings.ToLower(strings.TrimSpace(in.Status)), strings.TrimSpace(in.EffectiveMonth), strings.TrimSpace(in.Reason)
	if !uuidPattern.MatchString(in.CommandID) || !uuidPattern.MatchString(in.PersonID) || in.ExpectedRevision < 0 || len([]rune(in.AccountName)) < 1 || len([]rune(in.AccountName)) > 100 || len([]rune(in.BankName)) < 1 || len([]rune(in.BankName)) > 160 || !accountNumberPattern.MatchString(in.AccountNumber) || (in.Status != "active" && in.Status != "inactive") || !periodPattern.MatchString(in.EffectiveMonth) || in.Reason == "" || len([]rune(in.Reason)) > 500 || !in.Confirmed {
		return PaymentProfile{}, fmt.Errorf("invalid payment profile")
	}
	person, err := s.validatePerson(ctx, actor.TenantID, in.PersonID)
	if err != nil {
		return PaymentProfile{}, err
	}
	store, err := s.profileStore()
	if err != nil {
		return PaymentProfile{}, err
	}
	return store.UpsertPayment(ctx, actor, in, person, hash(in))
}

func (s *Service) validatePerson(ctx context.Context, tenant, id string) (PersonReference, error) {
	if s.personReferences == nil {
		return PersonReference{}, ErrReferenceUnavailable
	}
	value, err := s.personReferences.ValidatePerson(ctx, tenant, id)
	if err != nil {
		return PersonReference{}, err
	}
	// Employment status is a People-owned display and selection hint. It must
	// not become a Payroll eligibility rule: inactive people may still need a
	// final payment, correction, recovery, or a profile revision prepared for
	// rehire. The reference port validates the finite People status vocabulary.
	if value.ID != id || value.TenantID != tenant || value.Revision < 1 || (value.Status != "active" && value.Status != "inactive") || strings.TrimSpace(value.DisplayName) == "" || len([]rune(value.ReferenceCode)) > 64 || !sourceDigestPattern.MatchString(value.Digest) {
		return PersonReference{}, ErrReferenceInvalid
	}
	return value, nil
}

func (s *Service) GetCompensation(ctx context.Context, tenant, person string) (CompensationProfile, error) {
	store, err := s.profileStore()
	if err != nil {
		return CompensationProfile{}, err
	}
	return store.GetCompensation(ctx, tenant, person)
}
func (s *Service) ListCompensations(ctx context.Context, tenant string) ([]CompensationProfile, error) {
	store, err := s.profileStore()
	if err != nil {
		return nil, err
	}
	return store.ListCompensations(ctx, tenant)
}
func (s *Service) GetPayment(ctx context.Context, tenant, person string) (PaymentProfile, error) {
	store, err := s.profileStore()
	if err != nil {
		return PaymentProfile{}, err
	}
	return store.GetPayment(ctx, tenant, person)
}
func (s *Service) ListPayments(ctx context.Context, tenant string) ([]PaymentProfile, error) {
	store, err := s.profileStore()
	if err != nil {
		return nil, err
	}
	return store.ListPayments(ctx, tenant)
}
func (s *Service) ListPaymentHistory(ctx context.Context, tenant, person string) ([]PaymentProfile, error) {
	store, err := s.profileStore()
	if err != nil {
		return nil, err
	}
	return store.ListPaymentHistory(ctx, tenant, person)
}
func (s *Service) ReadSensitivePayment(ctx context.Context, actor Actor, person, purpose string) (SensitivePaymentProfile, error) {
	if err := validateActor(actor); err != nil {
		return SensitivePaymentProfile{}, err
	}
	person = strings.ToLower(strings.TrimSpace(person))
	purpose = strings.TrimSpace(purpose)
	if !uuidPattern.MatchString(person) || len([]rune(purpose)) < 1 || len([]rune(purpose)) > 300 {
		return SensitivePaymentProfile{}, fmt.Errorf("invalid sensitive account purpose")
	}
	store, err := s.profileStore()
	if err != nil {
		return SensitivePaymentProfile{}, err
	}
	return store.ReadSensitivePayment(ctx, actor, person, purpose)
}

func validateComponents(values []ContributionComponent) ([]ContributionComponent, error) {
	want := map[string]string{"pension": "social", "medical": "social", "unemployment": "social", "work_injury": "social", "maternity": "social", "housing_fund": "housing"}
	if len(values) != len(want) {
		return nil, fmt.Errorf("invalid contribution components")
	}
	seen := map[string]bool{}
	for i := range values {
		v := &values[i]
		v.Key, v.BaseKind, v.RoundingMode = strings.ToLower(strings.TrimSpace(v.Key)), strings.ToLower(strings.TrimSpace(v.BaseKind)), strings.ToLower(strings.TrimSpace(v.RoundingMode))
		if seen[v.Key] || want[v.Key] != v.BaseKind || v.IndividualRateBPS < 0 || v.IndividualRateBPS > 10000 || v.EmployerRateBPS < 0 || v.EmployerRateBPS > 10000 || v.IndividualFixedMinor < 0 || v.IndividualFixedMinor > maxPayrollMinor || v.EmployerFixedMinor < 0 || v.EmployerFixedMinor > maxPayrollMinor || v.BaseFloorMinor < 0 || v.BaseFloorMinor > maxPayrollMinor || v.BaseCapMinor < v.BaseFloorMinor || v.BaseCapMinor > maxPayrollMinor || (v.RoundingMode != "half_up" && v.RoundingMode != "down" && v.RoundingMode != "up" && v.RoundingMode != "yuan_half_up") {
			return nil, fmt.Errorf("invalid contribution component")
		}
		seen[v.Key] = true
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Key < values[j].Key })
	return values, nil
}

func (s *Service) CreateStandardScheme(ctx context.Context, actor Actor, in CreateStandardSchemeInput) (ContributionScheme, error) {
	if err := validateActor(actor); err != nil {
		return ContributionScheme{}, err
	}
	in.CommandID, in.Title, in.RegionCode, in.RegionName, in.EffectiveMonth, in.AuthoritySource, in.Reason = strings.ToLower(strings.TrimSpace(in.CommandID)), strings.TrimSpace(in.Title), strings.ToUpper(strings.TrimSpace(in.RegionCode)), strings.TrimSpace(in.RegionName), strings.TrimSpace(in.EffectiveMonth), strings.TrimSpace(in.AuthoritySource), strings.TrimSpace(in.Reason)
	components, err := validateComponents(in.Components)
	if err != nil {
		return ContributionScheme{}, err
	}
	in.Components = components
	if !uuidPattern.MatchString(in.CommandID) || in.Title == "" || in.RegionCode == "" || in.RegionName == "" || !periodPattern.MatchString(in.EffectiveMonth) || in.AuthoritySource == "" || in.AuthorityVerifiedAt.IsZero() || in.Reason == "" {
		return ContributionScheme{}, fmt.Errorf("invalid standard contribution scheme")
	}
	store, err := s.profileStore()
	if err != nil {
		return ContributionScheme{}, err
	}
	return store.CreateStandardScheme(ctx, actor, in, hash(in))
}

func (s *Service) CreateCustomScheme(ctx context.Context, actor Actor, in CreateCustomSchemeInput) (ContributionScheme, error) {
	if err := validateActor(actor); err != nil {
		return ContributionScheme{}, err
	}
	in.CommandID, in.SourceSchemeID, in.Title, in.EffectiveMonth, in.Reason = strings.ToLower(strings.TrimSpace(in.CommandID)), strings.ToLower(strings.TrimSpace(in.SourceSchemeID)), strings.TrimSpace(in.Title), strings.TrimSpace(in.EffectiveMonth), strings.TrimSpace(in.Reason)
	if len(in.Components) > 0 {
		components, err := validateComponents(in.Components)
		if err != nil {
			return ContributionScheme{}, err
		}
		in.Components = components
	}
	if !uuidPattern.MatchString(in.CommandID) || !uuidPattern.MatchString(in.SourceSchemeID) || in.SourceSchemeRevision < 1 || in.Title == "" || !periodPattern.MatchString(in.EffectiveMonth) || in.Reason == "" {
		return ContributionScheme{}, fmt.Errorf("invalid custom contribution scheme")
	}
	store, err := s.profileStore()
	if err != nil {
		return ContributionScheme{}, err
	}
	return store.CreateCustomScheme(ctx, actor, in, hash(in))
}

func (s *Service) ReviseCustomScheme(ctx context.Context, actor Actor, schemeID string, in ReviseCustomSchemeInput) (ContributionScheme, error) {
	if err := validateActor(actor); err != nil {
		return ContributionScheme{}, err
	}
	schemeID = strings.ToLower(strings.TrimSpace(schemeID))
	in.CommandID, in.Title, in.EffectiveMonth, in.Reason = strings.ToLower(strings.TrimSpace(in.CommandID)), strings.TrimSpace(in.Title), strings.TrimSpace(in.EffectiveMonth), strings.TrimSpace(in.Reason)
	components, err := validateComponents(in.Components)
	if err != nil {
		return ContributionScheme{}, err
	}
	in.Components = components
	if !uuidPattern.MatchString(schemeID) || !uuidPattern.MatchString(in.CommandID) || in.ExpectedRevision < 1 || in.Title == "" || !periodPattern.MatchString(in.EffectiveMonth) || in.Reason == "" || len([]rune(in.Reason)) > 500 {
		return ContributionScheme{}, fmt.Errorf("invalid custom contribution scheme revision")
	}
	store, err := s.profileStore()
	if err != nil {
		return ContributionScheme{}, err
	}
	return store.ReviseCustomScheme(ctx, actor, schemeID, in, hash(struct {
		SchemeID string
		Input    ReviseCustomSchemeInput
	}{schemeID, in}))
}
func (s *Service) ListSchemes(ctx context.Context, tenant string) ([]ContributionScheme, error) {
	store, err := s.profileStore()
	if err != nil {
		return nil, err
	}
	return store.ListSchemes(ctx, tenant)
}
