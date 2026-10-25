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
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/payroll/internal/payroll"
)

const (
	maxResponseBytes = 64 << 10
	audience         = "money-flow-ledger"
	action           = "money-flow-ledger.payroll-payment.record"
	resourceType     = "payroll_payment"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

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
	hasStatic := strings.TrimSpace(staticToken) != ""
	hasDynamic := credentials != nil
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || hasStatic == hasDynamic || timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("Money payment configuration is invalid")
	}
	return &HTTPClient{client: &http.Client{Timeout: timeout}, baseURL: baseURL, staticToken: staticToken, credentials: credentials}, nil
}

func (c *HTTPClient) Deliver(ctx context.Context, intent payroll.PaymentIntent) (payroll.PaymentDeliveryResult, error) {
	body, err := json.Marshal(struct {
		SchemaVersion          string `json:"schema_version"`
		CommandID              string `json:"command_id"`
		TenantID               string `json:"tenant_id"`
		PayrollRunID           string `json:"payroll_run_id"`
		PayrollPeriod          string `json:"payroll_period"`
		PayrollLabel           string `json:"payroll_label"`
		PayrollLineID          string `json:"payroll_line_id"`
		PayrollLineRevision    int64  `json:"payroll_line_revision"`
		PayrollLineDigest      string `json:"payroll_line_digest"`
		PersonID               string `json:"person_id"`
		PersonRevision         int64  `json:"person_revision"`
		PersonDigest           string `json:"person_digest"`
		PersonDisplayName      string `json:"person_display_name"`
		TotalNetPayableMinor   int64  `json:"total_net_payable_minor"`
		ActualAmountMinor      int64  `json:"actual_amount_minor"`
		OccurredOn             string `json:"occurred_on"`
		Note                   string `json:"note"`
		PaymentProfileID       string `json:"payment_profile_id"`
		PaymentProfileRevision int64  `json:"payment_profile_revision"`
		PaymentBankName        string `json:"payment_bank_name"`
		MaskedPaymentAccount   string `json:"masked_payment_account"`
	}{
		SchemaVersion: "1", CommandID: intent.CommandID, TenantID: intent.TenantID,
		PayrollRunID: intent.PayrollRunID, PayrollPeriod: intent.PayrollPeriod, PayrollLabel: intent.PayrollLabel,
		PayrollLineID: intent.PayrollLineID, PayrollLineRevision: intent.PayrollLineRevision, PayrollLineDigest: intent.PayrollLineDigest,
		PersonID: intent.PersonID, PersonRevision: intent.PersonRevision, PersonDigest: intent.PersonDigest, PersonDisplayName: intent.PersonDisplayName,
		TotalNetPayableMinor: intent.TotalNetPayableMinor, ActualAmountMinor: intent.AmountMinor, OccurredOn: intent.OccurredOn, Note: intent.Note,
		PaymentProfileID: intent.PaymentProfileID, PaymentProfileRevision: intent.PaymentProfileRevision,
		PaymentBankName: intent.BankName, MaskedPaymentAccount: intent.MaskedAccount,
	})
	if err != nil {
		return payroll.PaymentDeliveryResult{}, err
	}
	token := c.staticToken
	if c.credentials != nil {
		token, err = c.credentials.CredentialForTenant(ctx, intent.TenantID, audience, action, resourceType, intent.CommandID)
		if err != nil || strings.TrimSpace(token) == "" {
			return payroll.PaymentDeliveryResult{}, payroll.ErrPaymentUnknown
		}
	}
	return c.deliver(ctx, intent, body, token, true)
}

func (c *HTTPClient) deliver(ctx context.Context, intent payroll.PaymentIntent, body []byte, token string, retry bool) (payroll.PaymentDeliveryResult, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/payroll-payments", bytes.NewReader(body))
	if err != nil {
		return payroll.PaymentDeliveryResult{}, payroll.ErrPaymentUnknown
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return payroll.PaymentDeliveryResult{}, payroll.ErrPaymentUnknown
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && retry && c.credentials != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		c.credentials.InvalidateForTenant(intent.TenantID, audience, action, resourceType, intent.CommandID)
		fresh, credentialErr := c.credentials.CredentialForTenant(ctx, intent.TenantID, audience, action, resourceType, intent.CommandID)
		if credentialErr != nil || strings.TrimSpace(fresh) == "" {
			return payroll.PaymentDeliveryResult{}, payroll.ErrPaymentUnknown
		}
		return c.deliver(ctx, intent, body, fresh, false)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		if response.StatusCode >= 400 && response.StatusCode < 500 && response.StatusCode != http.StatusRequestTimeout && response.StatusCode != http.StatusTooManyRequests {
			return payroll.PaymentDeliveryResult{Status: "failed", FailureCode: failureCode(response.StatusCode)}, nil
		}
		return payroll.PaymentDeliveryResult{}, payroll.ErrPaymentUnknown
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(raw) > maxResponseBytes {
		return payroll.PaymentDeliveryResult{}, payroll.ErrPaymentUnknown
	}
	var wire struct {
		Status                  string `json:"status"`
		MoneyPaymentID          string `json:"money_payment_id"`
		MoneyPaymentRevision    int64  `json:"money_payment_revision"`
		MoneyAllocationID       string `json:"money_allocation_id"`
		MoneyAllocationRevision int64  `json:"money_allocation_revision"`
		AllocatedAmountMinor    int64  `json:"allocated_amount_minor"`
		OverageAmountMinor      int64  `json:"overage_amount_minor"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if decoder.Decode(&wire) != nil {
		return payroll.PaymentDeliveryResult{}, payroll.ErrPaymentUnknown
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return payroll.PaymentDeliveryResult{}, payroll.ErrPaymentUnknown
	}
	wire.MoneyPaymentID = strings.ToLower(strings.TrimSpace(wire.MoneyPaymentID))
	wire.MoneyAllocationID = strings.ToLower(strings.TrimSpace(wire.MoneyAllocationID))
	allocationValid := (wire.AllocatedAmountMinor == 0 && wire.MoneyAllocationID == "" && wire.MoneyAllocationRevision == 0) || (wire.AllocatedAmountMinor > 0 && uuidPattern.MatchString(wire.MoneyAllocationID) && wire.MoneyAllocationRevision >= 1)
	if wire.Status != "succeeded" || !uuidPattern.MatchString(wire.MoneyPaymentID) || wire.MoneyPaymentRevision < 1 || wire.AllocatedAmountMinor < 0 || wire.OverageAmountMinor < 0 || wire.AllocatedAmountMinor+wire.OverageAmountMinor != intent.AmountMinor || !allocationValid {
		return payroll.PaymentDeliveryResult{}, payroll.ErrPaymentUnknown
	}
	return payroll.PaymentDeliveryResult{Status: "succeeded", MoneyPaymentID: wire.MoneyPaymentID, MoneyPaymentRevision: wire.MoneyPaymentRevision, MoneyAllocationID: wire.MoneyAllocationID, MoneyAllocationRevision: wire.MoneyAllocationRevision, AllocatedAmountMinor: wire.AllocatedAmountMinor, OverageAmountMinor: wire.OverageAmountMinor}, nil
}

func failureCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "MONEY_INVALID_REQUEST"
	case http.StatusForbidden:
		return "MONEY_PAYMENT_FORBIDDEN"
	case http.StatusConflict:
		return "MONEY_COMMAND_CONFLICT"
	case http.StatusUnprocessableEntity:
		return "MONEY_PAYMENT_REJECTED"
	default:
		return "MONEY_CLIENT_ERROR"
	}
}

var _ payroll.PaymentDeliveryPort = (*HTTPClient)(nil)
