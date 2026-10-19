package accesscontract_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

const contractRoot = "../../contracts/access/v1"
const readProjectionRoot = "../../contracts/read-projections/v1"

var (
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
	actionPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
	resourcePattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	versionPattern    = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	uuidPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	decimalPattern    = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	digestPattern     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type metricProjectionScope struct {
	Kind         string `json:"kind"`
	TenantID     string `json:"tenant_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
}

type metricProjectionItem struct {
	MetricKey    string                `json:"metric_key"`
	Scope        metricProjectionScope `json:"scope"`
	Unit         string                `json:"unit"`
	Currency     string                `json:"currency"`
	Value        *string               `json:"value"`
	Redacted     bool                  `json:"redacted"`
	SourceDigest *string               `json:"source_digest"`
	TimeRange    string                `json:"time_range"`
	ObservedAt   string                `json:"observed_at"`
	DeepLink     string                `json:"deep_link"`
}

type metricProjection struct {
	SchemaVersion     string                 `json:"schema_version"`
	SourceApplication string                 `json:"source_application"`
	TenantID          string                 `json:"tenant_id"`
	DefinitionVersion string                 `json:"definition_version"`
	ObservedAt        string                 `json:"observed_at"`
	Items             []metricProjectionItem `json:"items"`
}

type resourceRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type scope struct {
	Kind      string        `json:"kind"`
	Resources []resourceRef `json:"resources"`
}

type credentialClaims struct {
	Version   string   `json:"ver"`
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Tenant    string   `json:"tid"`
	Audience  string   `json:"aud"`
	Name      string   `json:"name"`
	Actions   []string `json:"actions"`
	Scope     scope    `json:"scope"`
	IssuedAt  int64    `json:"iat"`
	NotBefore int64    `json:"nbf"`
	ExpiresAt int64    `json:"exp"`
	AuthTime  int64    `json:"auth_time"`
	ID        string   `json:"jti"`
}

type iamContext struct {
	SchemaVersion string   `json:"schema_version"`
	Issuer        string   `json:"issuer"`
	TenantID      string   `json:"tenant_id"`
	SubjectID     string   `json:"subject_id"`
	DisplayName   string   `json:"display_name"`
	Audience      string   `json:"audience"`
	CredentialID  string   `json:"credential_id"`
	IssuedAt      string   `json:"issued_at"`
	ExpiresAt     string   `json:"expires_at"`
	Actions       []string `json:"actions"`
	Scope         scope    `json:"scope"`
}

type capability struct {
	ID            string   `json:"id"`
	Description   string   `json:"description"`
	Risk          string   `json:"risk"`
	ResourceTypes []string `json:"resource_types"`
}

type capabilityManifest struct {
	SchemaVersion        string       `json:"schema_version"`
	ApplicationID        string       `json:"application_id"`
	ApplicationVersion   string       `json:"application_version"`
	Audience             string       `json:"audience"`
	CapabilityNamespace  string       `json:"capability_namespace,omitempty"`
	CapabilityNamespaces []string     `json:"capability_namespaces,omitempty"`
	Capabilities         []capability `json:"capabilities"`
}

type overlayRole struct {
	DisplayName string   `json:"display_name"`
	ScopeKind   string   `json:"scope_kind"`
	Actions     []string `json:"actions"`
}

type assignmentPolicyRef struct {
	PolicyID      string `json:"policy_id"`
	PolicyVersion string `json:"policy_version"`
}

type customerRoleCapabilityOverlay struct {
	SchemaVersion    string                 `json:"schema_version"`
	OverlayID        string                 `json:"overlay_id"`
	OverlayVersion   string                 `json:"overlay_version"`
	ApplicationID    string                 `json:"application_id"`
	Audience         string                 `json:"audience"`
	SiteRoles        map[string]overlayRole `json:"site_roles"`
	AssignmentPolicy *assignmentPolicyRef   `json:"assignment_policy,omitempty"`
}

type suiteApplication struct {
	ID                   string                   `json:"id"`
	Version              string                   `json:"version"`
	Enabled              bool                     `json:"enabled"`
	Required             bool                     `json:"required"`
	RoutePrefix          string                   `json:"route_prefix"`
	Upstream             string                   `json:"upstream"`
	ReadinessPath        string                   `json:"readiness_path"`
	Audience             string                   `json:"audience"`
	CapabilityNamespace  string                   `json:"capability_namespace,omitempty"`
	CapabilityNamespaces []string                 `json:"capability_namespaces,omitempty"`
	EntryAction          string                   `json:"entry_action,omitempty"`
	EntryActions         []string                 `json:"entry_actions,omitempty"`
	CapabilityManifest   string                   `json:"capability_manifest,omitempty"`
	ShowInWorkbench      *bool                    `json:"show_in_workbench,omitempty"`
	WorkItemProjection   *suiteWorkItemProjection `json:"work_item_projection,omitempty"`
	MetricProjection     *suiteMetricProjection   `json:"metric_projection,omitempty"`
}
type suiteWorkItemProjection struct {
	SchemaVersion string `json:"schema_version"`
	Path          string `json:"path"`
	Action        string `json:"action"`
}

func (app suiteApplication) entryCapabilities() []string {
	if app.EntryAction != "" {
		return []string{app.EntryAction}
	}
	return app.EntryActions
}

type suiteMetricProjection struct {
	SchemaVersion string `json:"schema_version"`
	Path          string `json:"path"`
	Action        string `json:"action"`
}

type suiteManifest struct {
	SchemaVersion string             `json:"schema_version"`
	SuiteID       string             `json:"suite_id"`
	TenantID      string             `json:"tenant_id"`
	Applications  []suiteApplication `json:"applications"`
}

type workloadPermission struct {
	Audience      string   `json:"audience"`
	Action        string   `json:"action"`
	ResourceTypes []string `json:"resource_types"`
}

type workloadIdentity struct {
	ID             string               `json:"id"`
	SubjectID      string               `json:"subject_id"`
	DisplayName    string               `json:"display_name"`
	PublicKeysPath string               `json:"public_keys_path"`
	Permissions    []workloadPermission `json:"permissions"`
}

type workloadPolicy struct {
	SchemaVersion string             `json:"schema_version"`
	Identities    []workloadIdentity `json:"identities"`
}

func TestPublishedSchemasAreJSONAndHaveStableIDs(t *testing.T) {
	names := []string{
		"iam-context.schema.json",
		"capability-credential-claims.schema.json",
		"capability-manifest.schema.json",
		"customer-role-capability-overlay.schema.json",
		"service-credential-request.schema.json",
		"suite-manifest.schema.json",
		"workload-assertion-claims.schema.json",
		"workload-identity-policy.schema.json",
		"workload-public-key-bundle.schema.json",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			var document map[string]any
			decodeFile(t, filepath.Join(contractRoot, name), &document)
			if document["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Fatalf("unexpected schema declaration: %v", document["$schema"])
			}
			id, _ := document["$id"].(string)
			if !strings.HasSuffix(id, "/contracts/access/v1/"+name) {
				t.Fatalf("schema ID is not stable: %q", id)
			}
		})
	}
}

func TestPublishedMetricProjectionSchemaAndExampleAreStable(t *testing.T) {
	var schema map[string]any
	decodeFile(t, filepath.Join(readProjectionRoot, "metric-projection.schema.json"), &schema)
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" ||
		schema["$id"] != "https://omsbizfoundry.local/contracts/read-projections/v1/metric-projection.schema.json" {
		t.Fatalf("metric projection schema identity is unstable: %#v", schema)
	}
	var projection metricProjection
	if err := decodeStrict(readFileAbsolute(t, filepath.Join(readProjectionRoot, "examples/money-flow-operating-summary.json")), &projection); err != nil {
		t.Fatal(err)
	}
	if err := validateMetricProjection(projection); err != nil {
		t.Fatal(err)
	}

	redacted := projection
	redacted.Items = slices.Clone(projection.Items)
	redacted.Items[0].Redacted = true
	redacted.Items[0].Value = nil
	redacted.Items[0].SourceDigest = nil
	if err := validateMetricProjection(redacted); err != nil {
		t.Fatalf("valid redacted projection rejected: %v", err)
	}

	redacted.Items[0].SourceDigest = projection.Items[0].SourceDigest
	if err := validateMetricProjection(redacted); err == nil {
		t.Fatal("redacted metric with a source digest was accepted")
	}
}

func TestPublishedExamplesConformToCrossFieldRules(t *testing.T) {
	claims := readStrict[credentialClaims](t, "examples/credential-claims.json")
	if err := validateClaims(claims, "contract-ledger", "22222222-2222-4222-8222-222222222222"); err != nil {
		t.Fatal(err)
	}
	context := readStrict[iamContext](t, "examples/iam-context.json")
	if err := validateContext(context, "contract-ledger", "22222222-2222-4222-8222-222222222222"); err != nil {
		t.Fatal(err)
	}
	contract := readStrict[capabilityManifest](t, "examples/contract-ledger-capabilities.json")
	if err := validateCapabilityManifest(contract); err != nil {
		t.Fatal(err)
	}
	money := readStrict[capabilityManifest](t, "examples/money-flow-ledger-capabilities.json")
	if err := validateCapabilityManifest(money); err != nil {
		t.Fatal(err)
	}
	people := readStrict[capabilityManifest](t, "examples/people-groups-capabilities.json")
	if err := validateCapabilityManifest(people); err != nil {
		t.Fatal(err)
	}
	evidence := readStrict[capabilityManifest](t, "examples/evidence-service-capabilities.json")
	if err := validateCapabilityManifest(evidence); err != nil {
		t.Fatal(err)
	}
	project := readStrict[capabilityManifest](t, "examples/project-workspace-capabilities.json")
	if err := validateCapabilityManifest(project); err != nil {
		t.Fatal(err)
	}
	engagement := readStrict[capabilityManifest](t, "examples/business-engagement-capabilities.json")
	if err := validateCapabilityManifest(engagement); err != nil {
		t.Fatal(err)
	}
	billing := readStrict[capabilityManifest](t, "examples/billing-invoice-capabilities.json")
	if err := validateCapabilityManifest(billing); err != nil {
		t.Fatal(err)
	}
	party := readStrict[capabilityManifest](t, "examples/party-directory-capabilities.json")
	if err := validateCapabilityManifest(party); err != nil {
		t.Fatal(err)
	}
	production := readStrict[capabilityManifest](t, "examples/production-operations-capabilities.json")
	if err := validateCapabilityManifest(production); err != nil {
		t.Fatal(err)
	}
	costOutsourcing := readStrict[capabilityManifest](t, "examples/cost-outsourcing-capabilities.json")
	if err := validateCapabilityManifest(costOutsourcing); err != nil {
		t.Fatal(err)
	}
	if engagement.Audience != "business-engagement" || engagement.Capabilities[0].ID != "business-engagement.create" {
		t.Fatal("business engagement capability namespace is unstable")
	}
	if party.Audience != "party-directory" || party.Capabilities[0].ID != "party-directory.read" {
		t.Fatal("party directory capability namespace is unstable")
	}
	if billing.Audience != "billing-invoice" || billing.Capabilities[0].ID != "billing-invoice.read" || billing.Capabilities[len(billing.Capabilities)-1].ID != "billing-invoice.reference.read" {
		t.Fatal("billing invoice capability namespace is unstable")
	}
	if production.Audience != "production-operations" || production.CapabilityNamespace != "production-work-item" || production.Capabilities[0].ID != "production-work-item.create" || production.Capabilities[len(production.Capabilities)-1].ID != "production-work-item.stage-material.history.read" {
		t.Fatal("production operations capability namespace is unstable")
	}
	if costOutsourcing.Audience != "cost-outsourcing" || len(costOutsourcing.CapabilityNamespaces) != 3 || costOutsourcing.Capabilities[0].ID != "cost-entry.create" {
		t.Fatal("cost and outsourcing must publish all native namespaces under one audience")
	}
	contractIDs := make([]string, 0, len(contract.Capabilities))
	for _, value := range contract.Capabilities {
		contractIDs = append(contractIDs, value.ID)
	}
	moneyIDs := make([]string, 0, len(money.Capabilities))
	for _, value := range money.Capabilities {
		moneyIDs = append(moneyIDs, value.ID)
	}
	peopleIDs := make([]string, 0, len(people.Capabilities))
	for _, value := range people.Capabilities {
		peopleIDs = append(peopleIDs, value.ID)
	}
	if !slices.Contains(contractIDs, "contract-ledger.reference.read") ||
		!slices.Contains(moneyIDs, "money-flow-ledger.reference.read") ||
		!slices.Contains(moneyIDs, "money-flow-ledger.statistics.read") ||
		!slices.Contains(moneyIDs, "money-flow-ledger.amount.read") ||
		!slices.Contains(moneyIDs, "money-flow-ledger.receipt-set.confirm") ||
		!slices.Contains(moneyIDs, "money-flow-ledger.receipt-set.reference.read") ||
		!slices.Contains(peopleIDs, "people-groups.person.reference.read") {
		t.Fatal("source reference capability namespaces are unstable")
	}
	overlay := readStrict[customerRoleCapabilityOverlay](t, "examples/reference-oa-c1-role-overlay.json")
	if err := validateRoleOverlay(overlay, project); err != nil {
		t.Fatal(err)
	}
	if len(overlay.SiteRoles["identity_super_admin"].Actions) != 0 {
		t.Fatal("identity super administrator must not inherit Project business capability")
	}
	productionOverlay := readStrict[customerRoleCapabilityOverlay](t, "examples/reference-oa-c4-role-overlay.json")
	if err := validateRoleOverlay(productionOverlay, production); err != nil {
		t.Fatal(err)
	}
	if len(productionOverlay.SiteRoles["identity_super_admin"].Actions) != 0 || len(productionOverlay.SiteRoles["business_manager"].Actions) != 0 || len(productionOverlay.SiteRoles["production_manager"].Actions) != len(production.Capabilities) {
		t.Fatal("reference OA C4 role separation is unstable")
	}
	costOverlay := readStrict[customerRoleCapabilityOverlay](t, "examples/reference-oa-c6-role-overlay.json")
	if err := validateRoleOverlay(costOverlay, costOutsourcing); err != nil {
		t.Fatal(err)
	}
	if len(costOverlay.SiteRoles["identity_super_admin"].Actions) != 0 ||
		len(costOverlay.SiteRoles["oa_business_admin"].Actions) != len(costOutsourcing.Capabilities)-3 ||
		slices.Contains(costOverlay.SiteRoles["oa_business_admin"].Actions, "outsourcing-payable-suggestion.create") {
		t.Fatal("reference OA C6 role separation is unstable")
	}
	suite := readStrict[suiteManifest](t, "examples/suite-manifest.json")
	if err := validateSuiteManifest(suite); err != nil {
		t.Fatal(err)
	}
}

func TestBillingWorkloadPolicyHasOnlyExactReferencePermissions(t *testing.T) {
	policy := readStrict[workloadPolicy](t, "examples/workload-identity-policy.json")
	if policy.SchemaVersion != "1" {
		t.Fatalf("schema version=%q", policy.SchemaVersion)
	}
	var billing *workloadIdentity
	for index := range policy.Identities {
		if policy.Identities[index].ID == "billing-invoice" {
			billing = &policy.Identities[index]
			break
		}
	}
	if billing == nil {
		t.Fatal("billing-invoice workload identity is missing")
	}
	if billing.SubjectID != "88888888-8888-4888-8888-888888888888" || billing.PublicKeysPath != "/workload-public/billing-invoice.json" {
		t.Fatalf("billing identity=%#v", billing)
	}
	want := []workloadPermission{
		{Audience: "party-directory", Action: "party-directory.reference.read", ResourceTypes: []string{"party"}},
		{Audience: "project-workspace", Action: "project-workspace.read", ResourceTypes: []string{"project"}},
		{Audience: "contract-ledger", Action: "contract-ledger.reference.read", ResourceTypes: []string{"contract", "settlement"}},
		{Audience: "money-flow-ledger", Action: "money-flow-ledger.reference.read", ResourceTypes: []string{"obligation"}},
		{Audience: "evidence-service", Action: "evidence-service.metadata.read", ResourceTypes: []string{"file"}},
	}
	if !slices.EqualFunc(billing.Permissions, want, func(left, right workloadPermission) bool {
		return left.Audience == right.Audience && left.Action == right.Action && slices.Equal(left.ResourceTypes, right.ResourceTypes)
	}) {
		t.Fatalf("billing permissions=%#v want=%#v", billing.Permissions, want)
	}
}

func TestProjectWorkloadPolicyCanOnlyComposeExactCommercialReferences(t *testing.T) {
	policy := readStrict[workloadPolicy](t, "examples/workload-identity-policy.json")
	var project *workloadIdentity
	for index := range policy.Identities {
		if policy.Identities[index].ID == "project-workspace" {
			project = &policy.Identities[index]
			break
		}
	}
	if project == nil {
		t.Fatal("project-workspace workload identity is missing")
	}
	wantCommercial := []workloadPermission{
		{Audience: "contract-ledger", Action: "contract-ledger.reference.read", ResourceTypes: []string{"contract", "settlement"}},
		{Audience: "money-flow-ledger", Action: "money-flow-ledger.reference.read", ResourceTypes: []string{"obligation", "transaction"}},
		{Audience: "billing-invoice", Action: "billing-invoice.reference.read", ResourceTypes: []string{"invoice_registration"}},
	}
	for _, want := range wantCommercial {
		found := false
		for _, permission := range project.Permissions {
			if permission.Audience == want.Audience && permission.Action == want.Action && slices.Equal(permission.ResourceTypes, want.ResourceTypes) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("project commercial permission missing: %#v from %#v", want, project.Permissions)
		}
	}
	wantMaterial := workloadPermission{Audience: "evidence-service", Action: "evidence-service.metadata.read", ResourceTypes: []string{"file"}}
	foundMaterial := false
	for _, permission := range project.Permissions {
		if permission.Audience == wantMaterial.Audience && permission.Action == wantMaterial.Action && slices.Equal(permission.ResourceTypes, wantMaterial.ResourceTypes) {
			foundMaterial = true
			break
		}
	}
	if !foundMaterial {
		t.Fatalf("project material permission missing: %#v from %#v", wantMaterial, project.Permissions)
	}
	wantMaterialSource := workloadPermission{Audience: "business-engagement", Action: "business-engagement.material-reference.read", ResourceTypes: []string{"business-engagement"}}
	foundMaterialSource := false
	for _, permission := range project.Permissions {
		if permission.Audience == wantMaterialSource.Audience && permission.Action == wantMaterialSource.Action && slices.Equal(permission.ResourceTypes, wantMaterialSource.ResourceTypes) {
			foundMaterialSource = true
			break
		}
	}
	if !foundMaterialSource {
		t.Fatalf("project material source permission missing: %#v from %#v", wantMaterialSource, project.Permissions)
	}
	var productionCount int
	for _, permission := range project.Permissions {
		if permission.Audience == "production-operations" {
			productionCount++
			if permission.Action != "production-work-item.reference.read" || !slices.Equal(permission.ResourceTypes, []string{"production-work-item"}) {
				t.Fatalf("Project Production capability expanded: %#v", permission)
			}
		}
		if permission.Audience == "evidence-service" && permission.Action != "evidence-service.metadata.read" {
			t.Fatalf("Project may not acquire file download/upload capabilities: %#v", permission)
		}
	}
	if productionCount != 1 {
		t.Fatalf("Project requires exactly one Production reference capability, got %d", productionCount)
	}
	wantRecorded := map[string][]string{
		"approval-audit-results/approval-audit-result.read": {"project", "approval_audit_result"},
		"cost-outsourcing/cost-entry.read":                  {"cost_entry"},
		"cost-outsourcing/outsourcing-engagement.read":      {"outsourcing_engagement"},
	}
	foundRecorded := map[string]bool{}
	for _, permission := range project.Permissions {
		if permission.Audience == "approval-audit-results" || permission.Audience == "cost-outsourcing" {
			key := permission.Audience + "/" + permission.Action
			want, ok := wantRecorded[key]
			if !ok || foundRecorded[key] || !slices.Equal(permission.ResourceTypes, want) {
				t.Fatalf("Project recorded material capability expanded: %#v", permission)
			}
			foundRecorded[key] = true
		}
	}
	if len(foundRecorded) != len(wantRecorded) {
		t.Fatal("missing exact recorded material capabilities")
	}
}

func TestBonusWorkloadPolicyHasOnlyExactSourceAndPayableSuggestionPermissions(t *testing.T) {
	policy := readStrict[workloadPolicy](t, "examples/workload-identity-policy.json")
	var bonus *workloadIdentity
	for index := range policy.Identities {
		if policy.Identities[index].ID == "bonus-distribution" {
			bonus = &policy.Identities[index]
			break
		}
	}
	if bonus == nil {
		t.Fatal("bonus-distribution workload identity is missing")
	}
	want := []workloadPermission{
		{Audience: "project-workspace", Action: "project-workspace.read", ResourceTypes: []string{"project"}},
		{Audience: "project-workspace", Action: "project-workspace.reference-oa-personnel.read", ResourceTypes: []string{"project"}},
		{Audience: "people-groups", Action: "people-groups.person.reference.read", ResourceTypes: []string{"person"}},
		{Audience: "contract-ledger", Action: "contract-ledger.reference.read", ResourceTypes: []string{"settlement"}},
		{Audience: "money-flow-ledger", Action: "money-flow-ledger.receipt-set.reference.read", ResourceTypes: []string{"receipt_set"}},
		{Audience: "money-flow-ledger", Action: "money-flow-ledger.payable-suggestion.submit", ResourceTypes: []string{"payable_suggestion"}},
		{Audience: "money-flow-ledger", Action: "money-flow-ledger.payable-suggestion.read", ResourceTypes: []string{"payable_suggestion"}},
		{Audience: "money-flow-ledger", Action: "money-flow-ledger.payable-suggestion.withdraw", ResourceTypes: []string{"payable_suggestion"}},
	}
	if bonus.SubjectID != "cccccccc-cccc-4ccc-8ccc-cccccccccccc" ||
		bonus.PublicKeysPath != "/workload-public/bonus-distribution.json" ||
		!slices.EqualFunc(bonus.Permissions, want, func(left, right workloadPermission) bool {
			return left.Audience == right.Audience && left.Action == right.Action && slices.Equal(left.ResourceTypes, right.ResourceTypes)
		}) {
		t.Fatalf("bonus workload identity=%#v want permissions=%#v", bonus, want)
	}
}

func TestCredentialNegativeCasesFailClosed(t *testing.T) {
	valid := readStrict[credentialClaims](t, "examples/credential-claims.json")
	tests := []struct {
		name   string
		mutate func(*credentialClaims)
	}{
		{"missing issuer", func(value *credentialClaims) { value.Issuer = "" }},
		{"wrong audience", func(value *credentialClaims) { value.Audience = "money-flow-ledger" }},
		{"wrong tenant", func(value *credentialClaims) { value.Tenant = "44444444-4444-4444-8444-444444444444" }},
		{"duplicate action", func(value *credentialClaims) { value.Actions = append(value.Actions, value.Actions[0]) }},
		{"tenant scope with resource", func(value *credentialClaims) {
			value.Scope.Resources = []resourceRef{{Type: "contract", ID: "c-1"}}
		}},
		{"empty resource scope", func(value *credentialClaims) { value.Scope.Kind = "resources" }},
		{"ttl over five minutes", func(value *credentialClaims) { value.ExpiresAt = value.IssuedAt + 301 }},
		{"expires before issue", func(value *credentialClaims) { value.ExpiresAt = value.IssuedAt - 1 }},
		{"invalid credential id", func(value *credentialClaims) { value.ID = "not-a-uuid" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := valid
			value.Actions = slices.Clone(valid.Actions)
			value.Scope.Resources = slices.Clone(valid.Scope.Resources)
			test.mutate(&value)
			if err := validateClaims(value, "contract-ledger", "22222222-2222-4222-8222-222222222222"); err == nil {
				t.Fatal("invalid credential was accepted")
			}
		})
	}
}

func TestManifestNegativeCasesFailClosed(t *testing.T) {
	validSuite := readStrict[suiteManifest](t, "examples/suite-manifest.json")
	t.Run("disabled IAM", func(t *testing.T) {
		value := validSuite
		value.Applications = slices.Clone(validSuite.Applications)
		value.Applications[0].Enabled = false
		if err := validateSuiteManifest(value); err == nil {
			t.Fatal("suite with disabled IAM was accepted")
		}
	})
	t.Run("duplicate route", func(t *testing.T) {
		value := validSuite
		value.Applications = slices.Clone(validSuite.Applications)
		value.Applications[1].RoutePrefix = value.Applications[0].RoutePrefix
		if err := validateSuiteManifest(value); err == nil {
			t.Fatal("suite with duplicate route was accepted")
		}
	})
	t.Run("overlapping route", func(t *testing.T) {
		value := validSuite
		value.Applications = slices.Clone(validSuite.Applications)
		value.Applications[1].RoutePrefix = value.Applications[0].RoutePrefix + "/admin"
		if err := validateSuiteManifest(value); err == nil {
			t.Fatal("suite with overlapping route was accepted")
		}
	})
	t.Run("unknown suite schema version", func(t *testing.T) {
		value := validSuite
		value.SchemaVersion = "2"
		if err := validateSuiteManifest(value); err == nil {
			t.Fatal("unknown suite schema version was accepted")
		}
	})
	projectionIndex := -1
	for index, app := range validSuite.Applications {
		if app.WorkItemProjection != nil {
			projectionIndex = index
			break
		}
	}
	if projectionIndex == -1 {
		t.Fatal("valid suite has no work item projection fixture")
	}
	for _, test := range []struct {
		name   string
		mutate func(*suiteManifest, *suiteWorkItemProjection)
	}{
		{name: "unknown work item projection schema", mutate: func(_ *suiteManifest, projection *suiteWorkItemProjection) { projection.SchemaVersion = "2" }},
		{name: "invalid work item projection path", mutate: func(_ *suiteManifest, projection *suiteWorkItemProjection) { projection.Path = "/work-items" }},
		{name: "foreign work item projection action", mutate: func(_ *suiteManifest, projection *suiteWorkItemProjection) {
			projection.Action = "identity-access.accounts.read"
		}},
		{name: "platform work item projection", mutate: func(value *suiteManifest, projection *suiteWorkItemProjection) {
			value.Applications[0].WorkItemProjection = projection
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := validSuite
			value.Applications = slices.Clone(validSuite.Applications)
			app := value.Applications[projectionIndex]
			projection := *app.WorkItemProjection
			test.mutate(&value, &projection)
			app.WorkItemProjection = &projection
			value.Applications[projectionIndex] = app
			if err := validateSuiteManifest(value); err == nil {
				t.Fatal("suite with an invalid work item projection was accepted")
			}
		})
	}
	metricIndex := -1
	for index, app := range validSuite.Applications {
		if app.MetricProjection != nil {
			metricIndex = index
			break
		}
	}
	if metricIndex == -1 {
		t.Fatal("valid suite has no metric projection fixture")
	}
	for _, test := range []struct {
		name   string
		mutate func(*suiteManifest, *suiteMetricProjection)
	}{
		{name: "unknown metric projection schema", mutate: func(_ *suiteManifest, projection *suiteMetricProjection) { projection.SchemaVersion = "2" }},
		{name: "invalid metric projection path", mutate: func(_ *suiteManifest, projection *suiteMetricProjection) { projection.Path = "/metrics" }},
		{name: "foreign metric projection action", mutate: func(_ *suiteManifest, projection *suiteMetricProjection) {
			projection.Action = "identity-access.accounts.read"
		}},
		{name: "platform metric projection", mutate: func(value *suiteManifest, projection *suiteMetricProjection) {
			value.Applications[0].MetricProjection = projection
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := validSuite
			value.Applications = slices.Clone(validSuite.Applications)
			app := value.Applications[metricIndex]
			projection := *app.MetricProjection
			test.mutate(&value, &projection)
			app.MetricProjection = &projection
			value.Applications[metricIndex] = app
			if err := validateSuiteManifest(value); err == nil {
				t.Fatal("suite with an invalid metric projection was accepted")
			}
		})
	}

	validCapabilities := readStrict[capabilityManifest](t, "examples/contract-ledger-capabilities.json")
	t.Run("capability outside application namespace", func(t *testing.T) {
		value := validCapabilities
		value.Capabilities = slices.Clone(validCapabilities.Capabilities)
		value.Capabilities[0].ID = "money-flow-ledger.view"
		if err := validateCapabilityManifest(value); err == nil {
			t.Fatal("foreign capability was accepted")
		}
	})
	t.Run("duplicate capability", func(t *testing.T) {
		value := validCapabilities
		value.Capabilities = append(slices.Clone(validCapabilities.Capabilities), validCapabilities.Capabilities[0])
		if err := validateCapabilityManifest(value); err == nil {
			t.Fatal("duplicate capability was accepted")
		}
	})
	t.Run("unknown capability schema version", func(t *testing.T) {
		value := validCapabilities
		value.SchemaVersion = "2"
		if err := validateCapabilityManifest(value); err == nil {
			t.Fatal("unknown capability schema version was accepted")
		}
	})

	validOverlay := readStrict[customerRoleCapabilityOverlay](t, "examples/reference-oa-c1-role-overlay.json")
	projectManifest := readStrict[capabilityManifest](t, "examples/project-workspace-capabilities.json")
	t.Run("overlay references unknown capability", func(t *testing.T) {
		value := validOverlay
		value.SiteRoles = cloneOverlayRoles(validOverlay.SiteRoles)
		role := value.SiteRoles["general_manager"]
		role.Actions = append(slices.Clone(role.Actions), "project-workspace.unknown")
		value.SiteRoles["general_manager"] = role
		if err := validateRoleOverlay(value, projectManifest); err == nil {
			t.Fatal("overlay with unknown capability was accepted")
		}
	})
	t.Run("overlay crosses application namespace", func(t *testing.T) {
		value := validOverlay
		value.Audience = "money-flow-ledger"
		if err := validateRoleOverlay(value, projectManifest); err == nil {
			t.Fatal("cross-application overlay was accepted")
		}
	})
}

func TestUnknownFieldsAreRejected(t *testing.T) {
	data := readFile(t, "examples/credential-claims.json")
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["role"] = "administrator"
	changed, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var claims credentialClaims
	if err := decodeStrict(changed, &claims); err == nil {
		t.Fatal("unknown role field was accepted")
	}
}

func validateClaims(value credentialClaims, audience, tenant string) error {
	if value.Version != "1" || value.Issuer == "" || !uuidPattern.MatchString(value.Subject) ||
		!uuidPattern.MatchString(value.Tenant) || !uuidPattern.MatchString(value.ID) {
		return fmt.Errorf("credential identity fields are invalid")
	}
	issuer, err := url.Parse(value.Issuer)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" {
		return fmt.Errorf("credential issuer must be an absolute HTTPS URL")
	}
	if value.Audience != audience || value.Tenant != tenant || !identifierPattern.MatchString(value.Audience) {
		return fmt.Errorf("credential audience or tenant mismatch")
	}
	if strings.TrimSpace(value.Name) == "" || len([]rune(value.Name)) > 100 {
		return fmt.Errorf("credential display name is invalid")
	}
	if err := validateActions(value.Actions, value.Audience); err != nil {
		return err
	}
	if err := validateScope(value.Scope); err != nil {
		return err
	}
	if value.IssuedAt <= 0 || value.NotBefore < value.IssuedAt-30 || value.NotBefore > value.IssuedAt+30 ||
		value.AuthTime <= 0 || value.AuthTime > value.IssuedAt || value.ExpiresAt <= value.IssuedAt || value.ExpiresAt-value.IssuedAt > 300 {
		return fmt.Errorf("credential times are invalid")
	}
	return nil
}

func validateContext(value iamContext, audience, tenant string) error {
	if value.SchemaVersion != "1" || value.Audience != audience || value.TenantID != tenant ||
		!uuidPattern.MatchString(value.SubjectID) || !uuidPattern.MatchString(value.TenantID) || !uuidPattern.MatchString(value.CredentialID) {
		return fmt.Errorf("IAM context identity fields are invalid")
	}
	issuer, err := url.Parse(value.Issuer)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" || strings.TrimSpace(value.DisplayName) == "" || len([]rune(value.DisplayName)) > 100 {
		return fmt.Errorf("IAM context issuer or display name is invalid")
	}
	if err := validateActions(value.Actions, value.Audience); err != nil {
		return err
	}
	if err := validateScope(value.Scope); err != nil {
		return err
	}
	issued, err := time.Parse(time.RFC3339, value.IssuedAt)
	if err != nil {
		return fmt.Errorf("parse issued_at: %w", err)
	}
	expires, err := time.Parse(time.RFC3339, value.ExpiresAt)
	if err != nil || !expires.After(issued) || expires.Sub(issued) > 5*time.Minute {
		return fmt.Errorf("IAM context expiry is invalid")
	}
	return nil
}

func validateCapabilityManifest(value capabilityManifest) error {
	if value.SchemaVersion != "1" || !identifierPattern.MatchString(value.ApplicationID) ||
		value.Audience != value.ApplicationID || !versionPattern.MatchString(value.ApplicationVersion) || len(value.Capabilities) == 0 {
		return fmt.Errorf("capability manifest identity is invalid")
	}
	namespaces, err := capabilityNamespaces(value)
	if err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, item := range value.Capabilities {
		if !actionPattern.MatchString(item.ID) || !actionHasNamespace(item.ID, namespaces) || strings.TrimSpace(item.Description) == "" {
			return fmt.Errorf("capability %q is invalid", item.ID)
		}
		if item.Risk != "read" && item.Risk != "write" && item.Risk != "privileged" {
			return fmt.Errorf("capability %q risk is invalid", item.ID)
		}
		resourceTypes := map[string]struct{}{}
		for _, resourceType := range item.ResourceTypes {
			if !resourcePattern.MatchString(resourceType) {
				return fmt.Errorf("capability %q resource type is invalid", item.ID)
			}
			if _, exists := resourceTypes[resourceType]; exists {
				return fmt.Errorf("capability %q has duplicate resource type", item.ID)
			}
			resourceTypes[resourceType] = struct{}{}
		}
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("duplicate capability %q", item.ID)
		}
		seen[item.ID] = struct{}{}
	}
	return nil
}

func validateRoleOverlay(value customerRoleCapabilityOverlay, manifest capabilityManifest) error {
	if value.SchemaVersion != "1" || !identifierPattern.MatchString(value.OverlayID) || !versionPattern.MatchString(value.OverlayVersion) ||
		value.ApplicationID != manifest.ApplicationID || value.Audience != manifest.Audience || len(value.SiteRoles) == 0 {
		return fmt.Errorf("customer role overlay identity is invalid")
	}
	knownActions := make(map[string]struct{}, len(manifest.Capabilities))
	namespaces, err := capabilityNamespaces(manifest)
	if err != nil {
		return err
	}
	for _, item := range manifest.Capabilities {
		knownActions[item.ID] = struct{}{}
	}
	for roleKey, role := range value.SiteRoles {
		if !resourcePattern.MatchString(roleKey) || strings.TrimSpace(role.DisplayName) == "" || len([]rune(role.DisplayName)) > 100 ||
			(role.ScopeKind != "tenant" && role.ScopeKind != "resources") {
			return fmt.Errorf("overlay role %q is invalid", roleKey)
		}
		seen := map[string]struct{}{}
		for _, action := range role.Actions {
			if !actionPattern.MatchString(action) || !actionHasNamespace(action, namespaces) {
				return fmt.Errorf("overlay role %q action %q is outside audience", roleKey, action)
			}
			if _, known := knownActions[action]; !known {
				return fmt.Errorf("overlay role %q references unknown action %q", roleKey, action)
			}
			if _, duplicate := seen[action]; duplicate {
				return fmt.Errorf("overlay role %q duplicates action %q", roleKey, action)
			}
			seen[action] = struct{}{}
		}
	}
	if value.AssignmentPolicy != nil && (!identifierPattern.MatchString(value.AssignmentPolicy.PolicyID) || !versionPattern.MatchString(value.AssignmentPolicy.PolicyVersion)) {
		return fmt.Errorf("assignment policy reference is invalid")
	}
	return nil
}

func capabilityNamespaces(value capabilityManifest) ([]string, error) {
	if value.CapabilityNamespace != "" && len(value.CapabilityNamespaces) != 0 {
		return nil, fmt.Errorf("single and multiple capability namespaces cannot both be declared")
	}
	namespaces := append([]string(nil), value.CapabilityNamespaces...)
	if len(namespaces) == 0 {
		namespace := value.CapabilityNamespace
		if namespace == "" {
			namespace = value.ApplicationID
		}
		namespaces = []string{namespace}
	}
	if len(value.CapabilityNamespaces) == 1 {
		return nil, fmt.Errorf("multiple capability namespaces require at least two entries")
	}
	seen := map[string]struct{}{}
	for _, namespace := range namespaces {
		if !identifierPattern.MatchString(namespace) {
			return nil, fmt.Errorf("capability namespace is invalid")
		}
		if _, duplicate := seen[namespace]; duplicate {
			return nil, fmt.Errorf("duplicate capability namespace")
		}
		seen[namespace] = struct{}{}
	}
	return namespaces, nil
}

func actionHasNamespace(action string, namespaces []string) bool {
	for _, namespace := range namespaces {
		if strings.HasPrefix(action, namespace+".") {
			return true
		}
	}
	return false
}

func cloneOverlayRoles(source map[string]overlayRole) map[string]overlayRole {
	result := make(map[string]overlayRole, len(source))
	for key, value := range source {
		value.Actions = slices.Clone(value.Actions)
		result[key] = value
	}
	return result
}

func validateSuiteManifest(value suiteManifest) error {
	if value.SchemaVersion != "1" || !identifierPattern.MatchString(value.SuiteID) || !uuidPattern.MatchString(value.TenantID) || len(value.Applications) < 2 {
		return fmt.Errorf("suite manifest identity is invalid")
	}
	ids := map[string]struct{}{}
	routes := map[string]struct{}{}
	audiences := map[string]struct{}{}
	iamValid := false
	for _, app := range value.Applications {
		if !identifierPattern.MatchString(app.ID) || app.Audience != app.ID || !versionPattern.MatchString(app.Version) || !strings.HasPrefix(app.RoutePrefix, "/") || !strings.HasPrefix(app.ReadinessPath, "/") {
			return fmt.Errorf("suite application %q is invalid", app.ID)
		}
		upstream, err := url.Parse(app.Upstream)
		if err != nil || upstream.Scheme != "http" || upstream.Host == "" || upstream.Path != "" {
			return fmt.Errorf("suite application %q upstream is invalid", app.ID)
		}
		if _, exists := ids[app.ID]; exists {
			return fmt.Errorf("duplicate application %q", app.ID)
		}
		for route := range routes {
			if routesOverlap(route, app.RoutePrefix) {
				return fmt.Errorf("overlapping route %q", app.RoutePrefix)
			}
		}
		if _, exists := audiences[app.Audience]; exists {
			return fmt.Errorf("duplicate audience %q", app.Audience)
		}
		ids[app.ID] = struct{}{}
		routes[app.RoutePrefix] = struct{}{}
		audiences[app.Audience] = struct{}{}
		if app.ID == "identity-access" {
			iamValid = app.Enabled && app.Required
		}
		platformEntry := app.ID == "identity-access" || app.ID == "unified-workbench"
		if platformEntry && (app.EntryAction != "" || app.EntryActions != nil) {
			return fmt.Errorf("platform application %q cannot declare an entry action", app.ID)
		}
		if app.CapabilityNamespace != "" && app.CapabilityNamespaces != nil {
			return fmt.Errorf("application %q capability namespace forms are exclusive", app.ID)
		}
		namespaces := app.CapabilityNamespaces
		if namespaces == nil {
			namespace := app.CapabilityNamespace
			if namespace == "" {
				namespace = app.Audience
			}
			namespaces = []string{namespace}
		}
		if len(namespaces) == 0 || len(namespaces) > 16 {
			return fmt.Errorf("application %q capability namespaces are unbounded", app.ID)
		}
		seenNamespaces := map[string]bool{}
		for _, namespace := range namespaces {
			if !identifierPattern.MatchString(namespace) || seenNamespaces[namespace] {
				return fmt.Errorf("application %q capability namespace is invalid", app.ID)
			}
			seenNamespaces[namespace] = true
		}
		native := func(action string) bool {
			for _, namespace := range namespaces {
				if strings.HasPrefix(action, namespace+".") {
					return true
				}
			}
			return false
		}
		if !platformEntry {
			entries := app.entryCapabilities()
			if (app.EntryAction != "" && app.EntryActions != nil) || len(entries) == 0 || len(entries) > 16 {
				return fmt.Errorf("application %q entry capabilities are invalid", app.ID)
			}
			seen := map[string]bool{}
			for _, action := range entries {
				if !actionPattern.MatchString(action) || !native(action) || seen[action] {
					return fmt.Errorf("application %q entry action is invalid", app.ID)
				}
				seen[action] = true
			}
		}
		if app.WorkItemProjection != nil {
			projection := app.WorkItemProjection
			if platformEntry || projection.SchemaVersion != "1" || !strings.HasPrefix(projection.Path, "/api/") || strings.Contains(projection.Path, "?") || !actionPattern.MatchString(projection.Action) || !native(projection.Action) || len(app.entryCapabilities()) == 0 || app.CapabilityManifest == "" {
				return fmt.Errorf("application %q work item projection is invalid", app.ID)
			}
		}
		if app.MetricProjection != nil {
			projection := app.MetricProjection
			if platformEntry || projection.SchemaVersion != "1" || !strings.HasPrefix(projection.Path, "/api/") || strings.Contains(projection.Path, "?") || !actionPattern.MatchString(projection.Action) || !native(projection.Action) || len(app.entryCapabilities()) == 0 || app.CapabilityManifest == "" {
				return fmt.Errorf("application %q metric projection is invalid", app.ID)
			}
		}
	}
	if !iamValid {
		return fmt.Errorf("identity-access must be enabled and required")
	}
	return nil
}

func routesOverlap(left, right string) bool {
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

func validateActions(actions []string, audience string) error {
	if len(actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}
	seen := map[string]struct{}{}
	for _, action := range actions {
		if !actionPattern.MatchString(action) || !strings.HasPrefix(action, audience+".") {
			return fmt.Errorf("action %q is outside audience namespace", action)
		}
		if _, exists := seen[action]; exists {
			return fmt.Errorf("duplicate action %q", action)
		}
		seen[action] = struct{}{}
	}
	return nil
}

func validateScope(value scope) error {
	switch value.Kind {
	case "tenant":
		if len(value.Resources) != 0 {
			return fmt.Errorf("tenant scope cannot contain resources")
		}
	case "resources":
		if len(value.Resources) == 0 {
			return fmt.Errorf("resource scope must contain resources")
		}
		seen := map[string]struct{}{}
		for _, resource := range value.Resources {
			key := resource.Type + "\x00" + resource.ID
			if !resourcePattern.MatchString(resource.Type) || strings.TrimSpace(resource.ID) == "" || len(resource.ID) > 128 {
				return fmt.Errorf("resource reference is incomplete")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate resource reference")
			}
			seen[key] = struct{}{}
		}
	default:
		return fmt.Errorf("unknown scope kind %q", value.Kind)
	}
	return nil
}

func validateMetricProjection(value metricProjection) error {
	if value.SchemaVersion != "1" || value.SourceApplication != "money-flow-ledger" ||
		!uuidPattern.MatchString(value.TenantID) || value.DefinitionVersion != "money-flow-operating-summary-v1" {
		return fmt.Errorf("metric projection identity is invalid")
	}
	observedAt, err := time.Parse(time.RFC3339, value.ObservedAt)
	if err != nil || observedAt.IsZero() {
		return fmt.Errorf("metric projection observed_at is invalid")
	}
	knownMetrics := map[string]struct{}{
		"receivable_outstanding": {}, "payable_outstanding": {},
		"receipt_effective": {}, "payment_effective": {},
		"receipt_unallocated": {}, "payment_unallocated": {},
	}
	seen := map[string]struct{}{}
	for _, item := range value.Items {
		if _, known := knownMetrics[item.MetricKey]; !known {
			return fmt.Errorf("unknown metric key %q", item.MetricKey)
		}
		if item.Scope.Kind != "ledger_owner" || item.Scope.TenantID != value.TenantID ||
			item.Scope.ResourceType != "party" || strings.TrimSpace(item.Scope.ResourceID) == "" {
			return fmt.Errorf("metric %q scope is invalid", item.MetricKey)
		}
		if item.Unit != "money" || len(item.Currency) != 3 || strings.ToUpper(item.Currency) != item.Currency ||
			item.TimeRange != "current" || !strings.HasPrefix(item.DeepLink, "/operating-summary/details?") {
			return fmt.Errorf("metric %q display contract is invalid", item.MetricKey)
		}
		if parsed, parseErr := time.Parse(time.RFC3339, item.ObservedAt); parseErr != nil || !parsed.Equal(observedAt) {
			return fmt.Errorf("metric %q observation is inconsistent", item.MetricKey)
		}
		if item.Redacted {
			if item.Value != nil || item.SourceDigest != nil {
				return fmt.Errorf("redacted metric %q leaks a value or digest", item.MetricKey)
			}
		} else if item.Value == nil || !decimalPattern.MatchString(*item.Value) ||
			item.SourceDigest == nil || !digestPattern.MatchString(*item.SourceDigest) {
			return fmt.Errorf("visible metric %q has invalid value evidence", item.MetricKey)
		}
		key := item.MetricKey + "\x00" + item.Scope.ResourceID + "\x00" + item.Currency
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate metric identity %q", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func readStrict[T any](t *testing.T, name string) T {
	t.Helper()
	var value T
	if err := decodeStrict(readFile(t, name), &value); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return value
}

func decodeFile(t *testing.T, name string, destination any) {
	t.Helper()
	if err := decodeStrict(readFileAbsolute(t, name), destination); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
}

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	return readFileAbsolute(t, filepath.Join(contractRoot, name))
}

func readFileAbsolute(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}

func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return fmt.Errorf("decode trailing JSON: %w", err)
	}
	return nil
}
