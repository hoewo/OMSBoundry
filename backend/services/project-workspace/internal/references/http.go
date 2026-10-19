package references

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"omsbizfoundry/project-workspace/internal/projects"
)

const maxResponseBytes = 1 << 20

type Endpoint struct {
	BaseURL     string
	AccessToken string
	Audience    string
	Action      string
	Credentials CredentialSource
}

type CredentialSource interface {
	Credential(context.Context, string, string, string, string) (string, error)
	Invalidate(string, string, string, string)
}

type HTTPValidator struct {
	client    *http.Client
	people    Endpoint
	resources map[string]Endpoint
}

func NewHTTPValidator(timeout time.Duration, people Endpoint, resources map[string]Endpoint) (*HTTPValidator, error) {
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("reference timeout must be between 1ns and 30s")
	}
	if err := validateEndpoint(people); err != nil {
		return nil, fmt.Errorf("people endpoint: %w", err)
	}
	copyResources := make(map[string]Endpoint, len(resources))
	for service, endpoint := range resources {
		if service == "" {
			return nil, fmt.Errorf("resource service is required")
		}
		if err := validateEndpoint(endpoint); err != nil {
			return nil, fmt.Errorf("%s endpoint: %w", service, err)
		}
		copyResources[service] = endpoint
	}
	return &HTTPValidator{
		client:    &http.Client{Timeout: timeout},
		people:    people,
		resources: copyResources,
	}, nil
}

func validateEndpoint(endpoint Endpoint) error {
	if endpoint.BaseURL == "" && endpoint.AccessToken == "" && endpoint.Credentials == nil {
		return nil
	}
	parsed, err := url.Parse(endpoint.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("base URL must use http or https")
	}
	if (endpoint.AccessToken == "") == (endpoint.Credentials == nil) {
		return fmt.Errorf("exactly one access token or credential source is required")
	}
	if endpoint.Credentials != nil && (strings.TrimSpace(endpoint.Audience) == "" || strings.TrimSpace(endpoint.Action) == "") {
		return fmt.Errorf("dynamic credential audience and action are required")
	}
	return nil
}

func (v *HTTPValidator) ValidatePeople(ctx context.Context, tenantID, kind, id string, revision int64) (projects.ReferenceSnapshot, error) {
	if v.people.BaseURL == "" {
		return projects.ReferenceSnapshot{}, projects.ErrReferenceUnavailable
	}
	path := "/api/v1/persons/" + url.PathEscape(id)
	if kind == "group" {
		path = "/api/v1/groups/" + url.PathEscape(id)
	}
	body, err := v.get(ctx, v.people, path, kind, id)
	if err != nil {
		return projects.ReferenceSnapshot{}, err
	}
	var envelope struct {
		ID          string `json:"id"`
		TenantID    string `json:"tenant_id"`
		DisplayName string `json:"display_name"`
		Name        string `json:"name"`
		Status      string `json:"status"`
		Revision    int64  `json:"revision"`
		Person      *struct {
			ID          string `json:"id"`
			TenantID    string `json:"tenant_id"`
			DisplayName string `json:"display_name"`
			Status      string `json:"status"`
			Revision    int64  `json:"revision"`
		} `json:"person"`
		Group *struct {
			ID       string `json:"id"`
			TenantID string `json:"tenant_id"`
			Name     string `json:"name"`
			Status   string `json:"status"`
			Revision int64  `json:"revision"`
		} `json:"group"`
	}
	if err := decode(body, &envelope); err != nil {
		return projects.ReferenceSnapshot{}, projects.ErrReferenceUnavailable
	}
	if envelope.Person != nil {
		envelope.ID, envelope.TenantID, envelope.DisplayName, envelope.Status, envelope.Revision = envelope.Person.ID, envelope.Person.TenantID, envelope.Person.DisplayName, envelope.Person.Status, envelope.Person.Revision
	}
	if envelope.Group != nil {
		envelope.ID, envelope.TenantID, envelope.DisplayName, envelope.Status, envelope.Revision = envelope.Group.ID, envelope.Group.TenantID, envelope.Group.Name, envelope.Group.Status, envelope.Group.Revision
	}
	if envelope.DisplayName == "" {
		envelope.DisplayName = envelope.Name
	}
	if envelope.ID != id || envelope.TenantID != tenantID || envelope.Status != "active" || envelope.Revision != revision || strings.TrimSpace(envelope.DisplayName) == "" {
		return projects.ReferenceSnapshot{}, projects.ErrReferenceInvalid
	}
	return projects.ReferenceSnapshot{Revision: envelope.Revision, DisplayName: envelope.DisplayName}, nil
}

