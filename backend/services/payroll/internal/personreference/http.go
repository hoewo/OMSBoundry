package personreference

import (
	"context"
	"crypto/sha256"
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

const maxResponseBytes = 64 << 10

var referenceUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type CredentialSource interface {
	CredentialForTenant(context.Context, string, string, string, string, string) (string, error)
	InvalidateForTenant(string, string, string, string, string)
}

type HTTPValidator struct {
	client               *http.Client
	baseURL, staticToken string
	credentials          CredentialSource
}

func NewHTTPValidator(baseURL, staticToken string, credentials CredentialSource, timeout time.Duration) (*HTTPValidator, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	hasStatic := strings.TrimSpace(staticToken) != ""
	hasDynamic := credentials != nil
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || hasStatic == hasDynamic || timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("People reference configuration is invalid")
	}
	return &HTTPValidator{client: &http.Client{Timeout: timeout}, baseURL: baseURL, staticToken: staticToken, credentials: credentials}, nil
}

func (v *HTTPValidator) ValidatePerson(ctx context.Context, tenantID, personID string) (payroll.PersonReference, error) {
	token := v.staticToken
	var err error
	if v.credentials != nil {
		token, err = v.credentials.CredentialForTenant(ctx, tenantID, "people-groups", "people-groups.person.reference.read", "person", personID)
		if err != nil || strings.TrimSpace(token) == "" {
			return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
		}
	}
	return v.get(ctx, tenantID, personID, token, true)
}

func (v *HTTPValidator) ResolvePersonForSubject(ctx context.Context, tenantID, subjectID string) (payroll.PersonReference, error) {
	token := v.staticToken
	var err error
	if v.credentials != nil {
		token, err = v.credentials.CredentialForTenant(ctx, tenantID, "people-groups", "people-groups.person.reference.read", "iam_subject", subjectID)
		if err != nil || strings.TrimSpace(token) == "" {
			return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
		}
	}
	return v.getSubject(ctx, tenantID, subjectID, token, true)
}

func (v *HTTPValidator) getSubject(ctx context.Context, tenantID, subjectID, token string, retry bool) (payroll.PersonReference, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/api/v1/references/iam-subjects/"+url.PathEscape(subjectID)+"/person", nil)
	if err != nil {
		return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, err := v.client.Do(req)
	if err != nil {
		return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized && retry && v.credentials != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxResponseBytes))
		v.credentials.InvalidateForTenant(tenantID, "people-groups", "people-groups.person.reference.read", "iam_subject", subjectID)
		fresh, credentialErr := v.credentials.CredentialForTenant(ctx, tenantID, "people-groups", "people-groups.person.reference.read", "iam_subject", subjectID)
		if credentialErr != nil {
			return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
		}
		return v.getSubject(ctx, tenantID, subjectID, fresh, false)
	}
	return decodeReferenceResponse(res, tenantID, "")
}
func (v *HTTPValidator) get(ctx context.Context, tenantID, personID, token string, retry bool) (payroll.PersonReference, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/api/v1/references/persons/"+url.PathEscape(personID), nil)
	if err != nil {
		return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, err := v.client.Do(req)
	if err != nil {
		return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized && retry && v.credentials != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxResponseBytes))
		v.credentials.InvalidateForTenant(tenantID, "people-groups", "people-groups.person.reference.read", "person", personID)
		fresh, e := v.credentials.CredentialForTenant(ctx, tenantID, "people-groups", "people-groups.person.reference.read", "person", personID)
		if e != nil {
			return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
		}
		return v.get(ctx, tenantID, personID, fresh, false)
	}
	return decodeReferenceResponse(res, tenantID, personID)
}

func decodeReferenceResponse(res *http.Response, tenantID, expectedPersonID string) (payroll.PersonReference, error) {
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusUnprocessableEntity {
		return payroll.PersonReference{}, payroll.ErrReferenceInvalid
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
	}
	var wire struct {
		SchemaVersion     string `json:"schema_version"`
		SourceApplication string `json:"source_application"`
		ID                string `json:"id"`
		TenantID          string `json:"tenant_id"`
		ResourceType      string `json:"resource_type"`
		ReferenceCode     string `json:"reference_code"`
		DisplayName       string `json:"display_name"`
		Status            string `json:"status"`
		Revision          int64  `json:"revision"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if decoder.Decode(&wire) != nil {
		return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return payroll.PersonReference{}, payroll.ErrReferenceUnavailable
	}
	if wire.SchemaVersion != "1" || wire.SourceApplication != "people-groups" || wire.ResourceType != "person" || (expectedPersonID != "" && wire.ID != expectedPersonID) || wire.TenantID != tenantID || !referenceUUIDPattern.MatchString(wire.ID) || wire.Revision < 1 || (wire.Status != "active" && wire.Status != "inactive") || strings.TrimSpace(wire.DisplayName) == "" {
		return payroll.PersonReference{}, payroll.ErrReferenceInvalid
	}
	digest := sha256.Sum256(body)
	return payroll.PersonReference{ID: wire.ID, TenantID: wire.TenantID, ReferenceCode: strings.TrimSpace(wire.ReferenceCode), DisplayName: wire.DisplayName, Status: wire.Status, Revision: wire.Revision, Digest: fmt.Sprintf("sha256:%x", digest)}, nil
}

var _ payroll.PersonReferencePort = (*HTTPValidator)(nil)
