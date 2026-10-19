package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
	actionPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
	resourcePattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	uuidPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

type Subject struct {
	ID                  string    `json:"id"`
	TenantID            string    `json:"tenant_id"`
	Username            string    `json:"username"`
	DisplayName         string    `json:"display_name"`
	Status              Status    `json:"status"`
	Administrator       bool      `json:"administrator"`
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
	SubjectID         string    `json:"subject_id"`
	TenantID          string    `json:"tenant_id"`
	Username          string    `json:"username"`
	DisplayName       string    `json:"display_name"`
	Administrator     bool      `json:"administrator"`
	SessionGeneration int64     `json:"-"`
	AuthTime          time.Time `json:"auth_time"`
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

type ActivationToken struct {
	ID        string
	SubjectID string
	Kind      TokenKind
	TokenHash []byte
	ExpiresAt time.Time
	CreatedBy string
	CreatedAt time.Time
}

type Grant struct {
	SubjectID string    `json:"subject_id"`
	Audience  string    `json:"audience"`
	Action    string    `json:"action"`
	Scope     Scope     `json:"scope"`
	GrantedBy string    `json:"granted_by"`
	GrantedAt time.Time `json:"granted_at"`
}

type Repository interface {
	HasSubjects(context.Context) (bool, error)
	Bootstrap(context.Context, string, string, Subject, string, AuditEntry) (bool, error)
	FindCredential(context.Context, string, string) (Credential, bool, error)
	RecordLoginFailure(context.Context, string, time.Time, int, time.Duration) error
	ResetLoginFailures(context.Context, string, time.Time) error
	CreateSession(context.Context, []byte, string, int64, time.Time, time.Time) error
	ResolveSession(context.Context, []byte, time.Time) (Principal, bool, error)
	RevokeSession(context.Context, []byte, time.Time) error
	Invite(context.Context, Subject, ActivationToken, AuditEntry) error
	Activate(context.Context, []byte, TokenKind, string, time.Time, AuditEntry) (Subject, error)
	CreateActivationToken(context.Context, ActivationToken, AuditEntry) error
	Grant(context.Context, Grant, AuditEntry) error
	RevokeGrant(context.Context, Grant, AuditEntry) error
	ListGrants(context.Context, string, string) ([]Grant, error)
	RecordAudit(context.Context, AuditEntry) error
	ListSubjects(context.Context, string) ([]Subject, error)
	ChangeStatus(context.Context, string, string, Status, AuditEntry) error
}

type Signer interface {
	Sign(any) (string, error)
}

type CapabilityCatalog interface {
	Contains(audience, action string) bool
	AllowsResource(audience, action, resourceType string) bool
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
	Subject   Subject   `json:"subject"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
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

func (s *Service) IssueWorkloadCredential(ctx context.Context, workload ServiceIdentity, audience, action string, scope Scope) (IssuedCredential, error) {
	if !identifierPattern.MatchString(workload.ID) || !uuidPattern.MatchString(workload.SubjectID) || strings.TrimSpace(workload.DisplayName) == "" || !uuidPattern.MatchString(workload.KeyID) || !uuidPattern.MatchString(workload.AssertionID) || workload.AuthTime.IsZero() {
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
	claims := CapabilityClaims{Version: "1", Issuer: s.issuer, Subject: workload.SubjectID, Tenant: s.tenantID, Audience: audience, Name: workload.DisplayName, Actions: []string{action}, Scope: normalizedScope, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: expiresAt.Unix(), AuthTime: workload.AuthTime.Unix(), ID: jti}
	token, err := s.signer.Sign(claims)
	if err != nil {
		return IssuedCredential{}, fmt.Errorf("sign workload capability credential: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return IssuedCredential{}, err
	}
	actor := AuditActor{Kind: "service_identity", ID: workload.SubjectID, DisplayName: workload.DisplayName}
	if err := s.repository.RecordAudit(ctx, s.audit(auditID, "iam_service_credential_issued", "capability_credential", jti, actor, now, map[string]any{
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
	subject := Subject{ID: subjectID, TenantID: s.tenantID, Username: username, DisplayName: displayName, Status: StatusActive, Administrator: true, SessionGeneration: 1, PasswordChangedAt: now, CreatedAt: now, UpdatedAt: now}
	return s.repository.Bootstrap(ctx, s.tenantID, s.tenantName, subject, passwordHash, AuditEntry{
		ID: auditID, TenantID: s.tenantID, Action: "iam_subject_bootstrapped", ObjectType: "iam_subject", ObjectID: subjectID,
		Actor: AuditActor{Kind: "bootstrap_configuration", ID: subjectID, DisplayName: displayName}, OccurredAt: now,
		Details: map[string]any{"username": username, "administrator": true},
	})
}

func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	normalized, normalizeErr := normalizeUsername(username)
	credential, found, err := s.repository.FindCredential(ctx, s.tenantID, normalized)
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
	if found && principal.TenantID != s.tenantID {
		return Principal{}, false, fmt.Errorf("session tenant mismatch")
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

func (s *Service) Invite(ctx context.Context, actor Principal, username, displayName string) (TokenResult, error) {
	if err := requireAdministrator(actor, s.tenantID); err != nil {
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
	subject := Subject{ID: subjectID, TenantID: s.tenantID, Username: normalized, DisplayName: displayName, Status: StatusPending, SessionGeneration: 1, CreatedAt: now, UpdatedAt: now}
	expiresAt := now.Add(24 * time.Hour)
	activation := ActivationToken{ID: tokenID, SubjectID: subjectID, Kind: TokenInvitation, TokenHash: hash, ExpiresAt: expiresAt, CreatedBy: actor.SubjectID, CreatedAt: now}
	err = s.repository.Invite(ctx, subject, activation, s.audit(auditID, "iam_subject_invited", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"username": normalized}))
	if err != nil {
		return TokenResult{}, err
	}
	return TokenResult{Subject: subject, Token: raw, ExpiresAt: expiresAt}, nil
}

func (s *Service) Activate(ctx context.Context, rawToken, password string) (Subject, error) {
	return s.consumeToken(ctx, rawToken, password, TokenInvitation, "iam_subject_activated")
}

func (s *Service) CreatePasswordReset(ctx context.Context, actor Principal, subjectID string) (TokenResult, error) {
	if err := requireAdministrator(actor, s.tenantID); err != nil {
		return TokenResult{}, err
	}
	if !uuidPattern.MatchString(subjectID) {
		return TokenResult{}, &ValidationError{Field: "subject_id", Message: "Subject ID 无效"}
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
	if err := s.repository.CreateActivationToken(ctx, token, s.audit(auditID, "iam_password_reset_issued", "iam_subject", subjectID, actor.Actor(), now, nil)); err != nil {
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
	entry := s.audit(auditID, action, "iam_subject", "pending-token-subject", AuditActor{Kind: "activation_token", ID: auditID, DisplayName: "一次性激活凭据"}, now, nil)
	return s.repository.Activate(ctx, hash, kind, passwordHash, now, entry)
}

func (s *Service) Grant(ctx context.Context, actor Principal, subjectID, audience, action string, scope Scope) error {
	if err := requireAdministrator(actor, s.tenantID); err != nil {
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
	return s.repository.Grant(ctx, grant, s.audit(auditID, "iam_capability_granted", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"audience": audience, "action": action, "scope": normalizedScope}))
}

func (s *Service) RevokeGrant(ctx context.Context, actor Principal, subjectID, audience, action string, scope Scope) error {
	if err := requireAdministrator(actor, s.tenantID); err != nil {
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
	return s.repository.RevokeGrant(ctx, grant, s.audit(auditID, "iam_capability_revoked", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"audience": audience, "action": action, "scope": normalizedScope}))
}

func (s *Service) IssueCredential(ctx context.Context, principal Principal, audience string, scope Scope) (IssuedCredential, error) {
	return s.IssueCredentialWithFact(ctx, principal, audience, scope, nil)
}

func (s *Service) IssueCredentialWithFact(ctx context.Context, principal Principal, audience string, scope Scope, fact *BusinessFactReference) (IssuedCredential, error) {
	if principal.TenantID != s.tenantID || !uuidPattern.MatchString(principal.SubjectID) {
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
	grants, err := s.repository.ListGrants(ctx, principal.SubjectID, audience)
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
	claims := CapabilityClaims{Version: "1", Issuer: s.issuer, Subject: principal.SubjectID, Tenant: s.tenantID, Audience: audience, Name: principal.DisplayName, Actions: actions, Scope: normalizedScope, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: expiresAt.Unix(), AuthTime: principal.AuthTime.Unix(), ID: jti}
	token, err := s.signer.Sign(claims)
	if err != nil {
		return IssuedCredential{}, fmt.Errorf("sign capability credential: %w", err)
	}
	if decision != nil {
		auditID, err := s.newID()
		if err != nil {
			return IssuedCredential{}, err
		}
		if err := s.repository.RecordAudit(ctx, s.audit(auditID, "iam_policy_credential_issued", "capability_credential", jti, principal.Actor(), now, map[string]any{
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
	if err := requireAdministrator(actor, s.tenantID); err != nil {
		return nil, err
	}
	return s.repository.ListSubjects(ctx, s.tenantID)
}

func (s *Service) ChangeStatus(ctx context.Context, actor Principal, subjectID string, status Status) error {
	if err := requireAdministrator(actor, s.tenantID); err != nil {
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
	return s.repository.ChangeStatus(ctx, s.tenantID, subjectID, status, s.audit(auditID, "iam_subject_status_changed", "iam_subject", subjectID, actor.Actor(), now, map[string]any{"status": status}))
}

func (s *Service) audit(id, action, objectType, objectID string, actor AuditActor, at time.Time, details map[string]any) AuditEntry {
	if details == nil {
		details = map[string]any{}
	}
	return AuditEntry{ID: id, TenantID: s.tenantID, Action: action, ObjectType: objectType, ObjectID: objectID, Actor: actor, OccurredAt: at, Details: details}
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
	if !usernamePattern.MatchString(value) {
		return "", &ValidationError{Field: "username", Message: "用户名必须为 3 至 64 位小写字母、数字或 ._-"}
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
func requireAdministrator(actor Principal, tenantID string) error {
	if actor.TenantID != tenantID || !actor.Administrator {
		return &AuthorizationError{}
	}
	return nil
}
func principalFromCredential(value Credential, authTime time.Time) Principal {
	return Principal{SubjectID: value.ID, TenantID: value.TenantID, Username: value.Username, DisplayName: value.DisplayName, Administrator: value.Administrator, SessionGeneration: value.SessionGeneration, AuthTime: authTime}
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
