package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

type TokenKind string

const (
	TokenInvitation    TokenKind = "invitation"
	TokenPasswordReset TokenKind = "password_reset"
)

var (
	usernamePattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)
	phonePattern      = regexp.MustCompile(`^1[3-9][0-9]{9}$`)
	phoneLikePattern  = regexp.MustCompile(`^[+0-9\s()-]+$`)
	emailPattern      = regexp.MustCompile(`^[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
	actionPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
	resourcePattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	uuidPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type Subject struct {
	ID                  string    `json:"id"`
	AccountID           string    `json:"-"`
	TenantID            string    `json:"tenant_id"`
	Username            string    `json:"username"`
	DisplayName         string    `json:"display_name"`
	Status              Status    `json:"status"`
	Administrator       bool      `json:"administrator"`
	SystemAdministrator bool      `json:"system_administrator"`
	Revision            int64     `json:"revision"`
	SessionGeneration   int64     `json:"-"`
	FailedLoginAttempts int       `json:"-"`
	LockedUntil         time.Time `json:"-"`
	PasswordChangedAt   time.Time `json:"password_changed_at,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type Credential struct {
	Subject
	PasswordHash string
}

type Principal struct {
	SubjectID           string    `json:"subject_id"`
	AccountID           string    `json:"-"`
	TenantID            string    `json:"tenant_id"`
	Username            string    `json:"username"`
	DisplayName         string    `json:"display_name"`
	Administrator       bool      `json:"administrator"`
	SystemAdministrator bool      `json:"system_administrator"`
	SessionGeneration   int64     `json:"-"`
	AuthTime            time.Time `json:"auth_time"`
}

type AuditActor struct {
	Kind        string
	ID          string
	DisplayName string
}

func (p Principal) Actor() AuditActor {
	return AuditActor{Kind: "iam_subject", ID: p.SubjectID, DisplayName: p.DisplayName}
}

type AuditEntry struct {
	ID         string
	TenantID   string
	Action     string
	ObjectType string
	ObjectID   string
	Actor      AuditActor
	OccurredAt time.Time
	Details    map[string]any
}

type TenantSummary struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Status             string    `json:"status"`
	UserCount          int       `json:"user_count"`
	AdministratorCount int       `json:"administrator_count"`
	CreatedAt          time.Time `json:"created_at"`
}

type OrganizationMembership struct {
	TenantID      string    `json:"tenant_id"`
	TenantName    string    `json:"tenant_name"`
	SubjectID     string    `json:"subject_id"`
	DisplayName   string    `json:"display_name"`
	Status        Status    `json:"status"`
	Administrator bool      `json:"administrator"`
	Current       bool      `json:"current"`
	CreatedAt     time.Time `json:"created_at"`
}

type SystemTenantInput struct {
	Name             string
	AdminUsername    string
	AdminDisplayName string
}

type ActivationToken struct {
	ID        string
	SubjectID string
	Kind      TokenKind
	TokenHash []byte
	ExpiresAt time.Time
	CreatedBy string
	CreatedAt time.Time
}

type RegistrationInvitation struct {
	TenantID      string    `json:"tenant_id"`
	TenantName    string    `json:"tenant_name"`
	Administrator bool      `json:"administrator"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type RegistrationResult struct {
	Action  string  `json:"action"`
	Subject Subject `json:"subject"`
}

type Grant struct {
	SubjectID  string    `json:"subject_id"`
	Audience   string    `json:"audience"`
	Action     string    `json:"action"`
	Scope      Scope     `json:"scope"`
	SourceKind string    `json:"source_kind,omitempty"`
	GrantedBy  string    `json:"granted_by"`
	GrantedAt  time.Time `json:"granted_at"`
}

type Repository interface {
	HasSubjects(context.Context) (bool, error)
	Bootstrap(context.Context, string, string, Subject, string, AuditEntry) (bool, error)
	TenantExists(context.Context, string) (bool, error)
	FindSubject(context.Context, string, string) (Subject, bool, error)
	FindAccountSubject(context.Context, string, string) (Subject, bool, error)
	FindCredential(context.Context, string) (Credential, bool, error)
	RecordLoginFailure(context.Context, string, time.Time, int, time.Duration) error
	ResetLoginFailures(context.Context, string, time.Time) error
	CreateSession(context.Context, []byte, string, int64, time.Time, time.Time) error
	RotateSession(context.Context, []byte, []byte, string, int64, time.Time, time.Time) (Principal, error)
	ResolveSession(context.Context, []byte, time.Time) (Principal, bool, error)
	RevokeSession(context.Context, []byte, time.Time) error
	Invite(context.Context, Subject, ActivationToken, AuditEntry) error
	JoinAccount(context.Context, Subject, AuditEntry) error
	Activate(context.Context, []byte, TokenKind, string, time.Time, AuditEntry) (Subject, error)
	PreviewRegistrationInvitation(context.Context, []byte, time.Time) (RegistrationInvitation, error)
	CompleteRegistrationInvitation(context.Context, []byte, string, string, string, *Credential, time.Time, AuditEntry) (RegistrationResult, error)
	CreateActivationToken(context.Context, string, ActivationToken, AuditEntry) error
	Grant(context.Context, string, Grant, AuditEntry) error
	RevokeGrant(context.Context, string, Grant, AuditEntry) error
	ListGrants(context.Context, string, string, string) ([]Grant, error)
	RecordAudit(context.Context, AuditEntry) error
	ListSubjects(context.Context, string) ([]Subject, error)
	UpdateSubjectDisplayName(context.Context, string, string, int64, string, AuditEntry) error
	ChangeStatus(context.Context, string, string, Status, AuditEntry) error
	ChangeAdministrator(context.Context, string, string, bool, string, AuditEntry) error
	RevokeAdministratorProfile(context.Context, string, string, string, AuditEntry) (int64, error)
	UpdateTenantName(context.Context, string, string, AuditEntry) error
	ListTenants(context.Context) ([]TenantSummary, error)
	ProvisionTenantInvitation(context.Context, string, string, Subject, ActivationToken, AuditEntry) error
	ProvisionTenantMembership(context.Context, string, string, Subject, AuditEntry) error
	ListAccountMemberships(context.Context, string) ([]OrganizationMembership, error)
}

type Signer interface {
	Sign(any) (string, error)
}

type CapabilityCatalog interface {
	Contains(audience, action string) bool
	AllowsResource(audience, action, resourceType string) bool
	Actions() map[string][]string
}

type BusinessFactReference struct {
	Kind     string `json:"kind"`
	Source   string `json:"source"`
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type PolicyDecision struct {
	DecisionID        string   `json:"decision_id"`
	PolicyID          string   `json:"policy_id"`
	PolicyVersion     string   `json:"policy_version"`
	SourceApplication string   `json:"source_application"`
	FactKind          string   `json:"fact_kind"`
	FactID            string   `json:"fact_id"`
	FactRevision      int64    `json:"fact_revision"`
	Actions           []string `json:"actions"`
}

type PolicyEvaluator interface {
	Evaluate(context.Context, Principal, string, Scope, BusinessFactReference) (PolicyDecision, error)
}

type Service struct {
	repository            Repository
	signer                Signer
	capabilities          CapabilityCatalog
	policy                PolicyEvaluator
	tenantID              string
	tenantName            string
	issuer                string
	sessionTTL            time.Duration
	credentialTTL         time.Duration
	workloadCredentialTTL time.Duration
	now                   func() time.Time
	newID                 func() (string, error)
	bcryptCost            int
	dummyHash             []byte
}

const administratorProfileSource = "administrator_profile"

type ServiceIdentity struct {
	ID          string
	SubjectID   string
	DisplayName string
	KeyID       string
	AssertionID string
	AuthTime    time.Time
}

type BootstrapInput struct {
	Username    string
	DisplayName string
	Password    string
}

type LoginResult struct {
	Principal Principal `json:"principal"`
	Token     string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
}

type TokenResult struct {
	Action    string    `json:"action,omitempty"`
	Subject   Subject   `json:"subject"`
	Token     string    `json:"token,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitzero"`
}

type TenantAdministratorResult struct {
	Action    string     `json:"action"`
	Subject   Subject    `json:"subject"`
	Token     string     `json:"token,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type CapabilityClaims struct {
	Version   string   `json:"ver"`
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Tenant    string   `json:"tid"`
	Audience  string   `json:"aud"`
	Name      string   `json:"name"`
	Actions   []string `json:"actions"`
	Scope     Scope    `json:"scope"`
	IssuedAt  int64    `json:"iat"`
	NotBefore int64    `json:"nbf"`
	ExpiresAt int64    `json:"exp"`
	AuthTime  int64    `json:"auth_time"`
	ID        string   `json:"jti"`
}

