package workloadidentity

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ResourceRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type Scope struct {
	Kind      string        `json:"kind"`
	Resources []ResourceRef `json:"resources"`
}

type exchangeTarget struct {
	Audience string `json:"audience"`
	Action   string `json:"action"`
	Scope    Scope  `json:"scope"`
}

type exchangeRequest struct {
	Assertion string `json:"assertion"`
	exchangeTarget
}

type cachedCredential struct {
	token     string
	expiresAt time.Time
}

// CredentialSource is the compatibility surface used by existing service
// adapters. Tenant-aware sources additionally implement
// TenantCredentialSource; static test doubles may remain tenant-neutral.
type CredentialSource interface {
	Credential(context.Context, string, string, string, string) (string, error)
	Invalidate(string, string, string, string)
}

type TenantCredentialSource interface {
	CredentialForTenant(context.Context, string, string, string, string, string) (string, error)
	InvalidateForTenant(string, string, string, string, string)
}

func CredentialForTenant(ctx context.Context, source CredentialSource, tenantID, audience, action, resourceType, resourceID string) (string, error) {
	if tenantSource, ok := source.(TenantCredentialSource); ok {
		return tenantSource.CredentialForTenant(ctx, tenantID, audience, action, resourceType, resourceID)
	}
	return source.Credential(ctx, audience, action, resourceType, resourceID)
}

func InvalidateForTenant(source CredentialSource, tenantID, audience, action, resourceType, resourceID string) {
	if tenantSource, ok := source.(TenantCredentialSource); ok {
		tenantSource.InvalidateForTenant(tenantID, audience, action, resourceType, resourceID)
		return
	}
	source.Invalidate(audience, action, resourceType, resourceID)
}

type TokenSource struct {
	client        *http.Client
	endpoint      string
	tenantID      string
	store         *KeyStore
	refreshBefore time.Duration
	now           func() time.Time
	mu            sync.Mutex
	cache         map[string]cachedCredential
}

func NewTokenSource(endpoint, tenantID, workloadID, subjectID, keyFile string, timeout, refreshBefore time.Duration) (*TokenSource, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("IAM service credential endpoint is invalid")
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if !uuidPattern.MatchString(tenantID) {
		return nil, fmt.Errorf("workload tenant configuration is invalid")
	}
	if timeout <= 0 || timeout > 30*time.Second || refreshBefore <= 0 || refreshBefore > time.Minute {
		return nil, fmt.Errorf("service credential timing configuration is invalid")
	}
	store, err := OpenKeyStore(keyFile, workloadID, subjectID)
	if err != nil {
		return nil, err
	}
	return &TokenSource{client: &http.Client{Timeout: timeout}, endpoint: strings.TrimRight(endpoint, "/"), tenantID: tenantID, store: store, refreshBefore: refreshBefore, now: func() time.Time { return time.Now().UTC() }, cache: map[string]cachedCredential{}}, nil
}

func (s *TokenSource) Credential(ctx context.Context, audience, action, resourceType, resourceID string) (string, error) {
	return s.CredentialForTenant(ctx, s.tenantID, audience, action, resourceType, resourceID)
}

// CredentialForTenant exchanges a request-bound credential for the trusted
// tenant carried by the current caller. It lets one service process serve
// multiple tenants without using its bootstrap tenant as an authorization
// boundary.
func (s *TokenSource) CredentialForTenant(ctx context.Context, tenantID, audience, action, resourceType, resourceID string) (string, error) {
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if !uuidPattern.MatchString(tenantID) {
		return "", fmt.Errorf("workload tenant is invalid")
	}
	target := exchangeTarget{Audience: audience, Action: action, Scope: Scope{Kind: "resources", Resources: []ResourceRef{{Type: resourceType, ID: resourceID}}}}
	key := tenantID + "\x00" + audience + "\x00" + action + "\x00" + resourceType + "\x00" + resourceID
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if cached, ok := s.cache[key]; ok && now.Add(s.refreshBefore).Before(cached.expiresAt) {
		return cached.token, nil
	}
	targetJSON, err := json.Marshal(target)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(targetJSON)
	assertion, _, err := s.store.SignAssertion(tenantID, hex.EncodeToString(hash[:]), now)
	if err != nil {
		return "", fmt.Errorf("sign workload assertion: %w", err)
	}
	payload, err := json.Marshal(exchangeRequest{Assertion: assertion, exchangeTarget: target})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("exchange workload assertion: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return "", fmt.Errorf("service credential endpoint returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	if err := decoder.Decode(&result); err != nil || strings.TrimSpace(result.Token) == "" || !result.ExpiresAt.After(now) || result.ExpiresAt.After(now.Add(5*time.Minute+30*time.Second)) {
		return "", fmt.Errorf("service credential response is invalid")
	}
	s.cache[key] = cachedCredential{token: result.Token, expiresAt: result.ExpiresAt}
	return result.Token, nil
}

func (s *TokenSource) Invalidate(audience, action, resourceType, resourceID string) {
	s.InvalidateForTenant(s.tenantID, audience, action, resourceType, resourceID)
}

func (s *TokenSource) InvalidateForTenant(tenantID, audience, action, resourceType, resourceID string) {
	key := strings.ToLower(strings.TrimSpace(tenantID)) + "\x00" + audience + "\x00" + action + "\x00" + resourceType + "\x00" + resourceID
	s.mu.Lock()
	delete(s.cache, key)
	s.mu.Unlock()
}
