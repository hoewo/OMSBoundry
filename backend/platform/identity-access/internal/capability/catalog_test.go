package capability

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCatalogAndRejectUnknownOrDuplicateAudience(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples", "contract-ledger-capabilities.json")
	projectPath := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples", "project-workspace-capabilities.json")
	moneyPath := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples", "money-flow-ledger-capabilities.json")
	peoplePath := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples", "people-groups-capabilities.json")
	engagementPath := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples", "business-engagement-capabilities.json")
	productionPath := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples", "production-operations-capabilities.json")
	resultsPath := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples", "approval-audit-results-capabilities.json")
	costPath := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples", "cost-outsourcing-capabilities.json")
	bonusPath := filepath.Join("..", "..", "..", "..", "..", "contracts", "access", "v1", "examples", "bonus-distribution-capabilities.json")
	catalog, err := Load([]string{path, projectPath, moneyPath, peoplePath, engagementPath, productionPath, resultsPath, costPath, bonusPath})
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.Contains("contract-ledger", "contract-ledger.view") {
		t.Fatal("published capability missing")
	}
	for _, action := range []string{"bonus-manual-allocation.read", "bonus-manual-allocation.amount.read", "bonus-manual-allocation.record", "bonus-manual-allocation.withdraw", "bonus-manual-allocation.history.read"} {
		if !catalog.Contains("bonus-distribution", action) || !catalog.AllowsResource("bonus-distribution", action, "business_bonus_project") || !catalog.AllowsResource("bonus-distribution", action, "production_bonus_project") || catalog.AllowsResource("bonus-distribution", action, "project") || catalog.AllowsResource("bonus-distribution", action, "bonus_pool") {
			t.Fatalf("manual bonus catalog scope isolation: %s", action)
		}
	}
	if catalog.AllowsResource("bonus-distribution", "bonus-manual-allocation.record", "reference_oa_bonus_allocation") || !catalog.AllowsResource("bonus-distribution", "bonus-manual-allocation.history.read", "reference_oa_bonus_allocation") {
		t.Fatal("manual bonus command/history exact-resource distinction missing")
	}
	if catalog.Contains("contract-ledger", "contract-ledger.unknown") {
		t.Fatal("unknown capability accepted")
	}
	if !catalog.AllowsResource("contract-ledger", "contract-ledger.reference.read", "settlement") || catalog.AllowsResource("contract-ledger", "contract-ledger.reference.read", "obligation") {
		t.Fatal("contract reference capability resource types are not enforced")
	}
	if !catalog.AllowsResource("money-flow-ledger", "money-flow-ledger.reference.read", "obligation") || catalog.AllowsResource("money-flow-ledger", "money-flow-ledger.reference.read", "contract") {
		t.Fatal("money reference capability resource types are not enforced")
	}
	if !catalog.AllowsResource("money-flow-ledger", "money-flow-ledger.receipt-set.confirm", "receipt_set") ||
		!catalog.AllowsResource("money-flow-ledger", "money-flow-ledger.receipt-set.reference.read", "receipt_set") ||
		catalog.AllowsResource("money-flow-ledger", "money-flow-ledger.receipt-set.reference.read", "transaction") {
		t.Fatal("money receipt set capabilities are not enforced")
	}
	if !catalog.AllowsResource("people-groups", "people-groups.person.reference.read", "person") ||
		catalog.AllowsResource("people-groups", "people-groups.person.reference.read", "group") {
		t.Fatal("people person reference capability is not enforced")
	}
	if !catalog.Contains("project-workspace", "project-workspace.read") || catalog.Contains("project-workspace", "project.read") {
		t.Fatal("project capabilities do not follow the audience namespace")
	}
	if !catalog.AllowsResource("project-workspace", "project-workspace.read", "project") || catalog.AllowsResource("project-workspace", "project-workspace.create", "contract") {
		t.Fatal("project capability resource types are not enforced")
	}
	if !catalog.Contains("business-engagement", "business-engagement.method.select") || catalog.Contains("business-engagement", "manage_business_info") {
		t.Fatal("business engagement must publish fine-grained native capabilities")
	}
	if !catalog.AllowsResource("business-engagement", "business-engagement.evidence.manage", "business-engagement") || catalog.AllowsResource("business-engagement", "business-engagement.evidence.manage", "evidence-file") {
		t.Fatal("business engagement capability resource types are not enforced")
	}
	if !catalog.Contains("production-operations", "production-work-item.create") || catalog.Contains("production-operations", "production-operations.create") {
		t.Fatal("explicit production capability namespace is not enforced")
	}
	if !catalog.Contains("approval-audit-results", "approval-audit-result.create") || catalog.Contains("approval-audit-results", "approval-audit-results.create") {
		t.Fatal("approval and audit results must use its explicit native capability namespace")
	}
	if !catalog.AllowsResource("approval-audit-results", "approval-audit-result.read", "approval_audit_result") || !catalog.AllowsResource("approval-audit-results", "approval-audit-result.read", "project") || catalog.AllowsResource("approval-audit-results", "approval-audit-result.read", "contract") {
		t.Fatal("approval and audit result capability resource types are not enforced")
	}
	if !catalog.Contains("cost-outsourcing", "cost-entry.create") || !catalog.Contains("cost-outsourcing", "outsourcing-engagement.assess") || catalog.Contains("cost-outsourcing", "cost-outsourcing.create") {
		t.Fatal("cost and outsourcing native namespaces are not enforced")
	}
	// The already implemented C7 suggestion namespace remains available in the
	// catalog; publication of a capability is not a Grant or an actual payment.
	if !catalog.Contains("bonus-distribution", "bonus-pool.business.calculate") || !catalog.Contains("bonus-distribution", "bonus-allocation.amount.read") || !catalog.AllowsResource("bonus-distribution", "bonus-payable-suggestion.create", "bonus_allocation") || catalog.AllowsResource("bonus-distribution", "bonus-payable-suggestion.create", "reference_oa_bonus_allocation") {
		t.Fatal("bonus distribution capability catalog mismatch")
	}
	if _, err := Load([]string{path, path}); err == nil {
		t.Fatal("duplicate audience accepted")
	}
}

func TestLoadCatalogRejectsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	data := []byte(`{"schema_version":"1","application_id":"app-one","application_version":"1.0.0","audience":"app-one","capabilities":[],"role":"admin"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load([]string{path}); err == nil {
		t.Fatal("unknown field accepted")
	}
}
