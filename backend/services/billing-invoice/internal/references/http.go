package references

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"omsbizfoundry/billing-invoice/internal/invoice"
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
	party    Endpoint
	project  Endpoint
	contract Endpoint
	money    Endpoint
	evidence Endpoint
}

func NewHTTPResolver(timeout time.Duration, party, project, contract, money, evidence Endpoint) (*HTTPResolver, error) {
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("reference timeout must be between 1ns and 30s")
	}
	for name, endpoint := range map[string]Endpoint{
		"party": party, "project": project, "contract": contract, "money": money, "evidence": evidence,
	} {
		if err := validateEndpoint(endpoint); err != nil {
			return nil, fmt.Errorf("%s endpoint: %w", name, err)
		}
	}
	return &HTTPResolver{
		client: &http.Client{Timeout: timeout}, party: party, project: project,
		contract: contract, money: money, evidence: evidence,
	}, nil
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

func (r *HTTPResolver) ResolveParty(ctx context.Context, tenantID string, reference invoice.PartyRef) (invoice.PartyReference, error) {
	body, err := r.get(ctx, tenantID, r.party, "/api/v1/references/parties/"+url.PathEscape(reference.ID), "party", reference.ID)
	if err != nil {
		return invoice.PartyReference{}, err
	}
	var value struct {
		SchemaVersion          string  `json:"schema_version"`
		SourceApplication      string  `json:"source_application"`
		ID                     string  `json:"id"`
		TenantID               string  `json:"tenant_id"`
		Kind                   string  `json:"kind"`
		LegalName              string  `json:"legal_name"`
		RegistrationIdentifier *string `json:"registration_identifier"`
		Status                 string  `json:"status"`
		Revision               int64   `json:"revision"`
	}
	if err := decode(body, &value); err != nil {
		return invoice.PartyReference{}, invoice.ErrReferenceUnavailable
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "party-directory" || value.ID != reference.ID ||
		value.TenantID != tenantID || value.Revision != reference.Revision || value.Status != "active" ||
		(value.Kind != "organization" && value.Kind != "individual") || strings.TrimSpace(value.LegalName) == "" {
		return invoice.PartyReference{}, invoice.ErrReferenceInvalid
	}
	return invoice.PartyReference{
		PartySnapshot: invoice.PartySnapshot{
			ID: value.ID, Revision: value.Revision, Kind: value.Kind, LegalName: value.LegalName,
			RegistrationIdentifier: value.RegistrationIdentifier,
		},
		TenantID: value.TenantID, Status: value.Status,
	}, nil
}

func (r *HTTPResolver) ResolveSource(ctx context.Context, tenantID string, reference invoice.SourceRef) (invoice.SourceReference, error) {
	if reference.ResourceID == nil || reference.SourceRevision == nil {
		return invoice.SourceReference{}, invoice.ErrReferenceInvalid
	}
	endpoint, path, application, resourceType, ok := r.sourceTarget(reference.Kind, *reference.ResourceID)
	if !ok {
		return invoice.SourceReference{}, invoice.ErrReferenceInvalid
	}
	body, err := r.get(ctx, tenantID, endpoint, path, resourceType, *reference.ResourceID)
	if err != nil {
		return invoice.SourceReference{}, err
	}
	var value struct {
		ID             string `json:"id"`
		TenantID       string `json:"tenant_id"`
		ResourceType   string `json:"resource_type"`
		DisplaySummary string `json:"display_summary"`
		Status         string `json:"status"`
		Revision       int64  `json:"revision"`
	}
	if err := decode(body, &value); err != nil {
		return invoice.SourceReference{}, invoice.ErrReferenceUnavailable
	}
	if value.ID != *reference.ResourceID || value.TenantID != tenantID || value.ResourceType != resourceType ||
		value.Revision != *reference.SourceRevision || strings.TrimSpace(value.DisplaySummary) == "" || !sourceStatusAvailable(reference.Kind, value.Status) {
		return invoice.SourceReference{}, invoice.ErrReferenceInvalid
	}
	return invoice.SourceReference{
		SourceApplication: application, TenantID: value.TenantID, ResourceID: value.ID,
		Revision: value.Revision, Status: value.Status, DisplaySummary: value.DisplaySummary,
	}, nil
}

func (r *HTTPResolver) ResolveEvidence(ctx context.Context, tenantID string, reference invoice.EvidenceRef) (invoice.EvidenceReference, error) {
	path := fmt.Sprintf("/v1/references/files/%s/versions/%d", url.PathEscape(reference.FileID), reference.Version)
	body, err := r.get(ctx, tenantID, r.evidence, path, "file", reference.FileID)
	if err != nil {
		return invoice.EvidenceReference{}, err
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
	}
	if err := decode(body, &value); err != nil {
		return invoice.EvidenceReference{}, invoice.ErrReferenceUnavailable
	}
	if value.ID != reference.FileID || value.FileID != reference.FileID || value.TenantID != tenantID ||
		value.ResourceType != "evidence-file" || value.Status != "available" || value.Version != reference.Version ||
		value.Revision != reference.Version || strings.TrimSpace(value.DisplayName) == "" {
		return invoice.EvidenceReference{}, invoice.ErrReferenceInvalid
	}
	return invoice.EvidenceReference{
		TenantID: value.TenantID, FileID: value.FileID, Version: value.Version, Revision: value.Revision, Status: value.Status,
	}, nil
}

func (r *HTTPResolver) sourceTarget(kind, id string) (Endpoint, string, string, string, bool) {
	escaped := url.PathEscape(id)
	switch kind {
	case invoice.SourceProject:
		return r.project, "/api/v1/references/projects/" + escaped, "project-workspace", "project", true
	case invoice.SourceContract:
		return r.contract, "/api/v1/references/contracts/" + escaped, "contract-ledger", "contract", true
	case invoice.SourceSettlement:
		return r.contract, "/api/v1/references/settlements/" + escaped, "contract-ledger", "settlement", true
	case invoice.SourceMoneyObligation:
		return r.money, "/api/v1/references/obligations/" + escaped, "money-flow-ledger", "obligation", true
	default:
		return Endpoint{}, "", "", "", false
	}
}

func sourceStatusAvailable(kind, status string) bool {
	switch kind {
	case invoice.SourceProject:
		return status == "active" || status == "pending_completion"
	case invoice.SourceContract:
		return status == "active" || status == "terminated" || status == "closed"
	case invoice.SourceSettlement:
		return status == "recorded"
	case invoice.SourceMoneyObligation:
		return status == "open" || status == "closed"
	default:
		return false
	}
}

func (r *HTTPResolver) get(ctx context.Context, tenantID string, endpoint Endpoint, path, resourceType, resourceID string) ([]byte, error) {
	token := endpoint.AccessToken
	if endpoint.Credentials != nil {
		var err error
		token, err = workloadidentity.CredentialForTenant(ctx, endpoint.Credentials, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if err != nil || strings.TrimSpace(token) == "" {
			return nil, invoice.ErrReferenceUnavailable
		}
	}
	return r.getWithToken(ctx, tenantID, endpoint, path, resourceType, resourceID, token, true)
}

func (r *HTTPResolver) getWithToken(ctx context.Context, tenantID string, endpoint Endpoint, path, resourceType, resourceID, token string, retry bool) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, invoice.ErrReferenceUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := r.client.Do(request)
	if err != nil {
		return nil, invoice.ErrReferenceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && endpoint.Credentials != nil && retry {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		workloadidentity.InvalidateForTenant(endpoint.Credentials, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		fresh, credentialErr := workloadidentity.CredentialForTenant(ctx, endpoint.Credentials, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if credentialErr != nil || strings.TrimSpace(fresh) == "" {
			return nil, invoice.ErrReferenceUnavailable
		}
		return r.getWithToken(ctx, tenantID, endpoint, path, resourceType, resourceID, fresh, false)
	}
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUnprocessableEntity {
		return nil, invoice.ErrReferenceInvalid
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, invoice.ErrReferenceUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return nil, invoice.ErrReferenceUnavailable
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