type Scope struct {
	Kind      string        `json:"kind"`
	Resources []ResourceRef `json:"resources"`
}

type ResourceRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type IssuedCredential struct {
	Token     string           `json:"token"`
	TokenType string           `json:"token_type"`
	ExpiresAt time.Time        `json:"expires_at"`
	Claims    CapabilityClaims `json:"claims"`
	Decision  *PolicyDecision  `json:"policy_decision,omitempty"`
}

type SubjectReference struct {
	SchemaVersion     string `json:"schema_version"`
	SourceApplication string `json:"source_application"`
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	Username          string `json:"username"`
	DisplayName       string `json:"display_name"`
	Status            Status `json:"status"`
	Revision          int64  `json:"revision"`
}

type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return e.Message }

type AuthenticationError struct{}

func (e *AuthenticationError) Error() string {
	return "用户名或密码错误，或账号暂不可用"
}

type AuthorizationError struct{}

func (e *AuthorizationError) Error() string { return "当前用户没有执行此操作的权限" }

type DependencyUnavailableError struct {
	Dependency string
	Err        error
}

func (e *DependencyUnavailableError) Error() string {
	if e.Dependency == "" {
		return "权限策略依赖暂时不可用"
	}
	return e.Dependency + " 暂时不可用"
}

func (e *DependencyUnavailableError) Unwrap() error { return e.Err }

type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

func NewService(repository Repository, signer Signer, capabilities CapabilityCatalog, tenantID, tenantName, issuer string, sessionTTL, credentialTTL time.Duration) (*Service, error) {
	if repository == nil || signer == nil || capabilities == nil {
		return nil, fmt.Errorf("identity repository, signer and capability catalog are required")
	}
	if !uuidPattern.MatchString(tenantID) || strings.TrimSpace(tenantName) == "" || strings.TrimSpace(issuer) == "" {
		return nil, fmt.Errorf("tenant and issuer configuration is invalid")
	}
	if sessionTTL <= 0 || credentialTTL <= 0 || credentialTTL > 5*time.Minute {
		return nil, fmt.Errorf("session and credential TTL are invalid")
	}
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("not-a-real-iam-password"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return &Service{
		repository: repository, signer: signer, capabilities: capabilities, tenantID: tenantID, tenantName: strings.TrimSpace(tenantName), issuer: strings.TrimRight(issuer, "/"),
		sessionTTL: sessionTTL, credentialTTL: credentialTTL, now: func() time.Time { return time.Now().UTC() }, newID: randomUUID,
		bcryptCost: bcrypt.DefaultCost, dummyHash: dummyHash,
	}, nil
}

func (s *Service) SetPolicyEvaluator(evaluator PolicyEvaluator) error {
	if evaluator == nil {
		return fmt.Errorf("policy evaluator is required")
	}
	s.policy = evaluator
	return nil
}

func (s *Service) SetWorkloadCredentialTTL(value time.Duration) error {
	if value <= 0 || value > time.Minute {
		return fmt.Errorf("workload credential TTL must be between 1ns and 1m")
	}
	s.workloadCredentialTTL = value
	return nil
}

func (s *Service) Issuer() string { return s.issuer }

func (s *Service) SubjectReference(ctx context.Context, tenantID, subjectID string) (SubjectReference, error) {
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	subjectID = strings.ToLower(strings.TrimSpace(subjectID))
	if !uuidPattern.MatchString(tenantID) || !uuidPattern.MatchString(subjectID) {
		return SubjectReference{}, &ValidationError{Field: "subject_id", Message: "账号引用无效"}
	}
	subject, found, err := s.repository.FindSubject(ctx, tenantID, subjectID)
	if err != nil {
		return SubjectReference{}, err
	}
	if !found {
		return SubjectReference{}, &NotFoundError{Message: "账号不存在"}
	}
	return SubjectReference{
		SchemaVersion: "1", SourceApplication: "identity-access", ID: subject.ID, TenantID: subject.TenantID,
		Username: subject.Username, DisplayName: subject.DisplayName, Status: subject.Status, Revision: subject.Revision,
	}, nil
}

func (s *Service) IssueWorkloadCredential(ctx context.Context, tenantID string, workload ServiceIdentity, audience, action string, scope Scope) (IssuedCredential, error) {
	if !uuidPattern.MatchString(tenantID) || !identifierPattern.MatchString(workload.ID) || !uuidPattern.MatchString(workload.SubjectID) || strings.TrimSpace(workload.DisplayName) == "" || !uuidPattern.MatchString(workload.KeyID) || !uuidPattern.MatchString(workload.AssertionID) || workload.AuthTime.IsZero() {
		return IssuedCredential{}, &AuthenticationError{}
	}
	found, err := s.repository.TenantExists(ctx, tenantID)
	if err != nil {
		return IssuedCredential{}, fmt.Errorf("verify workload tenant: %w", err)
	}
	if !found {
		return IssuedCredential{}, &AuthenticationError{}
	}
	if !s.capabilities.Contains(audience, action) {
		return IssuedCredential{}, &AuthorizationError{}
	}
	normalizedScope, err := normalizeScope(scope)
	if err != nil || normalizedScope.Kind != "resources" {
		return IssuedCredential{}, &AuthorizationError{}
	}
	for _, resource := range normalizedScope.Resources {
		if !s.capabilities.AllowsResource(audience, action, resource.Type) {
			return IssuedCredential{}, &AuthorizationError{}
		}
	}
	now := s.now()
	ttl := s.workloadCredentialTTL
	if ttl <= 0 {
		ttl = time.Minute
	}
	jti, err := s.newID()
	if err != nil {
		return IssuedCredential{}, err
	}
	expiresAt := now.Add(ttl)
	claims := CapabilityClaims{Version: "1", Issuer: s.issuer, Subject: workload.SubjectID, Tenant: tenantID, Audience: audience, Name: workload.DisplayName, Actions: []string{action}, Scope: normalizedScope, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: expiresAt.Unix(), AuthTime: workload.AuthTime.Unix(), ID: jti}
	token, err := s.signer.Sign(claims)
	if err != nil {
		return IssuedCredential{}, fmt.Errorf("sign workload capability credential: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return IssuedCredential{}, err
	}
	actor := AuditActor{Kind: "service_identity", ID: workload.SubjectID, DisplayName: workload.DisplayName}
	if err := s.repository.RecordAudit(ctx, s.audit(tenantID, auditID, "iam_service_credential_issued", "capability_credential", jti, actor, now, map[string]any{
		"workload_id": workload.ID, "workload_key_id": workload.KeyID, "assertion_id": workload.AssertionID,
		"audience": audience, "actions": []string{action}, "scope": normalizedScope,
	})); err != nil {
		return IssuedCredential{}, err
	}
	return IssuedCredential{Token: token, TokenType: "Bearer", ExpiresAt: expiresAt, Claims: claims}, nil
}

func (s *Service) EnsureBootstrap(ctx context.Context, input BootstrapInput) (bool, error) {
	hasSubjects, err := s.repository.HasSubjects(ctx)
	if err != nil {
		return false, fmt.Errorf("check IAM bootstrap: %w", err)
	}
	if hasSubjects {
		return false, nil
	}
	if strings.TrimSpace(input.Username) == "" || input.Password == "" {
		return false, &ValidationError{Field: "bootstrap", Message: "空数据库必须配置首位管理员用户名和密码"}
	}
	username, err := normalizeUsername(input.Username)
	if err != nil {
		return false, err
	}
	displayName, err := normalizeDisplayName(input.DisplayName, username)
	if err != nil {
		return false, err
	}
	passwordHash, err := s.hashPassword(input.Password)
	if err != nil {
		return false, err
	}
	subjectID, err := s.newID()
	if err != nil {
		return false, err
	}
	auditID, err := s.newID()
	if err != nil {
		return false, err
	}
	now := s.now()
	subject := Subject{ID: subjectID, TenantID: s.tenantID, Username: username, DisplayName: displayName, Status: StatusActive, SystemAdministrator: true, Revision: 1, SessionGeneration: 1, PasswordChangedAt: now, CreatedAt: now, UpdatedAt: now}
	return s.repository.Bootstrap(ctx, s.tenantID, s.tenantName, subject, passwordHash, AuditEntry{
		ID: auditID, TenantID: s.tenantID, Action: "iam_subject_bootstrapped", ObjectType: "iam_subject", ObjectID: subjectID,
		Actor: AuditActor{Kind: "bootstrap_configuration", ID: subjectID, DisplayName: displayName}, OccurredAt: now,
		Details: map[string]any{"username": username, "system_administrator": true},
	})
}

func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	normalized, normalizeErr := normalizeUsername(username)
	credential, found, err := s.repository.FindCredential(ctx, normalized)
	if err != nil {
		return LoginResult{}, fmt.Errorf("find IAM credential: %w", err)
	}
	if normalizeErr != nil || !found {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return LoginResult{}, &AuthenticationError{}
	}
	now := s.now()
	if credential.Status != StatusActive || credential.LockedUntil.After(now) || bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password)) != nil {
		if credential.Status == StatusActive && !credential.LockedUntil.After(now) {
			if err := s.repository.RecordLoginFailure(ctx, credential.ID, now, 5, 15*time.Minute); err != nil {
				return LoginResult{}, err
			}
		} else {
			_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		}
		return LoginResult{}, &AuthenticationError{}
	}
	if err := s.repository.ResetLoginFailures(ctx, credential.ID, now); err != nil {
		return LoginResult{}, err
	}
	if credential.Administrator {
		if err := s.ensureAdministratorProfile(ctx, credential.Subject); err != nil {
			return LoginResult{}, fmt.Errorf("reconcile administrator access: %w", err)
		}
	}
	rawToken, tokenHash, err := randomToken()
	if err != nil {
		return LoginResult{}, err
	}
	expiresAt := now.Add(s.sessionTTL)
	if err := s.repository.CreateSession(ctx, tokenHash, credential.ID, credential.SessionGeneration, now, expiresAt); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Principal: principalFromCredential(credential, now), Token: rawToken, ExpiresAt: expiresAt}, nil
}

