package moneypayment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"omsbizfoundry/internal-finance-requests/internal/finance"
)

type HTTPClient struct {
	client         *http.Client
	baseURL, token string
}

func NewHTTPClient(baseURL, token string, timeout time.Duration) (*HTTPClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || len(strings.TrimSpace(token)) < 24 || timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("Money payment configuration is invalid")
	}
	return &HTTPClient{client: &http.Client{Timeout: timeout}, baseURL: baseURL, token: token}, nil
}
func (c *HTTPClient) Record(ctx context.Context, input finance.MoneyPaymentRequest) (finance.MoneyPaymentResult, error) {
	body, err := json.Marshal(struct {
		SchemaVersion       string `json:"schema_version"`
		CommandID           string `json:"command_id"`
		TenantID            string `json:"tenant_id"`
		ApplicationID       string `json:"application_id"`
		ApplicationRevision int64  `json:"application_revision"`
		ApplicationDigest   string `json:"application_digest"`
		RequestType         string `json:"request_type"`
		Title               string `json:"title"`
		TotalPayableMinor   int64  `json:"total_payable_minor"`
		ActualAmountMinor   int64  `json:"actual_amount_minor"`
		OccurredOn          string `json:"occurred_on"`
		Note                string `json:"note"`
		Payee               struct {
			Kind         string                    `json:"kind"`
			EntityKind   string                    `json:"entity_kind,omitempty"`
			Reference    finance.ReferenceSnapshot `json:"reference"`
			DisplayName  string                    `json:"display_name"`
			BankName     string                    `json:"bank_name"`
			AccountLast4 string                    `json:"account_last4"`
		} `json:"payee"`
	}{SchemaVersion: input.SchemaVersion, CommandID: input.CommandID, TenantID: input.TenantID, ApplicationID: input.ApplicationID, ApplicationRevision: input.ApplicationRevision, ApplicationDigest: input.ApplicationDigest, RequestType: input.RequestType, Title: input.Title, TotalPayableMinor: input.TotalPayableMinor, ActualAmountMinor: input.ActualAmountMinor, OccurredOn: input.OccurredOn, Note: input.Note, Payee: struct {
		Kind         string                    `json:"kind"`
		EntityKind   string                    `json:"entity_kind,omitempty"`
		Reference    finance.ReferenceSnapshot `json:"reference"`
		DisplayName  string                    `json:"display_name"`
		BankName     string                    `json:"bank_name"`
		AccountLast4 string                    `json:"account_last4"`
	}{Kind: input.Payee.Kind, EntityKind: input.Payee.EntityKind, Reference: input.Payee.Reference, DisplayName: input.Payee.DisplayName, BankName: input.Payee.BankName, AccountLast4: input.Payee.AccountLast4}})
	if err != nil {
		return finance.MoneyPaymentResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/internal-finance-payments", bytes.NewReader(body))
	if err != nil {
		return finance.MoneyPaymentResult{}, finance.ErrPaymentUnknown
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return finance.MoneyPaymentResult{}, finance.ErrPaymentUnknown
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		if response.StatusCode >= 400 && response.StatusCode < 500 && response.StatusCode != http.StatusRequestTimeout && response.StatusCode != http.StatusTooManyRequests {
			return finance.MoneyPaymentResult{}, fmt.Errorf("%w: Money returned status %d", finance.ErrPaymentRejected, response.StatusCode)
		}
		return finance.MoneyPaymentResult{}, finance.ErrPaymentUnknown
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil || len(raw) > 64<<10 {
		return finance.MoneyPaymentResult{}, finance.ErrPaymentUnknown
	}
	var result finance.MoneyPaymentResult
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if decoder.Decode(&result) != nil {
		return finance.MoneyPaymentResult{}, finance.ErrPaymentUnknown
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return finance.MoneyPaymentResult{}, finance.ErrPaymentUnknown
	}
	return result, nil
}

var _ finance.MoneyPayments = (*HTTPClient)(nil)
