package references

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"omsbizfoundry/bonus-distribution/internal/bonus"
	"omsbizfoundry/bonus-distribution/internal/referencebonus"
	workloadidentity "omsbizfoundry/workload-identity"
)

const maxResponseBytes = 1 << 20

type CredentialSource interface {
	Credential(context.Context, string, string, string, string) (string, error)
	Invalidate(string, string, string, string)
}

var _ CredentialSource = (*workloadidentity.TokenSource)(nil)

type Endpoint struct {
	BaseURL     string
	AccessToken string
	Audience    string
	Action      string
	Credentials CredentialSource
}

type HTTPResolver struct {
	client          *http.Client
	project, people Endpoint
	contract, money Endpoint
	professional    Endpoint
}

func NewHTTPResolver(timeout time.Duration, project, people, contract, money Endpoint) (*HTTPResolver, error) {
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("reference timeout must be between 1ns and 30s")
	}
	for name, endpoint := range map[string]Endpoint{"project": project, "people": people} {
		if err := validateEndpoint(endpoint, false); err != nil {
			return nil, fmt.Errorf("%s endpoint: %w", name, err)
		}
	}
	for name, endpoint := range map[string]Endpoint{"contract": contract, "money": money} {
		if err := validateEndpoint(endpoint, true); err != nil {
			return nil, fmt.Errorf("%s endpoint: %w", name, err)
		}
	}
	if strings.TrimSpace(contract.BaseURL) == "" && strings.TrimSpace(money.BaseURL) == "" {
		return nil, fmt.Errorf("at least one source endpoint is required")
	}
	return &HTTPResolver{client: &http.Client{Timeout: timeout}, project: project, people: people, contract: contract, money: money}, nil
}

