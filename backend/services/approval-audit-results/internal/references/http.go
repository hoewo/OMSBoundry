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

	"omsbizfoundry/approval-audit-results/internal/results"
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
	contract Endpoint
	evidence Endpoint
}

func NewHTTPResolver(timeout time.Duration, project, contract, evidence Endpoint) (*HTTPResolver, error) {
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("reference timeout must be between 1ns and 30s")
	}
	for name, endpoint := range map[string]Endpoint{"project": project, "contract": contract, "evidence": evidence} {
		if err := validateEndpoint(endpoint); err != nil {
			return nil, fmt.Errorf("%s endpoint: %w", name, err)
		}
	}
	return &HTTPResolver{client: &http.Client{Timeout: timeout}, project: project, contract: contract, evidence: evidence}, nil
}

func validateEndpoint(endpoint Endpoint) error {
	parsed, err := url.Parse(strings.TrimSpace(endpoint.BaseURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("base URL must be an absolute http or https URL without query or fragment")
	}
	hasStatic := strings.TrimSpace(endpoint.AccessToken) != ""
	hasDynamic := endpoint.Credentials != nil
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
		RelationKey    string `json:"relation_key"`
	} `json:"resource_links"`
}

func (r *HTTPResolver) ResolveProject(ctx context.Context, tenantID string, reference results.ProjectRef) (results.ProjectReference, error) {
	value, err := r.projectReference(ctx, tenantID, reference)
	if err != nil {
		return results.ProjectReference{}, err
	}
	return results.ProjectReference{
		ProjectSnapshot: results.ProjectSnapshot{ID: value.ID, Revision: value.Revision, Code: value.ProjectNumber, DisplayName: value.DisplaySummary},
		TenantID:        value.TenantID, Status: value.Status,
	}, nil
}

func (r *HTTPResolver) projectReference(ctx context.Context, tenantID string, reference results.ProjectRef) (projectReference, error) {
	body, err := r.get(ctx, tenantID, r.project, "/api/v1/references/projects/"+url.PathEscape(reference.ID), "project", reference.ID)
	if err != nil {
		return projectReference{}, err
	}
	var value projectReference
	if err := decode(body, &value); err != nil {
		return projectReference{}, results.ErrReferenceUnavailable
	}
	if value.ID != reference.ID || value.TenantID != tenantID || value.ResourceType != "project" || (reference.Revision > 0 && value.Revision != reference.Revision) || value.Status != "active" || strings.TrimSpace(value.ProjectNumber) == "" || strings.TrimSpace(value.DisplaySummary) == "" {
		return projectReference{}, results.ErrReferenceInvalid
	}
	return value, nil
}

