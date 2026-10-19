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

	"omsbizfoundry/contract-ledger/internal/audit"
)

type Role string

const (
	RoleAdministrator Role = "administrator"
	RoleEditor        Role = "editor"
	RoleViewer        Role = "viewer"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

type Permission string

const (
	PermissionBusinessView    Permission = "business.view"
	PermissionBusinessWrite   Permission = "business.write"
	PermissionPartyRegister   Permission = "party.register"
	PermissionDraftManage     Permission = "draft.manage"
	PermissionLifecycleManage Permission = "lifecycle.manage"
	PermissionBusinessCorrect Permission = "business.correct"
	PermissionEvidenceManage  Permission = "evidence.manage"
	PermissionBusinessExport  Permission = "business.export"
	PermissionBusinessRestore Permission = "business.restore"
	PermissionIdentityManage  Permission = "identity.manage"
	PermissionReferenceRead   Permission = "reference.read"
)

const (
	ActionUserBootstrapped   = "local_user_bootstrapped"
	ActionUserCreated        = "local_user_created"
	ActionUserRoleChanged    = "local_user_role_changed"
	ActionUserStatusChanged  = "local_user_status_changed"
	ActionUserPasswordChange = "local_user_password_changed"
	ActionUserPasswordReset  = "local_user_password_reset"
)

var (
	usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)
	uuidPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

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
			PermissionBusinessView: "contract-ledger.view", PermissionPartyRegister: "contract-ledger.party.register",
			PermissionDraftManage: "contract-ledger.draft.manage", PermissionLifecycleManage: "contract-ledger.lifecycle.manage",
			PermissionBusinessCorrect: "contract-ledger.correct", PermissionEvidenceManage: "contract-ledger.evidence.manage",
			PermissionBusinessExport: "contract-ledger.export", PermissionBusinessRestore: "contract-ledger.restore",
			PermissionReferenceRead: "contract-ledger.reference.read",
		}[permission]
		if permission == PermissionBusinessWrite {
			return p.AllowedActions["contract-ledger.party.register"] || p.AllowedActions["contract-ledger.draft.manage"] || p.AllowedActions["contract-ledger.lifecycle.manage"] || p.AllowedActions["contract-ledger.evidence.manage"]
		}
		return action != "" && p.AllowedActions[action]
	}
	switch permission {
	case PermissionBusinessView:
		return p.Role == RoleAdministrator || p.Role == RoleEditor || p.Role == RoleViewer
	case PermissionBusinessWrite, PermissionPartyRegister, PermissionDraftManage, PermissionLifecycleManage, PermissionEvidenceManage:
		return p.Role == RoleAdministrator || p.Role == RoleEditor
	case PermissionBusinessCorrect, PermissionBusinessExport, PermissionBusinessRestore:
		return p.Role == RoleAdministrator
	case PermissionIdentityManage:
		return p.Role == RoleAdministrator
	default:
		return false
	}
}

func (p Principal) CanReference(resourceType, resourceID string) bool {
	return p.Provider == "iam" && p.ScopeKind == "resources" && p.Resources[resourceType][resourceID]
}

