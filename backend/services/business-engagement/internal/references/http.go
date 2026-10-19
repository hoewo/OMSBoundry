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

	"omsbizfoundry/business-engagement/internal/engagement"
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
	evidence Endpoint
}

func NewHTTPValidator(timeout time.Duration, project, evidence Endpoint) (*HTTPValidator, error) {
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("reference timeout must be between 1ns and 30s")
	}
	if err := validateEndpoint(project); err != nil {
		return nil, fmt.Errorf("project endpoint: %w", err)
	}
	if err := validateEndpoint(evidence); err != nil {
		return nil, fmt.Errorf("evidence endpoint: %w", err)
	}
	return &HTTPValidator{client: &http.Client{Timeout: timeout}, project: project, evidence: evidence}, nil
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

func (v *HTTPValidator) ValidateProject(ctx context.Context, tenantID, id string, revision int64) (engagement.ProjectReference, error) {
	var value engagement.ProjectReference
	if err := v.get(ctx, v.project, "/api/v1/references/projects/"+url.PathEscape(id), "project", id, &value); err != nil {
		return engagement.ProjectReference{}, err
	}
	if value.ID != id || value.TenantID != tenantID || value.Revision != revision {
		return engagement.ProjectReference{}, engagement.ErrReferenceInvalid
	}
	return value, nil
}

func (v *HTTPValidator) ValidateEvidence(ctx context.Context, tenantID, fileID string, version int, revision int64) (engagement.EvidenceReference, error) {
	var value engagement.EvidenceReference
	path := fmt.Sprintf("/v1/references/files/%s/versions/%d", url.PathEscape(fileID), version)
	if err := v.get(ctx, v.evidence, path, "file", fileID, &value); err != nil {
		return engagement.EvidenceReference{}, err
	}
	if value.FileID != fileID || value.TenantID != tenantID || value.Version != version || value.Revision != revision || strings.TrimSpace(value.DisplayName) == "" {
		return engagement.EvidenceReference{}, engagement.ErrReferenceInvalid
	}
	return value, nil
}

func (v *HTTPValidator) get(ctx context.Context, endpoint Endpoint, path, resourceType, resourceID string, destination any) error {
	token := endpoint.AccessToken
	if endpoint.CredentialSource != nil {
		var err error
		token, err = endpoint.CredentialSource.Credential(ctx, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if err != nil || strings.TrimSpace(token) == "" {
			return engagement.ErrReferenceUnavailable
		}
	}
	return v.getWithToken(ctx, endpoint, path, resourceType, resourceID, token, destination, true)
}

func (v *HTTPValidator) getWithToken(ctx context.Context, endpoint Endpoint, path, resourceType, resourceID, token string, destination any, retry bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint.BaseURL, "/")+path, nil)
	if err != nil {
		return engagement.ErrReferenceUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	response, err := v.client.Do(req)
	if err != nil {
		return engagement.ErrReferenceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && endpoint.CredentialSource != nil && retry {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		endpoint.CredentialSource.Invalidate(endpoint.Audience, endpoint.Action, resourceType, resourceID)
		fresh, credentialErr := endpoint.CredentialSource.Credential(ctx, endpoint.Audience, endpoint.Action, resourceType, resourceID)
		if credentialErr != nil {
			return engagement.ErrReferenceUnavailable
		}
		return v.getWithToken(ctx, endpoint, path, resourceType, resourceID, fresh, destination, false)
	}
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUnprocessableEntity {
		return engagement.ErrReferenceInvalid
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return engagement.ErrReferenceUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return engagement.ErrReferenceUnavailable
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if err := decoder.Decode(destination); err != nil {
		return engagement.ErrReferenceUnavailable
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return engagement.ErrReferenceUnavailable
	}
	return nil
}
