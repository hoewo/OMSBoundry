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

	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
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
	client   *http.Client
	project  Endpoint
	party    Endpoint
	contract Endpoint
	evidence Endpoint
}

func NewHTTPResolver(timeout time.Duration, project, party, contract, evidence Endpoint) (*HTTPResolver, error) {
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("reference timeout must be between 1ns and 30s")
	}
	for name, endpoint := range map[string]Endpoint{"project": project, "party": party, "contract": contract, "evidence": evidence} {
		if err := validateEndpoint(endpoint); err != nil {
			return nil, fmt.Errorf("%s endpoint: %w", name, err)
		}
	}
	return &HTTPResolver{client: &http.Client{Timeout: timeout}, project: project, party: party, contract: contract, evidence: evidence}, nil
}

func validateEndpoint(endpoint Endpoint) error {
	parsed, err := url.Parse(strings.TrimSpace(endpoint.BaseURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("base URL must be an absolute http or https URL without query or fragment")
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
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id"`
	ResourceType   string `json:"resource_type"`
	ProjectNumber  string `json:"project_number"`
	DisplaySummary string `json:"display_summary"`
	Status         string `json:"status"`
	Revision       int64  `json:"revision"`
	ResourceLinks  []struct {
		TargetService  string `json:"target_service"`
		ResourceType   string `json:"resource_type"`
		ResourceID     string `json:"resource_id"`
		SourceRevision *int64 `json:"source_revision"`
	} `json:"resource_links"`
}

func (r *HTTPResolver) ResolveProject(ctx context.Context, tenantID string, reference costoutsourcing.ProjectRef) (costoutsourcing.ProjectReference, error) {
	value, err := r.projectReference(ctx, tenantID, reference)
	if err != nil {
		return costoutsourcing.ProjectReference{}, err
	}
	return costoutsourcing.ProjectReference{ProjectSnapshot: costoutsourcing.ProjectSnapshot{ID: value.ID, Revision: value.Revision, Code: value.ProjectNumber, DisplayName: value.DisplaySummary}, TenantID: value.TenantID, Status: value.Status}, nil
}

func (r *HTTPResolver) projectReference(ctx context.Context, tenantID string, reference costoutsourcing.ProjectRef) (projectReference, error) {
	body, err := r.get(ctx, tenantID, r.project, "/api/v1/references/projects/"+url.PathEscape(reference.ID), "project", reference.ID)
	if err != nil {
		return projectReference{}, err
	}
	var value projectReference
	if err := decode(body, &value); err != nil {
		return projectReference{}, costoutsourcing.ErrReferenceUnavailable
	}
	if value.ID != reference.ID || value.TenantID != tenantID || value.ResourceType != "project" || (reference.Revision > 0 && value.Revision != reference.Revision) || value.Revision <= 0 || value.Status != "active" || strings.TrimSpace(value.ProjectNumber) == "" || strings.TrimSpace(value.DisplaySummary) == "" {
		return projectReference{}, costoutsourcing.ErrReferenceInvalid
	}
	return value, nil
}

func (r *HTTPResolver) ResolveParty(ctx context.Context, tenantID string, reference costoutsourcing.PartyRef) (costoutsourcing.PartyReference, error) {
	body, err := r.get(ctx, tenantID, r.party, "/api/v1/references/parties/"+url.PathEscape(reference.ID), "party", reference.ID)
	if err != nil {
		return costoutsourcing.PartyReference{}, err
	}
	var value struct {
		SchemaVersion     string `json:"schema_version"`
		SourceApplication string `json:"source_application"`
		ID                string `json:"id"`
		TenantID          string `json:"tenant_id"`
		Kind              string `json:"kind"`
		LegalName         string `json:"legal_name"`
		Status            string `json:"status"`
		Revision          int64  `json:"revision"`
	}
	if err := decode(body, &value); err != nil {
		return costoutsourcing.PartyReference{}, costoutsourcing.ErrReferenceUnavailable
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "party-directory" || value.ID != reference.ID || value.TenantID != tenantID || value.Revision != reference.Revision || value.Status != "active" || (value.Kind != "organization" && value.Kind != "individual") || strings.TrimSpace(value.LegalName) == "" {
		return costoutsourcing.PartyReference{}, costoutsourcing.ErrReferenceInvalid
	}
	return costoutsourcing.PartyReference{PartySnapshot: costoutsourcing.PartySnapshot{ID: value.ID, Revision: value.Revision, Kind: value.Kind, LegalName: value.LegalName}, TenantID: value.TenantID, Status: value.Status}, nil
}

func (r *HTTPResolver) ResolveContract(ctx context.Context, tenantID string, projectRef costoutsourcing.ProjectRef, partyRef costoutsourcing.PartyRef, reference costoutsourcing.ContractRef) (costoutsourcing.ContractReference, error) {
	project, err := r.projectReference(ctx, tenantID, projectRef)
	if err != nil {
		return costoutsourcing.ContractReference{}, err
	}
	if !hasExactProjectLink(project, reference) {
		return costoutsourcing.ContractReference{}, costoutsourcing.ErrReferenceInvalid
	}
	plural := reference.ResourceType + "s"
	if reference.Application != "contract-ledger" || (reference.ResourceType != "contract" && reference.ResourceType != "settlement") {
		return costoutsourcing.ContractReference{}, costoutsourcing.ErrReferenceInvalid
	}
	body, err := r.get(ctx, tenantID, r.contract, "/api/v1/references/"+plural+"/"+url.PathEscape(reference.ResourceID), reference.ResourceType, reference.ResourceID)
	if err != nil {
		return costoutsourcing.ContractReference{}, err
	}
	var value struct {
		SchemaVersion          string   `json:"schema_version"`
		SourceApplication      string   `json:"source_application"`
		ID                     string   `json:"id"`
		TenantID               string   `json:"tenant_id"`
		ResourceType           string   `json:"resource_type"`
		DisplaySummary         string   `json:"display_summary"`
		Status                 string   `json:"status"`
		Currency               string   `json:"currency"`
		Revision               int64    `json:"revision"`
		CurrentEffectiveAmount *string  `json:"current_effective_amount"`
		PartyIDs               []string `json:"party_ids"`
	}
	if err := decode(body, &value); err != nil {
		return costoutsourcing.ContractReference{}, costoutsourcing.ErrReferenceUnavailable
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "contract-ledger" || value.ID != reference.ResourceID || value.TenantID != tenantID || value.ResourceType != reference.ResourceType || value.Revision != reference.Revision || !eligibleContractState(value.ResourceType, value.Status) || strings.TrimSpace(value.DisplaySummary) == "" || value.CurrentEffectiveAmount == nil || !contains(value.PartyIDs, partyRef.ID) {
		return costoutsourcing.ContractReference{}, costoutsourcing.ErrReferenceInvalid
	}
	minor, err := decimalToMinor(*value.CurrentEffectiveAmount)
	if err != nil {
		return costoutsourcing.ContractReference{}, costoutsourcing.ErrReferenceInvalid
	}
	digest := sha256.Sum256(body)
	return costoutsourcing.ContractReference{ContractSnapshot: costoutsourcing.ContractSnapshot{ContractRef: reference, ProjectID: projectRef.ID, PartyID: partyRef.ID, State: value.Status, Currency: value.Currency, AmountMinor: minor, Digest: "sha256:" + hex.EncodeToString(digest[:]), DisplaySummary: value.DisplaySummary}, TenantID: value.TenantID, Eligible: true}, nil
}

func hasExactProjectLink(project projectReference, reference costoutsourcing.ContractRef) bool {
	for _, link := range project.ResourceLinks {
		if link.TargetService == reference.Application && link.ResourceType == reference.ResourceType && link.ResourceID == reference.ResourceID && link.SourceRevision != nil && *link.SourceRevision == reference.Revision {
			return true
		}
	}
	return false
}

func eligibleContractState(resourceType, state string) bool {
	if resourceType == "settlement" {
		return state == "recorded"
	}
	return state == "active" || state == "terminated" || state == "closed"
}

func (r *HTTPResolver) ResolveEvidence(ctx context.Context, tenantID string, reference costoutsourcing.EvidenceRef) (costoutsourcing.EvidenceReference, error) {
	path := fmt.Sprintf("/v1/references/files/%s/versions/%d", url.PathEscape(reference.FileID), reference.Version)
	body, err := r.get(ctx, tenantID, r.evidence, path, "file", reference.FileID)
	if err != nil {
		return costoutsourcing.EvidenceReference{}, err
	}
	var value struct {
		ID           string `json:"id"`
		FileID       string `json:"file_id"`
		TenantID     string `json:"tenant_id"`
		ResourceType string `json:"resource_type"`
		Status       string `json:"status"`
		DisplayName  string `json:"display_name"`
		SHA256       string `json:"sha256"`
		Version      int64  `json:"version"`
		Revision     int64  `json:"revision"`
	}
	if err := decode(body, &value); err != nil {
		return costoutsourcing.EvidenceReference{}, costoutsourcing.ErrReferenceUnavailable
	}
	if value.ID != reference.FileID || value.FileID != reference.FileID || value.TenantID != tenantID || value.ResourceType != "evidence-file" || value.Status != "available" || value.Version != reference.Version || value.Revision != reference.Version || strings.TrimSpace(value.DisplayName) == "" || len(value.SHA256) != 64 {
		return costoutsourcing.EvidenceReference{}, costoutsourcing.ErrReferenceInvalid
	}
	return costoutsourcing.EvidenceReference{EvidenceSnapshot: costoutsourcing.EvidenceSnapshot{EvidenceRef: reference, Digest: "sha256:" + strings.ToLower(value.SHA256), DisplayName: value.DisplayName}, TenantID: value.TenantID, Status: value.Status}, nil
}

func (r *HTTPResolver) get(ctx context.Context, tenantID string, endpoint Endpoint, path, resourceType, resourceID string) ([]byte, error) {
	token := endpoint.AccessToken
	if endpoint.Credentials != nil {
		var err error
		token, err = workloadidentity.CredentialForTenant(ctx, endpoint.Credentials, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if err != nil || strings.TrimSpace(token) == "" {
			return nil, costoutsourcing.ErrReferenceUnavailable
		}
	}
	return r.getWithToken(ctx, tenantID, endpoint, path, resourceType, resourceID, token, true)
}

func (r *HTTPResolver) getWithToken(ctx context.Context, tenantID string, endpoint Endpoint, path, resourceType, resourceID, token string, retry bool) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, costoutsourcing.ErrReferenceUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := r.client.Do(request)
	if err != nil {
		return nil, costoutsourcing.ErrReferenceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && endpoint.Credentials != nil && retry {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		workloadidentity.InvalidateForTenant(endpoint.Credentials, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		fresh, credentialErr := workloadidentity.CredentialForTenant(ctx, endpoint.Credentials, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if credentialErr != nil || strings.TrimSpace(fresh) == "" {
			return nil, costoutsourcing.ErrReferenceUnavailable
		}
		return r.getWithToken(ctx, tenantID, endpoint, path, resourceType, resourceID, fresh, false)
	}
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUnprocessableEntity {
		return nil, costoutsourcing.ErrReferenceInvalid
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, costoutsourcing.ErrReferenceUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return nil, costoutsourcing.ErrReferenceUnavailable
	}
	return body, nil
}

func decode(body []byte, destination any) error {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
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

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