func validateEndpoint(endpoint Endpoint, optional bool) error {
	if strings.TrimSpace(endpoint.BaseURL) == "" {
		if optional && endpoint.AccessToken == "" && endpoint.Credentials == nil && endpoint.Audience == "" && endpoint.Action == "" {
			return nil
		}
		return fmt.Errorf("base URL is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(endpoint.BaseURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return fmt.Errorf("base URL must be an absolute http or https URL without credentials, query or fragment")
	}
	hasStatic, hasDynamic := strings.TrimSpace(endpoint.AccessToken) != "", endpoint.Credentials != nil
	if hasStatic == hasDynamic {
		return fmt.Errorf("configure exactly one static access token or workload credential source")
	}
	if hasDynamic && (strings.TrimSpace(endpoint.Audience) == "" || strings.TrimSpace(endpoint.Action) == "") {
		return fmt.Errorf("dynamic credential audience and action are required")
	}
	return nil
}

type projectReference struct {
	ID              string   `json:"id"`
	TenantID        string   `json:"tenant_id"`
	ResourceType    string   `json:"resource_type"`
	ProjectNumber   string   `json:"project_number"`
	DisplaySummary  string   `json:"display_summary"`
	Status          string   `json:"status"`
	Revision        int64    `json:"revision"`
	ProjectTypeKeys []string `json:"project_type_keys"`
	NodeType        string   `json:"node_type"`
	ParentProjectID string   `json:"parent_project_id,omitempty"`
	ResourceLinks   []struct {
		TargetService  string `json:"target_service"`
		ResourceType   string `json:"resource_type"`
		ResourceID     string `json:"resource_id"`
		SourceRevision *int64 `json:"source_revision"`
		RelationKey    string `json:"relation_key"`
	} `json:"resource_links"`
}

func (r *HTTPResolver) ResolveProject(ctx context.Context, tenantID string, reference bonus.ProjectRef) (bonus.ProjectReference, error) {
	value, body, err := r.projectReference(ctx, tenantID, reference)
	if err != nil {
		return bonus.ProjectReference{}, err
	}
	return bonus.ProjectReference{ProjectSnapshot: bonus.ProjectSnapshot{ID: value.ID, Revision: value.Revision, Code: value.ProjectNumber, DisplayName: value.DisplaySummary, Digest: digest(body)}, TenantID: value.TenantID, State: value.Status}, nil
}

func (r *HTTPResolver) projectReference(ctx context.Context, tenantID string, reference bonus.ProjectRef) (projectReference, []byte, error) {
	body, err := r.get(ctx, r.project, "/api/v1/references/projects/"+url.PathEscape(reference.ID), "project", reference.ID)
	if err != nil {
		return projectReference{}, nil, err
	}
	var value projectReference
	if err = decode(body, &value); err != nil {
		return projectReference{}, nil, bonus.ErrReferenceUnavailable
	}
	if value.ID != reference.ID || value.TenantID != tenantID || value.ResourceType != "project" || value.Revision != reference.Revision || value.Status != "active" || strings.TrimSpace(value.ProjectNumber) == "" || strings.TrimSpace(value.DisplaySummary) == "" {
		return projectReference{}, nil, bonus.ErrReferenceInvalid
	}
	return value, body, nil
}

func (r *HTTPResolver) ResolveBusinessSource(ctx context.Context, tenantID string, project bonus.ProjectSnapshot, reference bonus.BusinessSourceRef) (bonus.SourceSnapshot, error) {
	if strings.TrimSpace(r.contract.BaseURL) == "" {
		return bonus.SourceSnapshot{}, bonus.ErrReferenceUnavailable
	}
	projectValue, _, err := r.projectReference(ctx, tenantID, bonus.ProjectRef{ID: project.ID, Revision: project.Revision})
	if err != nil {
		return bonus.SourceSnapshot{}, err
	}
	if !hasExactLink(projectValue, "contract-ledger", "settlement", reference.SettlementID, reference.Revision) {
		return bonus.SourceSnapshot{}, bonus.ErrReferenceInvalid
	}
	body, err := r.get(ctx, r.contract, "/api/v1/references/settlements/"+url.PathEscape(reference.SettlementID), "settlement", reference.SettlementID)
	if err != nil {
		return bonus.SourceSnapshot{}, err
	}
	var value struct {
		SchemaVersion          string `json:"schema_version"`
		SourceApplication      string `json:"source_application"`
		ID                     string `json:"id"`
		TenantID               string `json:"tenant_id"`
		ResourceType           string `json:"resource_type"`
		Status                 string `json:"status"`
		Currency               string `json:"currency"`
		Revision               int64  `json:"revision"`
		CurrentEffectiveAmount string `json:"current_effective_amount"`
		DisplaySummary         string `json:"display_summary"`
		Category               string `json:"category,omitempty"`
		ContractID             string `json:"contract_id"`
	}
	if err = decode(body, &value); err != nil {
		return bonus.SourceSnapshot{}, bonus.ErrReferenceUnavailable
	}
	amount, amountErr := decimalToMinor(value.CurrentEffectiveAmount)
	if amountErr != nil || value.SchemaVersion != "1" || value.SourceApplication != "contract-ledger" || value.ID != reference.SettlementID || value.TenantID != tenantID || value.ResourceType != "settlement" || value.Revision != reference.Revision || value.Status != "recorded" || amount <= 0 {
		return bonus.SourceSnapshot{}, bonus.ErrReferenceInvalid
	}
	item := bonus.SourceItemSnapshot{Type: bonus.SourceContractSettlement, ID: value.ID, Revision: value.Revision, AmountMinor: amount, Currency: value.Currency, State: "confirmed", Digest: digest(body)}
	return bonus.SourceSnapshot{Type: bonus.SourceContractSettlement, ID: value.ID, Revision: value.Revision, ProjectID: project.ID, Currency: value.Currency, TotalAmountMinor: amount, State: "confirmed", Digest: digest(body), Items: []bonus.SourceItemSnapshot{item}}, nil
}

func (r *HTTPResolver) ResolveProductionSource(ctx context.Context, tenantID string, project bonus.ProjectSnapshot, reference bonus.ProductionSourceRef) (bonus.SourceSnapshot, error) {
	if strings.TrimSpace(r.money.BaseURL) == "" {
		return bonus.SourceSnapshot{}, bonus.ErrReferenceUnavailable
	}
	body, err := r.get(ctx, r.money, "/api/v1/references/receipt-sets/"+url.PathEscape(reference.ReceiptSetID), "receipt_set", reference.ReceiptSetID)
	if err != nil {
		return bonus.SourceSnapshot{}, err
	}
	var value struct {
		SchemaVersion     string `json:"schema_version"`
		SourceApplication string `json:"source_application"`
		ID                string `json:"id"`
		TenantID          string `json:"tenant_id"`
		ResourceType      string `json:"resource_type"`
		ProjectID         string `json:"project_id"`
		Status            string `json:"status"`
		Currency          string `json:"currency"`
		Revision          int64  `json:"revision"`
		TotalAmountMinor  int64  `json:"total_amount_minor"`
		Items             []struct {
			TransactionID       string `json:"transaction_id"`
			TransactionRevision int64  `json:"transaction_revision"`
			AllocationID        string `json:"allocation_id"`
			AllocationRevision  int64  `json:"allocation_revision"`
			AmountMinor         int64  `json:"amount_minor"`
			Currency            string `json:"currency"`
			Status              string `json:"status"`
		}
	}
	if err = decode(body, &value); err != nil {
		return bonus.SourceSnapshot{}, bonus.ErrReferenceUnavailable
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "money-flow-ledger" || value.ID != reference.ReceiptSetID || value.TenantID != tenantID || value.ResourceType != "receipt-set" || value.ProjectID != project.ID || value.Revision != reference.Revision || value.Status != "confirmed" || value.Currency != "CNY" || value.TotalAmountMinor <= 0 || len(value.Items) == 0 {
		return bonus.SourceSnapshot{}, bonus.ErrReferenceInvalid
	}
	items := make([]bonus.SourceItemSnapshot, 0, len(value.Items))
	var itemTotal int64
	for _, item := range value.Items {
		if item.TransactionRevision != 1 || item.AllocationRevision != 1 || item.AmountMinor <= 0 || item.Currency != value.Currency || item.Status != "confirmed" || itemTotal > math.MaxInt64-item.AmountMinor {
			return bonus.SourceSnapshot{}, bonus.ErrReferenceInvalid
		}
		itemTotal += item.AmountMinor
		itemBytes, marshalErr := json.Marshal(item)
		if marshalErr != nil {
			return bonus.SourceSnapshot{}, bonus.ErrReferenceUnavailable
		}
		items = append(items, bonus.SourceItemSnapshot{Type: bonus.SourceMoneyReceiptItem, ID: item.AllocationID, Revision: item.AllocationRevision, TransactionID: item.TransactionID, TransactionRevision: item.TransactionRevision, AmountMinor: item.AmountMinor, Currency: item.Currency, State: item.Status, Digest: digest(itemBytes)})
	}
	if itemTotal != value.TotalAmountMinor {
		return bonus.SourceSnapshot{}, bonus.ErrReferenceInvalid
	}
	return bonus.SourceSnapshot{Type: bonus.SourceMoneyReceiptSet, ID: value.ID, Revision: value.Revision, ProjectID: value.ProjectID, Currency: value.Currency, TotalAmountMinor: value.TotalAmountMinor, State: value.Status, Digest: digest(body), Items: items}, nil
}

func (r *HTTPResolver) ResolveParticipant(ctx context.Context, tenantID string, project bonus.ProjectSnapshot, bonusType string, personRef bonus.PersonRef, assignmentRef bonus.AssignmentRef) (bonus.ParticipantReference, error) {
	return r.resolveParticipant(ctx, tenantID, project, bonusType, personRef, assignmentRef, eligibleRole)
}

func (r *HTTPResolver) ResolveManualParticipant(ctx context.Context, tenantID string, project bonus.ProjectSnapshot, bonusType string, personRef bonus.PersonRef, assignmentRef bonus.AssignmentRef) (bonus.ParticipantReference, error) {
	return r.resolveParticipant(ctx, tenantID, project, bonusType, personRef, assignmentRef, referencebonus.EligibleRole)
}

func (r *HTTPResolver) resolveParticipant(ctx context.Context, tenantID string, project bonus.ProjectSnapshot, bonusType string, personRef bonus.PersonRef, assignmentRef bonus.AssignmentRef, eligible func(string, string) bool) (bonus.ParticipantReference, error) {
	assignmentBody, err := r.get(ctx, r.project, "/api/v1/references/projects/"+url.PathEscape(project.ID)+"/assignments/"+url.PathEscape(assignmentRef.ID), "project", project.ID)
	if err != nil {
		return bonus.ParticipantReference{}, err
	}
	var assignment struct {
		ID             string  `json:"id"`
		TenantID       string  `json:"tenant_id"`
		ProjectID      string  `json:"project_id"`
		ResourceType   string  `json:"resource_type"`
		SubjectKind    string  `json:"subject_kind"`
		SubjectID      string  `json:"subject_id"`
		RoleKey        string  `json:"role_key"`
		Status         string  `json:"status"`
		DisplayName    string  `json:"display_name"`
		SourceRevision int64   `json:"source_revision"`
		Revision       int64   `json:"revision"`
		EffectiveFrom  string  `json:"effective_from"`
		EndedAt        *string `json:"ended_at,omitempty"`
	}
	if err = decode(assignmentBody, &assignment); err != nil {
		return bonus.ParticipantReference{}, bonus.ErrReferenceUnavailable
	}
	if assignment.ID != assignmentRef.ID || assignment.TenantID != tenantID || assignment.ProjectID != project.ID || assignment.ResourceType != "project-assignment" || assignment.SubjectKind != "person" || assignment.SubjectID != personRef.ID || assignment.SourceRevision != personRef.Revision || assignment.Revision != assignmentRef.Revision || assignment.Status != "active" || !eligible(bonusType, assignment.RoleKey) {
		return bonus.ParticipantReference{}, bonus.ErrReferenceInvalid
	}
	personBody, err := r.get(ctx, r.people, "/api/v1/references/persons/"+url.PathEscape(personRef.ID), "person", personRef.ID)
	if err != nil {
		return bonus.ParticipantReference{}, err
	}
	var person struct {
		SchemaVersion     string `json:"schema_version"`
		SourceApplication string `json:"source_application"`
		ID                string `json:"id"`
		TenantID          string `json:"tenant_id"`
		ResourceType      string `json:"resource_type"`
		DisplayName       string `json:"display_name"`
		Status            string `json:"status"`
		Revision          int64  `json:"revision"`
	}
	if err = decode(personBody, &person); err != nil {
		return bonus.ParticipantReference{}, bonus.ErrReferenceUnavailable
	}
	if person.SchemaVersion != "1" || person.SourceApplication != "people-groups" || person.ID != personRef.ID || person.TenantID != tenantID || person.ResourceType != "person" || person.Revision != personRef.Revision || person.Status != "active" || strings.TrimSpace(person.DisplayName) == "" {
		return bonus.ParticipantReference{}, bonus.ErrReferenceInvalid
	}
	return bonus.ParticipantReference{
		TenantID: tenantID, BonusType: bonusType, Active: true, Eligible: true,
		Person:     bonus.PersonSnapshot{ID: person.ID, Revision: person.Revision, DisplayName: person.DisplayName, Digest: digest(personBody)},
		Assignment: bonus.AssignmentSnapshot{ID: assignment.ID, Revision: assignment.Revision, ProjectID: assignment.ProjectID, PersonID: assignment.SubjectID, RoleKey: assignment.RoleKey, Digest: digest(assignmentBody)},
	}, nil
}

// Candidate discovery is a bounded read of Project's versioned public contract;
// every returned option is then independently verified against Person/Assignment.
func (r *HTTPResolver) ListManualCandidates(ctx context.Context, tenant string, ref bonus.ProjectRef, scope string) ([]bonus.ParticipantReference, error) {
	p, err := r.ResolveProject(ctx, tenant, ref)
	if err != nil {
		return nil, err
	}
	body, err := r.get(ctx, r.project, "/api/v1/projects/"+url.PathEscape(ref.ID), "project", ref.ID)
	if err != nil {
		return nil, err
	}
	var details struct {
		Project struct {
			ID       string `json:"id"`
			Tenant   string `json:"tenant_id"`
			Revision int64  `json:"revision"`
			Status   string `json:"status"`
		} `json:"project"`
		Assignments []struct {
			ID             string  `json:"id"`
			Tenant         string  `json:"tenant_id"`
			Project        string  `json:"project_id"`
			SubjectKind    string  `json:"subject_kind"`
			Subject        string  `json:"subject_id"`
			SourceRevision int64   `json:"source_revision"`
			Role           string  `json:"role_key"`
			Revision       int64   `json:"created_project_revision"`
			EndedAt        *string `json:"ended_at"`
		} `json:"assignments"`
	}
	if decode(body, &details) != nil || details.Project.ID != ref.ID || details.Project.Tenant != tenant || details.Project.Revision != ref.Revision || details.Project.Status != "active" || details.Assignments == nil || len(details.Assignments) > 1000 {
		return nil, bonus.ErrReferenceInvalid
	}
	out := []bonus.ParticipantReference{}
	for _, a := range details.Assignments {
		if a.EndedAt != nil || a.SubjectKind != "person" || !referencebonus.EligibleRole(scope, a.Role) {
			continue
		}
		if a.Tenant != tenant || a.Project != ref.ID {
			return nil, bonus.ErrReferenceInvalid
		}
		v, e := r.ResolveManualParticipant(ctx, tenant, p.ProjectSnapshot, scope, bonus.PersonRef{ID: a.Subject, Revision: a.SourceRevision}, bonus.AssignmentRef{ID: a.ID, Revision: a.Revision})
		if e != nil {
			return nil, e
		}
		out = append(out, v)
		if len(out) > 100 {
			return nil, bonus.ErrReferenceInvalid
		}
	}
	return out, nil
}

func eligibleRole(bonusType, role string) bool {
	return (bonusType == bonus.TypeBusiness && (role == "business_owner" || role == "participant")) || (bonusType == bonus.TypeProduction && (role == "production_owner" || role == "participant"))
}

func hasExactLink(project projectReference, service, resourceType, resourceID string, revision int64) bool {
	for _, link := range project.ResourceLinks {
		if link.TargetService == service && link.ResourceType == resourceType && link.ResourceID == resourceID && link.SourceRevision != nil && *link.SourceRevision == revision {
			return true
		}
	}
	return false
}

func (r *HTTPResolver) get(ctx context.Context, endpoint Endpoint, path, resourceType, resourceID string) ([]byte, error) {
	token := endpoint.AccessToken
	if endpoint.Credentials != nil {
		var err error
		token, err = endpoint.Credentials.Credential(ctx, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if err != nil || strings.TrimSpace(token) == "" {
			return nil, bonus.ErrReferenceUnavailable
		}
	}
	return r.getWithToken(ctx, endpoint, path, resourceType, resourceID, token, true)
}

func (r *HTTPResolver) getWithToken(ctx context.Context, endpoint Endpoint, path, resourceType, resourceID, token string, retry bool) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, bonus.ErrReferenceUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := r.client.Do(request)
	if err != nil {
		return nil, bonus.ErrReferenceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && endpoint.Credentials != nil && retry {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		endpoint.Credentials.Invalidate(endpoint.Audience, endpoint.Action, resourceType, resourceID)
		fresh, credentialErr := endpoint.Credentials.Credential(ctx, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if credentialErr != nil || strings.TrimSpace(fresh) == "" {
			return nil, bonus.ErrReferenceUnavailable
		}
		return r.getWithToken(ctx, endpoint, path, resourceType, resourceID, fresh, false)
	}
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUnprocessableEntity {
		return nil, bonus.ErrReferenceInvalid
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, bonus.ErrReferenceUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return nil, bonus.ErrReferenceUnavailable
	}
	return body, nil
}

func decode(body []byte, destination any) error {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	// Reference contracts validate their known identity, version, state, and
	// monetary invariants below. They must still tolerate additive fields from
	// the same schema version so one producer can evolve without forcing every
	// consumer to deploy in lockstep.
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("response contains trailing JSON")
	}
	return nil
}

func decimalToMinor(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "-") || strings.ContainsAny(raw, "eE+") {
		return 0, fmt.Errorf("invalid decimal amount")
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid decimal amount")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
		if len(fraction) > 2 {
			return 0, fmt.Errorf("amount has sub-cent precision")
		}
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	major, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || major > math.MaxInt64/100 {
		return 0, fmt.Errorf("amount overflows int64")
	}
	minor := int64(0)
	if fraction != "" {
		minor, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid decimal amount")
		}
	}
	if major > (math.MaxInt64-minor)/100 {
		return 0, fmt.Errorf("amount overflows int64")
	}
	return major*100 + minor, nil
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}