func (s *Service) ResolveSession(ctx context.Context, token string) (Principal, bool, error) {
	hash, valid := tokenHash(token)
	if !valid {
		return Principal{}, false, nil
	}
	principal, found, err := s.repository.ResolveSession(ctx, hash, s.now())
	if err != nil {
		return Principal{}, false, fmt.Errorf("resolve IAM session: %w", err)
	}
	return principal, found, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	hash, valid := tokenHash(token)
	if !valid {
		return nil
	}
	return s.repository.RevokeSession(ctx, hash, s.now())
}

func (s *Service) ListOrganizations(ctx context.Context, actor Principal) ([]OrganizationMembership, error) {
	if !uuidPattern.MatchString(actor.SubjectID) || actor.SystemAdministrator {
		return nil, &AuthorizationError{}
	}
	return s.repository.ListAccountMemberships(ctx, actor.SubjectID)
}

func (s *Service) SwitchOrganization(ctx context.Context, token string, actor Principal, tenantID string) (LoginResult, error) {
	if !uuidPattern.MatchString(actor.SubjectID) || actor.SystemAdministrator {
		return LoginResult{}, &AuthorizationError{}
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if !uuidPattern.MatchString(tenantID) {
		return LoginResult{}, &ValidationError{Field: "tenant_id", Message: "组织 ID 无效"}
	}
	if tenantID == actor.TenantID {
		return LoginResult{}, &ConflictError{Message: "已经在当前组织"}
	}
	memberships, err := s.repository.ListAccountMemberships(ctx, actor.SubjectID)
	if err != nil {
		return LoginResult{}, err
	}
	var target OrganizationMembership
	for _, membership := range memberships {
		if membership.TenantID == tenantID {
			target = membership
			break
		}
	}
	if target.SubjectID == "" || target.Status != StatusActive {
		return LoginResult{}, &AuthorizationError{}
	}
	subject, found, err := s.repository.FindSubject(ctx, tenantID, target.SubjectID)
	if err != nil {
		return LoginResult{}, err
	}
	if !found || subject.AccountID != actor.AccountID || subject.Status != StatusActive {
		return LoginResult{}, &AuthorizationError{}
	}
	if subject.Administrator {
		if err := s.ensureAdministratorProfile(ctx, subject); err != nil {
			return LoginResult{}, fmt.Errorf("reconcile administrator access: %w", err)
		}
	}
	oldHash, valid := tokenHash(token)
	if !valid {
		return LoginResult{}, &AuthenticationError{}
	}
	rawToken, newHash, err := randomToken()
	if err != nil {
		return LoginResult{}, err
	}
	now := s.now()
	expiresAt := now.Add(s.sessionTTL)
	principal, err := s.repository.RotateSession(ctx, oldHash, newHash, subject.ID, subject.SessionGeneration, now, expiresAt)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Principal: principal, Token: rawToken, ExpiresAt: expiresAt}, nil
}

func (s *Service) Invite(ctx context.Context, actor Principal, username, displayName string, administrator bool) (TokenResult, error) {
	if err := requireAdministrator(actor); err != nil {
		return TokenResult{}, err
	}
	normalized, err := normalizeUsername(username)
	if err != nil {
		return TokenResult{}, err
	}
	displayName, err = normalizeDisplayName(displayName, normalized)
	if err != nil {
		return TokenResult{}, err
	}
	now := s.now()
	existing, found, err := s.repository.FindCredential(ctx, normalized)
	if err != nil {
		return TokenResult{}, err
	}
	if found {
		if existing.SystemAdministrator {
			return TokenResult{}, &ConflictError{Message: "系统管理员账号不能加入业务组织"}
		}
		if existing.Status != StatusActive {
			return TokenResult{}, &ConflictError{Message: "该登录账号尚未激活或当前不可用"}
		}
		if _, joined, err := s.repository.FindAccountSubject(ctx, actor.TenantID, existing.AccountID); err != nil {
			return TokenResult{}, err
		} else if joined {
			return TokenResult{}, &ConflictError{Message: "该账号已经加入当前组织"}
		}
		subjectID, err := s.newID()
		if err != nil {
			return TokenResult{}, err
		}
		auditID, err := s.newID()
		if err != nil {
			return TokenResult{}, err
		}
		subject := Subject{ID: subjectID, AccountID: existing.AccountID, TenantID: actor.TenantID, Username: normalized, DisplayName: displayName, Status: StatusActive, Administrator: administrator, Revision: 1, SessionGeneration: 1, PasswordChangedAt: existing.PasswordChangedAt, CreatedAt: now, UpdatedAt: now}
		audit := s.audit(actor.TenantID, auditID, "iam_account_joined_tenant", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"login_identifier": normalized, "administrator": administrator, "account_id": existing.AccountID})
		if err := s.repository.JoinAccount(ctx, subject, audit); err != nil {
			return TokenResult{}, err
		}
		if administrator {
			if err := s.ensureAdministratorProfile(ctx, subject); err != nil {
				return TokenResult{}, err
			}
		}
		return TokenResult{Action: "joined", Subject: subject}, nil
	}
	subjectID, err := s.newID()
	if err != nil {
		return TokenResult{}, err
	}
	tokenID, err := s.newID()
	if err != nil {
		return TokenResult{}, err
	}
	auditID, err := s.newID()
	if err != nil {
		return TokenResult{}, err
	}
	raw, hash, err := randomToken()
	if err != nil {
		return TokenResult{}, err
	}
	subject := Subject{ID: subjectID, TenantID: actor.TenantID, Username: normalized, DisplayName: displayName, Status: StatusPending, Administrator: administrator, Revision: 1, SessionGeneration: 1, CreatedAt: now, UpdatedAt: now}
	expiresAt := now.Add(24 * time.Hour)
	activation := ActivationToken{ID: tokenID, SubjectID: subjectID, Kind: TokenInvitation, TokenHash: hash, ExpiresAt: expiresAt, CreatedBy: actor.SubjectID, CreatedAt: now}
	err = s.repository.Invite(ctx, subject, activation, s.audit(actor.TenantID, auditID, "iam_subject_invited", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"login_identifier": normalized, "administrator": administrator}))
	if err != nil {
		return TokenResult{}, err
	}
	return TokenResult{Subject: subject, Token: raw, ExpiresAt: expiresAt}, nil
}

