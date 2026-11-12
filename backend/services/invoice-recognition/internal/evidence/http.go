package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	contract "omsbizfoundry/invoice-recognition-contract"
)

type CredentialSource interface {
	CredentialForTenant(context.Context, string, string, string, string, string) (string, error)
	InvalidateForTenant(string, string, string, string, string)
}

type HTTPClient struct {
	client               *http.Client
	baseURL, staticToken string
	credentials          CredentialSource
}

func NewHTTPClient(baseURL, staticToken string, credentials CredentialSource, timeout time.Duration) (*HTTPClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	hasStatic, hasDynamic := strings.TrimSpace(staticToken) != "", credentials != nil
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || hasStatic == hasDynamic || timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("Evidence client configuration is invalid")
	}
	return &HTTPClient{client: &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, baseURL: baseURL, staticToken: staticToken, credentials: credentials}, nil
}

func (c *HTTPClient) Open(ctx context.Context, tenantID string, reference contract.EvidenceReference) (io.ReadCloser, error) {
	if err := c.verifyMetadata(ctx, tenantID, reference); err != nil {
		return nil, err
	}
	grantToken, err := c.token(ctx, tenantID, "evidence-service.download", reference.FileID)
	if err != nil {
		return nil, err
	}
	grantURL, err := c.createGrant(ctx, tenantID, reference, grantToken, true)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, grantURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, fmt.Errorf("Evidence download returned HTTP %d", response.StatusCode)
	}
	return response.Body, nil
}

func (c *HTTPClient) verifyMetadata(ctx context.Context, tenantID string, reference contract.EvidenceReference) error {
	token, err := c.token(ctx, tenantID, "evidence-service.metadata.read", reference.FileID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/v1/references/files/%s/versions/%d", url.PathEscape(reference.FileID), reference.Version)
	body, status, err := c.authorized(ctx, http.MethodGet, c.baseURL+path, nil, token)
	if status == http.StatusUnauthorized && c.credentials != nil {
		c.credentials.InvalidateForTenant(tenantID, "evidence-service", "evidence-service.metadata.read", "file", reference.FileID)
		token, err = c.token(ctx, tenantID, "evidence-service.metadata.read", reference.FileID)
		if err == nil {
			body, status, err = c.authorized(ctx, http.MethodGet, c.baseURL+path, nil, token)
		}
	}
	if err != nil || status < 200 || status >= 300 {
		return fmt.Errorf("Evidence metadata unavailable")
	}
	var metadata struct {
		FileID         string `json:"file_id"`
		TenantID       string `json:"tenant_id"`
		Status         string `json:"status"`
		Version        int64  `json:"version"`
		SizeBytes      int64  `json:"size_bytes"`
		MediaType      string `json:"media_type"`
		SHA256         string `json:"sha256"`
		SecurityStatus string `json:"security_status"`
	}
	if json.Unmarshal(body, &metadata) != nil || metadata.FileID != reference.FileID || metadata.TenantID != tenantID || metadata.Status != "available" || metadata.Version != reference.Version || metadata.SizeBytes != reference.SizeBytes || strings.ToLower(metadata.MediaType) != strings.ToLower(reference.MIMEType) || strings.TrimPrefix(reference.ContentSHA256, "sha256:") != strings.ToLower(metadata.SHA256) || (metadata.SecurityStatus != "passed" && metadata.SecurityStatus != "not_required") {
		return fmt.Errorf("Evidence version metadata does not match the recognition request")
	}
	return nil
}

func (c *HTTPClient) createGrant(ctx context.Context, tenantID string, reference contract.EvidenceReference, token string, retry bool) (string, error) {
	payload := []byte(`{"disposition":"inline","delivery_mode":"direct"}`)
	path := fmt.Sprintf("/v1/files/%s/versions/%d/access-grants", url.PathEscape(reference.FileID), reference.Version)
	body, status, err := c.authorized(ctx, http.MethodPost, c.baseURL+path, payload, token)
	if status == http.StatusUnauthorized && retry && c.credentials != nil {
		c.credentials.InvalidateForTenant(tenantID, "evidence-service", "evidence-service.download", "file", reference.FileID)
		fresh, credentialErr := c.token(ctx, tenantID, "evidence-service.download", reference.FileID)
		if credentialErr != nil {
			return "", credentialErr
		}
		return c.createGrant(ctx, tenantID, reference, fresh, false)
	}
	if err != nil || status < 200 || status >= 300 {
		return "", fmt.Errorf("Evidence access grant unavailable")
	}
	var grant struct {
		FileID  string `json:"file_id"`
		Version int64  `json:"version"`
		URL     string `json:"url"`
		Method  string `json:"method"`
	}
	if json.Unmarshal(body, &grant) != nil || grant.FileID != reference.FileID || grant.Version != reference.Version || grant.Method != http.MethodGet {
		return "", fmt.Errorf("Evidence access grant is invalid")
	}
	parsed, parseErr := url.Parse(grant.URL)
	if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("Evidence access URL is invalid")
	}
	// Local Evidence grants intentionally carry the public browser origin. A
	// service container must not depend on public DNS or hairpin routing to read
	// the same protected bytes. Keep the signed path and query intact, but use
	// the configured service-to-service origin for the local data-plane route.
	if index := strings.Index(parsed.Path, "/v1/local-files/"); index >= 0 {
		internal, err := url.Parse(c.baseURL)
		if err != nil {
			return "", fmt.Errorf("Evidence internal URL is invalid")
		}
		internal.Path = strings.TrimRight(internal.Path, "/") + parsed.Path[index:]
		internal.RawQuery = parsed.RawQuery
		return internal.String(), nil
	}
	return grant.URL, nil
}

func (c *HTTPClient) token(ctx context.Context, tenantID, action, fileID string) (string, error) {
	if c.credentials == nil {
		return c.staticToken, nil
	}
	return c.credentials.CredentialForTenant(ctx, tenantID, "evidence-service", action, "file", fileID)
}
func (c *HTTPClient) authorized(ctx context.Context, method, target string, payload []byte, token string) ([]byte, int, error) {
	request, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 256<<10))
	return body, response.StatusCode, err
}
