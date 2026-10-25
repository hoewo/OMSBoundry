package referenceoa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	workloadidentity "omsbizfoundry/workload-identity"
)

type CredentialSource interface {
	Credential(context.Context, string, string, string, string) (string, error)
	Invalidate(string, string, string, string)
}
type HTTPFileValidator struct {
	client      *http.Client
	base, token string
	credentials CredentialSource
}

func NewHTTPFileValidator(base, token string, credentials CredentialSource, timeout time.Duration) (*HTTPFileValidator, error) {
	u, e := url.Parse(base)
	if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || timeout <= 0 || timeout > 30*time.Second || (strings.TrimSpace(token) != "") == (credentials != nil) {
		return nil, fmt.Errorf("invalid external Evidence configuration")
	}
	return &HTTPFileValidator{client: &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, base: strings.TrimRight(base, "/"), token: token, credentials: credentials}, nil
}
func (v *HTTPFileValidator) ValidateFile(ctx context.Context, tenant, id string, version int) (FileReference, error) {
	if !uuid.MatchString(tenant) || !uuid.MatchString(id) || version < 1 {
		return FileReference{}, ErrInvalid
	}
	for attempt := 0; attempt < 2; attempt++ {
		token := v.token
		if v.credentials != nil {
			var e error
			token, e = workloadidentity.CredentialForTenant(ctx, v.credentials, tenant, "evidence-service", "evidence-service.metadata.read", "file", id)
			if e != nil || strings.TrimSpace(token) == "" {
				return FileReference{}, ErrEvidenceUnavailable
			}
		}
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/references/files/%s/versions/%d", v.base, url.PathEscape(id), version), nil)
		if e != nil {
			return FileReference{}, ErrEvidenceUnavailable
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		response, e := v.client.Do(req)
		if e != nil {
			return FileReference{}, ErrEvidenceUnavailable
		}
		body, e := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
		response.Body.Close()
		if e != nil || len(body) > 64<<10 {
			return FileReference{}, ErrEvidenceUnavailable
		}
		if response.StatusCode == 401 && v.credentials != nil && attempt == 0 {
			workloadidentity.InvalidateForTenant(v.credentials, tenant, "evidence-service", "evidence-service.metadata.read", "file", id)
			continue
		}
		if response.StatusCode == 403 || response.StatusCode == 404 || response.StatusCode == 422 {
			return FileReference{}, ErrScope
		}
		if response.StatusCode != 200 {
			return FileReference{}, ErrEvidenceUnavailable
		}
		var ref FileReference
		if e = json.Unmarshal(body, &ref); e != nil {
			return ref, ErrEvidenceUnavailable
		}
		if e = validateFile(ref, tenant, id, version); e != nil {
			return ref, e
		}
		return ref, nil
	}
	return FileReference{}, ErrEvidenceUnavailable
}