// CreateRegistrationInvitation creates a tenant-scoped, single-use invitation
// without collecting another person's name or login identifier.  The pending
// subject uses an internal identifier that can never be used for login and is
// replaced atomically when the recipient completes registration.
func (s *Service) CreateRegistrationInvitation(ctx context.Context, actor Principal, administrator bool) (TokenResult, error) {
	if err := requireAdministrator(actor); err != nil {
		return TokenResult{}, err
	}
	now := s.now()
	subjectID, err := s.newID()
	if err != nil {
		return TokenResult{}, err
	}
	tokenID, err := s.newID()
	if err != nil {
		return TokenResult{}, err
	}
	auditID, err := s.newID()
	if err != nil {
		return TokenResult{}, err
	}
	raw, hash, err := randomToken()
	if err != nil {
		return TokenResult{}, err
	}
	placeholder := "invite-" + strings.ReplaceAll(subjectID, "-", "")
	subject := Subject{ID: subjectID, TenantID: actor.TenantID, Username: placeholder, DisplayName: "待受邀人注册", Status: StatusPending, Administrator: administrator, Revision: 1, SessionGeneration: 1, CreatedAt: now, UpdatedAt: now}
	expiresAt := now.Add(24 * time.Hour)
	activation := ActivationToken{ID: tokenID, SubjectID: subjectID, Kind: TokenInvitation, TokenHash: hash, ExpiresAt: expiresAt, CreatedBy: actor.SubjectID, CreatedAt: now}
	entry := s.audit(actor.TenantID, auditID, "iam_registration_invitation_created", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"administrator": administrator})
	if err := s.repository.Invite(ctx, subject, activation, entry); err != nil {
		return TokenResult{}, err
	}
	return TokenResult{Action: "registration_invited", Subject: subject, Token: raw, ExpiresAt: expiresAt}, nil
}

func (s *Service) PreviewRegistrationInvitation(ctx context.Context, rawToken string) (RegistrationInvitation, error) {
	hash, valid := tokenHash(rawToken)
	if !valid {
		return RegistrationInvitation{}, &AuthenticationError{}
	}
	return s.repository.PreviewRegistrationInvitation(ctx, hash, s.now())
}

func (s *Service) CompleteRegistrationInvitation(ctx context.Context, rawToken, username, displayName, password string) (RegistrationResult, error) {
	hash, valid := tokenHash(rawToken)
	if !valid {
		return RegistrationResult{}, &AuthenticationError{}
	}
	normalized, err := normalizeUsername(username)
	if err != nil {
		return RegistrationResult{}, err
	}
	if !phonePattern.MatchString(normalized) && !emailPattern.MatchString(normalized) {
		return RegistrationResult{}, &ValidationError{Field: "login_identifier", Message: "注册账号必须选择有效手机号或邮箱"}
	}
	displayName, err = normalizeDisplayName(displayName, normalized)
	if err != nil {
		return RegistrationResult{}, err
	}
	existing, found, err := s.repository.FindCredential(ctx, normalized)
	if err != nil {
		return RegistrationResult{}, err
	}
	var passwordHash string
	var existingAccount *Credential
	if found {
		if existing.SystemAdministrator {
			return RegistrationResult{}, &ConflictError{Message: "系统管理员账号不能加入业务组织"}
		}
		if existing.Status != StatusActive || bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte(password)) != nil {
			return RegistrationResult{}, &AuthenticationError{}
		}
		passwordHash = existing.PasswordHash
		existingAccount = &existing
	} else {
		passwordHash, err = s.hashPassword(password)
		if err != nil {
			return RegistrationResult{}, err
		}
	}
	auditID, err := s.newID()
	if err != nil {
		return RegistrationResult{}, err
	}
	now := s.now()
	entry := s.audit("", auditID, "iam_registration_invitation_completed", "iam_subject", "pending-registration-subject", AuditActor{Kind: "activation_token", ID: auditID, DisplayName: "一次性注册邀请"}, now, map[string]any{"login_identifier": normalized, "existing_account": existingAccount != nil})
	return s.repository.CompleteRegistrationInvitation(ctx, hash, normalized, displayName, passwordHash, existingAccount, now, entry)
}

// ReissueInvitation replaces the one-time activation credential for an
// existing pending account.  The repository invalidates every older unused
// invitation before persisting the replacement so the administrator can
// safely recover a lost or expired setup link.
func (s *Service) ReissueInvitation(ctx context.Context, actor Principal, subjectID string) (TokenResult, error) {
	if err := requireAdministrator(actor); err != nil {
		return TokenResult{}, err
	}
	subjectID = strings.ToLower(strings.TrimSpace(subjectID))
	if !uuidPattern.MatchString(subjectID) {
		return TokenResult{}, &ValidationError{Field: "subject_id", Message: "Subject ID 无效"}
	}
	subject, found, err := s.repository.FindSubject(ctx, actor.TenantID, subjectID)
	if err != nil {
		return TokenResult{}, err
	}
	if !found || subject.SystemAdministrator {
		return TokenResult{}, &NotFoundError{Message: "用户不存在"}
	}
	if subject.Status != StatusPending {
		return TokenResult{}, &ConflictError{Message: "只有待激活账号可以重新生成激活链接"}
	}
	now := s.now()
	raw, hash, err := randomToken()
	if err != nil {
		return TokenResult{}, err
	}
	tokenID, err := s.newID()
	if err != nil {
		return TokenResult{}, err
	}
	auditID, err := s.newID()
	if err != nil {
		return TokenResult{}, err
	}
	expiresAt := now.Add(24 * time.Hour)
	token := ActivationToken{ID: tokenID, SubjectID: subjectID, Kind: TokenInvitation, TokenHash: hash, ExpiresAt: expiresAt, CreatedBy: actor.SubjectID, CreatedAt: now}
	action := "iam_invitation_reissued"
	details := map[string]any{"login_identifier": subject.Username}
	resultAction := ""
	if isRegistrationPlaceholder(subject.Username) {
		action = "iam_registration_invitation_reissued"
		details = map[string]any{"administrator": subject.Administrator}
		resultAction = "registration_invited"
	}
	entry := s.audit(actor.TenantID, auditID, action, "iam_subject", subjectID, actor.Actor(), now, details)
	if err := s.repository.CreateActivationToken(ctx, actor.TenantID, token, entry); err != nil {
		return TokenResult{}, err
	}
	return TokenResult{Action: resultAction, Subject: subject, Token: raw, ExpiresAt: expiresAt}, nil
}

func (s *Service) Activate(ctx context.Context, rawToken, password string) (Subject, error) {
	return s.consumeToken(ctx, rawToken, password, TokenInvitation, "iam_subject_activated")
}

