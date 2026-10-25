package subjectreference

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

	"omsbizfoundry/people-groups/internal/peoplegroups"
)

const maxResponseBytes = 64 << 10

type CredentialSource interface {
	CredentialForTenant(context.Context, string, string, string, string, string) (string, error)
	InvalidateForTenant(string, string, string, string, string)
}

type HTTPValidator struct {
	client      *http.Client
	baseURL     string
	credentials CredentialSource
}

func NewHTTPValidator(baseURL string, credentials CredentialSource, timeout time.Duration) (*HTTPValidator, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" || credentials == nil || timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("IAM subject reference configuration is invalid")
	}
	return &HTTPValidator{client: &http.Client{Timeout: timeout}, baseURL: strings.TrimRight(baseURL, "/"), credentials: credentials}, nil
}

func (v *HTTPValidator) ValidateIAMSubject(ctx context.Context, tenantID, subjectID string) (peoplegroups.IAMSubjectReference, error) {
	token, err := v.credentials.CredentialForTenant(ctx, tenantID, "identity-access", "identity-access.subject.reference.read", "iam_subject", subjectID)
	if err != nil || strings.TrimSpace(token) == "" {
		return peoplegroups.IAMSubjectReference{}, peoplegroups.ErrSubjectReferenceUnavailable
	}
	return v.get(ctx, tenantID, subjectID, token, true)
}

func (v *HTTPValidator) get(ctx context.Context, tenantID, subjectID, token string, retry bool) (peoplegroups.IAMSubjectReference, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/api/v1/references/subjects/"+url.PathEscape(subjectID), nil)
	if err != nil {
		return peoplegroups.IAMSubjectReference{}, peoplegroups.ErrSubjectReferenceUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := v.client.Do(request)
	if err != nil {
		return peoplegroups.IAMSubjectReference{}, peoplegroups.ErrSubjectReferenceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && retry {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		v.credentials.InvalidateForTenant(tenantID, "identity-access", "identity-access.subject.reference.read", "iam_subject", subjectID)
		fresh, credentialErr := v.credentials.CredentialForTenant(ctx, tenantID, "identity-access", "identity-access.subject.reference.read", "iam_subject", subjectID)
		if credentialErr != nil {
			return peoplegroups.IAMSubjectReference{}, peoplegroups.ErrSubjectReferenceUnavailable
		}
		return v.get(ctx, tenantID, subjectID, fresh, false)
	}
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUnprocessableEntity {
		return peoplegroups.IAMSubjectReference{}, peoplegroups.ErrSubjectReferenceInvalid
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return peoplegroups.IAMSubjectReference{}, peoplegroups.ErrSubjectReferenceUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return peoplegroups.IAMSubjectReference{}, peoplegroups.ErrSubjectReferenceUnavailable
	}
	var value peoplegroups.IAMSubjectReference
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if err := decoder.Decode(&value); err != nil {
		return peoplegroups.IAMSubjectReference{}, peoplegroups.ErrSubjectReferenceUnavailable
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return peoplegroups.IAMSubjectReference{}, peoplegroups.ErrSubjectReferenceUnavailable
	}
	if value.SchemaVersion != "1" || value.SourceApplication != "identity-access" || value.ID != subjectID || value.TenantID != tenantID || value.Revision < 1 || (value.Status != "pending" && value.Status != "active" && value.Status != "disabled") {
		return peoplegroups.IAMSubjectReference{}, peoplegroups.ErrSubjectReferenceInvalid
	}
	return value, nil
}
