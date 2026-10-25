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

	"omsbizfoundry/design-operations/internal/design"
	workloadidentity "omsbizfoundry/workload-identity"
)

const maxResponseBytes = 1 << 20

type CredentialSource interface {
	Credential(context.Context, string, string, string, string) (string, error)
	Invalidate(string, string, string, string)
}

type Endpoint struct {
	BaseURL          string
	AccessToken      string
	CredentialSource CredentialSource
	Audience         string
	Action           string
}

type HTTPValidator struct {
	client   *http.Client
	project  Endpoint
	people   Endpoint
	evidence Endpoint
}

func NewHTTPValidator(timeout time.Duration, project, people, evidence Endpoint) (*HTTPValidator, error) {
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("reference timeout must be between 1ns and 30s")
	}
	if err := validateEndpoint(project); err != nil {
		return nil, fmt.Errorf("project endpoint: %w", err)
	}
	if err := validateEndpoint(people); err != nil {
		return nil, fmt.Errorf("people endpoint: %w", err)
	}
	if err := validateEndpoint(evidence); err != nil {
		return nil, fmt.Errorf("evidence endpoint: %w", err)
	}
	return &HTTPValidator{client: &http.Client{Timeout: timeout}, project: project, people: people, evidence: evidence}, nil
}

func validateEndpoint(endpoint Endpoint) error {
	parsed, err := url.Parse(endpoint.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("base URL must use http or https")
	}
	hasStatic := strings.TrimSpace(endpoint.AccessToken) != ""
	hasDynamic := endpoint.CredentialSource != nil && strings.TrimSpace(endpoint.Audience) != "" && strings.TrimSpace(endpoint.Action) != ""
	if hasStatic == hasDynamic {
		return fmt.Errorf("configure exactly one static or workload credential source")
	}
	return nil
}

func (v *HTTPValidator) ValidateProject(ctx context.Context, tenantID, id string, revision int64) (design.ProjectReference, error) {
	var value design.ProjectReference
	if err := v.get(ctx, tenantID, v.project, "/api/v1/references/projects/"+url.PathEscape(id), "project", id, &value); err != nil {
		return design.ProjectReference{}, err
	}
	if value.ID != id || value.TenantID != tenantID || value.Revision != revision {
		return design.ProjectReference{}, design.ErrReferenceInvalid
	}
	return value, nil
}

func (v *HTTPValidator) ValidateAssignment(ctx context.Context, tenantID, projectID, id string, revision int64) (design.AssignmentReference, error) {
	var value design.AssignmentReference
	path := "/api/v1/references/projects/" + url.PathEscape(projectID) + "/assignments/" + url.PathEscape(id)
	if err := v.get(ctx, tenantID, v.project, path, "project", projectID, &value); err != nil {
		return design.AssignmentReference{}, err
	}
	if value.ID != id || value.TenantID != tenantID || value.ProjectID != projectID || value.Revision != revision {
		return design.AssignmentReference{}, design.ErrReferenceInvalid
	}
	return value, nil
}

func (v *HTTPValidator) ValidatePersonIAMSubject(ctx context.Context, tenantID, id string, revision int64) (design.PersonIAMSubjectFact, error) {
	var value design.PersonIAMSubjectFact
	path := "/api/v1/policy-facts/persons/" + url.PathEscape(id)
	if err := v.get(ctx, tenantID, v.people, path, "person", id, &value); err != nil {
		return design.PersonIAMSubjectFact{}, err
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "people-groups" || value.FactKind != "person-iam-subject-link" ||
		value.ID != id || value.TenantID != tenantID || value.Status != "active" || value.Revision != revision || !design.ValidUUID(value.IAMSubjectID) {
		return design.PersonIAMSubjectFact{}, design.ErrReferenceInvalid
	}
	return value, nil
}

func (v *HTTPValidator) ValidateEvidence(ctx context.Context, tenantID, fileID string, version int, revision int64) (design.EvidenceReference, error) {
	var value design.EvidenceReference
	path := fmt.Sprintf("/v1/references/files/%s/versions/%d", url.PathEscape(fileID), version)
	if err := v.get(ctx, tenantID, v.evidence, path, "file", fileID, &value); err != nil {
		return design.EvidenceReference{}, err
	}
	if value.FileID != fileID || value.TenantID != tenantID || value.Version != version || value.Revision != revision || strings.TrimSpace(value.DisplayName) == "" {
		return design.EvidenceReference{}, design.ErrReferenceInvalid
	}
	return value, nil
}

func (v *HTTPValidator) get(ctx context.Context, tenantID string, endpoint Endpoint, path, resourceType, resourceID string, destination any) error {
	token := endpoint.AccessToken
	if endpoint.CredentialSource != nil {
		var err error
		token, err = workloadidentity.CredentialForTenant(ctx, endpoint.CredentialSource, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if err != nil || strings.TrimSpace(token) == "" {
			return design.ErrReferenceUnavailable
		}
	}
	return v.getWithToken(ctx, tenantID, endpoint, path, resourceType, resourceID, token, destination, true)
}

func (v *HTTPValidator) getWithToken(ctx context.Context, tenantID string, endpoint Endpoint, path, resourceType, resourceID, token string, destination any, retry bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.BaseURL+path, nil)
	if err != nil {
		return design.ErrReferenceUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	response, err := v.client.Do(req)
	if err != nil {
		return design.ErrReferenceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && endpoint.CredentialSource != nil && retry {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		workloadidentity.InvalidateForTenant(endpoint.CredentialSource, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		fresh, credentialErr := workloadidentity.CredentialForTenant(ctx, endpoint.CredentialSource, tenantID, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if credentialErr != nil {
			return design.ErrReferenceUnavailable
		}
		return v.getWithToken(ctx, tenantID, endpoint, path, resourceType, resourceID, fresh, destination, false)
	}
	switch response.StatusCode {
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return design.ErrReferenceInvalid
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return design.ErrReferenceUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return design.ErrReferenceUnavailable
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if err := decoder.Decode(destination); err != nil {
		return design.ErrReferenceUnavailable
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return design.ErrReferenceUnavailable
	}
	return nil
}