func (s *Service) CreatePasswordReset(ctx context.Context, actor Principal, subjectID string) (TokenResult, error) {
	if err := requireAdministrator(actor); err != nil {
		return TokenResult{}, err
	}
	if !uuidPattern.MatchString(subjectID) {
		return TokenResult{}, &ValidationError{Field: "subject_id", Message: "Subject ID 无效"}
	}
	subject, found, err := s.repository.FindSubject(ctx, actor.TenantID, subjectID)
	if err != nil {
		return TokenResult{}, err
	}
	if !found || subject.SystemAdministrator {
		return TokenResult{}, &NotFoundError{Message: "用户不存在"}
	}
	now := s.now()
	raw, hash, err := randomToken()
	if err != nil {
		return TokenResult{}, err
	}
	tokenID, err := s.newID()
	if err != nil {
		return TokenResult{}, err
	}
	auditID, err := s.newID()
	if err != nil {
		return TokenResult{}, err
	}
	expiresAt := now.Add(time.Hour)
	token := ActivationToken{ID: tokenID, SubjectID: subjectID, Kind: TokenPasswordReset, TokenHash: hash, ExpiresAt: expiresAt, CreatedBy: actor.SubjectID, CreatedAt: now}
	if err := s.repository.CreateActivationToken(ctx, actor.TenantID, token, s.audit(actor.TenantID, auditID, "iam_password_reset_issued", "iam_subject", subjectID, actor.Actor(), now, nil)); err != nil {
		return TokenResult{}, err
	}
	return TokenResult{Subject: Subject{ID: subjectID}, Token: raw, ExpiresAt: expiresAt}, nil
}

func (s *Service) CompletePasswordReset(ctx context.Context, rawToken, password string) (Subject, error) {
	return s.consumeToken(ctx, rawToken, password, TokenPasswordReset, "iam_password_reset_completed")
}

func (s *Service) consumeToken(ctx context.Context, rawToken, password string, kind TokenKind, action string) (Subject, error) {
	hash, valid := tokenHash(rawToken)
	if !valid {
		return Subject{}, &AuthenticationError{}
	}
	passwordHash, err := s.hashPassword(password)
	if err != nil {
		return Subject{}, err
	}
	auditID, err := s.newID()
	if err != nil {
		return Subject{}, err
	}
	now := s.now()
	entry := s.audit("", auditID, action, "iam_subject", "pending-token-subject", AuditActor{Kind: "activation_token", ID: auditID, DisplayName: "一次性激活凭据"}, now, nil)
	return s.repository.Activate(ctx, hash, kind, passwordHash, now, entry)
}

func (s *Service) Grant(ctx context.Context, actor Principal, subjectID, audience, action string, scope Scope) error {
	if err := requireAdministrator(actor); err != nil {
		return err
	}
	if !uuidPattern.MatchString(subjectID) {
		return &ValidationError{Field: "subject_id", Message: "Subject ID 无效"}
	}
	subject, found, err := s.repository.FindSubject(ctx, actor.TenantID, subjectID)
	if err != nil {
		return err
	}
	if !found || subject.SystemAdministrator {
		return &NotFoundError{Message: "用户不存在"}
	}
	audience = strings.TrimSpace(audience)
	action = strings.TrimSpace(action)
	if !identifierPattern.MatchString(audience) || !actionPattern.MatchString(action) {
		return &ValidationError{Field: "action", Message: "Capability 或目标 Audience 格式无效"}
	}
	if !s.capabilities.Contains(audience, action) {
		return &ValidationError{Field: "action", Message: "Capability 未由目标应用清单发布"}
	}
	normalizedScope, err := normalizeScope(scope)
	if err != nil {
		return err
	}
	for _, resource := range normalizedScope.Resources {
		if !s.capabilities.AllowsResource(audience, action, resource.Type) {
			return &ValidationError{Field: "scope", Message: "Capability 未声明该资源类型"}
		}
	}
	now := s.now()
	auditID, err := s.newID()
	if err != nil {
		return err
	}
	grant := Grant{SubjectID: subjectID, Audience: audience, Action: action, Scope: normalizedScope, SourceKind: "manual", GrantedBy: actor.SubjectID, GrantedAt: now}
	return s.repository.Grant(ctx, actor.TenantID, grant, s.audit(actor.TenantID, auditID, "iam_capability_granted", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"audience": audience, "action": action, "scope": normalizedScope}))
}

func (s *Service) RevokeGrant(ctx context.Context, actor Principal, subjectID, audience, action string, scope Scope) error {
	if err := requireAdministrator(actor); err != nil {
		return err
	}
	if !uuidPattern.MatchString(subjectID) {
		return &ValidationError{Field: "subject_id", Message: "Subject ID 无效"}
	}
	audience = strings.TrimSpace(audience)
	action = strings.TrimSpace(action)
	if !identifierPattern.MatchString(audience) || !actionPattern.MatchString(action) {
		return &ValidationError{Field: "action", Message: "Capability 或目标 Audience 格式无效"}
	}
	if !s.capabilities.Contains(audience, action) {
		return &ValidationError{Field: "action", Message: "Capability 未由目标应用清单发布"}
	}
	normalizedScope, err := normalizeScope(scope)
	if err != nil {
		return err
	}
	for _, resource := range normalizedScope.Resources {
		if !s.capabilities.AllowsResource(audience, action, resource.Type) {
			return &ValidationError{Field: "scope", Message: "Capability 未声明该资源类型"}
		}
	}
	now := s.now()
	auditID, err := s.newID()
	if err != nil {
		return err
	}
	grant := Grant{SubjectID: subjectID, Audience: audience, Action: action, Scope: normalizedScope, GrantedBy: actor.SubjectID, GrantedAt: now}
	return s.repository.RevokeGrant(ctx, actor.TenantID, grant, s.audit(actor.TenantID, auditID, "iam_capability_revoked", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"audience": audience, "action": action, "scope": normalizedScope}))
}

func (s *Service) IssueCredential(ctx context.Context, principal Principal, audience string, scope Scope) (IssuedCredential, error) {
	return s.IssueCredentialWithFact(ctx, principal, audience, scope, nil)
}

