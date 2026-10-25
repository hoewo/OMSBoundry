package personreference

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"omsbizfoundry/identity-access/internal/apikey"
	"omsbizfoundry/identity-access/internal/identity"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Resolver struct {
	baseURL        string
	serviceSubject string
	issuer         string
	signer         identity.Signer
	client         *http.Client
	now            func() time.Time
}

func New(baseURL, serviceSubject, issuer string, signer identity.Signer, timeout time.Duration) (*Resolver, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("People reference base URL is invalid")
	}
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if !uuidPattern.MatchString(serviceSubject) || issuer == "" || signer == nil || timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("People reference identity configuration is invalid")
	}
	return &Resolver{baseURL: baseURL, serviceSubject: serviceSubject, issuer: issuer, signer: signer, client: &http.Client{Timeout: timeout}, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (r *Resolver) ResolveBySubject(ctx context.Context, tenantID, subjectID string) (apikey.PersonReference, error) {
	if !uuidPattern.MatchString(tenantID) || !uuidPattern.MatchString(subjectID) {
		return apikey.PersonReference{}, fmt.Errorf("Tenant or Subject reference is invalid")
	}
	now := r.now()
	jti, err := randomUUID()
	if err != nil {
		return apikey.PersonReference{}, err
	}
	token, err := r.signer.Sign(identity.CapabilityClaims{
		Version: "1", Issuer: r.issuer, Subject: r.serviceSubject, Tenant: tenantID,
		Audience: "people-groups", Name: "IAM API Key Person Resolver",
		Actions:  []string{"people-groups.person.reference.read"},
		Scope:    identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "iam_subject", ID: subjectID}}},
		IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Unix(), ID: jti,
	})
	if err != nil {
		return apikey.PersonReference{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/v1/references/iam-subjects/"+url.PathEscape(subjectID)+"/person", nil)
	if err != nil {
		return apikey.PersonReference{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := r.client.Do(request)
	if err != nil {
		return apikey.PersonReference{}, fmt.Errorf("read Person reference: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return apikey.PersonReference{}, fmt.Errorf("Person reference returned HTTP %d", response.StatusCode)
	}
	var value struct {
		SchemaVersion     string `json:"schema_version"`
		SourceApplication string `json:"source_application"`
		ResourceType      string `json:"resource_type"`
		ID                string `json:"id"`
		TenantID          string `json:"tenant_id"`
		Status            string `json:"status"`
		Revision          int64  `json:"revision"`
		ReferenceCode     string `json:"reference_code"`
		DisplayName       string `json:"display_name"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return apikey.PersonReference{}, fmt.Errorf("decode Person reference: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return apikey.PersonReference{}, fmt.Errorf("Person reference has trailing data")
	}
	value.ReferenceCode = strings.TrimSpace(value.ReferenceCode)
	value.DisplayName = strings.TrimSpace(value.DisplayName)
	if value.SchemaVersion != "1" || value.SourceApplication != "people-groups" || value.ResourceType != "person" || !uuidPattern.MatchString(value.ID) || value.TenantID != tenantID || value.Revision < 1 || value.DisplayName == "" || len([]rune(value.DisplayName)) > 100 || (value.Status != "active" && value.Status != "inactive") {
		return apikey.PersonReference{}, fmt.Errorf("Person reference is inconsistent")
	}
	digestInput, _ := json.Marshal(struct {
		ID            string `json:"id"`
		Revision      int64  `json:"revision"`
		ReferenceCode string `json:"reference_code"`
		DisplayName   string `json:"display_name"`
		Status        string `json:"status"`
	}{value.ID, value.Revision, value.ReferenceCode, value.DisplayName, value.Status})
	digest := sha256.Sum256(digestInput)
	return apikey.PersonReference{ID: value.ID, TenantID: value.TenantID, Status: value.Status, Revision: value.Revision, ReferenceCode: value.ReferenceCode, DisplayName: value.DisplayName, Digest: fmt.Sprintf("sha256:%x", digest)}, nil
}

func randomUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
