package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"omsbizfoundry/money-flow-ledger/internal/audit"
)

type Role string

const RoleAdministrator Role = "administrator"

type Permission string

const (
	PermissionView                      Permission = "view"
	PermissionPartyRegister             Permission = "party.register"
	PermissionObligationManage          Permission = "obligation.manage"
	PermissionCashRecord                Permission = "cash.record"
	PermissionCashReverse               Permission = "cash.reverse"
	PermissionCashEvidenceManage        Permission = "cash.evidence.manage"
	PermissionAllocationManage          Permission = "allocation.manage"
	PermissionTransferRecord            Permission = "transfer.record"
	PermissionExport                    Permission = "export"
	PermissionStatisticsRead            Permission = "statistics.read"
	PermissionAmountRead                Permission = "amount.read"
	PermissionReferenceRead             Permission = "reference.read"
	PermissionReceiptSetConfirm         Permission = "receipt_set.confirm"
	PermissionReceiptSetReferenceRead   Permission = "receipt_set.reference.read"
	PermissionPayableSuggestionSubmit   Permission = "payable_suggestion.submit"
	PermissionPayableSuggestionRead     Permission = "payable_suggestion.read"
	PermissionPayableSuggestionWithdraw Permission = "payable_suggestion.withdraw"
	PermissionPayableSuggestionDecide   Permission = "payable_suggestion.decide"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"

	ActionUserBootstrapped = "local_user_bootstrapped"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

type User struct {
	ID                  string
	Username            string
	DisplayName         string
	Role                Role
	Status              Status
	SessionGeneration   int64
	FailedLoginAttempts int
	LockedUntil         time.Time
	PasswordChangedAt   time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Credential struct {
	User
	PasswordHash string
}

type Principal struct {
	UserID            string
	Username          string
	DisplayName       string
	Role              Role
	SessionGeneration int64
	Provider          string
	TenantID          string
	CredentialID      string
	AllowedActions    map[string]bool
	ScopeKind         string
	Resources         map[string]map[string]bool
}

func (p Principal) Can(permission Permission) bool {
	if p.Provider == "iam" {
		action := map[Permission]string{
			PermissionView: "money-flow-ledger.view", PermissionPartyRegister: "money-flow-ledger.party.register",
			PermissionObligationManage: "money-flow-ledger.obligation.manage", PermissionCashRecord: "money-flow-ledger.cash.record",
			PermissionCashReverse: "money-flow-ledger.cash.reverse", PermissionAllocationManage: "money-flow-ledger.allocation.manage",
			PermissionCashEvidenceManage: "money-flow-ledger.cash.evidence.manage",
			PermissionTransferRecord:     "money-flow-ledger.transfer.record", PermissionExport: "money-flow-ledger.export",
			PermissionStatisticsRead: "money-flow-ledger.statistics.read", PermissionAmountRead: "money-flow-ledger.amount.read",
			PermissionReferenceRead:             "money-flow-ledger.reference.read",
			PermissionReceiptSetConfirm:         "money-flow-ledger.receipt-set.confirm",
			PermissionReceiptSetReferenceRead:   "money-flow-ledger.receipt-set.reference.read",
			PermissionPayableSuggestionSubmit:   "money-flow-ledger.payable-suggestion.submit",
			PermissionPayableSuggestionRead:     "money-flow-ledger.payable-suggestion.read",
			PermissionPayableSuggestionWithdraw: "money-flow-ledger.payable-suggestion.withdraw",
			PermissionPayableSuggestionDecide:   "money-flow-ledger.payable-suggestion.decide",
		}[permission]
		return action != "" && p.AllowedActions[action]
	}
	return p.Role == RoleAdministrator
}

func (p Principal) CanReference(resourceType, resourceID string) bool {
	return p.Provider == "iam" && p.ScopeKind == "resources" && p.Resources[resourceType][resourceID]
}

func (p Principal) Actor() audit.ActorRef {
	kind := audit.ActorKindLocalUser
	if p.Provider == "iam" {
		kind = audit.ActorKindIAMSubject
	} else if p.Provider == "reference-template" {
		kind = "template_subject"
	}
	return audit.ActorRef{
		Kind:        kind,
		ID:          p.UserID,
		DisplayName: p.DisplayName,
	}
}

type BootstrapInput struct {
	Username    string
	DisplayName string
	Password    string
}

type LoginResult struct {
	Principal Principal
	Token     string
	ExpiresAt time.Time
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type AuthenticationError struct{}

func (e *AuthenticationError) Error() string {
	return "用户名或密码错误，或账号暂不可用"
}

type Repository interface {
	HasUsers(context.Context) (bool, error)
	Bootstrap(context.Context, User, string, audit.Entry) (bool, error)
	FindCredential(context.Context, string) (Credential, bool, error)
	RecordLoginFailure(context.Context, string, time.Time, int, time.Duration) error
	ResetLoginFailures(context.Context, string, time.Time) error
	CreateSession(context.Context, []byte, string, int64, time.Time, time.Time) error
	ResolveSession(context.Context, []byte, time.Time) (Principal, bool, error)
	RevokeSession(context.Context, []byte, time.Time) error
}

type Service struct {
	repository        Repository
	now               func() time.Time
	newID             func() (string, error)
	sessionTTL        time.Duration
	bcryptCost        int
	dummyPasswordHash []byte
}

func NewService(repository Repository, sessionTTL time.Duration) (*Service, error) {
	return newService(repository, sessionTTL, bcrypt.DefaultCost)
}

func newService(repository Repository, sessionTTL time.Duration, bcryptCost int) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("identity repository is required")
	}
	if sessionTTL <= 0 {
		return nil, fmt.Errorf("session TTL must be positive")
	}
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("not-a-real-money-flow-ledger-password"), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}
	return &Service{
		repository:        repository,
		now:               func() time.Time { return time.Now().UTC() },
		newID:             randomUUID,
		sessionTTL:        sessionTTL,
		bcryptCost:        bcryptCost,
		dummyPasswordHash: dummyHash,
	}, nil
}