func (s *Service) IssueCredentialWithFact(ctx context.Context, principal Principal, audience string, scope Scope, fact *BusinessFactReference) (IssuedCredential, error) {
	if principal.SystemAdministrator || !uuidPattern.MatchString(principal.TenantID) || !uuidPattern.MatchString(principal.SubjectID) {
		return IssuedCredential{}, &AuthorizationError{}
	}
	audience = strings.TrimSpace(audience)
	if !identifierPattern.MatchString(audience) {
		return IssuedCredential{}, &ValidationError{Field: "audience", Message: "Audience 无效"}
	}
	normalizedScope, err := normalizeScope(scope)
	if err != nil {
		return IssuedCredential{}, err
	}
	grants, err := s.repository.ListGrants(ctx, principal.TenantID, principal.SubjectID, audience)
	if err != nil {
		return IssuedCredential{}, err
	}
	actions := authorizedActions(s.capabilities, audience, normalizedScope, grants)
	var decision *PolicyDecision
	if fact != nil {
		if s.policy == nil {
			return IssuedCredential{}, &AuthorizationError{}
		}
		value, err := s.policy.Evaluate(ctx, principal, audience, normalizedScope, *fact)
		if err != nil {
			return IssuedCredential{}, err
		}
		if value.DecisionID == "" || value.PolicyID == "" || value.PolicyVersion == "" || value.FactID != fact.ID || value.FactRevision != fact.Revision || len(value.Actions) == 0 {
			return IssuedCredential{}, fmt.Errorf("policy evaluator returned an invalid decision")
		}
		for _, action := range value.Actions {
			if !s.capabilities.Contains(audience, action) {
				return IssuedCredential{}, fmt.Errorf("policy evaluator returned an unknown capability")
			}
			allowedForScope := normalizedScope.Kind == "tenant"
			for _, resource := range normalizedScope.Resources {
				allowedForScope = s.capabilities.AllowsResource(audience, action, resource.Type)
				if !allowedForScope {
					break
				}
			}
			if !allowedForScope {
				return IssuedCredential{}, fmt.Errorf("policy evaluator returned a capability outside the requested scope")
			}
			actions = append(actions, action)
		}
		sort.Strings(actions)
		actions = compactStrings(actions)
		value.Actions = append([]string(nil), value.Actions...)
		sort.Strings(value.Actions)
		decision = &value
	}
	if len(actions) == 0 {
		return IssuedCredential{}, &AuthorizationError{}
	}
	jti, err := s.newID()
	if err != nil {
		return IssuedCredential{}, err
	}
	now := s.now()
	expiresAt := now.Add(s.credentialTTL)
	claims := CapabilityClaims{Version: "1", Issuer: s.issuer, Subject: principal.SubjectID, Tenant: principal.TenantID, Audience: audience, Name: principal.DisplayName, Actions: actions, Scope: normalizedScope, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: expiresAt.Unix(), AuthTime: principal.AuthTime.Unix(), ID: jti}
	token, err := s.signer.Sign(claims)
	if err != nil {
		return IssuedCredential{}, fmt.Errorf("sign capability credential: %w", err)
	}
	if decision != nil {
		auditID, err := s.newID()
		if err != nil {
			return IssuedCredential{}, err
		}
		if err := s.repository.RecordAudit(ctx, s.audit(principal.TenantID, auditID, "iam_policy_credential_issued", "capability_credential", jti, principal.Actor(), now, map[string]any{
			"audience": audience, "actions": actions, "scope": normalizedScope, "policy_decision": decision,
		})); err != nil {
			return IssuedCredential{}, err
		}
	}
	return IssuedCredential{Token: token, TokenType: "Bearer", ExpiresAt: expiresAt, Claims: claims, Decision: decision}, nil
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func normalizeScope(scope Scope) (Scope, error) {
	if scope.Kind == "" {
		scope.Kind = "tenant"
	}
	if scope.Kind == "tenant" {
		if len(scope.Resources) != 0 {
			return Scope{}, &ValidationError{Field: "scope", Message: "Tenant Scope 不能包含资源"}
		}
		return Scope{Kind: "tenant", Resources: []ResourceRef{}}, nil
	}
	if scope.Kind != "resources" || len(scope.Resources) == 0 || len(scope.Resources) > 32 {
		return Scope{}, &ValidationError{Field: "scope", Message: "Resource Scope 必须包含 1 至 32 个资源"}
	}
	resources := make([]ResourceRef, 0, len(scope.Resources))
	seen := map[string]struct{}{}
	for _, resource := range scope.Resources {
		resource.Type = strings.TrimSpace(resource.Type)
		resource.ID = strings.TrimSpace(resource.ID)
		if !resourcePattern.MatchString(resource.Type) || resource.ID == "" || len(resource.ID) > 128 {
			return Scope{}, &ValidationError{Field: "scope", Message: "Resource Scope 包含无效资源"}
		}
		key := resource.Type + "\x00" + resource.ID
		if _, duplicate := seen[key]; duplicate {
			return Scope{}, &ValidationError{Field: "scope", Message: "Resource Scope 包含重复资源"}
		}
		seen[key] = struct{}{}
		resources = append(resources, resource)
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Type == resources[j].Type {
			return resources[i].ID < resources[j].ID
		}
		return resources[i].Type < resources[j].Type
	})
	return Scope{Kind: "resources", Resources: resources}, nil
}

func authorizedActions(catalog CapabilityCatalog, audience string, scope Scope, grants []Grant) []string {
	tenantActions := map[string]bool{}
	resourceActions := map[string]map[string]bool{}
	for _, grant := range grants {
		if grant.Scope.Kind == "tenant" {
			tenantActions[grant.Action] = true
			continue
		}
		for _, resource := range grant.Scope.Resources {
			if resourceActions[grant.Action] == nil {
				resourceActions[grant.Action] = map[string]bool{}
			}
			resourceActions[grant.Action][resource.Type+"\x00"+resource.ID] = true
		}
	}
	actions := make([]string, 0, len(grants))
	if scope.Kind == "tenant" {
		for action := range tenantActions {
			actions = append(actions, action)
		}
	} else {
		candidates := map[string]bool{}
		for action := range tenantActions {
			candidates[action] = true
		}
		for action := range resourceActions {
			candidates[action] = true
		}
		for action := range candidates {
			allowed := true
			for _, resource := range scope.Resources {
				if !catalog.AllowsResource(audience, action, resource.Type) || (!tenantActions[action] && !resourceActions[action][resource.Type+"\x00"+resource.ID]) {
					allowed = false
					break
				}
			}
			if allowed {
				actions = append(actions, action)
			}
		}
	}
	sort.Strings(actions)
	return actions
}

func (s *Service) ListSubjects(ctx context.Context, actor Principal) ([]Subject, error) {
	if err := requireAdministrator(actor); err != nil {
		return nil, err
	}
	return s.repository.ListSubjects(ctx, actor.TenantID)
}

func (s *Service) UpdateSubjectDisplayName(ctx context.Context, actor Principal, subjectID string, expectedRevision int64, rawDisplayName string) error {
	if err := requireAdministrator(actor); err != nil {
		return err
	}
	if !uuidPattern.MatchString(subjectID) {
		return &ValidationError{Field: "subject_id", Message: "Subject ID 无效"}
	}
	if expectedRevision < 1 {
		return &ValidationError{Field: "expected_revision", Message: "账号 revision 无效"}
	}
	displayName := strings.TrimSpace(rawDisplayName)
	if displayName == "" {
		return &ValidationError{Field: "display_name", Message: "姓名不能为空"}
	}
	var err error
	displayName, err = normalizeDisplayName(displayName, "")
	if err != nil {
		return err
	}
	auditID, err := s.newID()
	if err != nil {
		return err
	}
	return s.repository.UpdateSubjectDisplayName(ctx, actor.TenantID, subjectID, expectedRevision, displayName, s.audit(actor.TenantID, auditID, "iam_subject_display_name_changed", "iam_subject", subjectID, actor.Actor(), s.now(), map[string]any{"display_name": displayName, "expected_revision": expectedRevision}))
}

func (s *Service) ChangeStatus(ctx context.Context, actor Principal, subjectID string, status Status) error {
	if err := requireAdministrator(actor); err != nil {
		return err
	}
	if subjectID == actor.SubjectID && status == StatusDisabled {
		return &ConflictError{Message: "不能停用当前管理员"}
	}
	if !uuidPattern.MatchString(subjectID) || (status != StatusActive && status != StatusDisabled) {
		return &ValidationError{Field: "status", Message: "账号状态无效"}
	}
	now := s.now()
	auditID, err := s.newID()
	if err != nil {
		return err
	}
	return s.repository.ChangeStatus(ctx, actor.TenantID, subjectID, status, s.audit(actor.TenantID, auditID, "iam_subject_status_changed", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"status": status}))
}

func (s *Service) ChangeAdministrator(ctx context.Context, actor Principal, subjectID string, administrator bool) error {
	if err := requireAdministrator(actor); err != nil {
		return err
	}
	if !uuidPattern.MatchString(subjectID) {
		return &ValidationError{Field: "subject_id", Message: "Subject ID 无效"}
	}
	if subjectID == actor.SubjectID && !administrator {
		return &ConflictError{Message: "不能撤销当前管理员自己的管理员身份"}
	}
	now := s.now()
	auditID, err := s.newID()
	if err != nil {
		return err
	}
	if err := s.repository.ChangeAdministrator(ctx, actor.TenantID, subjectID, administrator, administratorProfileSource, s.audit(actor.TenantID, auditID, "iam_subject_administrator_changed", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"administrator": administrator})); err != nil {
		return err
	}
	if !administrator {
		return nil
	}
	subject, found, err := s.repository.FindSubject(ctx, actor.TenantID, subjectID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("administrator subject disappeared after promotion")
	}
	if subject.Status != StatusActive {
		return nil
	}
	subject.Administrator = true
	return s.ensureAdministratorProfile(ctx, subject)
}