func (r *HTTPResolver) ResolveSource(ctx context.Context, tenantID string, projectRef results.ProjectRef, reference results.SourceRef) (results.SourceReference, error) {
	project, err := r.projectReference(ctx, tenantID, projectRef)
	if err != nil {
		return results.SourceReference{}, err
	}
	if !hasExactLink(project, reference) {
		return results.SourceReference{}, results.ErrReferenceInvalid
	}
	path := "/api/v1/references/" + reference.ResourceType + "s/" + url.PathEscape(reference.ResourceID)
	body, err := r.get(ctx, tenantID, r.contract, path, reference.ResourceType, reference.ResourceID)
	if err != nil {
		return results.SourceReference{}, err
	}
	var value struct {
		SchemaVersion          string  `json:"schema_version"`
		SourceApplication      string  `json:"source_application"`
		ID                     string  `json:"id"`
		TenantID               string  `json:"tenant_id"`
		ResourceType           string  `json:"resource_type"`
		DisplaySummary         string  `json:"display_summary"`
		Status                 string  `json:"status"`
		Revision               int64   `json:"revision"`
		Category               string  `json:"category"`
		Currency               string  `json:"currency"`
		CurrentEffectiveAmount *string `json:"current_effective_amount"`
	}
	if err := decode(body, &value); err != nil {
		return results.SourceReference{}, results.ErrReferenceUnavailable
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "contract-ledger" || value.ID != reference.ResourceID || value.TenantID != tenantID || value.ResourceType != reference.ResourceType || value.Revision != reference.Revision || !eligibleState(value.ResourceType, value.Status) || strings.TrimSpace(value.DisplaySummary) == "" || value.Currency != "CNY" || value.CurrentEffectiveAmount == nil {
		return results.SourceReference{}, results.ErrReferenceInvalid
	}
	minor, err := decimalToMinor(*value.CurrentEffectiveAmount)
	if err != nil {
		return results.SourceReference{}, results.ErrReferenceInvalid
	}
	components, ok := componentsForCategory(value.Category, minor)
	if !ok {
		return results.SourceReference{}, results.ErrReferenceInvalid
	}
	digest := sha256.Sum256(body)
	return results.SourceReference{
		SourceSnapshot: results.SourceSnapshot{
			SourceRef: reference, State: value.Status, ProjectID: projectRef.ID, Currency: value.Currency,
			AmountProfile: results.AmountProfileReferenceOAV1, AmountComponents: components, TotalAmountMinor: minor,
			Digest: "sha256:" + hex.EncodeToString(digest[:]), DisplaySummary: value.DisplaySummary,
		},
		TenantID: tenantID, Eligible: true,
	}, nil
}

func hasExactLink(project projectReference, reference results.SourceRef) bool {
	for _, link := range project.ResourceLinks {
		if link.TargetService == "contract-ledger" && link.ResourceType == reference.ResourceType && link.ResourceID == reference.ResourceID && link.SourceRevision != nil && *link.SourceRevision == reference.Revision {
			return true
		}
	}
	return false
}

func eligibleState(resourceType, state string) bool {
	if resourceType == "settlement" {
		return state == "recorded"
	}
	return state == "active" || state == "terminated" || state == "closed"
}

func componentsForCategory(category string, amount int64) ([]results.AmountComponent, bool) {
	category = strings.ToLower(strings.TrimSpace(category))
	kind := ""
	switch category {
	case "design", results.ComponentDesignFee:
		kind = results.ComponentDesignFee
	case "survey", results.ComponentSurveyFee:
		kind = results.ComponentSurveyFee
	case "consultation", results.ComponentConsultationFee:
		kind = results.ComponentConsultationFee
	default:
		return nil, false
	}
	values := []results.AmountComponent{
		{Kind: results.ComponentDesignFee},
		{Kind: results.ComponentSurveyFee},
		{Kind: results.ComponentConsultationFee},
	}
	for index := range values {
		if values[index].Kind == kind {
			values[index].AmountMinor = amount
		}
	}
	return values, true
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

func (r *HTTPResolver) ResolveEvidence(ctx context.Context, tenantID string, reference results.EvidenceRef) (results.EvidenceReference, error) {
	path := fmt.Sprintf("/v1/references/files/%s/versions/%d", url.PathEscape(reference.FileID), reference.Version)
	body, err := r.get(ctx, tenantID, r.evidence, path, "file", reference.FileID)
	if err != nil {
		return results.EvidenceReference{}, err
	}
	var value struct {
		ID           string `json:"id"`
		FileID       string `json:"file_id"`
		TenantID     string `json:"tenant_id"`
		ResourceType string `json:"resource_type"`
		Status       string `json:"status"`
		Version      int64  `json:"version"`
		Revision     int64  `json:"revision"`
		DisplayName  string `json:"display_name"`
		SHA256       string `json:"sha256"`
	}
	if err := decode(body, &value); err != nil {
		return results.EvidenceReference{}, results.ErrReferenceUnavailable
	}
	if value.ID != reference.FileID || value.FileID != reference.FileID || value.TenantID != tenantID || value.ResourceType != "evidence-file" || value.Status != "available" || value.Version != reference.Version || value.Revision != reference.Version || strings.TrimSpace(value.DisplayName) == "" || len(value.SHA256) != 64 {
		return results.EvidenceReference{}, results.ErrReferenceInvalid
	}
	return results.EvidenceReference{
		EvidenceSnapshot: results.EvidenceSnapshot{EvidenceRef: reference, Digest: "sha256:" + strings.ToLower(value.SHA256), DisplayName: value.DisplayName},
		TenantID:         value.TenantID, Status: value.Status,
	}, nil
}

func (r *HTTPResolver) get(ctx context.Context, tenantID string, endpoint Endpoint, path, resourceType, resourceID string) ([]byte, error) {
	token := endpoint.AccessToken
	if endpoint.Credentials != nil {
		var err error
		token, err = workloadidentity.CredentialForTenant(ctx, endpoint.Credentials, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if err != nil || strings.TrimSpace(token) == "" {
			return nil, results.ErrReferenceUnavailable
		}
	}
	return r.getWithToken(ctx, tenantID, endpoint, path, resourceType, resourceID, token, true)
}

func (r *HTTPResolver) getWithToken(ctx context.Context, tenantID string, endpoint Endpoint, path, resourceType, resourceID, token string, retry bool) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, results.ErrReferenceUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := r.client.Do(request)
	if err != nil {
		return nil, results.ErrReferenceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && endpoint.Credentials != nil && retry {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		workloadidentity.InvalidateForTenant(endpoint.Credentials, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		fresh, credentialErr := workloadidentity.CredentialForTenant(ctx, endpoint.Credentials, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if credentialErr != nil || strings.TrimSpace(fresh) == "" {
			return nil, results.ErrReferenceUnavailable
		}
		return r.getWithToken(ctx, tenantID, endpoint, path, resourceType, resourceID, fresh, false)
	}
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUnprocessableEntity {
		return nil, results.ErrReferenceInvalid
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, results.ErrReferenceUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return nil, results.ErrReferenceUnavailable
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
