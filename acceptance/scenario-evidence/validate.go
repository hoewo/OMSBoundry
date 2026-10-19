package scenarioevidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const SchemaVersion = "customer-scenario-evidence/v1"

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)

type Document struct {
	SchemaVersion      string          `json:"schema_version"`
	ScenarioID         string          `json:"scenario_id"`
	Title              string          `json:"title"`
	SourceKind         string          `json:"source_kind"`
	ClaimScope         string          `json:"claim_scope"`
	AssessmentStatus   string          `json:"assessment_status"`
	Problem            Problem         `json:"problem"`
	SourceEvidence     SourceEvidence  `json:"source_evidence"`
	Facts              []Fact          `json:"facts"`
	Applications       Applications    `json:"applications"`
	Differences        []Difference    `json:"differences"`
	Connectors         []Connector     `json:"connectors"`
	BoundarySignals    BoundarySignals `json:"boundary_signals"`
	AcceptanceEvidence []string        `json:"acceptance_evidence"`
	Limitations        []string        `json:"limitations"`
}

type Problem struct {
	Statement          string   `json:"statement"`
	ObservableOutcomes []string `json:"observable_outcomes"`
	OutOfScope         []string `json:"out_of_scope"`
}

type SourceEvidence struct {
	EvidenceRefs             []string `json:"evidence_refs"`
	CustomerConfirmationRefs []string `json:"customer_confirmation_refs"`
}