func (s *Service) UpdateTenantName(ctx context.Context, actor Principal, rawName string) error {
	if err := requireAdministrator(actor); err != nil {
		return err
	}
	name, err := normalizeTenantName(rawName)
	if err != nil {
		return err
	}
	auditID, err := s.newID()
	if err != nil {
		return err
	}
	return s.repository.UpdateTenantName(ctx, actor.TenantID, name, s.audit(actor.TenantID, auditID, "iam_tenant_name_changed", "tenant", actor.TenantID, actor.Actor(), s.now(), map[string]any{"name": name}))
}

func (s *Service) ListTenants(ctx context.Context, actor Principal) ([]TenantSummary, error) {
	if err := requireSystemAdministrator(actor); err != nil {
		return nil, err
	}
	return s.repository.ListTenants(ctx)
}

func (s *Service) CreateTenant(ctx context.Context, actor Principal, input SystemTenantInput) (TokenResult, TenantSummary, error) {
	if err := requireSystemAdministrator(actor); err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	name, err := normalizeTenantName(input.Name)
	if err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	username, err := normalizeUsername(input.AdminUsername)
	if err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	displayName, err := normalizeDisplayName(input.AdminDisplayName, username)
	if err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	existing, found, err := s.repository.FindCredential(ctx, username)
	if err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	if found && existing.SystemAdministrator {
		return TokenResult{}, TenantSummary{}, &ConflictError{Message: "系统管理员账号不能加入业务组织"}
	}
	if found && existing.Status != StatusActive {
		return TokenResult{}, TenantSummary{}, &ConflictError{Message: "该登录账号尚未激活或当前不可用"}
	}
	tenantID, err := s.newID()
	if err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	subjectID, err := s.newID()
	if err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	now := s.now()
	if found {
		auditID, err := s.newID()
		if err != nil {
			return TokenResult{}, TenantSummary{}, err
		}
		subject := Subject{ID: subjectID, AccountID: existing.AccountID, TenantID: tenantID, Username: username, DisplayName: displayName, Status: StatusActive, Administrator: true, Revision: 1, SessionGeneration: 1, PasswordChangedAt: existing.PasswordChangedAt, CreatedAt: now, UpdatedAt: now}
		audit := s.audit(tenantID, auditID, "iam_tenant_created_with_existing_account", "tenant", tenantID, actor.Actor(), now, map[string]any{"tenant_name": name, "administrator_subject_id": subjectID, "account_id": existing.AccountID})
		if err := s.repository.ProvisionTenantMembership(ctx, tenantID, name, subject, audit); err != nil {
			return TokenResult{}, TenantSummary{}, err
		}
		if err := s.ensureAdministratorProfile(ctx, subject); err != nil {
			return TokenResult{}, TenantSummary{}, err
		}
		return TokenResult{Action: "joined", Subject: subject}, TenantSummary{ID: tenantID, Name: name, Status: "active", UserCount: 1, AdministratorCount: 1, CreatedAt: now}, nil
	}
	tokenID, err := s.newID()
	if err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	auditID, err := s.newID()
	if err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	raw, hash, err := randomToken()
	if err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	subject := Subject{ID: subjectID, TenantID: tenantID, Username: username, DisplayName: displayName, Status: StatusPending, Administrator: true, Revision: 1, SessionGeneration: 1, CreatedAt: now, UpdatedAt: now}
	token := ActivationToken{ID: tokenID, SubjectID: subjectID, Kind: TokenInvitation, TokenHash: hash, ExpiresAt: now.Add(24 * time.Hour), CreatedBy: actor.SubjectID, CreatedAt: now}
	audit := s.audit(tenantID, auditID, "iam_tenant_created", "tenant", tenantID, actor.Actor(), now, map[string]any{"tenant_name": name, "administrator_subject_id": subjectID})
	if err := s.repository.ProvisionTenantInvitation(ctx, tenantID, name, subject, token, audit); err != nil {
		return TokenResult{}, TenantSummary{}, err
	}
	return TokenResult{Subject: subject, Token: raw, ExpiresAt: token.ExpiresAt}, TenantSummary{ID: tenantID, Name: name, Status: "active", UserCount: 1, AdministratorCount: 1, CreatedAt: now}, nil
}

func (s *Service) AssignTenantAdministrator(ctx context.Context, actor Principal, tenantID, username, displayName string) (TenantAdministratorResult, error) {
	if err := requireSystemAdministrator(actor); err != nil {
		return TenantAdministratorResult{}, err
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if !uuidPattern.MatchString(tenantID) {
		return TenantAdministratorResult{}, &ValidationError{Field: "tenant_id", Message: "组织 ID 无效"}
	}
	exists, err := s.repository.TenantExists(ctx, tenantID)
	if err != nil {
		return TenantAdministratorResult{}, err
	}
	if !exists {
		return TenantAdministratorResult{}, &NotFoundError{Message: "组织不存在"}
	}
	normalized, err := normalizeUsername(username)
	if err != nil {
		return TenantAdministratorResult{}, err
	}
	existing, found, err := s.repository.FindCredential(ctx, normalized)
	if err != nil {
		return TenantAdministratorResult{}, err
	}
	if found {
		if existing.SystemAdministrator {
			return TenantAdministratorResult{}, &ConflictError{Message: "系统管理员账号不能设为组织管理员"}
		}
		if existing.Status != StatusActive {
			return TenantAdministratorResult{}, &ConflictError{Message: "该账号尚未激活或已停用，请先在组织账号管理中处理"}
		}
		membership, joined, err := s.repository.FindAccountSubject(ctx, tenantID, existing.AccountID)
		if err != nil {
			return TenantAdministratorResult{}, err
		}
		if !joined {
			displayName, err = normalizeDisplayName(displayName, normalized)
			if err != nil {
				return TenantAdministratorResult{}, err
			}
			subjectID, err := s.newID()
			if err != nil {
				return TenantAdministratorResult{}, err
			}
			auditID, err := s.newID()
			if err != nil {
				return TenantAdministratorResult{}, err
			}
			now := s.now()
			membership = Subject{ID: subjectID, AccountID: existing.AccountID, TenantID: tenantID, Username: normalized, DisplayName: displayName, Status: StatusActive, Administrator: true, Revision: 1, SessionGeneration: 1, PasswordChangedAt: existing.PasswordChangedAt, CreatedAt: now, UpdatedAt: now}
			audit := s.audit(tenantID, auditID, "iam_tenant_administrator_joined", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"login_identifier": normalized, "account_id": existing.AccountID})
			if err := s.repository.JoinAccount(ctx, membership, audit); err != nil {
				return TenantAdministratorResult{}, err
			}
			if err := s.ensureAdministratorProfile(ctx, membership); err != nil {
				return TenantAdministratorResult{}, err
			}
			return TenantAdministratorResult{Action: "joined", Subject: membership}, nil
		}
		if membership.Status != StatusActive {
			return TenantAdministratorResult{}, &ConflictError{Message: "该账号在本组织尚未激活或已停用，请先在组织账号管理中处理"}
		}
		if membership.Administrator {
			if err := s.ensureAdministratorProfile(ctx, membership); err != nil {
				return TenantAdministratorResult{}, err
			}
			return TenantAdministratorResult{Action: "already_administrator", Subject: membership}, nil
		}
		now := s.now()
		auditID, err := s.newID()
		if err != nil {
			return TenantAdministratorResult{}, err
		}
		audit := s.audit(tenantID, auditID, "iam_tenant_administrator_promoted", "iam_subject", membership.ID, actor.Actor(), now, map[string]any{"login_identifier": normalized})
		if err := s.repository.ChangeAdministrator(ctx, tenantID, membership.ID, true, administratorProfileSource, audit); err != nil {
			return TenantAdministratorResult{}, err
		}
		membership.Administrator = true
		membership.Revision++
		membership.SessionGeneration++
		membership.UpdatedAt = now
		if err := s.ensureAdministratorProfile(ctx, membership); err != nil {
			return TenantAdministratorResult{}, err
		}
		return TenantAdministratorResult{Action: "promoted", Subject: membership}, nil
	}
	displayName, err = normalizeDisplayName(displayName, normalized)
	if err != nil {
		return TenantAdministratorResult{}, err
	}
	subjectID, err := s.newID()
	if err != nil {
		return TenantAdministratorResult{}, err
	}
	tokenID, err := s.newID()
	if err != nil {
		return TenantAdministratorResult{}, err
	}
	auditID, err := s.newID()
	if err != nil {
		return TenantAdministratorResult{}, err
	}
	raw, hash, err := randomToken()
	if err != nil {
		return TenantAdministratorResult{}, err
	}
	now := s.now()
	subject := Subject{ID: subjectID, TenantID: tenantID, Username: normalized, DisplayName: displayName, Status: StatusPending, Administrator: true, Revision: 1, SessionGeneration: 1, CreatedAt: now, UpdatedAt: now}
	token := ActivationToken{ID: tokenID, SubjectID: subjectID, Kind: TokenInvitation, TokenHash: hash, ExpiresAt: now.Add(24 * time.Hour), CreatedBy: actor.SubjectID, CreatedAt: now}
	audit := s.audit(tenantID, auditID, "iam_tenant_administrator_invited", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"login_identifier": normalized})
	if err := s.repository.Invite(ctx, subject, token, audit); err != nil {
		return TenantAdministratorResult{}, err
	}
	return TenantAdministratorResult{Action: "invited", Subject: subject, Token: raw, ExpiresAt: &token.ExpiresAt}, nil
}