func (v *HTTPValidator) ValidateParty(ctx context.Context, tenantID, id string, revision int64) (projects.PartySnapshot, error) {
	endpoint, ok := v.resources["party-directory"]
	if !ok || endpoint.BaseURL == "" {
		return projects.PartySnapshot{}, projects.ErrReferenceUnavailable
	}
	body, err := v.get(ctx, endpoint, "/api/v1/references/parties/"+url.PathEscape(id), "party", id)
	if err != nil {
		return projects.PartySnapshot{}, err
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
		return projects.PartySnapshot{}, projects.ErrReferenceUnavailable
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "party-directory" || value.ID != id || value.TenantID != tenantID || value.Status != "active" || value.Revision != revision || (value.Kind != "organization" && value.Kind != "individual") || strings.TrimSpace(value.LegalName) == "" {
		return projects.PartySnapshot{}, projects.ErrReferenceInvalid
	}
	return projects.PartySnapshot{PartyID: value.ID, SourceRevision: value.Revision, Kind: value.Kind, LegalName: value.LegalName}, nil
}

func (v *HTTPValidator) ValidateResource(ctx context.Context, tenantID, service, resourceType, id string, revision *int64) error {
	value, err := v.ResolveResource(ctx, tenantID, service, resourceType, id)
	if err != nil {
		return err
	}
	if revision != nil && value.Revision != *revision {
		return projects.ErrReferenceInvalid
	}
	return nil
}

func (v *HTTPValidator) ResolveCompletionResults(ctx context.Context, tenantID, projectID string) ([]projects.CompletionResultReference, error) {
	endpoint, ok := v.resources["approval-audit-results"]
	if !ok || endpoint.BaseURL == "" {
		return nil, projects.ErrReferenceUnavailable
	}
	body, err := v.get(ctx, endpoint, "/api/v1/references/projects/"+url.PathEscape(projectID)+"/approval-audit-results", "project", projectID)
	if err != nil {
		return nil, err
	}
	var value struct {
		SchemaVersion     string                               `json:"schema_version"`
		SourceApplication string                               `json:"source_application"`
		ProjectID         string                               `json:"project_id"`
		Items             []projects.CompletionResultReference `json:"items"`
	}
	if err := decode(body, &value); err != nil {
		return nil, projects.ErrReferenceUnavailable
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "approval-audit-results" || value.ProjectID != projectID || len(value.Items) > 2 {
		return nil, projects.ErrReferenceInvalid
	}
	seen := map[string]bool{}
	for _, item := range value.Items {
		if item.TenantID != tenantID || item.ProjectID != projectID || item.ResourceType != "approval_audit_result" ||
			(item.Kind != "approval" && item.Kind != "audit") || (item.State != "recorded" && item.State != "withdrawn") ||
			!projects.ValidUUID(item.ID) || item.Revision < 1 || strings.TrimSpace(item.Digest) == "" || strings.TrimSpace(item.DisplaySummary) == "" || seen[item.Kind] {
			return nil, projects.ErrReferenceInvalid
		}
		seen[item.Kind] = true
	}
	return value.Items, nil
}

func (v *HTTPValidator) ResolveResource(ctx context.Context, tenantID, service, resourceType, id string) (projects.ResourceReference, error) {
	endpoint, ok := v.resources[service]
	if !ok || endpoint.BaseURL == "" {
		return projects.ResourceReference{}, projects.ErrReferenceUnavailable
	}
	path, ok := referencePath(service, resourceType, id)
	if !ok {
		return projects.ResourceReference{}, projects.ErrReferenceInvalid
	}
	// One audience contains two native namespaces. Select exactly one fixed
	// read action from the closed resource type, never from client input.
	if service == "cost-outsourcing" && endpoint.Credentials != nil {
		if endpoint.Audience != service {
			return projects.ResourceReference{}, projects.ErrReferenceInvalid
		}
		switch resourceType {
		case "cost_entry":
			endpoint.Action = "cost-entry.read"
		case "outsourcing_engagement":
			endpoint.Action = "outsourcing-engagement.read"
		default:
			return projects.ResourceReference{}, projects.ErrReferenceInvalid
		}
	}
	authorizationResourceType := resourceType
	if service == "evidence-service" && resourceType == "evidence-file" {
		authorizationResourceType = "file"
	}
	body, err := v.get(ctx, endpoint, path, authorizationResourceType, id)
	if err != nil {
		return projects.ResourceReference{}, err
	}
	var value struct {
		SchemaVersion          string  `json:"schema_version"`
		SourceApplication      string  `json:"source_application"`
		ID                     string  `json:"id"`
		TenantID               string  `json:"tenant_id"`
		ResourceType           string  `json:"resource_type"`
		DisplaySummary         string  `json:"display_summary"`
		Status                 string  `json:"status"`
		Revision               *int64  `json:"revision"`
		Currency               string  `json:"currency"`
		CurrentEffectiveAmount *string `json:"current_effective_amount"`
		Direction              string  `json:"direction"`
		TransactionType        string  `json:"transaction_type"`
		EffectiveAmount        string  `json:"effective_amount"`
		AllocatedAmount        string  `json:"allocated_amount"`
		RemittedAmount         string  `json:"remitted_amount"`
		OutstandingAmount      string  `json:"outstanding_amount"`
		UnallocatedAmount      string  `json:"unallocated_amount"`
		BalanceSequence        int64   `json:"balance_sequence"`
		OccurredOn             string  `json:"occurred_on"`
		IssuedOn               string  `json:"issued_on"`
		AmountMinor            int64   `json:"amount_minor"`
		Version                int64   `json:"version"`
		FileName               string  `json:"file_name"`
		SizeBytes              int64   `json:"size_bytes"`
		MediaType              string  `json:"media_type"`
		SHA256                 string  `json:"sha256"`
		ProjectID              string  `json:"project_id"`
		ResultKind             string  `json:"result_kind"`
		Digest                 string  `json:"digest"`
		PartyReferences        []struct {
			ID        string `json:"id"`
			Revision  int64  `json:"revision"`
			Kind      string `json:"kind"`
			LegalName string `json:"legal_name"`
			Source    string `json:"source"`
			Role      string `json:"role"`
		} `json:"party_references"`
		Items []struct {
			RelationKey            string `json:"relation_key"`
			FileID                 string `json:"file_id"`
			Version                int64  `json:"version"`
			EvidenceSourceRevision int64  `json:"evidence_source_revision"`
			DisplayName            string `json:"display_name"`
		} `json:"items"`
	}
	if err := decode(body, &value); err != nil {
		return projects.ResourceReference{}, projects.ErrReferenceUnavailable
	}
	if service == "billing-invoice" && resourceType == "invoice_registration" {
		value.ResourceType = resourceType
	}
	if value.ID != id || value.TenantID != tenantID || value.ResourceType != resourceType || value.Revision == nil || *value.Revision < 1 ||
		value.Status == "archived" || value.Status == "cancelled" || value.Status == "deleted" {
		return projects.ResourceReference{}, projects.ErrReferenceInvalid
	}
	result := projects.ResourceReference{
		SourceApplication: value.SourceApplication, ID: value.ID, TenantID: value.TenantID,
		ResourceType: value.ResourceType, DisplaySummary: strings.TrimSpace(value.DisplaySummary),
		Status: value.Status, Revision: *value.Revision,
	}
	parties := make([]projects.CommercialPartySnapshot, 0, len(value.PartyReferences))
	for _, party := range value.PartyReferences {
		if strings.TrimSpace(party.ID) == "" || party.Revision < 1 || strings.TrimSpace(party.LegalName) == "" {
			return projects.ResourceReference{}, projects.ErrReferenceInvalid
		}
		parties = append(parties, projects.CommercialPartySnapshot{ID: party.ID, Revision: party.Revision, Kind: party.Kind, LegalName: party.LegalName, Source: party.Source, Role: party.Role})
	}
	switch service + "/" + resourceType {
	case "contract-ledger/contract", "contract-ledger/settlement":
		if value.SchemaVersion != "1" || value.SourceApplication != "contract-ledger" || result.DisplaySummary == "" || value.Currency == "" {
			return projects.ResourceReference{}, projects.ErrReferenceInvalid
		}
		result.Contract = &projects.ContractCommercialFacts{Currency: value.Currency, CurrentEffectiveAmount: value.CurrentEffectiveAmount, Parties: parties}
	case "money-flow-ledger/obligation":
		if value.SchemaVersion != "1" || value.SourceApplication != "money-flow-ledger" || result.DisplaySummary == "" || value.Currency == "" || value.Direction == "" || value.EffectiveAmount == "" || value.AllocatedAmount == "" || value.RemittedAmount == "" || value.OutstandingAmount == "" {
			return projects.ResourceReference{}, projects.ErrReferenceInvalid
		}
		result.Obligation = &projects.ObligationCommercialFacts{
			Direction: value.Direction, Currency: value.Currency, EffectiveAmount: value.EffectiveAmount,
			AllocatedAmount: value.AllocatedAmount, RemittedAmount: value.RemittedAmount,
			OutstandingAmount: value.OutstandingAmount, BalanceSequence: value.BalanceSequence, Parties: parties,
		}
	case "money-flow-ledger/transaction":
		if value.SchemaVersion != "1" || value.SourceApplication != "money-flow-ledger" || result.DisplaySummary == "" || value.Currency == "" || value.TransactionType == "" || value.EffectiveAmount == "" || value.AllocatedAmount == "" || value.UnallocatedAmount == "" || value.OccurredOn == "" {
			return projects.ResourceReference{}, projects.ErrReferenceInvalid
		}
		result.Transaction = &projects.TransactionCommercialFacts{
			TransactionType: value.TransactionType, Currency: value.Currency, EffectiveAmount: value.EffectiveAmount,
			AllocatedAmount: value.AllocatedAmount, UnallocatedAmount: value.UnallocatedAmount, OccurredOn: value.OccurredOn, Parties: parties,
		}
	case "billing-invoice/invoice_registration":
		if value.SchemaVersion != "1" || value.SourceApplication != "billing-invoice" || value.Currency == "" || value.IssuedOn == "" || value.AmountMinor < 1 {
			return projects.ResourceReference{}, projects.ErrReferenceInvalid
		}
		result.DisplaySummary = "开票登记 " + value.IssuedOn
		result.Invoice = &projects.InvoiceCommercialFacts{IssuedOn: value.IssuedOn, AmountMinor: value.AmountMinor, Currency: value.Currency}
	case "evidence-service/evidence-file":
		if value.SchemaVersion != "1" || value.SourceApplication != "evidence-service" || value.Status != "available" ||
			value.Version != result.Revision || strings.TrimSpace(value.FileName) == "" || value.SizeBytes < 1 ||
			strings.TrimSpace(value.MediaType) == "" || len(strings.TrimSpace(value.SHA256)) != 64 {
			return projects.ResourceReference{}, projects.ErrReferenceInvalid
		}
		result.DisplaySummary = value.FileName
		result.Evidence = &projects.EvidenceFileFacts{
			Version: value.Version, FileName: value.FileName, SizeBytes: value.SizeBytes,
			MediaType: value.MediaType, SHA256: value.SHA256,
		}
	case "business-engagement/business-engagement", "production-operations/production-work-item", "approval-audit-results/approval_audit_result", "cost-outsourcing/cost_entry", "cost-outsourcing/outsourcing_engagement":
		if value.SchemaVersion != "1" || value.SourceApplication != service ||
			!projects.ValidUUID(value.ProjectID) || len(strings.TrimSpace(value.Digest)) != 64 || len(value.Items) > 100 {
			return projects.ResourceReference{}, projects.ErrReferenceInvalid
		}
		recordedSource := service == "approval-audit-results" || service == "cost-outsourcing"
		if service == "production-operations" || recordedSource {
			// The source owns workflow meaning; this consumer verifies its v1
			// wire identity, explicit array and deterministic reference digest.
			if value.Items == nil || value.ProjectID != strings.ToLower(strings.TrimSpace(value.ProjectID)) ||
				(service == "production-operations" && value.Status != "draft" && value.Status != "in-progress" && value.Status != "completed") ||
				(recordedSource && value.Status != "recorded" && value.Status != "withdrawn") ||
				(service == "approval-audit-results" && value.ResultKind != "approval" && value.ResultKind != "audit") {
				return projects.ResourceReference{}, projects.ErrReferenceInvalid
			}
			raw, err := json.Marshal(value.Items)
			if err != nil {
				return projects.ResourceReference{}, projects.ErrReferenceInvalid
			}
			digest := sha256.Sum256(raw)
			if value.Digest != hex.EncodeToString(digest[:]) {
				return projects.ResourceReference{}, projects.ErrReferenceInvalid
			}
		}
		items := make([]projects.MaterialSourceItem, 0, len(value.Items))
		seen := make(map[string]bool, len(value.Items))
		seenRelations := make(map[string]bool, len(value.Items))
		previous := ""
		for index, item := range value.Items {
			key := item.RelationKey + "/" + item.FileID + fmt.Sprintf("/%020d", item.Version)
			if strings.TrimSpace(item.RelationKey) == "" || !projects.ValidUUID(item.FileID) || item.Version < 1 ||
				item.EvidenceSourceRevision < 1 || strings.TrimSpace(item.DisplayName) == "" || seen[key] || (!recordedSource && previous != "" && key < previous) {
				return projects.ResourceReference{}, projects.ErrReferenceInvalid
			}
			if service == "production-operations" && (seenRelations[item.RelationKey] || !productionMaterialRelationPattern.MatchString(item.RelationKey) || item.FileID != strings.ToLower(strings.TrimSpace(item.FileID)) || item.DisplayName != strings.TrimSpace(item.DisplayName) || utf8.RuneCountInString(item.DisplayName) > 255) {
				return projects.ResourceReference{}, projects.ErrReferenceInvalid
			}
			if recordedSource && (item.RelationKey != strings.TrimSpace(item.RelationKey) || utf8.RuneCountInString(item.RelationKey) > 100 || item.FileID != strings.ToLower(strings.TrimSpace(item.FileID)) || item.DisplayName != strings.TrimSpace(item.DisplayName) || utf8.RuneCountInString(item.DisplayName) > 500 || item.EvidenceSourceRevision != item.Version) {
				return projects.ResourceReference{}, projects.ErrReferenceInvalid
			}
			if recordedSource && index > 0 {
				prior := value.Items[index-1]
				if item.RelationKey < prior.RelationKey || (item.RelationKey == prior.RelationKey && (item.FileID < prior.FileID || (item.FileID == prior.FileID && item.Version <= prior.Version))) {
					return projects.ResourceReference{}, projects.ErrReferenceInvalid
				}
			}
			seenRelations[item.RelationKey] = true
			seen[key], previous = true, key
			items = append(items, projects.MaterialSourceItem{
				RelationKey: item.RelationKey, FileID: item.FileID, Version: item.Version,
				EvidenceSourceRevision: item.EvidenceSourceRevision, DisplayName: strings.TrimSpace(item.DisplayName),
			})
		}
		result.MaterialSource = &projects.MaterialSourceFacts{ProjectID: value.ProjectID, Digest: strings.TrimSpace(value.Digest), Items: items}
	}
	return result, nil
}

func referencePath(service, resourceType, id string) (string, bool) {
	escaped := url.PathEscape(id)
	switch service + "/" + resourceType {
	case "contract-ledger/contract":
		return "/api/v1/references/contracts/" + escaped, true
	case "contract-ledger/settlement":
		return "/api/v1/references/settlements/" + escaped, true
	case "money-flow-ledger/obligation":
		return "/api/v1/references/obligations/" + escaped, true
	case "money-flow-ledger/transaction":
		return "/api/v1/references/transactions/" + escaped, true
	case "billing-invoice/invoice_registration":
		return "/api/v1/references/invoice-registrations/" + escaped, true
	case "evidence-service/evidence-file":
		return "/v1/references/files/" + escaped, true
	case "business-engagement/business-engagement":
		return "/api/v1/references/business-engagements/" + escaped + "/materials", true
	case "design-operations/design-work-item":
		return "/api/v1/references/design-work-items/" + escaped, true
	case "production-operations/production-work-item":
		return "/api/v1/references/production-work-items/" + escaped + "/materials", true
	case "approval-audit-results/approval_audit_result":
		return "/api/v1/references/approval-audit-results/" + escaped + "/materials", true
	case "cost-outsourcing/cost_entry":
		return "/api/v1/references/cost-entries/" + escaped + "/materials", true
	case "cost-outsourcing/outsourcing_engagement":
		return "/api/v1/references/outsourcing-engagements/" + escaped + "/materials", true
	default:
		return "", false
	}
}

var productionMaterialRelationPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}\.[a-z][a-z0-9-]{0,63}$`)

func (v *HTTPValidator) get(ctx context.Context, endpoint Endpoint, path, resourceType, resourceID string) ([]byte, error) {
	token := endpoint.AccessToken
	if endpoint.Credentials != nil {
		var err error
		token, err = endpoint.Credentials.Credential(ctx, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if err != nil {
			return nil, projects.ErrReferenceUnavailable
		}
	}
	return v.getWithToken(ctx, endpoint, path, resourceType, resourceID, token, true)
}

func (v *HTTPValidator) getWithToken(ctx context.Context, endpoint Endpoint, path, resourceType, resourceID, token string, retry bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.BaseURL+path, nil)
	if err != nil {
		return nil, projects.ErrReferenceUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	response, err := v.client.Do(req)
	if err != nil {
		return nil, projects.ErrReferenceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && endpoint.Credentials != nil && retry {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		endpoint.Credentials.Invalidate(endpoint.Audience, endpoint.Action, resourceType, resourceID)
		fresh, credentialErr := endpoint.Credentials.Credential(ctx, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if credentialErr != nil {
			return nil, projects.ErrReferenceUnavailable
		}
		return v.getWithToken(ctx, endpoint, path, resourceType, resourceID, fresh, false)
	}
	switch {
	case response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUnprocessableEntity:
		return nil, projects.ErrReferenceInvalid
	case response.StatusCode == http.StatusForbidden:
		return nil, projects.ErrReferenceForbidden
	case response.StatusCode < 200 || response.StatusCode >= 300:
		return nil, projects.ErrReferenceUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return nil, projects.ErrReferenceUnavailable
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