type Fact struct {
	ID           string   `json:"id"`
	Owner        string   `json:"owner"`
	Meaning      string   `json:"meaning"`
	States       []string `json:"states"`
	Correction   string   `json:"correction"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type Applications struct {
	Enabled  []string `json:"enabled"`
	Disabled []string `json:"disabled"`
}

type Difference struct {
	ID                string          `json:"id"`
	Requirement       string          `json:"requirement"`
	Layer             string          `json:"layer"`
	Disposition       string          `json:"disposition"`
	Rationale         string          `json:"rationale"`
	FactRefs          []string        `json:"fact_refs"`
	DeliveryArtifacts []string        `json:"delivery_artifacts"`
	EvidenceRefs      []string        `json:"evidence_refs"`
	PolicyEvidence    *PolicyEvidence `json:"policy_evidence,omitempty"`
}

type PolicyEvidence struct {
	Version             string   `json:"version"`
	Inputs              []string `json:"inputs"`
	Output              string   `json:"output"`
	Correction          string   `json:"correction"`
	RuntimeEvidenceRefs []string `json:"runtime_evidence_refs"`
}

type Connector struct {
	ID                 string `json:"id"`
	DifferenceRef      string `json:"difference_ref"`
	SourceApplication  string `json:"source_application"`
	TargetApplication  string `json:"target_application"`
	SourceContract     string `json:"source_contract"`
	TargetContract     string `json:"target_contract"`
	Idempotency        string `json:"idempotency"`
	FailureRecovery    string `json:"failure_recovery"`
	TargetConfirmation string `json:"target_confirmation"`
}

type BoundarySignals struct {
	CustomerCoreBranches        *int `json:"customer_core_branches"`
	CustomerDatabases           *int `json:"customer_databases"`
	SharedBusinessTables        *int `json:"shared_business_tables"`
	CustomerBusinessMigrations  *int `json:"customer_business_migrations"`
	CrossAppInternalReads       *int `json:"cross_app_internal_reads"`
	OnsiteCorePatches           *int `json:"onsite_core_patches"`
	HiddenMandatoryDependencies *int `json:"hidden_mandatory_dependencies"`
}

func Parse(data []byte) (Document, error) {
	if err := validateJSONShape(data); err != nil {
		return Document{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document Document
	if err := decoder.Decode(&document); err != nil {
		return Document{}, fmt.Errorf("decode scenario evidence: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return Document{}, err
	}
	return document, nil
}

func validateJSONShape(data []byte) error {
	var raw any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("decode scenario evidence shape: %w", err)
	}
	root, err := requiredObject(raw, "$", "schema_version", "scenario_id", "title", "source_kind", "claim_scope", "assessment_status", "problem", "source_evidence", "facts", "applications", "differences", "connectors", "boundary_signals", "acceptance_evidence", "limitations")
	if err != nil {
		return err
	}
	if _, err := requiredObject(root["problem"], "problem", "statement", "observable_outcomes", "out_of_scope"); err != nil {
		return err
	}
	if _, err := requiredObject(root["source_evidence"], "source_evidence", "evidence_refs", "customer_confirmation_refs"); err != nil {
		return err
	}
	if _, err := requiredObject(root["applications"], "applications", "enabled", "disabled"); err != nil {
		return err
	}
	if _, err := requiredObject(root["boundary_signals"], "boundary_signals", "customer_core_branches", "customer_databases", "shared_business_tables", "customer_business_migrations", "cross_app_internal_reads", "onsite_core_patches", "hidden_mandatory_dependencies"); err != nil {
		return err
	}
	arrayPaths := []string{"problem.observable_outcomes", "problem.out_of_scope", "source_evidence.evidence_refs", "source_evidence.customer_confirmation_refs", "facts", "applications.enabled", "applications.disabled", "differences", "connectors", "acceptance_evidence", "limitations"}
	for _, path := range arrayPaths {
		if _, err := requiredArray(valueAt(root, path), path); err != nil {
			return err
		}
	}
	facts, _ := requiredArray(root["facts"], "facts")
	for index, item := range facts {
		path := fmt.Sprintf("facts[%d]", index)
		fact, err := requiredObject(item, path, "id", "owner", "meaning", "states", "correction", "evidence_refs")
		if err != nil {
			return err
		}
		if _, err := requiredArray(fact["states"], path+".states"); err != nil {
			return err
		}
		if _, err := requiredArray(fact["evidence_refs"], path+".evidence_refs"); err != nil {
			return err
		}
	}
	differences, _ := requiredArray(root["differences"], "differences")
	for index, item := range differences {
		path := fmt.Sprintf("differences[%d]", index)
		difference, err := requiredObject(item, path, "id", "requirement", "layer", "disposition", "rationale", "fact_refs", "delivery_artifacts", "evidence_refs")
		if err != nil {
			return err
		}
		for _, key := range []string{"fact_refs", "delivery_artifacts", "evidence_refs"} {
			if _, err := requiredArray(difference[key], path+"."+key); err != nil {
				return err
			}
		}
		if policy, exists := difference["policy_evidence"]; exists {
			policyObject, err := requiredObject(policy, path+".policy_evidence", "version", "inputs", "output", "correction", "runtime_evidence_refs")
			if err != nil {
				return err
			}
			if _, err := requiredArray(policyObject["inputs"], path+".policy_evidence.inputs"); err != nil {
				return err
			}
			if _, err := requiredArray(policyObject["runtime_evidence_refs"], path+".policy_evidence.runtime_evidence_refs"); err != nil {
				return err
			}
		}
	}
	connectors, _ := requiredArray(root["connectors"], "connectors")
	for index, item := range connectors {
		path := fmt.Sprintf("connectors[%d]", index)
		if _, err := requiredObject(item, path, "id", "difference_ref", "source_application", "target_application", "source_contract", "target_contract", "idempotency", "failure_recovery", "target_confirmation"); err != nil {
			return err
		}
	}
	return nil
}

func requiredObject(value any, path string, keys ...string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", path)
	}
	for _, key := range keys {
		if _, exists := object[key]; !exists {
			return nil, fmt.Errorf("%s.%s is required", path, key)
		}
	}
	return object, nil
}

func requiredArray(value any, path string) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", path)
	}
	return items, nil
}

func valueAt(root map[string]any, path string) any {
	var current any = root
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

func ValidateBytes(data []byte) error {
	document, err := Parse(data)
	if err != nil {
		return err
	}
	return Validate(document)
}

func Validate(document Document) error {
	var issues []string
	if document.SchemaVersion != SchemaVersion {
		issues = append(issues, "schema_version must be customer-scenario-evidence/v1")
	}
	checkIdentifier(&issues, "scenario_id", document.ScenarioID)
	checkText(&issues, "title", document.Title)
	checkEnum(&issues, "source_kind", document.SourceKind, "reference_product", "real_customer_anonymized", "hypothetical_probe")
	checkEnum(&issues, "claim_scope", document.ClaimScope, "none", "reference_baseline", "single_customer_scenario")
	checkEnum(&issues, "assessment_status", document.AssessmentStatus, "draft", "evidence_ready", "passed", "failed")
	checkText(&issues, "problem.statement", document.Problem.Statement)
	checkStrings(&issues, "problem.observable_outcomes", document.Problem.ObservableOutcomes)
	checkStrings(&issues, "problem.out_of_scope", document.Problem.OutOfScope)
	checkStrings(&issues, "source_evidence.evidence_refs", document.SourceEvidence.EvidenceRefs)
	checkStrings(&issues, "source_evidence.customer_confirmation_refs", document.SourceEvidence.CustomerConfirmationRefs)
	checkStrings(&issues, "acceptance_evidence", document.AcceptanceEvidence)
	checkStrings(&issues, "limitations", document.Limitations)

	validateClaimBoundary(&issues, document)
	facts := validateFacts(&issues, document.Facts)
	validateApplications(&issues, document.Applications)
	differences := validateDifferences(&issues, document, facts)
	validateConnectors(&issues, document.Connectors, differences)
	validateBoundarySignals(&issues, document.AssessmentStatus, document.BoundarySignals)
	validateEvidenceReadiness(&issues, document)

	if len(issues) > 0 {
		return errors.New(strings.Join(issues, "; "))
	}
	return nil
}

func validateClaimBoundary(issues *[]string, document Document) {
	switch document.SourceKind {
	case "reference_product":
		if document.ClaimScope != "reference_baseline" {
			*issues = append(*issues, "reference_product may only claim reference_baseline")
		}
		if len(document.SourceEvidence.CustomerConfirmationRefs) != 0 {
			*issues = append(*issues, "reference_product cannot carry customer confirmation")
		}
	case "real_customer_anonymized":
		if document.ClaimScope != "single_customer_scenario" {
			*issues = append(*issues, "real_customer_anonymized may only claim single_customer_scenario")
		}
	case "hypothetical_probe":
		if document.ClaimScope != "none" {
			*issues = append(*issues, "hypothetical_probe cannot claim scenario acceptance")
		}
		if document.AssessmentStatus == "passed" || document.AssessmentStatus == "failed" {
			*issues = append(*issues, "hypothetical_probe cannot have a final assessment status")
		}
	}
	if document.ClaimScope == "single_customer_scenario" && document.SourceKind != "real_customer_anonymized" {
		*issues = append(*issues, "single_customer_scenario requires real_customer_anonymized evidence")
	}
}

func validateFacts(issues *[]string, facts []Fact) map[string]struct{} {
	seen := map[string]struct{}{}
	for index, fact := range facts {
		path := fmt.Sprintf("facts[%d]", index)
		checkIdentifier(issues, path+".id", fact.ID)
		checkText(issues, path+".owner", fact.Owner)
		checkText(issues, path+".meaning", fact.Meaning)
		checkText(issues, path+".correction", fact.Correction)
		checkStrings(issues, path+".states", fact.States)
		if len(fact.States) == 0 {
			*issues = append(*issues, path+".states must not be empty")
		}
		checkStrings(issues, path+".evidence_refs", fact.EvidenceRefs)
		if _, exists := seen[fact.ID]; exists {
			*issues = append(*issues, path+".id is duplicated")
		}
		seen[fact.ID] = struct{}{}
	}
	return seen
}

func validateApplications(issues *[]string, applications Applications) {
	enabled := validateIdentifierList(issues, "applications.enabled", applications.Enabled)
	disabled := validateIdentifierList(issues, "applications.disabled", applications.Disabled)
	for id := range enabled {
		if _, exists := disabled[id]; exists {
			*issues = append(*issues, "applications cannot enable and disable the same application "+id)
		}
	}
}

func validateDifferences(issues *[]string, document Document, facts map[string]struct{}) map[string]Difference {
	seen := map[string]Difference{}
	for index, difference := range document.Differences {
		path := fmt.Sprintf("differences[%d]", index)
		checkIdentifier(issues, path+".id", difference.ID)
		checkText(issues, path+".requirement", difference.Requirement)
		checkEnum(issues, path+".layer", difference.Layer, "stable_core", "configuration", "policy", "typed_extension", "customer_assembly", "customer_specific")
		checkEnum(issues, path+".disposition", difference.Disposition, "implemented", "accepted_deferred", "rejected", "needs_business_evidence")
		checkText(issues, path+".rationale", difference.Rationale)
		checkStrings(issues, path+".delivery_artifacts", difference.DeliveryArtifacts)
		checkStrings(issues, path+".evidence_refs", difference.EvidenceRefs)
		for _, ref := range difference.FactRefs {
			checkIdentifier(issues, path+".fact_refs", ref)
			if _, exists := facts[ref]; !exists {
				*issues = append(*issues, path+".fact_refs references unknown fact "+ref)
			}
		}
		if _, exists := seen[difference.ID]; exists {
			*issues = append(*issues, path+".id is duplicated")
		}
		seen[difference.ID] = difference

		if difference.Disposition == "implemented" && document.AssessmentStatus != "draft" {
			if len(difference.DeliveryArtifacts) == 0 {
				*issues = append(*issues, path+" implemented difference requires delivery_artifacts")
			}
			if len(difference.EvidenceRefs) == 0 {
				*issues = append(*issues, path+" implemented difference requires evidence_refs")
			}
		}
		if difference.Layer == "policy" && difference.Disposition == "implemented" {
			validatePolicyEvidence(issues, path+".policy_evidence", difference.PolicyEvidence)
		} else if difference.PolicyEvidence != nil {
			*issues = append(*issues, path+" policy_evidence is only valid for an implemented policy")
		}
	}
	return seen
}

func validatePolicyEvidence(issues *[]string, path string, evidence *PolicyEvidence) {
	if evidence == nil {
		*issues = append(*issues, path+" is required for an implemented policy")
		return
	}
	checkText(issues, path+".version", evidence.Version)
	validateIdentifierList(issues, path+".inputs", evidence.Inputs)
	if len(evidence.Inputs) == 0 {
		*issues = append(*issues, path+".inputs must not be empty")
	}
	checkText(issues, path+".output", evidence.Output)
	checkText(issues, path+".correction", evidence.Correction)
	checkStrings(issues, path+".runtime_evidence_refs", evidence.RuntimeEvidenceRefs)
	if len(evidence.RuntimeEvidenceRefs) == 0 {
		*issues = append(*issues, path+".runtime_evidence_refs must not be empty")
	}
}

func validateConnectors(issues *[]string, connectors []Connector, differences map[string]Difference) {
	seen := map[string]struct{}{}
	for index, connector := range connectors {
		path := fmt.Sprintf("connectors[%d]", index)
		checkIdentifier(issues, path+".id", connector.ID)
		checkIdentifier(issues, path+".difference_ref", connector.DifferenceRef)
		checkIdentifier(issues, path+".source_application", connector.SourceApplication)
		checkIdentifier(issues, path+".target_application", connector.TargetApplication)
		checkText(issues, path+".source_contract", connector.SourceContract)
		checkText(issues, path+".target_contract", connector.TargetContract)
		checkText(issues, path+".idempotency", connector.Idempotency)
		checkText(issues, path+".failure_recovery", connector.FailureRecovery)
		checkText(issues, path+".target_confirmation", connector.TargetConfirmation)
		if connector.SourceApplication == connector.TargetApplication {
			*issues = append(*issues, path+" source and target applications must differ")
		}
		difference, exists := differences[connector.DifferenceRef]
		if !exists {
			*issues = append(*issues, path+" references unknown difference "+connector.DifferenceRef)
		} else if difference.Layer != "customer_assembly" && difference.Layer != "customer_specific" {
			*issues = append(*issues, path+" must belong to customer_assembly or customer_specific")
		} else if difference.Disposition != "implemented" {
			*issues = append(*issues, path+" cannot implement a connector for a non-implemented difference")
		}
		if _, exists := seen[connector.ID]; exists {
			*issues = append(*issues, path+".id is duplicated")
		}
		seen[connector.ID] = struct{}{}
	}
}

func validateBoundarySignals(issues *[]string, status string, signals BoundarySignals) {
	values := []struct {
		name  string
		value *int
	}{
		{"customer_core_branches", signals.CustomerCoreBranches},
		{"customer_databases", signals.CustomerDatabases},
		{"shared_business_tables", signals.SharedBusinessTables},
		{"customer_business_migrations", signals.CustomerBusinessMigrations},
		{"cross_app_internal_reads", signals.CrossAppInternalReads},
		{"onsite_core_patches", signals.OnsiteCorePatches},
		{"hidden_mandatory_dependencies", signals.HiddenMandatoryDependencies},
	}
	for _, item := range values {
		if item.value != nil && *item.value < 0 {
			*issues = append(*issues, "boundary_signals."+item.name+" must be non-negative")
		}
		if status == "passed" {
			if item.value == nil {
				*issues = append(*issues, "passed assessment requires boundary_signals."+item.name)
			} else if *item.value != 0 {
				*issues = append(*issues, "passed assessment requires boundary_signals."+item.name+" to be zero")
			}
		}
	}
}

func validateEvidenceReadiness(issues *[]string, document Document) {
	if document.AssessmentStatus == "draft" {
		return
	}
	if len(document.Problem.ObservableOutcomes) == 0 {
		*issues = append(*issues, "non-draft assessment requires observable outcomes")
	}
	if len(document.SourceEvidence.EvidenceRefs) == 0 {
		*issues = append(*issues, "non-draft assessment requires source evidence")
	}
	if len(document.Facts) == 0 {
		*issues = append(*issues, "non-draft assessment requires facts")
	}
	if len(document.Applications.Enabled) == 0 {
		*issues = append(*issues, "non-draft assessment requires enabled applications")
	}
	if len(document.Differences) == 0 {
		*issues = append(*issues, "non-draft assessment requires differences")
	}
	if document.AssessmentStatus == "passed" || document.AssessmentStatus == "failed" {
		if document.ClaimScope == "none" {
			*issues = append(*issues, "final assessment requires an explicit claim_scope")
		}
		if len(document.AcceptanceEvidence) == 0 {
			*issues = append(*issues, "final assessment requires acceptance_evidence")
		}
	}
	if document.SourceKind == "real_customer_anonymized" && (document.AssessmentStatus == "passed" || document.AssessmentStatus == "failed") && len(document.SourceEvidence.CustomerConfirmationRefs) == 0 {
		*issues = append(*issues, "final real-customer assessment requires customer_confirmation_refs")
	}
}

func validateIdentifierList(issues *[]string, path string, values []string) map[string]struct{} {
	seen := map[string]struct{}{}
	for _, value := range values {
		checkIdentifier(issues, path, value)
		if _, exists := seen[value]; exists {
			*issues = append(*issues, path+" contains duplicate "+value)
		}
		seen[value] = struct{}{}
	}
	return seen
}

func checkIdentifier(issues *[]string, path, value string) {
	if !identifierPattern.MatchString(value) {
		*issues = append(*issues, path+" must be a lowercase kebab-case identifier")
	}
}

func checkText(issues *[]string, path, value string) {
	if strings.TrimSpace(value) == "" {
		*issues = append(*issues, path+" must not be empty")
	}
}

func checkStrings(issues *[]string, path string, values []string) {
	for _, value := range values {
		checkText(issues, path, value)
	}
}

func checkEnum(issues *[]string, path, value string, allowed ...string) {
	for _, candidate := range allowed {
		if value == candidate {
			return
		}
	}
	*issues = append(*issues, path+" has unsupported value "+value)
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing scenario evidence: %w", err)
	}
	return errors.New("scenario evidence contains multiple JSON documents")
}