// ReconcileAdministratorProfiles applies the locked OMS administrator profile
// to every active organization administrator and removes only grants that were
// sourced from that profile after a user is no longer an administrator. It is
// safe to run at every IAM start and after capability manifest changes.
func (s *Service) ReconcileAdministratorProfiles(ctx context.Context) error {
	tenants, err := s.repository.ListTenants(ctx)
	if err != nil {
		return err
	}
	for _, tenant := range tenants {
		subjects, err := s.repository.ListSubjects(ctx, tenant.ID)
		if err != nil {
			return err
		}
		for _, subject := range subjects {
			if subject.Status == StatusActive && subject.Administrator {
				if err := s.ensureAdministratorProfile(ctx, subject); err != nil {
					return err
				}
				continue
			}
			if _, err := s.revokeAdministratorProfile(ctx, subject); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) ensureAdministratorProfile(ctx context.Context, subject Subject) error {
	if !subject.Administrator || subject.SystemAdministrator || subject.Status != StatusActive {
		return nil
	}
	actor := AuditActor{Kind: "service_identity", ID: "iam-administrator-profile-reconciler", DisplayName: "OMS 管理员权限模板"}
	catalog := s.capabilities.Actions()
	audiences := make([]string, 0, len(catalog))
	for audience := range catalog {
		audiences = append(audiences, audience)
	}
	sort.Strings(audiences)
	for _, audience := range audiences {
		actions := catalog[audience]
		for _, action := range actions {
			auditID, err := s.newID()
			if err != nil {
				return err
			}
			now := s.now()
			grant := Grant{
				SubjectID: subject.ID, Audience: audience, Action: action,
				Scope: Scope{Kind: "tenant", Resources: []ResourceRef{}}, SourceKind: administratorProfileSource,
				GrantedBy: subject.ID, GrantedAt: now,
			}
			entry := s.audit(subject.TenantID, auditID, "iam_administrator_profile_capability_granted", "iam_subject", subject.ID, actor, now, map[string]any{
				"profile_id": "oms-administrator-full-access-v1", "audience": audience, "action": action,
			})
			if err := s.repository.Grant(ctx, subject.TenantID, grant, entry); err != nil {
				var conflict *ConflictError
				if errors.As(err, &conflict) {
					continue
				}
				return err
			}
		}
	}
	return nil
}

func (s *Service) revokeAdministratorProfile(ctx context.Context, subject Subject) (int64, error) {
	auditID, err := s.newID()
	if err != nil {
		return 0, err
	}
	now := s.now()
	actor := AuditActor{Kind: "service_identity", ID: "iam-administrator-profile-reconciler", DisplayName: "OMS 管理员权限模板"}
	entry := s.audit(subject.TenantID, auditID, "iam_administrator_profile_revoked", "iam_subject", subject.ID, actor, now, map[string]any{"profile_id": "oms-administrator-full-access-v1"})
	return s.repository.RevokeAdministratorProfile(ctx, subject.TenantID, subject.ID, administratorProfileSource, entry)
}

func (s *Service) audit(tenantID, id, action, objectType, objectID string, actor AuditActor, at time.Time, details map[string]any) AuditEntry {
	if details == nil {
		details = map[string]any{}
	}
	return AuditEntry{ID: id, TenantID: tenantID, Action: action, ObjectType: objectType, ObjectID: objectID, Actor: actor, OccurredAt: at, Details: details}
}

func (s *Service) hashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 72 {
		return "", &ValidationError{Field: "password", Message: "密码必须为 12 至 72 字节"}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func normalizeUsername(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	phone := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(value)
	if strings.HasPrefix(phone, "+86") {
		phone = strings.TrimPrefix(phone, "+86")
	} else if strings.HasPrefix(phone, "0086") {
		phone = strings.TrimPrefix(phone, "0086")
	}
	if phonePattern.MatchString(phone) {
		return phone, nil
	}
	// A phone-like value must never fall through to the legacy username rule.
	// Keep short historical numeric usernames compatible, while rejecting
	// mistyped phone numbers that would otherwise create an unreachable account.
	if phoneLikePattern.MatchString(value) && len(phone) >= 7 {
		return "", &ValidationError{Field: "username", Message: "请输入有效的中国大陆手机号"}
	}
	if !(len(value) <= 254 && emailPattern.MatchString(value)) && !usernamePattern.MatchString(value) {
		return "", &ValidationError{Field: "username", Message: "登录账号必须是有效手机号、邮箱或 3 至 64 位用户名"}
	}
	return value, nil
}

func normalizeTenantName(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len([]rune(value)) > 100 {
		return "", &ValidationError{Field: "name", Message: "组织名称必须为 1 至 100 个字符"}
	}
	return value, nil
}
func normalizeDisplayName(raw, fallback string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}
	if len([]rune(value)) > 100 {
		return "", &ValidationError{Field: "display_name", Message: "显示名不能超过 100 个字符"}
	}
	return value, nil
}
func isRegistrationPlaceholder(value string) bool {
	return strings.HasPrefix(value, "invite-") && len(value) == len("invite-")+32
}
func requireAdministrator(actor Principal) error {
	if !uuidPattern.MatchString(actor.TenantID) || !uuidPattern.MatchString(actor.SubjectID) || !actor.Administrator || actor.SystemAdministrator {
		return &AuthorizationError{}
	}
	return nil
}
func requireSystemAdministrator(actor Principal) error {
	if !uuidPattern.MatchString(actor.SubjectID) || !actor.SystemAdministrator || actor.Administrator {
		return &AuthorizationError{}
	}
	return nil
}
func principalFromCredential(value Credential, authTime time.Time) Principal {
	return Principal{SubjectID: value.ID, AccountID: value.AccountID, TenantID: value.TenantID, Username: value.Username, DisplayName: value.DisplayName, Administrator: value.Administrator, SystemAdministrator: value.SystemAdministrator, SessionGeneration: value.SessionGeneration, AuthTime: authTime}
}

func randomToken() (string, []byte, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(data)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}
func tokenHash(raw string) ([]byte, bool) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) != 32 {
		return nil, false
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:], true
}
func randomUUID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", data[0:4], data[4:6], data[6:8], data[8:10], data[10:16]), nil
}
