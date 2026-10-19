package partymigration

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

const maxPartyResponseBytes = 1 << 20

type CredentialSource interface {
	Credential(context.Context, string, string, string, string) (string, error)
	Invalidate(string, string, string, string)
}

type PartyEndpoint struct {
	BaseURL     string
	AccessToken string
	Audience    string
	Action      string
	Credentials CredentialSource
}

type HTTPPartyValidator struct {
	client   *http.Client
	endpoint PartyEndpoint
}

func NewHTTPPartyValidator(timeout time.Duration, endpoint PartyEndpoint) (*HTTPPartyValidator, error) {
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("Party validation timeout must be between 1ns and 30s")
	}
	endpoint.BaseURL = strings.TrimRight(strings.TrimSpace(endpoint.BaseURL), "/")
	parsed, err := url.Parse(endpoint.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("Party base URL must use http or https")
	}
	if (strings.TrimSpace(endpoint.AccessToken) == "") == (endpoint.Credentials == nil) {
		return nil, fmt.Errorf("exactly one Party access token or credential source is required")
	}
	if endpoint.Credentials != nil && (strings.TrimSpace(endpoint.Audience) == "" || strings.TrimSpace(endpoint.Action) == "") {
		return nil, fmt.Errorf("dynamic Party credential audience and action are required")
	}
	return &HTTPPartyValidator{client: &http.Client{Timeout: timeout}, endpoint: endpoint}, nil
}

func (v *HTTPPartyValidator) VerifyParty(ctx context.Context, tenantID, partyID string, revision int64) (TargetSnapshot, error) {
	token := strings.TrimSpace(v.endpoint.AccessToken)
	if v.endpoint.Credentials != nil {
		var err error
		token, err = v.endpoint.Credentials.Credential(ctx, v.endpoint.Audience, v.endpoint.Action, "party", partyID)
		if err != nil {
			return TargetSnapshot{}, ErrTargetUnavailable
		}
	}
	return v.verifyWithToken(ctx, tenantID, partyID, revision, token, true)
}

func (v *HTTPPartyValidator) verifyWithToken(ctx context.Context, tenantID, partyID string, revision int64, token string, retry bool) (TargetSnapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.endpoint.BaseURL+"/api/v1/references/parties/"+url.PathEscape(partyID), nil)
	if err != nil {
		return TargetSnapshot{}, ErrTargetUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := v.client.Do(request)
	if err != nil {
		return TargetSnapshot{}, ErrTargetUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && v.endpoint.Credentials != nil && retry {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxPartyResponseBytes))
		v.endpoint.Credentials.Invalidate(v.endpoint.Audience, v.endpoint.Action, "party", partyID)
		fresh, credentialErr := v.endpoint.Credentials.Credential(ctx, v.endpoint.Audience, v.endpoint.Action, "party", partyID)
		if credentialErr != nil {
			return TargetSnapshot{}, ErrTargetUnavailable
		}
		return v.verifyWithToken(ctx, tenantID, partyID, revision, fresh, false)
	}
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusConflict || response.StatusCode == http.StatusUnprocessableEntity {
		return TargetSnapshot{}, ErrTargetInvalid
	}
	if response.StatusCode != http.StatusOK {
		return TargetSnapshot{}, ErrTargetUnavailable
	}
	limited := io.LimitReader(response.Body, maxPartyResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil || len(body) > maxPartyResponseBytes {
		return TargetSnapshot{}, ErrTargetUnavailable
	}
	var value struct {
		SchemaVersion          string  `json:"schema_version"`
		SourceApplication      string  `json:"source_application"`
		ID                     string  `json:"id"`
		TenantID               string  `json:"tenant_id"`
		Kind                   string  `json:"kind"`
		LegalName              string  `json:"legal_name"`
		RegistrationIdentifier *string `json:"registration_identifier"`
		Status                 string  `json:"status"`
		Revision               int64   `json:"revision"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		return TargetSnapshot{}, ErrTargetUnavailable
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "party-directory" || value.ID != partyID || value.TenantID != tenantID || value.Status != "active" || value.Revision != revision || (value.Kind != "organization" && value.Kind != "individual") || strings.TrimSpace(value.LegalName) == "" {
		return TargetSnapshot{}, ErrTargetInvalid
	}
	return TargetSnapshot{PartyID: value.ID, Revision: value.Revision, Kind: value.Kind, LegalName: value.LegalName, RegistrationIdentifier: value.RegistrationIdentifier}, nil
}