func (s *Service) EnsureBootstrap(ctx context.Context, input BootstrapInput) (bool, error) {
	hasUsers, err := s.repository.HasUsers(ctx)
	if err != nil {
		return false, fmt.Errorf("check local identity bootstrap: %w", err)
	}
	if hasUsers {
		return false, nil
	}
	if strings.TrimSpace(input.Username) == "" || input.Password == "" {
		return false, &ValidationError{
			Field:   "bootstrap",
			Message: "空用户库必须设置 BOOTSTRAP_ADMIN_USERNAME 和 BOOTSTRAP_ADMIN_PASSWORD",
		}
	}
	return s.bootstrap(ctx, input)
}

func (s *Service) bootstrap(ctx context.Context, input BootstrapInput) (bool, error) {
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
	userID, err := s.newID()
	if err != nil {
		return false, fmt.Errorf("generate bootstrap user ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return false, fmt.Errorf("generate bootstrap audit ID: %w", err)
	}
	now := s.now()
	user := User{
		ID:                userID,
		Username:          username,
		DisplayName:       displayName,
		Role:              RoleAdministrator,
		Status:            StatusActive,
		SessionGeneration: 1,
		PasswordChangedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	created, err := s.repository.Bootstrap(ctx, user, passwordHash, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectLocalUser,
		ObjectID:   userID,
		Action:     ActionUserBootstrapped,
		Actor: audit.ActorRef{
			Kind:        "bootstrap_configuration",
			ID:          userID,
			DisplayName: displayName,
		},
		OccurredAt: now,
		Summary:    "建立 Money Flow 首位本地管理员：" + username,
		Details:    map[string]any{"role": RoleAdministrator, "username": username},
	})
	if err != nil {
		return false, fmt.Errorf("bootstrap local administrator: %w", err)
	}
	return created, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	normalizedUsername, usernameErr := normalizeUsername(username)
	if usernameErr != nil {
		_ = bcrypt.CompareHashAndPassword(s.dummyPasswordHash, []byte(password))
		return LoginResult{}, &AuthenticationError{}
	}
	credential, found, err := s.repository.FindCredential(ctx, normalizedUsername)
	if err != nil {
		return LoginResult{}, fmt.Errorf("find login credential: %w", err)
	}
	if !found {
		_ = bcrypt.CompareHashAndPassword(s.dummyPasswordHash, []byte(password))
		return LoginResult{}, &AuthenticationError{}
	}
	now := s.now()
	if credential.Status != StatusActive || credential.LockedUntil.After(now) {
		_ = bcrypt.CompareHashAndPassword(s.dummyPasswordHash, []byte(password))
		return LoginResult{}, &AuthenticationError{}
	}
	if bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password)) != nil {
		if err := s.repository.RecordLoginFailure(ctx, credential.ID, now, 5, 15*time.Minute); err != nil {
			return LoginResult{}, fmt.Errorf("record login failure: %w", err)
		}
		return LoginResult{}, &AuthenticationError{}
	}
	if err := s.repository.ResetLoginFailures(ctx, credential.ID, now); err != nil {
		return LoginResult{}, fmt.Errorf("reset login failures: %w", err)
	}
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return LoginResult{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(rawToken)
	tokenHash := sha256.Sum256([]byte(token))
	expiresAt := now.Add(s.sessionTTL)
	if err := s.repository.CreateSession(ctx, tokenHash[:], credential.ID, credential.SessionGeneration, now, expiresAt); err != nil {
		return LoginResult{}, fmt.Errorf("create local session: %w", err)
	}
	return LoginResult{
		Principal: principalFromUser(credential.User),
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) ResolveSession(ctx context.Context, token string) (Principal, bool, error) {
	if !validSessionToken(token) {
		return Principal{}, false, nil
	}
	tokenHash := sha256.Sum256([]byte(token))
	principal, found, err := s.repository.ResolveSession(ctx, tokenHash[:], s.now())
	if err != nil {
		return Principal{}, false, fmt.Errorf("resolve local session: %w", err)
	}
	return principal, found, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if !validSessionToken(token) {
		return nil
	}
	tokenHash := sha256.Sum256([]byte(token))
	if err := s.repository.RevokeSession(ctx, tokenHash[:], s.now()); err != nil {
		return fmt.Errorf("revoke local session: %w", err)
	}
	return nil
}

func (s *Service) hashPassword(password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func normalizeUsername(raw string) (string, error) {
	username := strings.ToLower(strings.TrimSpace(raw))
	if !usernamePattern.MatchString(username) {
		return "", &ValidationError{Field: "username", Message: "用户名须为 3 至 64 位小写字母、数字、点、下划线或连字符，并以字母或数字开头"}
	}
	return username, nil
}

func normalizeDisplayName(raw, fallback string) (string, error) {
	displayName := strings.TrimSpace(raw)
	if displayName == "" {
		displayName = fallback
	}
	if len([]rune(displayName)) > 100 {
		return "", &ValidationError{Field: "display_name", Message: "显示名称不能超过 100 个字符"}
	}
	return displayName, nil
}

func validatePassword(password string) error {
	length := len([]byte(password))
	if length < 12 || length > 72 {
		return &ValidationError{Field: "password", Message: "密码长度必须为 12 至 72 字节"}
	}
	return nil
}

func principalFromUser(user User) Principal {
	return Principal{
		UserID:            user.ID,
		Username:          user.Username,
		DisplayName:       user.DisplayName,
		Role:              user.Role,
		SessionGeneration: user.SessionGeneration,
	}
}

func validSessionToken(token string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
}

func randomUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16],
	), nil
}
