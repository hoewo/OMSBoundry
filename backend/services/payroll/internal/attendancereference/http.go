package attendancereference

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	contract "omsbizfoundry/attendance-summary-contract"
	"omsbizfoundry/payroll/internal/payroll"
)

const maxResponseBytes = 128 << 10

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
		return nil, fmt.Errorf("attendance summary configuration is invalid")
	}
	return &HTTPClient{client: &http.Client{Timeout: timeout}, baseURL: baseURL, staticToken: staticToken, credentials: credentials}, nil
}
func (c *HTTPClient) GetMonthlySummary(ctx context.Context, tenantID, summaryID string) (contract.MonthlySummary, error) {
	token := c.staticToken
	var err error
	if c.credentials != nil {
		token, err = c.credentials.CredentialForTenant(ctx, tenantID, "workforce-attendance", "workforce-attendance.summary.read", "monthly_summary", summaryID)
		if err != nil {
			return contract.MonthlySummary{}, err
		}
	}
	return c.get(ctx, tenantID, summaryID, token, true)
}
func (c *HTTPClient) get(ctx context.Context, tenantID, summaryID, token string, retry bool) (contract.MonthlySummary, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/monthly-summaries/"+url.PathEscape(summaryID), nil)
	if err != nil {
		return contract.MonthlySummary{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return contract.MonthlySummary{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && retry && c.credentials != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		c.credentials.InvalidateForTenant(tenantID, "workforce-attendance", "workforce-attendance.summary.read", "monthly_summary", summaryID)
		fresh, credentialErr := c.credentials.CredentialForTenant(ctx, tenantID, "workforce-attendance", "workforce-attendance.summary.read", "monthly_summary", summaryID)
		if credentialErr != nil {
			return contract.MonthlySummary{}, credentialErr
		}
		return c.get(ctx, tenantID, summaryID, fresh, false)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contract.MonthlySummary{}, fmt.Errorf("attendance summary returned HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	decoder.DisallowUnknownFields()
	var summary contract.MonthlySummary
	if err = decoder.Decode(&summary); err != nil || contract.Validate(summary) != nil || summary.ID != summaryID || summary.TenantID != tenantID {
		return contract.MonthlySummary{}, fmt.Errorf("attendance summary response is invalid")
	}
	return summary, nil
}

var _ payroll.AttendanceSummaryPort = (*HTTPClient)(nil)