func (p Principal) Actor() audit.ActorRef {
	kind := audit.ActorKindLocalUser
	if p.Provider == "iam" {
		kind = audit.ActorKindIAMSubject
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

type CreateUserInput struct {
	Username    string
	DisplayName string
	Password    string
	Role        string
	Actor       audit.ActorRef
}

type ChangeRoleInput struct {
	UserID string
	Role   string
	Actor  audit.ActorRef
}

type ChangeStatusInput struct {
	UserID string
	Status string
	Actor  audit.ActorRef
}

type ChangePasswordInput struct {
	UserID            string
	CurrentPassword   string
	NewPassword       string
	SessionGeneration int64
	Actor             audit.ActorRef
}

type ResetPasswordInput struct {
	UserID      string
	NewPassword string
	Actor       audit.ActorRef
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

type DuplicateUsernameError struct{ Username string }

func (e *DuplicateUsernameError) Error() string { return "用户名已存在" }

type UserNotFoundError struct{ UserID string }

func (e *UserNotFoundError) Error() string { return "用户不存在" }

type LastAdministratorError struct{}

func (e *LastAdministratorError) Error() string { return "必须保留至少一名有效管理员" }

type SelfDisableError struct{}

func (e *SelfDisableError) Error() string { return "不能停用当前登录用户" }

type NoChangeError struct{}

func (e *NoChangeError) Error() string { return "提交内容没有变化" }

type Repository interface {
	HasUsers(context.Context) (bool, error)
	Bootstrap(context.Context, User, string, audit.Entry) (bool, error)
	FindCredential(context.Context, string) (Credential, bool, error)
	FindCredentialByID(context.Context, string) (Credential, bool, error)
	RecordLoginFailure(context.Context, string, time.Time, int, time.Duration) error
	ResetLoginFailures(context.Context, string, time.Time) error
	CreateSession(context.Context, []byte, string, int64, time.Time, time.Time) error
	ResolveSession(context.Context, []byte, time.Time) (Principal, bool, error)
	RevokeSession(context.Context, []byte, time.Time) error
	ListUsers(context.Context) ([]User, error)
	CreateUser(context.Context, User, string, audit.Entry) error
	ChangeRole(context.Context, string, Role, audit.Entry) error
	ChangeStatus(context.Context, string, Status, string, audit.Entry) error
	ChangePassword(context.Context, string, int64, string, time.Time, audit.Entry) error
	ResetPassword(context.Context, string, string, time.Time, audit.Entry) error
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
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("not-a-real-contract-ledger-password"), bcryptCost)
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

func (s *Service) Bootstrap(ctx context.Context, input BootstrapInput) (bool, error) {
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
		Summary:    "建立首位本地管理员：" + username,
		Details:    map[string]any{"role": RoleAdministrator, "username": username},
	})
	if err != nil {
		return false, fmt.Errorf("bootstrap local administrator: %w", err)
	}
	return created, nil
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
			Message: "空数据库必须设置 BOOTSTRAP_ADMIN_USERNAME 和 BOOTSTRAP_ADMIN_PASSWORD",
		}
	}
	return s.Bootstrap(ctx, input)
}

func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	normalizedUsername, usernameErr := normalizeUsername(username)
	credential, found, err := s.repository.FindCredential(ctx, normalizedUsername)
	if err != nil {
		return LoginResult{}, fmt.Errorf("find login credential: %w", err)
	}
	if usernameErr != nil || !found {
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

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	users, err := s.repository.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local users: %w", err)
	}
	return users, nil
}

