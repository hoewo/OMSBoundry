package identity

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// TenantProvisioningRepository is intentionally smaller than Repository so
// the platform-only provisioning command cannot gain access to login or grant
// operations by accident.
type TenantProvisioningRepository interface {
	ProvisionTenant(context.Context, string, string, Subject, string, AuditEntry) error
}

type TenantProvisionInput struct {
	TenantID            string
	TenantName          string
	AdminUsername       string
	AdminDisplayName    string
	AdminPassword       string
	OperatorID          string
	OperatorDisplayName string
}

type TenantProvisioner struct {
	repository TenantProvisioningRepository
	now        func() time.Time
	newID      func() (string, error)
	bcryptCost int
}

func NewTenantProvisioner(repository TenantProvisioningRepository) (*TenantProvisioner, error) {
	if repository == nil {
		return nil, fmt.Errorf("tenant provisioning repository is required")
	}
	return &TenantProvisioner{
		repository: repository,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      randomUUID,
		bcryptCost: bcrypt.DefaultCost,
	}, nil
}

// Provision creates one tenant and its first administrator atomically. The
// caller is an out-of-band platform operator; no tenant session can authorize
// creation of another tenant.
func (p *TenantProvisioner) Provision(ctx context.Context, input TenantProvisionInput) (Subject, error) {
	tenantID := strings.ToLower(strings.TrimSpace(input.TenantID))
	tenantName := strings.TrimSpace(input.TenantName)
	operatorID := strings.TrimSpace(input.OperatorID)
	operatorName := strings.TrimSpace(input.OperatorDisplayName)
	if !uuidPattern.MatchString(tenantID) {
		return Subject{}, &ValidationError{Field: "tenant_id", Message: "Tenant ID 无效"}
	}
	if tenantName == "" || len([]rune(tenantName)) > 100 {
		return Subject{}, &ValidationError{Field: "tenant_name", Message: "租户名称必须为 1 至 100 个字符"}
	}
	if operatorID == "" || len(operatorID) > 128 || operatorName == "" || len([]rune(operatorName)) > 100 {
		return Subject{}, &ValidationError{Field: "operator", Message: "平台操作人无效"}
	}
	username, err := normalizeUsername(input.AdminUsername)
	if err != nil {
		return Subject{}, err
	}
	displayName, err := normalizeDisplayName(input.AdminDisplayName, username)
	if err != nil {
		return Subject{}, err
	}
	if len(input.AdminPassword) < 12 || len(input.AdminPassword) > 72 {
		return Subject{}, &ValidationError{Field: "admin_password", Message: "管理员密码必须为 12 至 72 字节"}
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.AdminPassword), p.bcryptCost)
	if err != nil {
		return Subject{}, err
	}
	subjectID, err := p.newID()
	if err != nil {
		return Subject{}, err
	}
	auditID, err := p.newID()
	if err != nil {
		return Subject{}, err
	}
	now := p.now()
	subject := Subject{
		ID: subjectID, TenantID: tenantID, Username: username, DisplayName: displayName,
		Status: StatusActive, Administrator: true, Revision: 1, SessionGeneration: 1,
		PasswordChangedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	entry := AuditEntry{
		ID: auditID, TenantID: tenantID, Action: "iam_tenant_provisioned", ObjectType: "tenant", ObjectID: tenantID,
		Actor: AuditActor{Kind: "platform_operator", ID: operatorID, DisplayName: operatorName}, OccurredAt: now,
		Details: map[string]any{"tenant_name": tenantName, "administrator_subject_id": subjectID, "administrator_username": username},
	}
	if err := p.repository.ProvisionTenant(ctx, tenantID, tenantName, subject, string(passwordHash), entry); err != nil {
		return Subject{}, err
	}
	return subject, nil
}
