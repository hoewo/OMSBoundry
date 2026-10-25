package evidenceref

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"omsbizfoundry/internal-finance-requests/internal/finance"
)

type CredentialSource interface {
	CredentialForTenant(context.Context, string, string, string, string, string) (string, error)
	InvalidateForTenant(string, string, string, string, string)
}

type Client struct {
	baseURL, staticToken string
	credentials          CredentialSource
	client               *http.Client
}

func New(baseURL, staticToken string, credentials CredentialSource, timeout time.Duration) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (strings.TrimSpace(staticToken) == "") == (credentials == nil) || timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("Evidence reference configuration is invalid")
	}
	return &Client{baseURL: baseURL, staticToken: staticToken, credentials: credentials, client: &http.Client{Timeout: timeout}}, nil
}

func (c *Client) Resolve(ctx context.Context, tenantID string, ref finance.EvidenceRef) (finance.EvidenceVersion, error) {
	token := c.staticToken
	var err error
	if c.credentials != nil {
		token, err = c.credentials.CredentialForTenant(ctx, tenantID, "evidence-service", "evidence-service.metadata.read", "file", ref.FileID)
		if err != nil {
			return finance.EvidenceVersion{}, finance.ErrDependency
		}
	}
	return c.get(ctx, tenantID, ref, token, true)
}

func (c *Client) get(ctx context.Context, tenantID string, ref finance.EvidenceRef, token string, retry bool) (finance.EvidenceVersion, error) {
	path := fmt.Sprintf("/v1/references/files/%s/versions/%d", url.PathEscape(ref.FileID), ref.Version)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return finance.EvidenceVersion{}, finance.ErrDependency
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return finance.EvidenceVersion{}, finance.ErrDependency
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && retry && c.credentials != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		c.credentials.InvalidateForTenant(tenantID, "evidence-service", "evidence-service.metadata.read", "file", ref.FileID)
		fresh, credentialErr := c.credentials.CredentialForTenant(ctx, tenantID, "evidence-service", "evidence-service.metadata.read", "file", ref.FileID)
		if credentialErr != nil {
			return finance.EvidenceVersion{}, finance.ErrDependency
		}
		return c.get(ctx, tenantID, ref, fresh, false)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return finance.EvidenceVersion{}, finance.ErrDependency
	}
	var wire struct {
		SchemaVersion     string `json:"schema_version"`
		SourceApplication string `json:"source_application"`
		ResourceType      string `json:"resource_type"`
		ID                string `json:"id"`
		FileID            string `json:"file_id"`
		TenantID          string `json:"tenant_id"`
		Status            string `json:"status"`
		Version           int64  `json:"version"`
		Revision          int64  `json:"revision"`
		DisplayName       string `json:"display_name"`
		DisplaySummary    string `json:"display_summary"`
		FileName          string `json:"file_name"`
		SizeBytes         int64  `json:"size_bytes"`
		MediaType         string `json:"media_type"`
		SHA256            string `json:"sha256"`
		ValidationProfile string `json:"validation_profile"`
		SecurityStatus    string `json:"security_status"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return finance.EvidenceVersion{}, finance.ErrDependency
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return finance.EvidenceVersion{}, finance.ErrDependency
	}
	if wire.SchemaVersion != "1" || wire.SourceApplication != "evidence-service" || wire.ResourceType != "evidence-file" || wire.TenantID != tenantID || wire.ID != ref.FileID || wire.FileID != ref.FileID || wire.Version != ref.Version || wire.Revision != ref.Version {
		return finance.EvidenceVersion{}, finance.ErrDependency
	}
	return finance.EvidenceVersion{FileID: wire.FileID, Version: wire.Version, Status: wire.Status, SHA256: wire.SHA256, MediaType: wire.MediaType, SizeBytes: wire.SizeBytes, ValidationProfile: wire.ValidationProfile, SecurityStatus: wire.SecurityStatus}, nil
}

var _ finance.EvidenceReferences = (*Client)(nil)
