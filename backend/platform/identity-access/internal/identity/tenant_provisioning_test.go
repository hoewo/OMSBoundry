package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type captureTenantRepository struct {
	tenantID     string
	tenantName   string
	subject      Subject
	passwordHash string
	audit        AuditEntry
	err          error
}

func (r *captureTenantRepository) ProvisionTenant(_ context.Context, tenantID, tenantName string, subject Subject, passwordHash string, entry AuditEntry) error {
	r.tenantID, r.tenantName, r.subject, r.passwordHash, r.audit = tenantID, tenantName, subject, passwordHash, entry
	return r.err
}

func TestTenantProvisionerCreatesTenantAdministratorAndAudit(t *testing.T) {
	repository := &captureTenantRepository{}
	provisioner, err := NewTenantProvisioner(repository)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 8, 30, 0, 0, time.UTC)
	ids := []string{"33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"}
	provisioner.now = func() time.Time { return now }
	provisioner.newID = func() (string, error) {
		value := ids[0]
		ids = ids[1:]
		return value, nil
	}
	result, err := provisioner.Provision(context.Background(), TenantProvisionInput{
		TenantID: "22222222-2222-4222-8222-222222222222", TenantName: "第二企业",
		AdminUsername: "Admin.B", AdminDisplayName: "第二企业管理员", AdminPassword: "correct horse battery staple",
		OperatorID: "platform-operator-1", OperatorDisplayName: "平台管理员",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "33333333-3333-4333-8333-333333333333" || result.TenantID != repository.tenantID || result.Username != "admin.b" || !result.Administrator || result.Status != StatusActive || result.SessionGeneration != 1 {
		t.Fatalf("administrator = %#v", result)
	}
	if repository.tenantName != "第二企业" || repository.audit.ID != "44444444-4444-4444-8444-444444444444" || repository.audit.TenantID != result.TenantID || repository.audit.Actor.Kind != "platform_operator" || repository.audit.Action != "iam_tenant_provisioned" {
		t.Fatalf("audit = %#v", repository.audit)
	}
	if repository.passwordHash == "correct horse battery staple" || bcrypt.CompareHashAndPassword([]byte(repository.passwordHash), []byte("correct horse battery staple")) != nil {
		t.Fatal("administrator password was not stored as a bcrypt verifier")
	}
}

func TestTenantProvisionerValidatesBeforeWriting(t *testing.T) {
	repository := &captureTenantRepository{}
	provisioner, err := NewTenantProvisioner(repository)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provisioner.Provision(context.Background(), TenantProvisionInput{
		TenantID: "client-supplied", TenantName: "第二企业", AdminUsername: "admin-b", AdminDisplayName: "管理员",
		AdminPassword: "correct horse battery staple", OperatorID: "operator", OperatorDisplayName: "平台管理员",
	})
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Field != "tenant_id" {
		t.Fatalf("error = %v, want tenant_id validation", err)
	}
	if repository.tenantID != "" {
		t.Fatal("invalid provisioning request reached the repository")
	}
}

func TestTenantProvisionerPropagatesConflict(t *testing.T) {
	repository := &captureTenantRepository{err: &ConflictError{Message: "租户已存在"}}
	provisioner, err := NewTenantProvisioner(repository)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provisioner.Provision(context.Background(), TenantProvisionInput{
		TenantID: "22222222-2222-4222-8222-222222222222", TenantName: "第二企业",
		AdminUsername: "admin-b", AdminDisplayName: "管理员", AdminPassword: "correct horse battery staple",
		OperatorID: "operator", OperatorDisplayName: "平台管理员",
	})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want conflict", err)
	}
}
