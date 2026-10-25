package money

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

	"omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
	contract "omsbizfoundry/payable-suggestion-contract"
	workloadidentity "omsbizfoundry/workload-identity"
)

const (
	maxResponseBytes                = 1 << 20
	actionPayableSuggestionSubmit   = "money-flow-ledger.payable-suggestion.submit"
	actionPayableSuggestionRead     = "money-flow-ledger.payable-suggestion.read"
	actionPayableSuggestionWithdraw = "money-flow-ledger.payable-suggestion.withdraw"
)

type CredentialSource interface {
	Credential(context.Context, string, string, string, string) (string, error)
	Invalidate(string, string, string, string)
}

var _ CredentialSource = (*workloadidentity.TokenSource)(nil)

type Endpoint struct {
	BaseURL     string
	AccessToken string
	Credentials CredentialSource
}

type Client struct {
	client   *http.Client
	endpoint Endpoint
}

func NewClient(timeout time.Duration, endpoint Endpoint) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint.BaseURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("Money base URL must be an absolute http or https URL")
	}
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("Money timeout must be between 1ns and 30s")
	}
	if (strings.TrimSpace(endpoint.AccessToken) == "") == (endpoint.Credentials == nil) {
		return nil, fmt.Errorf("configure exactly one Money static token or workload credential source")
	}
	endpoint.BaseURL = strings.TrimRight(endpoint.BaseURL, "/")
	return &Client{client: &http.Client{Timeout: timeout}, endpoint: endpoint}, nil
}

func (c *Client) SubmitPayableSuggestion(ctx context.Context, tenantID, suggestionID string, value costoutsourcing.PayableSubmission) (costoutsourcing.PayableIntake, error) {
	wire := contract.Submission{SchemaVersion: contract.SchemaVersion, SuggestionID: value.SuggestionID, TenantID: value.TenantID, ProjectID: value.ProjectID,
		Source:       contract.SourceSnapshot{Application: contract.SourceApplication, ResourceType: value.Source.Type, ResourceID: value.Source.ID, Revision: value.Source.Revision, Digest: value.Source.Digest},
		LedgerOwner:  contract.PartySnapshot{ID: value.LedgerOwner.ID, Revision: value.LedgerOwner.Revision, Kind: value.LedgerOwner.Kind, LegalName: value.LedgerOwner.LegalName},
		Counterparty: contract.PartySnapshot{ID: value.Counterparty.ID, Revision: value.Counterparty.Revision, Kind: value.Counterparty.Kind, LegalName: value.Counterparty.LegalName},
		AmountMinor:  value.AmountMinor, Currency: value.Currency, DueOn: value.DueOn, Purpose: value.Purpose, RequestDigest: value.RequestDigest}
	var response struct {
		Intake contract.Intake `json:"intake"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v1/payable-suggestion-intakes", actionPayableSuggestionSubmit, tenantID, suggestionID, wire, &response); err != nil {
		return costoutsourcing.PayableIntake{}, err
	}
	return intakeFrom(response.Intake), nil
}

func (c *Client) GetPayableSuggestion(ctx context.Context, tenantID, suggestionID string) (costoutsourcing.PayableIntake, error) {
	var response contract.Intake
	path := "/api/v1/payable-suggestion-intakes/" + url.PathEscape(suggestionID)
	if err := c.do(ctx, http.MethodGet, path, actionPayableSuggestionRead, tenantID, suggestionID, nil, &response); err != nil {
		return costoutsourcing.PayableIntake{}, err
	}
	return intakeFrom(response), nil
}

func (c *Client) WithdrawPayableSuggestion(ctx context.Context, tenantID, suggestionID, commandID string, expectedRevision int64, reason string) (costoutsourcing.PayableIntake, error) {
	value := contract.Withdrawal{CommandID: commandID, ExpectedRevision: expectedRevision, Reason: reason}
	var response struct {
		Intake contract.Intake `json:"intake"`
	}
	path := "/api/v1/payable-suggestion-intakes/" + url.PathEscape(suggestionID) + "/withdrawal"
	if err := c.do(ctx, http.MethodPost, path, actionPayableSuggestionWithdraw, tenantID, suggestionID, value, &response); err != nil {
		return costoutsourcing.PayableIntake{}, err
	}
	return intakeFrom(response.Intake), nil
}

func (c *Client) do(ctx context.Context, method, path, action, tenantID, suggestionID string, input, output any) error {
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return err
		}
	}
	token := c.endpoint.AccessToken
	if c.endpoint.Credentials != nil {
		token, err = workloadidentity.CredentialForTenant(ctx, c.endpoint.Credentials, tenantID, "money-flow-ledger", action, "payable_suggestion", suggestionID)
		if err != nil {
			return costoutsourcing.ErrTargetUnavailable
		}
	}
	return c.doWithToken(ctx, method, path, action, tenantID, suggestionID, body, token, output, true)
}

func (c *Client) doWithToken(ctx context.Context, method, path, action, tenantID, suggestionID string, body []byte, token string, output any, retry bool) error {
	request, err := http.NewRequestWithContext(ctx, method, c.endpoint.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return costoutsourcing.ErrTargetUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(request)
	if err != nil {
		return costoutsourcing.ErrTargetUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && c.endpoint.Credentials != nil && retry {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		workloadidentity.InvalidateForTenant(c.endpoint.Credentials, tenantID, "money-flow-ledger", action, "payable_suggestion", suggestionID)
		fresh, credentialErr := workloadidentity.CredentialForTenant(ctx, c.endpoint.Credentials, tenantID, "money-flow-ledger", action, "payable_suggestion", suggestionID)
		if credentialErr != nil {
			return costoutsourcing.ErrTargetUnavailable
		}
		return c.doWithToken(ctx, method, path, action, tenantID, suggestionID, body, fresh, output, false)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return costoutsourcing.ErrTargetUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(raw) > maxResponseBytes || json.Unmarshal(raw, output) != nil {
		return costoutsourcing.ErrTargetUnavailable
	}
	return nil
}

func intakeFrom(value contract.Intake) costoutsourcing.PayableIntake {
	result := costoutsourcing.PayableIntake{SuggestionID: value.SuggestionID, TenantID: value.TenantID, RequestDigest: value.RequestDigest, Status: value.Status, Revision: value.Revision, DecisionReason: value.DecisionReason}
	if value.Obligation != nil {
		result.Obligation = &costoutsourcing.ObligationReference{ID: value.Obligation.ID, Revision: value.Obligation.Revision}
	}
	return result
}
