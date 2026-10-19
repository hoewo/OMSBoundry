package cashevidence

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CredentialSource interface {
	Credential(context.Context, string, string, string, string) (string, error)
	Invalidate(string, string, string, string)
}

type Endpoint struct {
	BaseURL     string
	AccessToken string
	Audience    string
	Action      string
	Credentials CredentialSource
}

type HTTPValidator struct {
	client   *http.Client
	endpoint Endpoint
}

func NewHTTPValidator(timeout time.Duration, endpoint Endpoint) (*HTTPValidator, error) {
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("Evidence validation timeout must be between 1ns and 30s")
	}
	endpoint.BaseURL = strings.TrimRight(strings.TrimSpace(endpoint.BaseURL), "/")
	parsed, err := url.Parse(endpoint.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("Evidence base URL must be an absolute http or https URL")
	}
	if (strings.TrimSpace(endpoint.AccessToken) == "") == (endpoint.Credentials == nil) {
		return nil, fmt.Errorf("configure exactly one Evidence access token or credential source")
	}
	if endpoint.Credentials != nil && (strings.TrimSpace(endpoint.Audience) == "" || strings.TrimSpace(endpoint.Action) == "") {
		return nil, fmt.Errorf("dynamic Evidence credential audience and action are required")
	}
	return &HTTPValidator{client: &http.Client{Timeout: timeout}, endpoint: endpoint}, nil
}

func (v *HTTPValidator) Resolve(ctx context.Context, tenantID string, ref Ref) (FileReference, error) {
	token := strings.TrimSpace(v.endpoint.AccessToken)
	if v.endpoint.Credentials != nil {
		var err error
		token, err = v.endpoint.Credentials.Credential(ctx, v.endpoint.Audience, v.endpoint.Action, "file", ref.FileID)
		if err != nil || strings.TrimSpace(token) == "" {
			return FileReference{}, ErrEvidenceUnavailable
		}
	}
	return v.resolve(ctx, tenantID, ref, token, true)
}

func (v *HTTPValidator) resolve(ctx context.Context, tenantID string, ref Ref, token string, retry bool) (FileReference, error) {
	path := fmt.Sprintf("/v1/references/files/%s/versions/%d", url.PathEscape(ref.FileID), ref.Version)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.endpoint.BaseURL+path, nil)
	if err != nil {
		return FileReference{}, ErrEvidenceUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := v.client.Do(request)
	if err != nil {
		return FileReference{}, ErrEvidenceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && v.endpoint.Credentials != nil && retry {
		io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		v.endpoint.Credentials.Invalidate(v.endpoint.Audience, v.endpoint.Action, "file", ref.FileID)
		fresh, e := v.endpoint.Credentials.Credential(ctx, v.endpoint.Audience, v.endpoint.Action, "file", ref.FileID)
		if e != nil {
			return FileReference{}, ErrEvidenceUnavailable
		}
		return v.resolve(ctx, tenantID, ref, fresh, false)
	}
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusConflict || response.StatusCode == http.StatusUnprocessableEntity {
		return FileReference{}, ErrEvidenceInvalid
	}
	if response.StatusCode != http.StatusOK {
		return FileReference{}, ErrEvidenceUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return FileReference{}, ErrEvidenceUnavailable
	}
	var value struct {
		SchemaVersion     string `json:"schema_version"`
		SourceApplication string `json:"source_application"`
		ID                string `json:"id"`
		FileID            string `json:"file_id"`
		TenantID          string `json:"tenant_id"`
		ResourceType      string `json:"resource_type"`
		Status            string `json:"status"`
		Version           int64  `json:"version"`
		Revision          int64  `json:"revision"`
		DisplayName       string `json:"display_name"`
	}
	if json.Unmarshal(body, &value) != nil {
		return FileReference{}, ErrEvidenceUnavailable
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "evidence-service" || value.ID != ref.FileID || value.FileID != ref.FileID || value.TenantID != tenantID || value.ResourceType != "evidence-file" || value.Status != "available" || value.Version != ref.Version || value.Revision != ref.Version || strings.TrimSpace(value.DisplayName) == "" {
		return FileReference{}, ErrEvidenceInvalid
	}
	return FileReference{TenantID: value.TenantID, FileID: value.FileID, Version: value.Version, Revision: value.Revision, Status: value.Status, Name: value.DisplayName}, nil
}