func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (User, error) {
	actor, err := audit.NormalizeActor(input.Actor)
	if err != nil {
		return User{}, &ValidationError{Field: "actor", Message: err.Error()}
	}
	username, err := normalizeUsername(input.Username)
	if err != nil {
		return User{}, err
	}
	displayName, err := normalizeDisplayName(input.DisplayName, username)
	if err != nil {
		return User{}, err
	}
	role, err := normalizeRole(input.Role)
	if err != nil {
		return User{}, err
	}
	passwordHash, err := s.hashPassword(input.Password)
	if err != nil {
		return User{}, err
	}
	userID, auditID, err := s.newEntityAndAuditIDs()
	if err != nil {
		return User{}, err
	}
	now := s.now()
	user := User{
		ID:                userID,
		Username:          username,
		DisplayName:       displayName,
		Role:              role,
		Status:            StatusActive,
		SessionGeneration: 1,
		PasswordChangedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repository.CreateUser(ctx, user, passwordHash, audit.Entry{
		ID:         auditID,
		ObjectType: audit.ObjectLocalUser,
		ObjectID:   userID,
		Action:     ActionUserCreated,
		Actor:      actor,
		OccurredAt: now,
		Summary:    fmt.Sprintf("创建本地用户 %s，角色 %s", username, role),
		Details:    map[string]any{"username": username, "role": role},
	}); err != nil {
		return User{}, fmt.Errorf("create local user: %w", err)
	}
	return user, nil
}

func (s *Service) ChangeRole(ctx context.Context, input ChangeRoleInput) error {
	actor, userID, auditID, err := s.normalizeMutation(input.Actor, input.UserID)
	if err != nil {
		return err
	}
	role, err := normalizeRole(input.Role)
	if err != nil {
		return err
	}
	now := s.now()
	if err := s.repository.ChangeRole(ctx, userID, role, audit.Entry{
		ID: auditID, ObjectType: audit.ObjectLocalUser, ObjectID: userID,
		Action: ActionUserRoleChanged, Actor: actor, OccurredAt: now,
		Summary: "调整本地用户角色为 " + string(role),
		Details: map[string]any{"role": role},
	}); err != nil {
		return fmt.Errorf("change local user role: %w", err)
	}
	return nil
}

func (s *Service) ChangeStatus(ctx context.Context, input ChangeStatusInput) error {
	actor, userID, auditID, err := s.normalizeMutation(input.Actor, input.UserID)
	if err != nil {
		return err
	}
	status, err := normalizeStatus(input.Status)
	if err != nil {
		return err
	}
	now := s.now()
	if err := s.repository.ChangeStatus(ctx, userID, status, actor.ID, audit.Entry{
		ID: auditID, ObjectType: audit.ObjectLocalUser, ObjectID: userID,
		Action: ActionUserStatusChanged, Actor: actor, OccurredAt: now,
		Summary: "调整本地用户状态为 " + string(status),
		Details: map[string]any{"status": status},
	}); err != nil {
		return fmt.Errorf("change local user status: %w", err)
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, input ChangePasswordInput) error {
	actor, userID, auditID, err := s.normalizeMutation(input.Actor, input.UserID)
	if err != nil {
		return err
	}
	if actor.ID != userID {
		return &ValidationError{Field: "user_id", Message: "只能修改当前用户密码"}
	}
	credential, found, err := s.repository.FindCredentialByID(ctx, actor.ID)
	if err != nil {
		return fmt.Errorf("read current password: %w", err)
	}
	if !found || bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(input.CurrentPassword)) != nil {
		return &AuthenticationError{}
	}
	passwordHash, err := s.hashPassword(input.NewPassword)
	if err != nil {
		return err
	}
	now := s.now()
	if err := s.repository.ChangePassword(ctx, userID, input.SessionGeneration, passwordHash, now, audit.Entry{
		ID: auditID, ObjectType: audit.ObjectLocalUser, ObjectID: userID,
		Action: ActionUserPasswordChange, Actor: actor, OccurredAt: now,
		Summary: "修改自己的本地用户密码", Details: map[string]any{},
	}); err != nil {
		return fmt.Errorf("change local user password: %w", err)
	}
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	actor, userID, auditID, err := s.normalizeMutation(input.Actor, input.UserID)
	if err != nil {
		return err
	}
	passwordHash, err := s.hashPassword(input.NewPassword)
	if err != nil {
		return err
	}
	now := s.now()
	if err := s.repository.ResetPassword(ctx, userID, passwordHash, now, audit.Entry{
		ID: auditID, ObjectType: audit.ObjectLocalUser, ObjectID: userID,
		Action: ActionUserPasswordReset, Actor: actor, OccurredAt: now,
		Summary: "管理员重置本地用户密码", Details: map[string]any{},
	}); err != nil {
		return fmt.Errorf("reset local user password: %w", err)
	}
	return nil
}

func (s *Service) normalizeMutation(actorValue audit.ActorRef, rawUserID string) (audit.ActorRef, string, string, error) {
	actor, err := audit.NormalizeActor(actorValue)
	if err != nil {
		return audit.ActorRef{}, "", "", &ValidationError{Field: "actor", Message: err.Error()}
	}
	userID := strings.ToLower(strings.TrimSpace(rawUserID))
	if !uuidPattern.MatchString(userID) {
		return audit.ActorRef{}, "", "", &ValidationError{Field: "user_id", Message: "用户标识无效"}
	}
	auditID, err := s.newID()
	if err != nil {
		return audit.ActorRef{}, "", "", fmt.Errorf("generate identity audit ID: %w", err)
	}
	return actor, userID, auditID, nil
}

func (s *Service) newEntityAndAuditIDs() (string, string, error) {
	entityID, err := s.newID()
	if err != nil {
		return "", "", fmt.Errorf("generate local user ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return "", "", fmt.Errorf("generate identity audit ID: %w", err)
	}
	return entityID, auditID, nil
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

func normalizeRole(raw string) (Role, error) {
	role := Role(strings.ToLower(strings.TrimSpace(raw)))
	if role != RoleAdministrator && role != RoleEditor && role != RoleViewer {
		return "", &ValidationError{Field: "role", Message: "请选择有效角色"}
	}
	return role, nil
}

func normalizeStatus(raw string) (Status, error) {
	status := Status(strings.ToLower(strings.TrimSpace(raw)))
	if status != StatusActive && status != StatusDisabled {
		return "", &ValidationError{Field: "status", Message: "请选择有效状态"}
	}
	return status, nil
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
